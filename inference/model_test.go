package inference

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"writer/model"
)

type failing struct{ err error }

func (f failing) Complete(context.Context, model.Request, func(model.Delta)) (model.Reply, error) {
	return model.Reply{}, f.err
}

func TestCompleteReportsTheOutputLimit(t *testing.T) {
	limits := model.Model{ID: "test", Window: 20000, Output: 4000, CompactAt: 80}

	cut := Model{Model: limits, Completer: failing{err: fmt.Errorf("deepseek: %w", model.ErrTruncated)}}
	_, err := cut.Complete(context.Background(), model.Request{}, nil)
	if err == nil || err.Error() != "inference: reply exceeds maximum output 4000" {
		t.Fatalf("cut off at the most: %v", err)
	}
	if _, err = cut.Complete(context.Background(), model.Request{MaxTokens: 400}, nil); !errors.Is(err, model.ErrTruncated) || strings.Contains(err.Error(), "设置") {
		t.Fatalf("cut off at what was asked: %v", err)
	}
}

func TestSetupErrorsForModelLimits(t *testing.T) {
	var setupError *model.SetupError

	m := Model{Model: model.Model{Window: 8192, Output: 1024}, Completer: failing{err: model.ErrTruncated}}
	_, err := m.Complete(context.Background(), model.Request{}, nil)
	if !errors.As(err, &setupError) {
		t.Fatal("configured output limit lost its setup identity", err)
	}
	_, err = m.Complete(context.Background(), model.Request{MaxTokens: 10}, nil)
	if errors.As(err, &setupError) || !errors.Is(err, model.ErrTruncated) {
		t.Fatal("request's own output cap became a setup error", err)
	}
}

func TestReady(t *testing.T) {
	if (Model{}).Ready() || !(Model{Completer: failing{}}).Ready() {
		t.Fatal("readiness must depend on the client")
	}
}
