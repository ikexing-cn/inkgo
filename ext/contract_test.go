package ext_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"writer/ext"
)

type contractStateStore struct{ saves int }

func (*contractStateStore) LoadState(ext.Book, string) map[string]json.RawMessage {
	return map[string]json.RawMessage{"saved": json.RawMessage(`true`)}
}
func (s *contractStateStore) SaveState(ext.Book, string, map[string]json.RawMessage) { s.saves++ }

func TestBookStateRequiresSuccessfulActivation(t *testing.T) {
	for _, stage := range []string{"Provide", "OnStart", "LoadState", "normal", "runtime"} {
		t.Run(stage, func(t *testing.T) {
			store := &contractStateStore{}
			var order []string
			var cleaned int
			h := host(t, ext.Options{BookState: store}, ext.Module{ID: "source", Setup: func(r *ext.Registrar) {
				ext.Provide(r, ext.ServiceKey[int]("source.service"), func(s *ext.Session) int {
					if s.Book().Dir != "" {
						order = append(order, "Provide")
						if stage == "Provide" {
							panic("factory")
						}
					}
					return 1
				})
				r.OnStart(func(s *ext.Session) error {
					s.OnStop(func() { cleaned++ })
					if s.Book().Dir != "" {
						order = append(order, "OnStart")
						if stage == "OnStart" {
							return errors.New("start")
						}
					}
					return nil
				})
				r.Persist(ext.BookState{Load: func(map[string]json.RawMessage) {
					if len(order) > 0 {
						order = append(order, "LoadState")
						if stage == "LoadState" {
							panic("load")
						}
					}
				}, Save: func() map[string]json.RawMessage { return nil }})
			}})
			must(t, h.Open(ext.Book{Dir: "book"}))
			if stage == "runtime" {
				_ = ext.Guard(h, "source", "source.query", func() { panic("runtime") })
				h.Settle()
			}
			h.Close()
			want := 0
			if stage == "normal" || stage == "runtime" {
				want = 1
				if !reflect.DeepEqual(order, []string{"Provide", "OnStart", "LoadState"}) {
					t.Fatal(order)
				}
			}
			if store.saves != want {
				t.Fatalf("saves = %d, want %d", store.saves, want)
			}
			wantCleaned := 2
			if stage == "Provide" {
				wantCleaned = 1
			}
			if cleaned != wantCleaned {
				t.Fatalf("cleanups = %d, want %d", cleaned, wantCleaned)
			}
		})
	}
}

func TestOldCallPanicDoesNotFailReopenedSession(t *testing.T) {
	for _, kind := range []string{"Guard", "plain", "ReadPoint", "Original", "Before", "After", "Around", "Overwrite", "Replace"} {
		t.Run(kind, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			boom := errors.New("old call")
			fail := func() { close(entered); <-release; panic(boom) }
			hook := ext.HookKey[int, int]("source.run")
			point := ext.PointKey[func(int) int]("source.items")
			restart := "source"
			var notices int
			var reader *ext.PointReader[func(int) int]
			source := ext.Module{ID: "source", Setup: func(r *ext.Registrar) {
				ext.DeclareHook(r, hook)
				ext.Declare(r, point, ext.Hidden)
				cp := ext.DeclareCalls(r, point, func(f func(int) int) func(int) int { return f })
				ext.Contribute(r, point, "item", func(n int) int { return n })
				if kind == "Replace" || kind == "ReadPoint" {
					hook = ext.HookOf(cp, "source.item")
				}
			}}
			addon := ext.Module{ID: "addon", Optional: []string{"source"}, Setup: func(r *ext.Registrar) {
				if kind == "ReadPoint" {
					reader = ext.ReadPoint(r, point)
				}
				switch kind {
				case "Original":
					ext.Before(r, hook, "noop", func(*int) {})
				case "Before":
					ext.Before(r, hook, "fail", func(*int) { fail() })
				case "After":
					ext.After(r, hook, "fail", func(int, *int) { fail() })
				case "Around":
					ext.Around(r, hook, "fail", func(int, func(int) int) int { fail(); return 0 })
				case "Overwrite":
					ext.Overwrite(r, hook, "fail", func(int) int { fail(); return 0 })
				case "Replace":
					ext.Replace(r, point, "source.item", "fail", func(int) int { fail(); return 0 })
				}
			}}
			if kind != "Guard" && kind != "plain" && kind != "Original" && kind != "ReadPoint" {
				restart = "addon"
			}
			h := host(t, ext.Options{OnFailure: func(string, ext.Failure) { notices++ }}, source, addon)
			done := make(chan error, 1)
			go func() {
				if kind == "Guard" {
					done <- ext.Guard(h, "source", "source.query", fail)
					return
				}
				if kind == "ReadPoint" {
					items, sessions := reader.Capture()
					if len(items) != 1 || !sessions["source"].Active() {
						done <- errors.New("invalid worker capture")
						return
					}
					_, err := ext.CallReadPoint(reader, items[0].ID, 1, func(int) int { fail(); return 0 })
					done <- err
					return
				}
				_, err := ext.Call(h, hook, 1, func(n int) int {
					if restart == "source" {
						fail()
					}
					return n
				})
				done <- err
			}()
			<-entered
			old := ext.ActivationOf(h, restart)
			must(t, h.Set(restart, false))
			must(t, h.Set(restart, true))
			current := ext.ActivationOf(h, restart)
			if old == current || !current.Active() {
				t.Fatal("session was not restarted")
			}
			close(release)
			err := <-done
			if restart == "source" && (!errors.Is(err, ext.ErrCallFailed) || !errors.Is(err, boom)) {
				t.Fatal(err)
			}
			state(t, h, restart, ext.On)
			if len(h.Ledger(restart).Errors) != 0 || notices != 0 {
				t.Fatal("stale failure recorded", h.Ledger(restart), notices)
			}
			h.Settle()
			if !current.Active() {
				t.Fatal("settled new session")
			}
		})
	}
}

func TestSettleJoinsTasksAndRunsAllCleanupWithoutSwitchChange(t *testing.T) {
	var calls []string
	var h *ext.Host
	exited := make(chan struct{})
	h = host(t, ext.Options{}, ext.Module{ID: "source", Setup: func(r *ext.Registrar) {
		r.OnStart(func(s *ext.Session) error {
			s.Go(func(ctx context.Context) { <-ctx.Done(); h.Ledger("source"); close(exited) })
			s.OnStop(func() {
				select {
				case <-exited:
				default:
					t.Error("cleanup before task exit")
				}
				calls = append(calls, "session")
			})
			return nil
		})
		r.OnStop(func(*ext.Session) { calls = append(calls, "source") })
	}}, ext.Module{ID: "consumer", Requires: []string{"source"}, Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { calls = append(calls, "consumer") }) }})
	_ = ext.Guard(h, "source", "source.query", func() { panic("failed") })
	if len(calls) != 0 {
		t.Fatal("cleanup ran inside call")
	}
	h.Settle()
	if !reflect.DeepEqual(calls, []string{"consumer", "session", "source"}) {
		t.Fatal(calls)
	}
	h.Settle()
	h.Close()
	if len(calls) != 3 {
		t.Fatal("cleanup repeated", calls)
	}
}

func TestTaskPanicIsSettledOnItsSession(t *testing.T) {
	start := make(chan struct{})
	failed := make(chan ext.Failure, 1)
	cleaned := false
	h := host(t, ext.Options{OnFailure: func(_ string, f ext.Failure) { failed <- f }}, ext.Module{ID: "source", Setup: func(r *ext.Registrar) {
		r.OnStart(func(s *ext.Session) error {
			s.Go(func(context.Context) { <-start; panic("task failed") })
			s.OnStop(func() { cleaned = true })
			return nil
		})
	}})
	s := ext.ActivationOf(h, "source")
	close(start)
	f := <-failed
	if f.Stage != "Task" || s.Active() {
		t.Fatal(f, s.Active())
	}
	h.Settle()
	if !cleaned {
		t.Fatal("task failure cleanup did not run")
	}
	state(t, h, "source", ext.Failed)
}
