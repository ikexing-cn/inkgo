package desk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/workspace"
)

func awaitJob(t *testing.T, a *App) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); !a.jobs.pending(); {
		if time.Now().After(end) {
			t.Fatal("worker did not return")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOldRequestCannotFinishNewWork(t *testing.T) {
	a := newApp("")
	oldReady, newReady := make(chan struct{}), make(chan struct{})
	defer close(newReady)
	var oldCtx context.Context
	oldApplied := false
	a.work(ui.Services{}, "old", func(ctx context.Context) (func(), error) {
		oldCtx = ctx
		<-oldReady
		return func() { oldApplied = true }, errors.New("old failure")
	})
	a.resetView()
	a.work(ui.Services{}, "new", func(ctx context.Context) (func(), error) {
		<-newReady
		return nil, nil
	})
	close(oldReady)
	awaitJob(t, a)
	a.poll()
	if oldApplied || a.busy != "new" || a.cancel == nil || a.errorText() != "" || oldCtx.Err() != context.Canceled {
		t.Fatal("old result affected the new request", oldApplied, a.busy, a.errorText())
	}
	a.cancel()
}

type capturedStream struct {
	aiport.Service
	progress func(aiport.Progress)
	step     func(workspace.Event)
}

func (s *capturedStream) Talk(_ *workspace.Store, _ aiport.Material, _ *aiport.Beside, progress func(aiport.Progress), step func(workspace.Event)) (aiport.Talk, error) {
	s.progress, s.step = progress, step
	return nil, nil
}

func TestOldStreamCannotEnterNewDiscussion(t *testing.T) {
	a, _, _ := fixture(t)
	stream := &capturedStream{Service: a.ai}
	a.ai = stream
	start := true
	stillTester(func(c *ui.Context) {
		if start {
			start = false
			if _, err := a.talk(c); err != nil {
				t.Fatal(err)
			}
		}
	}, 64, 64)
	old := a.turn
	a.resetView()
	a.SetError(errors.New("current error"))
	stream.progress(aiport.Progress{Text: "old reply"})
	stream.step(workspace.Event{Revision: "old revision"})
	a.poll()
	_, text, steps, _ := a.turn.live.read()
	if text != "" || len(steps) != 0 || a.errorText() != "current error" {
		t.Fatal("old stream changed the new discussion")
	}
	_, text, steps, _ = old.live.read()
	if text != "" || len(steps) != 1 {
		t.Fatal("old stream did not remain with its own turn")
	}
}
