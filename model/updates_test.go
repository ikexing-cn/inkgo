package model

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNextModelCheck(t *testing.T) {
	now := time.Now()
	if got := nextModelCheck(now.Add(-23*time.Hour), now); got != time.Hour {
		t.Fatal(got)
	}
	if nextModelCheck(time.Time{}, now) != 0 {
		t.Fatal("first check delayed")
	}
}
func TestModelUpdatesManualAndStatus(t *testing.T) {
	var calls atomic.Int32
	fail := true
	client := &http.Client{Transport: handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"openai":{"models":{"test":{"limit":{"context":9000,"output":100}}}}}`))
	})}}
	u := NewModelUpdates(t.TempDir()+"/providers.yaml", "https://example.com", client)
	defer u.Close()
	if err := u.sync(context.Background(), false); err != nil || calls.Load() != 0 {
		t.Fatal(err, calls.Load())
	}
	if err := u.Sync(context.Background()); err == nil || u.Status().Failure != "http" {
		t.Fatal(err, u.Status())
	}
	if err := u.sync(context.Background(), false); err != nil || calls.Load() != 1 {
		t.Fatal(err, calls.Load())
	}
	fail = false
	if err := u.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || u.Status().Failure != "" || u.Status().Success.IsZero() {
		t.Fatal(u.Status(), calls.Load())
	}
}
func TestModelUpdatesCoalesce(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	client := &http.Client{Transport: handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(503)
	})}}
	u := NewModelUpdates(t.TempDir()+"/providers.yaml", "https://example.com", client)
	defer u.Close()
	first := make(chan error, 1)
	u.mu.Lock()
	u.enabled = true
	u.mu.Unlock()
	go func() { first <- u.sync(context.Background(), false) }()
	<-started
	second := make(chan error, 1)
	go func() { second <- u.Sync(context.Background()) }()
	// A second caller must wait rather than start another transport request.
	select {
	case <-second:
		t.Fatal("did not join pending request")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if <-first == nil || <-second == nil {
		t.Fatal("missing shared failure")
	}
}
func TestModelUpdatesResumeAndPersist(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	b, _ := json.Marshal(refreshState{Checked: now, Success: now.Add(-time.Hour), Failure: "connect"})
	os.WriteFile(filepath.Join(dir, "models-meta.json"), b, 0600)
	var calls atomic.Int32
	u := NewModelUpdates(filepath.Join(dir, "providers.yaml"), "https://example.com", &http.Client{Transport: handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) })}})
	defer u.Close()
	if u.Status().Failure != "connect" || u.Status().Success.IsZero() {
		t.Fatal(u.Status())
	}
	u.SetEnabled(true)
	if err := u.sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	u.SetEnabled(false)
	if calls.Load() != 0 {
		t.Fatal("requested within 24 hours")
	}
}
