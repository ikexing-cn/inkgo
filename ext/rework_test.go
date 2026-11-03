package ext_test

import (
	"testing"
	"time"

	"writer/ext"
)

type callResult struct {
	value int
	err   error
}

func awaitCall(t *testing.T, done <-chan callResult, want int) {
	t.Helper()
	select {
	case got := <-done:
		if got.err != nil || got.value != want {
			t.Fatalf("Call = (%d, %v), want (%d, nil)", got.value, got.err, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Call did not finish before timeout")
	}
}

func TestCallSameHookIsReentrantFromAroundAndOriginal(t *testing.T) {
	for _, from := range []string{"around", "original"} {
		t.Run(from, func(t *testing.T) {
			var h *ext.Host
			h = host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
				ext.Around(r, hostHook, "wrap", func(n int, next func(int) int) int {
					if from == "original" {
						return next(n) + 2
					}
					if n == 0 {
						return next(n)
					}
					value, err := ext.Call(h, hostHook, 0, func(int) int { return 10 })
					if err != nil {
						panic(err)
					}
					return next(n) + value
				})
			}})
			done := make(chan callResult, 1)
			go func() {
				value, err := ext.Call(h, hostHook, 1, func(n int) int {
					if from == "around" {
						return n
					}
					value, err := ext.Call(h, hostHook, 0, func(int) int { return 10 })
					if err != nil {
						panic(err)
					}
					return n + value
				})
				done <- callResult{value, err}
			}()
			want := 11
			if from == "original" {
				want = 15
			}
			awaitCall(t, done, want)
		})
	}
}

func TestConcurrentCallSameInjectedHookDoesNotWaitForBlockedOriginal(t *testing.T) {
	h := host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Around(r, hostHook, "wrap", func(n int, next func(int) int) int { return next(n) + 3 })
		ext.After(r, hostHook, "after", func(_ int, out *int) { *out++ })
	}})
	started, release := make(chan struct{}), make(chan struct{})
	// Release the blocked worker even if a timeout assertion fails.
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	first, second := make(chan callResult, 1), make(chan callResult, 1)
	go func() {
		v, err := ext.Call(h, hostHook, 1, func(int) int { close(started); <-release; return 100 })
		first <- callResult{v, err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first original did not start")
	}
	go func() { v, err := ext.Call(h, hostHook, 2, func(int) int { return 200 }); second <- callResult{v, err} }()
	awaitCall(t, second, 204)
	close(release)
	released = true
	awaitCall(t, first, 104)
}

func TestInjectionAndChainReasonsAreStructured(t *testing.T) {
	h := host(t, hookOptions(), overwriter("a", 10), overwriter("b", 20))
	assertReason := func(id string, want ext.Reason) {
		t.Helper()
		if got := h.Ledger(id).Injections[0].Reason; got != want {
			t.Fatalf("%s ledger reason = %+v, want %+v", id, got, want)
		}
		found := false
		for _, link := range h.Chain(string(hostHook)) {
			if link.ID == id+".replace" {
				found = true
				if link.Reason != want {
					t.Fatalf("%s chain reason = %+v, want %+v", id, link.Reason, want)
				}
			}
		}
		if want.Kind != ext.ReasonInjectorOff && !found {
			t.Fatalf("missing chain link for %s", id)
		}
	}
	assertReason("a", ext.Reason{Kind: ext.ReasonConflict})
	assertReason("b", ext.Reason{Kind: ext.ReasonConflict})
	must(t, h.Choose(string(hostHook), "a.replace"))
	assertReason("a", ext.Reason{Kind: ext.ReasonEnabled})
	assertReason("b", ext.Reason{Kind: ext.ReasonNotSelected, ID: "a.replace"})
	for _, link := range h.Chain(string(hostHook)) {
		if link.Original && link.Reason != (ext.Reason{Kind: ext.ReasonReplaced, ID: "a.replace"}) {
			t.Fatal(link)
		}
	}
	must(t, h.Set("a", false))
	assertReason("a", ext.Reason{Kind: ext.ReasonInjectorOff, ID: "a"})
	assertReason("b", ext.Reason{Kind: ext.ReasonEnabled})
	must(t, h.Set("b", false))
	for _, link := range h.Chain(string(hostHook)) {
		if link.Original && link.Reason != (ext.Reason{Kind: ext.ReasonEnabled}) {
			t.Fatal(link)
		}
	}
}

func TestInactiveTargetReasonIdentifiesOwnerInLedgerAndChain(t *testing.T) {
	h := host(t, ext.Options{}, baseModule(), ext.Module{ID: "a", Optional: []string{"base"}, Setup: func(r *ext.Registrar) { ext.Before(r, ownedHook, "before", func(*int) {}) }})
	must(t, h.Set("base", false))
	want := ext.Reason{Kind: ext.ReasonTargetOff, ID: "base"}
	if i := h.Ledger("a").Injections[0]; i.Reason != want {
		t.Fatal(i)
	}
	for _, link := range h.Chain(string(ownedHook)) {
		if link.Active || link.Reason != want {
			t.Fatal(link)
		}
	}
	must(t, h.Set("base", true))
	for _, link := range h.Chain(string(ownedHook)) {
		if !link.Active || link.Reason != (ext.Reason{Kind: ext.ReasonEnabled}) {
			t.Fatal(link)
		}
	}
}

func TestAroundPanicAfterNextKeepsResultAndRunsOriginalOnce(t *testing.T) {
	h := host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Around(r, hostHook, "wrap", func(n int, next func(int) int) int {
			next(n)
			panic("after next")
		})
	}})
	runs := 0
	got, err := ext.Call(h, hostHook, 1, func(n int) int { runs++; return n + 1 })
	if err != nil || got != 2 || runs != 1 {
		t.Fatalf("Call = (%d, %v) after %d runs, want (2, nil) after 1", got, err, runs)
	}
}
