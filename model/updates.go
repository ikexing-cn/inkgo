package model

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type syncError struct {
	kind string
	err  error
}

func (e *syncError) Error() string { return e.err.Error() }
func (e *syncError) Unwrap() error { return e.err }
func failureKind(err error) string {
	var e *syncError
	if errors.As(err, &e) {
		return e.kind
	}
	return "cache"
}

// ModelUpdateState is persisted beside the directory; Failure is a locale-independent reason.
type ModelUpdateState = refreshState

// ModelUpdates owns one scheduler and coalesces manual and automatic requests.
type modelFlight struct {
	done chan struct{}
	err  error
}

type ModelUpdates struct {
	state    ModelUpdateState
	changed  func()
	mu       sync.Mutex
	dir, url string
	client   *http.Client
	enabled  bool
	cancel   context.CancelFunc
	running  *modelFlight

	wake chan struct{}
	stop chan struct{}
}

func NewModelUpdates(configPath, url string, client *http.Client) *ModelUpdates {
	if url == "" {
		url = ModelsURL
	}
	if client == nil {
		client = modelsClient()
	}
	u := &ModelUpdates{dir: filepath.Dir(configPath), url: url, client: client, wake: make(chan struct{}, 1), stop: make(chan struct{})}
	u.state = u.readStatus()
	go u.loop()
	return u
}
func (u *ModelUpdates) Status() ModelUpdateState { u.mu.Lock(); defer u.mu.Unlock(); return u.state }
func (u *ModelUpdates) OnChange(f func())        { u.mu.Lock(); u.changed = f; u.mu.Unlock() }
func (u *ModelUpdates) readStatus() ModelUpdateState {
	var s ModelUpdateState
	b, _ := os.ReadFile(filepath.Join(u.dir, "models-meta.json"))
	_ = json.Unmarshal(b, &s)
	return s
}
func nextModelCheck(checked, now time.Time) time.Duration {
	if checked.IsZero() {
		return 0
	}
	return max(0, checked.Add(24*time.Hour).Sub(now))
}
func (u *ModelUpdates) SetEnabled(on bool) {
	u.mu.Lock()
	u.enabled = on
	if !on && u.cancel != nil {
		u.cancel()
	}
	u.mu.Unlock()
	select {
	case u.wake <- struct{}{}:
	default:
	}
}
func (u *ModelUpdates) Close()                         { u.SetEnabled(false); close(u.stop) }
func (u *ModelUpdates) Sync(ctx context.Context) error { return u.sync(ctx, true) }
func (u *ModelUpdates) sync(ctx context.Context, manual bool) error {
	u.mu.Lock()
	if flight := u.running; flight != nil {
		u.mu.Unlock()
		select {
		case <-flight.done:
			return flight.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if !manual && !u.enabled {
		u.mu.Unlock()
		return nil
	}
	flight := &modelFlight{done: make(chan struct{})}
	u.running = flight
	if !manual {
		ctx, u.cancel = context.WithCancel(ctx)
	}
	u.mu.Unlock()
	now := time.Now()
	err := refreshModelsForce(ctx, u.dir, u.url, u.client, now, manual)
	u.mu.Lock()
	u.state = u.readStatus()
	if err != nil && ctx.Err() == nil {
		u.state.Failure = failureKind(err)
		u.state.Checked = now
	}
	changed := u.changed
	flight.err = err
	u.running = nil
	u.cancel = nil
	close(flight.done)
	u.mu.Unlock()
	if changed != nil {
		changed()
	}
	return err
}
func (u *ModelUpdates) loop() {
	for {
		u.mu.Lock()
		on := u.enabled
		u.mu.Unlock()
		delay := 24 * time.Hour
		if on {
			delay = nextModelCheck(u.Status().Checked, time.Now())
		}
		timer := time.NewTimer(delay)
		select {
		case <-u.stop:
			timer.Stop()
			return
		case <-u.wake:
			timer.Stop()
		case <-timer.C:
			if on {
				_ = u.sync(context.Background(), false)
			}
		}
	}
}
