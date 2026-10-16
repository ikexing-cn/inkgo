package ai

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// replies is a model that gives each of its replies in turn, failing on
// one without text or a context done.
type replies []model.Reply

func (r *replies) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	next := (*r)[0]
	*r = (*r)[1:]
	if err := ctx.Err(); err != nil {
		return next, err
	}
	if next.Text == "" {
		return next, errors.New("服务不可用")
	}
	return next, nil
}

// Each request is a line of the AI record, for the purpose its context
// carries and with its cost at the prices built in: done, failed,
// cancelled, and cancelled by its maker for having enough, which is done.
// A trace folder gets each whole; the record and the work get no text.
func TestEachRequestIsRecorded(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	trace := filepath.Join(t.TempDir(), "trace")
	m := counted{&replies{
		{Text: "好。", Usage: model.Usage{Input: 1_000_000, CacheRead: 500_000, Output: 1000}},
		{Usage: model.Usage{Input: 1000}},
		{Text: "取消了。"},
		{Text: "够了。", Usage: model.Usage{Input: 10, Output: 2}},
	}, st, "deepseek/deepseek-flash", "deepseek-flash", trace}
	parts := []workspace.Part{{Name: workspace.PartRules, Tokens: 900}, {Name: workspace.PartTalk, Tokens: 4000}}
	talk := workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureTalk, Talk: "s1", Parts: parts})
	m.Complete(talk, model.Request{System: "讨论的规则全文", Effort: model.EffortHigh}, nil)
	check := workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureCheck, Chapter: "c1"})
	m.Complete(check, model.Request{}, nil)
	gone, cancel := context.WithCancel(check)
	cancel()
	m.Complete(gone, model.Request{}, nil)
	enough, stop := context.WithCancelCause(workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureContinue, Chapter: "c1"}))
	stop(workspace.ErrEnough)
	m.Complete(enough, model.Request{Effort: model.EffortNone}, nil)

	got, err := st.Records()
	if err != nil || len(got) != 4 {
		t.Fatalf("recorded %+v %v", got, err)
	}
	want := 0.5*0.3 + 0.5*0.006 + 0.001*1.2
	if r := got[0]; r.Feature != workspace.FeatureTalk || r.Talk != "s1" || r.Model != "deepseek/deepseek-flash" || r.Effort != "high" ||
		!slices.Equal(r.Parts, parts) || r.Usage.Input != 1_000_000 || r.Outcome != workspace.OutcomeDone || r.At.IsZero() ||
		r.Currency != "$" || r.Cost < want-1e-9 || r.Cost > want+1e-9 {
		t.Errorf("the talk: %+v", r)
	}
	if r := got[1]; r.Feature != workspace.FeatureCheck || r.Chapter != "c1" || r.Outcome != workspace.OutcomeFailed || r.Error != "服务不可用" || r.Usage.Input != 1000 {
		t.Errorf("the failed check: %+v", r)
	}
	if r := got[2]; r.Outcome != workspace.OutcomeCancelled || r.Error != "" {
		t.Errorf("the cancelled check: %+v", r)
	}
	if r := got[3]; r.Feature != workspace.FeatureContinue || r.Outcome != workspace.OutcomeDone || r.Effort != "none" {
		t.Errorf("the sentence taken: %+v", r)
	}
	if b, err := os.ReadFile(filepath.Join(st.Root(), "ai-log.jsonl")); err != nil || strings.Contains(string(b), "规则全文") || strings.Contains(string(b), "好。") {
		t.Errorf("the record holds the text: %s %v", b, err)
	}
	files, err := os.ReadDir(trace)
	if err != nil || len(files) != 4 || !strings.HasSuffix(files[0].Name(), "-talk.json") {
		t.Fatalf("traced %v %v", files, err)
	}
	if b, _ := os.ReadFile(filepath.Join(trace, files[0].Name())); !strings.Contains(string(b), "讨论的规则全文") || !strings.Contains(string(b), "好。") {
		t.Errorf("the trace lacks the request or its reply: %s", b)
	}
	filepath.WalkDir(st.Root(), func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, "-talk.json") {
			t.Errorf("traced into the work: %s", p)
		}
		return err
	})
}

// Off, nothing that sends is built: the catalog of a service, the talk
// and the pen fail at once. On again, they are built as before.
func TestOffBuildsNothing(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New("")
	var built atomic.Int32
	s.NewWriter = func(*workspace.Store) (*writing.Writer, error) {
		built.Add(1)
		return &writing.Writer{Store: st}, nil
	}
	s.SetOn(false)
	if _, err := s.Catalog("deepseek"); !errors.Is(err, ErrOff) {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := s.Talk(st, nil, nil, nil); !errors.Is(err, ErrOff) {
		t.Fatalf("talk: %v", err)
	}
	if _, err := s.Pen(st); !errors.Is(err, ErrOff) {
		t.Fatalf("pen: %v", err)
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("built %d with the AI off", n)
	}
	s.SetOn(true)
	if _, err := s.Pen(st); err != nil {
		t.Fatal(err)
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("built %d with the AI on again", n)
	}
}
