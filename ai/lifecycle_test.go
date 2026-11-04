package ai

import (
	"context"
	"errors"
	"testing"
	"time"
	"writer/model"
	"writer/workspace"
)

type waitingForStop struct{ ready chan struct{} }

func (w waitingForStop) Complete(ctx context.Context, _ model.Request, _ func(model.Delta)) (model.Reply, error) {
	close(w.ready)
	<-ctx.Done()
	return model.Reply{}, ctx.Err()
}

func TestClosingServiceCancelsAnInflightRequest(t *testing.T) {
	service := New("")
	st, err := workspace.Create(t.TempDir(), "test", "test", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ready := make(chan struct{})
	request := counted{Completer: waitingForStop{ready}, life: service.life, st: st}
	done := make(chan error, 1)
	go func() {
		_, err := request.Complete(context.Background(), model.Request{}, nil)
		done <- err
	}()
	<-ready
	service.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request survived shutdown")
	}
}
