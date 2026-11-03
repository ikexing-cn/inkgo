package ext_test

import (
	"errors"
	"testing"

	"writer/ext"
)

func TestGuardPanicStopsOwnerAndRequiredDependentsWithoutAChain(t *testing.T) {
	cause := errors.New("query failed")
	var notified []ext.Failure
	h := host(t, ext.Options{OnFailure: func(id string, f ext.Failure) {
		if id != "source" {
			t.Errorf("unexpected failure owner %q", id)
		}
		notified = append(notified, f)
	}}, ext.Module{ID: "source"}, ext.Module{ID: "consumer", Requires: []string{"source"}}, ext.Module{ID: "healthy"})
	err := ext.Guard(h, "source", "source.query", func() { panic(cause) })
	if !errors.Is(err, ext.ErrCallFailed) || !errors.Is(err, cause) {
		t.Fatal("failure identity lost", err)
	}
	state(t, h, "source", ext.Failed)
	state(t, h, "consumer", ext.Off)
	state(t, h, "healthy", ext.On)
	failures := h.Ledger("source").Errors
	if len(failures) != 1 || len(notified) != 1 {
		t.Fatal(failures, notified)
	}
	f := failures[0]
	if f.Stage != "Guard" || f.Target != "source.query" || f.Chain != "" || f.Injection != "" || !errors.Is(f.Err, cause) {
		t.Fatal(f)
	}
	if len(h.Chain("source.query")) != 0 {
		t.Fatal("plain callback entered injection chains")
	}
	called := false
	if err := ext.Guard(h, "source", "source.query", func() { called = true }); !errors.Is(err, ext.ErrCallFailed) || called {
		t.Fatal("failed owner called", err)
	}
	if len(notified) != 1 {
		t.Fatal("inactive callback reported a second failure")
	}
}

func TestGuardSkipsInactiveOrUnknownOwnersWithAnIdentifiableError(t *testing.T) {
	h := host(t, ext.Options{}, ext.Module{ID: "source"}, ext.Module{ID: "consumer", Requires: []string{"source"}})
	must(t, h.Set("source", false))
	for _, id := range []string{"source", "consumer", "missing"} {
		called := false
		err := ext.Guard(h, id, id+".query", func() { called = true })
		if !errors.Is(err, ext.ErrCallFailed) || called {
			t.Fatal(id, called, err)
		}
	}
}

func TestGuardHostOwnedPanicEscapesUnchanged(t *testing.T) {
	h := host(t, ext.Options{})
	cause := errors.New("host fault")
	defer func() {
		if got := recover(); got != cause {
			t.Errorf("host panic = %v, want %v", got, cause)
		}
	}()
	_ = ext.Guard(h, ext.HostOwner, "host.query", func() { panic(cause) })
	t.Fatal("host panic was swallowed")
}

func TestGuardQueriesAreReentrantAndDoNotDeclareHooks(t *testing.T) {
	h := host(t, ext.Options{}, ext.Module{ID: "source"})
	count := 0
	must(t, ext.Guard(h, "source", "source.outer", func() {
		must(t, ext.Guard(h, "source", "source.inner", func() { count++ }))
	}))
	if count != 1 || len(h.Chain("source.outer")) != 0 || len(h.Chain("source.inner")) != 0 {
		t.Fatal("query changed hook declarations")
	}
}
