package check

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

type checkFunc func(context.Context, string) (workspace.Check, error)

func (c checkFunc) Check(ctx context.Context, chapter string) (workspace.Check, error) {
	return c(ctx, chapter)
}

func TestCheckRunnerKeepsItsMailboxAndCancellation(t *testing.T) {
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	old := make(chan checkDone, 1)
	c := checkRunner{Done: old}
	store := new(workspace.Store)
	c.start(checkFunc(func(ctx context.Context, chapter string) (workspace.Check, error) {
		started <- ctx
		<-release
		return workspace.Check{Of: chapter}, nil
	}), store, "chapter", ui.Services{})
	var ctx context.Context
	select {
	case ctx = <-started:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("check did not start")
	}
	c.Done = make(chan checkDone, 1)
	c.stop()
	close(release)
	select {
	case got := <-old:
		if got.Chapter != "chapter" || got.Store != store || got.Check.Of != "chapter" || !errors.Is(got.Err, context.Canceled) || ctx.Err() != context.Canceled {
			t.Fatal(got, ctx.Err())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("check did not return to its original mailbox")
	}
	select {
	case got := <-c.Done:
		t.Fatal("a running check used a replacement mailbox", got)
	default:
	}
}

func TestCheckRunnerReturnsItsResult(t *testing.T) {
	wantErr := errors.New("model failed")
	for _, err := range []error{nil, wantErr} {
		var c checkRunner
		contexts := make(chan context.Context, 1)
		c.start(checkFunc(func(ctx context.Context, chapter string) (workspace.Check, error) {
			contexts <- ctx
			return workspace.Check{Of: chapter}, err
		}), nil, "chapter", ui.Services{})
		select {
		case got := <-c.Done:
			if got.Check.Of != "chapter" || got.Err != err || (<-contexts).Err() != context.Canceled {
				t.Fatal(got)
			}
		case <-time.After(3 * time.Second):
			c.stop()
			t.Fatal("check did not return")
		}
	}
}

func TestCheckRunnerFinishedDoesNotBlock(t *testing.T) {
	var c checkRunner
	c.stop()
	if got, ok := c.finished(); ok || got.Chapter != "" || got.Store != nil || got.Err != nil {
		t.Fatal("an idle runner returned a result", got, ok)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Cancel = cancel
	c.Done = make(chan checkDone, 1)
	c.Done <- checkDone{Chapter: "old-work"}
	if got, ok := c.finished(); !ok || got.Chapter != "old-work" || c.Cancel != nil || ctx.Err() != nil {
		t.Fatal("the runner did not hand back its result", got, ok)
	}
	if _, ok := c.finished(); ok {
		t.Fatal("a result was returned twice")
	}
}
