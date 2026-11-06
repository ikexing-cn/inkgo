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
	talkmod "writer/ext/talk"

	"writer/ext/ai/aiapi"
	discussion "writer/ext/talk/engine"
	"writer/inference"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// replies is a model that gives each of its replies in turn, failing on
// one without text or a context done.
type replies []model.Reply

// Preloaded maps the facts the talk read onto the port: its tokens, the
// chapter's cards, whether the chapter before goes whole, and the outline
// source.

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

func TestRoutedModelNamesAreRecordedVerbatim(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "雨城", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	t.Setenv("WRITER_TEST_ROUTED_KEY", "fake")
	s := New("")
	s.cfg = model.Config{Schema: 1, Providers: []model.Provider{{
		ID: "test-relay", Name: "Relay", BaseURL: "http://127.0.0.1:1/v1",
		Protocol: model.ProtocolChat, Env: "WRITER_TEST_ROUTED_KEY",
	}}}
	sel := workspace.Selection{Provider: "test-relay", Model: "vendor/deepseek-flash"}
	m, err := s.open(sel, st)
	if err != nil {
		t.Fatal(err)
	}
	const name = "test-relay/vendor/deepseek-flash"
	if m.Name != name {
		t.Fatal(m.Name)
	}
	c := m.Completer.(counted)
	c.calls = func() func(aiapi.Call) { return func(call aiapi.Call) { _ = call.Store.Log(call.Record) } }
	c.Completer = &replies{{Text: "雨还在下。", Usage: model.Usage{Input: 1_000_000}}}
	m.Completer = c
	session, err := st.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	c.calls = func() func(aiapi.Call) { return func(call aiapi.Call) { _ = call.Store.Log(call.Record) } }
	m.Completer = c
	e := discussion.Engine{Store: st, Talk: aiapi.Handle{Model: aiapi.Model(m.Model), Name: m.Name, Effort: m.Effort, Completer: m}}
	turn, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说。"})
	if err != nil || turn.Reply.Model != name {
		t.Fatal(turn, err)
	}
	events, err := st.Events(session)
	if err != nil || events[len(events)-1].Model != name {
		t.Fatal(events, err)
	}
	records, err := st.Records()
	if err != nil || len(records) != 1 || records[0].Model != name {
		t.Fatal(records, err)
	}
	for _, file := range []string{filepath.Join("ext", "talk", "sessions", session+".jsonl"), "ext/usage/ai-log.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(st.Root(), file))
		if err != nil || !strings.Contains(string(raw), `"model":"test-relay/vendor/deepseek-flash"`) {
			t.Fatal(file, string(raw), err)
		}
	}
}

func TestSetupErrorsCrossThePort(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(filepath.Join(t.TempDir(), "bad.yaml"))
	s.cfg = model.DefaultConfig()
	if err = os.WriteFile(s.file, []byte("schema: ["), 0600); err != nil {
		t.Fatal(err)
	}
	for name, request := range map[string]func() error{
		"config":          s.Reload,
		"service form":    func() error { return s.SaveProvider(aiapi.Provider{}) },
		"key":             func() error { return s.SetKey("invalid id", "unused") },
		"catalog service": func() error { _, err := s.Catalog("missing"); return err },
		"talk model": func() error {
			_, err := talkmod.Build(s, st, aiapi.Material{Setting: true, Outline: true}, nil, nil, nil, nil)
			return err
		},
		"writing model":    func() error { _, err := s.Pen(st, aiapi.Material{Setting: true, Outline: true}); return err },
		"selected service": func() error { _, err := s.open(workspace.Selection{Provider: "missing", Model: "m"}, st); return err },

		"pen runtime model": func() error {
			_, err := (pen{&writing.Writer{Store: st}}).Predict(context.Background(), aiapi.Scene{})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := request()
			var setup *aiapi.SetupError
			if !errors.As(err, &setup) || setup.Unwrap() == nil || setup.Error() != setup.Unwrap().Error() {
				t.Fatalf("configuration error lost its identity or text: %v", err)
			}
		})
	}
	for _, setup := range []bool{false, true} {
		cause := error(errors.New("模型请求失败"))
		if setup {
			cause = &model.SetupError{Err: cause}
		}
		for name, request := range map[string]func() error{
			"request": func() error {
				c := counted{Completer: failingRequest{cause}, st: st}
				_, err := c.Complete(context.Background(), model.Request{}, nil)
				return err
			},
			"catalog": func() error {
				_, err := (catalog{Catalog: failingRequest{cause}}).ListModels(context.Background())
				return err
			},
		} {
			err := request()
			var config *aiapi.SetupError
			if errors.As(err, &config) != setup || !errors.Is(err, cause) || err.Error() != cause.Error() {
				t.Fatalf("%s: setup %v: %v", name, setup, err)
			}
		}
	}
	s.BindSwitch(func() bool { return false })
	_, err = s.Catalog("missing")
	var setup *aiapi.SetupError
	if !errors.Is(err, ErrOff) || errors.As(err, &setup) {
		t.Fatal("turning AI off is not a setup failure", err)
	}
}

type failingRequest struct{ err error }

func (f failingRequest) Complete(context.Context, model.Request, func(model.Delta)) (model.Reply, error) {
	return model.Reply{}, f.err
}

func (f failingRequest) ListModels(context.Context) ([]string, error) { return nil, f.err }

// Each request is a line of the AI record, for the purpose its context
// carries: done, failed,
// cancelled, and cancelled by its maker for having enough, which is done.
// A trace folder gets each whole; the record and the work get no text.
func TestModelHandleRecordsEveryOutcomeAndFullTraces(t *testing.T) {
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文", "zh-Hans")
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
	}, st, "deepseek/deepseek-flash", nil, trace, nil, func() func(aiapi.Call) { return func(call aiapi.Call) { _ = call.Store.Log(call.Record) } }}
	service := New("")
	w := st.Work()
	w.Discussion = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash"}
	if err := st.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	service.Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		return inference.Model{Completer: m}, nil
	}
	handle, err := service.Model(st, aiapi.RoleTalk)
	if err != nil {
		t.Fatal(err)
	}
	parts := []workspace.Part{{Name: workspace.PartRules, Tokens: 900}, {Name: workspace.PartTalk, Tokens: 4000}}
	talk := workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureTalk, Talk: "s1", Parts: parts})
	handle.Completer.Complete(talk, model.Request{System: "讨论的规则全文", Effort: model.EffortHigh}, nil)
	check := workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureCheck, Chapter: "c1"})
	handle.Completer.Complete(check, model.Request{}, nil)
	gone, cancel := context.WithCancel(check)
	cancel()
	handle.Completer.Complete(gone, model.Request{}, nil)
	enough, stop := context.WithCancelCause(workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureContinue, Chapter: "c1"}))
	stop(workspace.ErrEnough)
	handle.Completer.Complete(enough, model.Request{Effort: model.EffortNone}, nil)

	got, err := st.Records()
	if err != nil || len(got) != 4 {
		t.Fatalf("recorded %+v %v", got, err)
	}
	if r := got[0]; r.Feature != workspace.FeatureTalk || r.Talk != "s1" || r.Model != "deepseek/deepseek-flash" || r.Effort != "high" ||
		!slices.Equal(r.Parts, parts) || r.Usage.Input != 1_000_000 || r.Outcome != workspace.OutcomeDone || r.At.IsZero() {
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
	if b, err := os.ReadFile(filepath.Join(st.Root(), "ext/usage/ai-log.jsonl")); err != nil || strings.Contains(string(b), "规则全文") || strings.Contains(string(b), "好。") {
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
	st, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "测试作品", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	w := st.Work()
	w.Discussion = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash"}
	if err := st.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	s := New("")
	var built atomic.Int32
	s.Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		built.Add(1)
		return inference.Model{}, nil
	}
	on := false
	s.BindSwitch(func() bool { return on })
	if _, err := s.Catalog("deepseek"); !errors.Is(err, ErrOff) {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := talkmod.Build(s, st, aiapi.Material{Setting: true, Outline: true}, nil, nil, nil, nil); !errors.Is(err, ErrOff) {
		t.Fatalf("talk: %v", err)
	}
	if _, err := s.Pen(st, aiapi.Material{Setting: true, Outline: true}); !errors.Is(err, ErrOff) {
		t.Fatalf("pen: %v", err)
	}
	if n := built.Load(); n != 0 {
		t.Fatalf("built %d with the AI off", n)
	}
	on = true
	if _, err := s.Pen(st, aiapi.Material{Setting: true, Outline: true}); err != nil {
		t.Fatal(err)
	}
	if n := built.Load(); n != 1 {
		t.Fatalf("built %d with the AI on again", n)
	}
}

func TestSendingDoesNotReadTheSwitch(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "雨城", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	w := st.Work()
	w.Discussion = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash"}
	if err = st.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "城市总在下雨"},
	}}); err != nil {
		t.Fatal(err)
	}
	chapter, err := st.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.WriteChapter(chapter, strings.Repeat("雨下了一夜。", 50), nil); err != nil {
		t.Fatal(err)
	}
	sent := errors.New("fake request")
	s := New("")
	s.Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		return inference.Model{Completer: failingRequest{sent}, Model: model.Model{Window: 32000, Output: 4000}}, nil
	}
	var reads int
	s.BindSwitch(func() bool { reads++; return true })
	talk, err := talkmod.Build(s, st, aiapi.Material{Setting: true, Outline: true}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pen, err := s.Pen(st, aiapi.Material{Setting: true, Outline: true})
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("builders read the switch %d times", reads)
	}
	session, err := st.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	s.BindSwitch(func() bool {
		t.Error("a background request read the interface's switch")
		return false
	})
	done := make(chan []error, 1)
	go func() {
		ctx := context.Background()
		_, talkErr := talk.Say(ctx, session, workspace.Event{Text: "接着说。"})
		_, predictErr := pen.Predict(ctx, aiapi.Scene{Chapter: chapter, Before: "雨停了。"})
		_, checkErr := pen.Check(ctx, chapter)
		done <- []error{talkErr, predictErr, checkErr}
	}()
	for _, err := range <-done {
		if !errors.Is(err, sent) {
			t.Error(err)
		}
	}
}

func TestFailedRequestIsRecordedInTheWordsTheAuthorSaw(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "Book", "Fantasy", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	words := func(err error) string { return "shown: " + err.Error() }
	c := counted{Completer: failingRequest{errors.New("model: refused")}, st: st, name: "deepseek/deepseek-flash", words: words, calls: func() func(aiapi.Call) { return func(call aiapi.Call) { _ = call.Store.Log(call.Record) } }}
	ctx := workspace.WithPurpose(context.Background(), workspace.Purpose{Feature: workspace.FeatureCheck, Chapter: "c1"})
	if _, err := c.Complete(ctx, model.Request{}, nil); err == nil {
		t.Fatal("the failure was lost")
	}
	records, err := st.Records()
	if err != nil || len(records) != 1 || records[0].Error != "shown: model: refused" {
		t.Fatal(records, err)
	}
}
