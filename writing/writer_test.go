package writing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"

	"writer/inference"
	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

func requestBytes(t *testing.T, calls []model.Request, want string) {
	t.Helper()
	b, err := json.Marshal(calls)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(b))
	if got != want {
		t.Fatalf("request bytes: got %s, want %s\n%s", got, want, b)
	}
}

func TestPredictionRequestBytes(t *testing.T) {
	f := &fake{pieces: []string{"灯亮了。"}}
	w, chapter, _ := book(t, f)
	if _, err := w.Predict(context.Background(), Scene{Chapter: chapter, Before: "雨停了。", After: "井里有光。"}); err != nil {
		t.Fatal(err)
	}
	requestBytes(t, f.got, "bd42c9cb0475c8cbb16cb8eee51eb4ad9c0929d8756b748ff2412de2130e3c1c")
}

// fake streams its reply in pieces and keeps the requests it was sent.
type fake struct {
	pieces   []string
	err      error
	got      []model.Request
	purposes []workspace.Purpose // of each request, as its context tells
	cause    error               // why the last request stopped early
}

func (f *fake) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	f.got = append(f.got, req)
	f.purposes = append(f.purposes, workspace.PurposeOf(ctx))
	var all string
	for _, p := range f.pieces {
		if ctx.Err() != nil {
			f.cause = context.Cause(ctx)
			return model.Reply{}, ctx.Err()
		}
		all += p
		if on != nil {
			on(model.Delta{Text: p})
		}
	}
	return model.Reply{Text: all}, f.err
}

// book is a work with a setting, a volume of two chapters, and a writer.
func book(t *testing.T, f *fake) (*Writer, string, workspace.Revision) {
	t.Helper()
	s, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "雨城", "网文")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	session, _ := s.NewSession()
	m, _ := s.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "城市总在下雨。她怕黑。主角不会飞。"})
	r, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "城市总在下雨", Sources: []workspace.Source{{Session: session, Message: m.ID, Quote: "城市总在下雨"}}},
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "风格", Text: "第三人称，短句", Sources: []workspace.Source{{Session: session, Message: m.ID, Quote: "她怕黑"}}},
		{Op: workspace.OpAdd, Place: workspace.PlaceDropped, Category: "人物", Text: "主角会飞", Sources: []workspace.Source{{Session: session, Message: m.ID, Quote: "主角不会飞"}}},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "她有个妹妹", Sources: []workspace.Source{{Session: session, Message: m.ID, Quote: "她怕黑"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	vol, _ := s.NewChapter("", "第一卷", true)
	if err = s.WriteOutline(vol, "她第一次走进旧城。"); err != nil {
		t.Fatal(err)
	}
	one, _ := s.NewChapter(vol, "第一章", false)
	if _, err = s.WriteChapter(one, "雨下了一夜。\n她走到井边。", nil); err != nil {
		t.Fatal(err)
	}
	two, _ := s.NewChapter(vol, "第二章", false)
	if err = s.WriteOutline(two, "她修好井边的灯。"); err != nil {
		t.Fatal(err)
	}
	return &Writer{Material: aiapi.Material{Setting: true, Outline: true}, Store: s, Model: inference.Model{Model: limits, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}}, two, r
}

// What a prediction shows the model is the work's words, never its IDs,
// and what it sends is told by part; what is open of the setting is not
// the author's yet and stays out.
func TestPredictShowsTheWorksWords(t *testing.T) {
	f := &fake{pieces: []string{"灯亮了。"}}
	w, two, r := book(t, f)
	if _, err := w.Predict(context.Background(), Scene{Chapter: two, Before: "雨停了。", After: "井里有光。"}); err != nil {
		t.Fatal(err)
	}
	req := f.got[0]
	all := req.System + "\n" + req.Messages[0].Content
	for _, want := range []string{"城市总在下雨", "【风格】", "第三人称，短句", "已放弃", "主角会飞", "她第一次走进旧城。", "她修好井边的灯。", "她走到井边。", "【本章前文】\n雨停了。", "【本章后文】\n井里有光。"} {
		if !strings.Contains(all, want) {
			t.Errorf("the request lacks %q", want)
		}
	}
	if strings.Contains(all, "她有个妹妹") {
		t.Error("an open entry went into the prose's request")
	}
	set, _ := w.Store.Setting()
	tree, _ := w.Store.Chapters()
	var ids []string
	for _, e := range set.Entries {
		ids = append(ids, e.ID)
	}
	tree.Walk(func(n *workspace.Node, _ int) { ids = append(ids, n.ID) })
	ids = append(ids, r.ID, w.Store.Work().ID)
	for _, id := range ids {
		if strings.Contains(all, id) {
			t.Errorf("the request shows the ID %s", id)
		}
	}
	// The setting comes before the task, so what a provider caches is
	// shared with the check.
	if s, task := strings.Index(req.System, "城市总在下雨"), strings.Index(req.System, "这次的任务是"); s < 0 || task < s {
		t.Errorf("the task comes before the setting:\n%s", req.System)
	}
	parts := f.purposes[0].Parts
	for _, part := range []string{workspace.PartRules, workspace.PartSetting, workspace.PartOutline, workspace.PartProse, workspace.PartInput} {
		if !slices.ContainsFunc(parts, func(p workspace.Part) bool { return p.Name == part && p.Tokens > 0 }) {
			t.Errorf("the record has no %s: %+v", part, parts)
		}
	}
}

// A prediction is one sentence, asked without reasoning, and stops
// reading once the sentence ends, as having enough: the record counts it
// done.
func TestPredictOneSentence(t *testing.T) {
	f := &fake{pieces: []string{"她推开", "门，“谁？”", "她问。然后", "又一句。"}}
	w, two, _ := book(t, f)
	s, err := w.Predict(context.Background(), Scene{Chapter: two, Before: "雨停了。"})
	if err != nil {
		t.Fatal(err)
	}
	if s != "她推开门，“谁？”" {
		t.Fatalf("%q", s)
	}
	if f.got[0].Effort != model.EffortNone || f.got[0].MaxTokens != nextOutput {
		t.Fatalf("request %+v", f.got[0])
	}
	if p := f.purposes[0]; p.Feature != workspace.FeatureContinue || p.Chapter != two || len(p.Parts) == 0 || !errors.Is(f.cause, workspace.ErrEnough) {
		t.Fatalf("the record would say %+v, stopped for %v", p, f.cause)
	}
}

func TestSentence(t *testing.T) {
	for in, want := range map[string]string{
		"她走了。后来":   "她走了。",
		"“走吧！”他说":  "“走吧！”",
		"\n她……停住了": "她……",
		"没有句号":     "没有句号",
		"一行\n两行":   "一行",
	} {
		if got, _ := sentence(in); got != want {
			t.Errorf("sentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetupErrorsForWritingLimits(t *testing.T) {
	var setup *model.SetupError
	if !errors.As(ErrNoModel, &setup) {
		t.Fatal(ErrNoModel)
	}
	w, id, _ := book(t, &fake{})
	w.Model.Window, w.Model.Output = 1, 0
	_, _, err := w.prepare("续写", Scene{Chapter: id}, "")
	if !errors.As(err, &setup) {
		t.Fatal("outline budget lost its setup identity", err)
	}
}

func TestBudgetCutsTheTextBeforeFromItsStart(t *testing.T) {
	f := &fake{pieces: []string{"她走了。"}}
	w, two, _ := book(t, f)
	long := strings.Repeat("雨", 20000)
	if _, err := w.Predict(context.Background(), Scene{Chapter: two, Before: long + "最后一句。"}); err != nil {
		t.Fatal(err)
	}
	req := f.got[0]
	msg := req.Messages[0].Content
	if !strings.Contains(msg, "最后一句。") || prompt.Estimate(req.System+msg) > w.Model.Input() {
		t.Fatalf("estimate %d", prompt.Estimate(req.System+msg))
	}
}

// limits are a test model's: room for a chapter of some size.
var limits = model.Model{ID: "test", Window: 20000, Output: 4000, CompactAt: 80}

// The prose of a chapter is written and checked to the setting as it
// stands there: a change from a chapter after is not shown, only counted.
func TestTheProseIsWrittenToTheSettingAsItStands(t *testing.T) {
	f := &fake{}
	w, two, r := book(t, f)
	s := w.Store
	three, err := s.NewChapter("", "第三章", false)
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := s.Chapters()
	node, _ := tree.Node(two)
	one, _ := tree.Previous(node)
	rain := r.Ops[0].Entry
	if _, err = s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpChange, Entry: rain, From: two, Place: workspace.PlaceBody, Text: "城市的雨停了"},
		{Op: workspace.OpChange, Entry: rain, From: three, Place: workspace.PlaceBody, Text: "雨又下了起来"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		at, has, ahead string
		not            []string
	}{
		{one.ID, "城市总在下雨", "还有 2 处变化，未列出", []string{"雨停了", "雨又下了"}},
		{two, "城市的雨停了", "还有 1 处变化，未列出", []string{"城市总在下雨", "雨又下了"}},
		{three, "雨又下了起来", "", []string{"城市总在下雨", "雨停了", "处变化"}},
	} {
		f.pieces = []string{"她推开门。"}
		if _, err := w.Predict(context.Background(), Scene{Chapter: c.at, Before: "雨停了吗？"}); err != nil {
			t.Fatal(err)
		}
		system := f.got[len(f.got)-1].System
		if !strings.Contains(system, c.has) || !strings.Contains(system, c.ahead) {
			t.Errorf("at %s: %s", c.at, system)
		}
		for _, not := range c.not {
			if strings.Contains(strings.ReplaceAll(system, "雨停了吗", ""), not) {
				t.Errorf("at %s the model is told %q", c.at, not)
			}
		}
	}
	if _, err = s.WriteChapter(two, strings.Repeat("她蹲下去拧开灯座。", 30), nil); err != nil {
		t.Fatal(err)
	}
	f.pieces = []string{`{"findings":[]}`}
	if _, err = w.Check(context.Background(), two); err != nil {
		t.Fatal(err)
	}
	if msg := f.got[len(f.got)-1].Messages[0].Content; !strings.Contains(msg, "城市的雨停了") || strings.Contains(msg, "城市总在下雨") || strings.Contains(msg, "雨又下了") {
		t.Fatal(msg)
	}
}
