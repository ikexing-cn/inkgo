package writing

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"writer/discussion"
	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// fake streams its reply in pieces and keeps the requests it was sent.
type fake struct {
	pieces []string
	err    error
	got    []model.Request
}

func (f *fake) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	f.got = append(f.got, req)
	var all string
	for _, p := range f.pieces {
		if ctx.Err() != nil {
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
	return &Writer{Store: s, Model: discussion.Model{Model: limits, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}}, two, r
}

// What a draft shows the model is the work's words, never its IDs, and
// the record lists what it shows.
func TestDraftShowsWhatItRecords(t *testing.T) {
	f := &fake{pieces: []string{"　　灯亮了。\n\n", "  她笑了。\n"}}
	w, two, r := book(t, f)
	job, err := w.Draft(Scene{Chapter: two}, 800)
	if err != nil {
		t.Fatal(err)
	}
	req := job.Request
	all := req.System + "\n" + req.Messages[0].Content
	for _, want := range []string{"城市总在下雨", "【风格】", "第三人称，短句", "已放弃", "主角会飞", "她第一次走进旧城。", "她修好井边的灯。", "她走到井边。", "篇幅约 800 字"} {
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
	if got := strings.Join(job.Record.Shown, "|"); got != "卷纲：第一卷|章纲：第二章|上一章《第一章》结尾 12 字" {
		t.Errorf("shown %q", got)
	}
	if len(job.Record.Setting) != 3 || job.Record.Op != workspace.AIDraft || job.Record.Model != "deepseek/deepseek-flash" || job.Record.Instruction != "约 800 字" {
		t.Errorf("record %+v", job.Record)
	}
	// The setting comes before the task, so drafts and rewrites share the
	// start a provider caches; the record has what each part took.
	if s, task := strings.Index(req.System, "城市总在下雨"), strings.Index(req.System, "这次的任务是"); s < 0 || task < s {
		t.Errorf("the task comes before the setting:\n%s", req.System)
	}
	for _, part := range []string{workspace.PartRules, workspace.PartSetting, workspace.PartOutline, workspace.PartProse, workspace.PartInput} {
		if job.Record.Sent == nil || job.Record.Sent.Part(part) == 0 {
			t.Errorf("the record has no %s: %+v", part, job.Record.Sent)
		}
	}
	var streamed []string
	text, err := w.Run(context.Background(), job, func(s string) { streamed = append(streamed, s) })
	if err != nil {
		t.Fatal(err)
	}
	if text != "灯亮了。\n她笑了。" || strings.Join(streamed, "") != text {
		t.Fatalf("text %q streamed %q", text, streamed)
	}
	if f.got[0].Effort != model.EffortHigh || f.got[0].MaxTokens != min(draftOutput, limits.Output) {
		t.Fatalf("request %+v", f.got[0])
	}
}

func TestDraftNeedsAnOutline(t *testing.T) {
	w, two, _ := book(t, &fake{})
	if err := w.Store.WriteOutline(two, " "); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Draft(Scene{Chapter: two}, 0); !errors.Is(err, ErrNoOutline) {
		t.Fatal(err)
	}
}

// A draft cut off keeps what it wrote.
func TestDraftCutOffKeepsWhatCame(t *testing.T) {
	w, two, _ := book(t, &fake{pieces: []string{"灯亮了。"}, err: errors.New("模型回复被截断")})
	job, err := w.Draft(Scene{Chapter: two}, 0)
	if err != nil {
		t.Fatal(err)
	}
	text, err := w.Run(context.Background(), job, nil)
	if text != "灯亮了。" || err == nil || !strings.Contains(err.Error(), "没有写完") {
		t.Fatalf("%q %v", text, err)
	}
}

func TestRewriteShowsTheWordsAround(t *testing.T) {
	f := &fake{pieces: []string{"她慢慢走到井边。"}}
	w, two, _ := book(t, f)
	job, err := w.Rewrite(Scene{Chapter: two, Before: "雨停了。\n\n", After: "井里有光。"}, "她站在井边。", "写慢一点")
	if err != nil {
		t.Fatal(err)
	}
	msg := job.Request.Messages[0].Content
	at := func(s string) int {
		i := strings.Index(msg, s)
		if i < 0 {
			t.Fatalf("no %q in %s", s, msg)
		}
		return i
	}
	if !(at("雨停了。") < at("她站在井边。") && at("她站在井边。") < at("写慢一点") && at("写慢一点") < at("井里有光。")) {
		t.Fatal(msg)
	}
	if got := strings.Join(job.Record.Shown, "|"); got != "卷纲：第一卷|章纲：第二章|上一章《第一章》结尾 12 字|本章前文 4 字|本章后文 5 字" {
		t.Errorf("shown %q", got)
	}
	if _, err = w.Rewrite(Scene{Chapter: two}, "x", " "); err == nil {
		t.Fatal("a rewrite without an instruction")
	}
}

// A prediction is one sentence, asked without reasoning, and stops
// reading once the sentence ends.
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

func TestBudgetCutsTheTextBeforeFromItsStart(t *testing.T) {
	w, two, _ := book(t, &fake{})
	long := strings.Repeat("雨", 20000)
	job, err := w.Rewrite(Scene{Chapter: two, Before: long + "最后一句。"}, "她", "改")
	if err != nil {
		t.Fatal(err)
	}
	msg := job.Request.Messages[0].Content
	if !strings.Contains(msg, "最后一句。") || prompt.Estimate(job.Request.System+msg) > w.Model.Input() {
		t.Fatalf("estimate %d", prompt.Estimate(job.Request.System+msg))
	}
}

// limits are a test model's: room for a chapter of some size.
var limits = model.Model{ID: "test", Window: 20000, Output: 4000, CompactAt: 80}
