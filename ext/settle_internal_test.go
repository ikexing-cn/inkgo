package ext

import (
	"testing"
	"time"
)

func TestIdleSettleDoesNotTakeLocks(t *testing.T) {
	h, err := New(Options{}, Module{ID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.opMu.Lock()
	h.mu.Lock()
	done := make(chan struct{})
	go func() { h.Settle(); close(done) }()
	// Release locks even on failure, so the test cannot strand the goroutine.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("idle Settle waited for a host lock")
	}
	h.mu.Unlock()
	h.opMu.Unlock()
	<-done
}
