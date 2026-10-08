package writing

import (
	"context"
	"strings"
	"testing"

	"writer/model"
	"writer/workspace"
)

// A summary is of the chapter on disk, asks for no reasoning, and knows the
// prose it was made from.
func TestSummarizeAChapter(t *testing.T) {
	f := &fake{pieces: []string{"  林晚夜里走进旧城，", "在井边发现灯被人拧坏。\n"}}
	w, two, _ := book(t, f)
	text := strings.Repeat("她蹲下去拧开灯座。", 30)
	if _, err := w.Store.WriteChapter(two, text, nil); err != nil {
		t.Fatal(err)
	}
	sum, err := w.Summarize(context.Background(), two)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Text != "林晚夜里走进旧城，在井边发现灯被人拧坏。" || sum.Of != workspace.ProseHash(text) || sum.By != "deepseek/deepseek-flash" || sum.Words != workspace.Count(text) {
		t.Fatalf("%+v", sum)
	}
	req := f.got[0]
	if req.Effort != model.EffortNone || req.System != summaryRules || !strings.Contains(req.Messages[0].Content, "【章名】第二章") || strings.Contains(req.Messages[0].Content, two) {
		t.Fatalf("%+v", req)
	}
	one := earlierID(t, w, two)
	if _, err = w.Summarize(context.Background(), one); err == nil {
		t.Fatal("summarized a chapter too short for one")
	}

	// A summary that trails off is not kept.
	f.pieces = []string{"门卫室灯亮，", "(function(){})"}
	if _, err = w.Summarize(context.Background(), two); err == nil {
		t.Fatal("kept a summary that trails off")
	}
}

func earlierID(t *testing.T, w *Writer, id string) string {
	t.Helper()
	tree, _ := w.Store.Chapters()
	n, _ := tree.Node(id)
	return earlier(tree, n)[0].ID
}

// A draft is told what the chapters before it told, nearest kept first,
// and the record says how many summaries may be out of date.
func TestDraftRecapsTheChaptersBefore(t *testing.T) {
	f := &fake{pieces: []string{"灯亮了。"}}
	w, two, _ := book(t, f)
	s := w.Store
	one := earlierID(t, w, two)
	prologue, _ := s.NewChapter("", "序章", false)
	if err := s.MoveChapter(prologue, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChapter(prologue, strings.Repeat("旧城很早就没有灯了。", 30), nil); err != nil {
		t.Fatal(err)
	}
	pro, _ := s.ReadChapter(prologue)
	if err := s.WriteSummary(prologue, workspace.Summary{Text: "旧城断电多年。", Of: workspace.ProseHash(pro.Text)}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteSummary(one, workspace.Summary{Text: "林晚\n走到井边。", Of: "old"}); err != nil {
		t.Fatal(err)
	}
	if missing, have, err := Unsummarized(s, two); err != nil || len(missing) != 0 || have != 2 {
		t.Fatal(missing, err) // the first chapter is short, and has one anyway
	}
	job, err := w.Draft(Scene{Chapter: two}, 0)
	if err != nil {
		t.Fatal(err)
	}
	msg := job.Request.Messages[0].Content
	want := "【前情提要】（前面各章的摘要，按顺序）\n《序章》旧城断电多年。\n《第一章》林晚 走到井边。"
	if !strings.Contains(msg, want) || strings.Index(msg, "【前情提要】") > strings.Index(msg, "【本章：第二章】") {
		t.Fatalf("%s", msg)
	}
	if !strings.Contains(strings.Join(job.Record.Shown, "|"), "前情提要 2 章（其中 1 章正文改过，摘要可能过时）") {
		t.Fatal(job.Record.Shown)
	}
	if strings.Contains(msg, prologue) || strings.Contains(msg, one) {
		t.Fatal("an ID was sent")
	}

	// A rewrite sees no recap; nor does a draft of the first chapter.
	rw, _ := w.Rewrite(Scene{Chapter: two, Before: "灯"}, "灯", "改短")
	if strings.Contains(rw.Request.Messages[0].Content, "前情提要") {
		t.Fatal("a rewrite got the recap")
	}
	if err := s.WriteOutline(prologue, "断电。"); err != nil {
		t.Fatal(err)
	}
	first, _ := w.Draft(Scene{Chapter: prologue}, 0)
	if strings.Contains(first.Request.Messages[0].Content, "前情提要") {
		t.Fatal("the first chapter got a recap")
	}

	// Chapters long enough without a summary are the ones to fill in.
	if err := s.WriteSummary(prologue, workspace.Summary{}); err != nil {
		t.Fatal(err)
	}
	if missing, _, err := Unsummarized(s, two); err != nil || len(missing) != 1 || missing[0] != prologue {
		t.Fatal(missing, err)
	}
}

// The recap keeps the nearest chapters when they do not all fit.
func TestRecapKeepsTheNearest(t *testing.T) {
	w, two, _ := book(t, &fake{})
	s := w.Store
	var ids []string
	for i := range 6 {
		id, _ := s.NewChapter("", "章"+string(rune('A'+i)), false)
		if err := s.MoveChapter(id, "", i); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteSummary(id, workspace.Summary{Text: strings.Repeat(string(rune('甲'+i)), 1000), Of: "x"}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	tree, _ := s.Chapters()
	n, _ := tree.Node(two)
	text, chapters, stale, err := w.recap(tree, n, 1<<20)
	if err != nil || chapters != 3 || stale != 3 || strings.Contains(text, "《章C》") || !strings.Contains(text, "《章D》") || !strings.HasSuffix(text, strings.Repeat(string(rune(0x7532+5)), 1000)) {
		t.Fatalf("%d %d %v %q", chapters, stale, err, text[:40])
	}
}
