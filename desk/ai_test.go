package desk

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/editor"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// stream is a writing model that streams its pieces; with hold, it waits
// after the first until let through or stopped.
type stream struct {
	pieces []string
	hold   chan struct{}
	mu     sync.Mutex
	asked  []model.Request
}

func (f *stream) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	f.mu.Unlock()
	var all strings.Builder
	for i, p := range f.pieces {
		if i == 1 && f.hold != nil {
			select {
			case <-f.hold:
			case <-ctx.Done():
				return model.Reply{Text: all.String()}, ctx.Err()
			}
		}
		if ctx.Err() != nil {
			return model.Reply{Text: all.String()}, ctx.Err()
		}
		all.WriteString(p)
		on(model.Delta{Text: p})
	}
	return model.Reply{Text: all.String()}, nil
}

func penWith(a *App, f *stream) {
	a.Pen = func(s *workspace.Store) (*writing.Writer, error) {
		return &writing.Writer{Store: s, Model: discussion.Model{Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}}, nil
	}
	a.write.penKey = "" // build the editors' writer again
}

func until(t *testing.T, tt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting: "+what, tt.Texts())
		}
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
}

// Drafting needs an outline; the draft streams in read-only, is kept as
// one undo step with its mark, and the chapter before it is a snapshot
// recording what the AI saw.
func TestDraftAChapterFromItsOutline(t *testing.T) {
	a, tt, id := writingFixture(t, "")
	f := &stream{pieces: []string{"雨下了一夜。", "\n她醒了。"}, hold: make(chan struct{})}
	penWith(a, f)
	tt.Frame()
	if err := clickMore(tt, "起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.draftOpen {
		t.Fatal("drafting opened with no outline")
	}
	a.write.outline = "她在雨夜醒来。"
	tt.Frame()
	if err := tt.Click("起草本章"); err != nil { // the menu stayed open
		t.Fatal(err)
	}
	tt.Frame()
	a.write.words = "800"
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the first piece", func() bool { return a.write.ed.Doc.String() == "雨下了一夜。" })
	if !a.write.ed.ReadOnly() || !tt.HasText("停止") {
		t.Fatal("not read-only while drafting", tt.Texts())
	}
	tt.Type("不该进去")
	close(f.hold)
	until(t, tt, "the draft to end", func() bool { return a.write.run == nil })
	ed := a.write.ed
	if ed.Doc.String() != "雨下了一夜。\n她醒了。" || ed.ReadOnly() {
		t.Fatalf("drafted %q", ed.Doc.String())
	}
	if ms := ed.Doc.Marks(); len(ms) != 1 || ms[0].Kind != editor.MarkAIDraft || ms[0].End != ed.Doc.Len() {
		t.Fatalf("marks %+v", ms)
	}
	if got := disk(t, a, "第一章.txt"); got != "雨下了一夜。\n她醒了。" {
		t.Fatalf("on disk %q", got)
	}
	snaps, _ := a.Store.Snapshots(id)
	s := snaps[0]
	if s.Reason != workspace.SnapAI || s.AI == nil || s.AI.Op != workspace.AIDraft || s.AI.Instruction != "约 800 字" || s.AI.Model != "deepseek/deepseek-flash" {
		t.Fatalf("snapshot %+v", s)
	}
	if !strings.Contains(f.asked[0].Messages[len(f.asked[0].Messages)-1].Content, "她在雨夜醒来。") {
		t.Fatal("the outline was not sent")
	}
	ed.Undo()
	if ed.Doc.String() != "" {
		t.Fatalf("one undo left %q", ed.Doc.String())
	}
}

// What becomes of the words already there is chosen in the dialog, and
// asked afresh each time it opens.
func TestDraftOverWhatIsThere(t *testing.T) {
	a, tt, _ := writingFixture(t, "她醒了")
	penWith(a, &stream{})
	a.write.outline = "她醒来。"
	tt.Frame()
	for _, want := range []string{draftReplace, draftAppend} {
		if err := clickMore(tt, "起草本章"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if want == draftReplace {
			if err := tt.Click(draftReplace); err != nil {
				t.Fatal(err, tt.Texts())
			}
			tt.Frame()
		}
		tt.Frame()
		if a.write.draftMode != want {
			t.Fatalf("mode %q, want %q", a.write.draftMode, want)
		}
		a.write.draftOpen = false
		tt.Frame()
	}
}

// Stopping a draft keeps what it wrote, after the text there was.
func TestStopADraftKeepsWhatCame(t *testing.T) {
	a, tt, _ := writingFixture(t, "她醒了")
	f := &stream{pieces: []string{"雨还在下。", "不该出现。"}, hold: make(chan struct{})}
	penWith(a, f)
	a.write.outline = "她醒来。"
	tt.Frame()
	if err := clickMore(tt, "起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.draftMode != draftAppend {
		t.Fatal("drafting does not append by default")
	}
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the first piece", func() bool { return strings.HasSuffix(a.write.ed.Doc.String(), "雨还在下。") })
	if err := tt.Click("停止"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to stop", func() bool { return a.write.run == nil })
	if got := a.write.ed.Doc.String(); got != "她醒了\n雨还在下。" {
		t.Fatalf("after stop %q", got)
	}
	if !strings.Contains(a.notice, "已停止") {
		t.Fatalf("notice %q", a.notice)
	}
}

// A rewrite is shown beside the words selected; dropping it changes
// nothing, accepting it keeps the words replaced as a snapshot.
func TestRewriteASelection(t *testing.T) {
	a, tt, id := writingFixture(t, "她走到井边。井里有光。")
	f := &stream{pieces: []string{"她慢慢", "走到井边。"}}
	penWith(a, f)
	a.write.ed.Select(0, 6)
	tt.Frame()
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.instruction = "放慢"
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the rewrite", func() bool { return a.write.proposed != nil })
	if _, _, text, ok := a.write.ed.Proposal(); !ok || text != "她慢慢走到井边。" {
		t.Fatalf("proposal %q", text)
	}
	if err := tt.Click("放弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.ed.Doc.String() != "她走到井边。井里有光。" || len(a.write.ed.Doc.Marks()) != 0 || a.write.ed.ReadOnly() {
		t.Fatal("dropping changed the text")
	}
	before, _ := a.Store.Snapshots(id)

	a.write.ed.Select(0, 6)
	tt.Frame()
	if !a.write.rewriting { // dropping keeps the rewrite open, for another try
		t.Fatal("the rewrite closed")
	}
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the rewrite", func() bool { return a.write.proposed != nil })
	if err := tt.Click("采纳"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	ed := a.write.ed
	if ed.Doc.String() != "她慢慢走到井边。井里有光。" {
		t.Fatalf("accepted %q", ed.Doc.String())
	}
	if ms := ed.Doc.Marks(); len(ms) != 1 || ms[0].Kind != editor.MarkAIAccepted || ed.Doc.Slice(ms[0].Start, ms[0].End) != "她慢慢走到井边。" {
		t.Fatalf("marks %+v", ms)
	}
	snaps, _ := a.Store.Snapshots(id)
	if len(snaps) != len(before)+1 || snaps[0].AI == nil || snaps[0].AI.Op != workspace.AIRewrite || snaps[0].AI.Instruction != "放慢" {
		t.Fatalf("snapshots %+v", snaps)
	}
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "她走到井边。井里有光。" {
		t.Fatalf("snapshot text %q", text)
	}
	if err := clickMore(tt, "快照"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("改写 · deepseek/deepseek-flash · High · 放慢") {
		t.Fatal("the AI record is not shown", tt.Texts())
	}
}

// Cmd+J asks for one sentence; Tab keeps it, marked with the model.
func TestContinueOnDemand(t *testing.T) {
	a, tt, _ := writingFixture(t, "她推开门")
	f := &stream{pieces: []string{"，屋里", "很暗。她", "停住了。"}}
	penWith(a, f)
	ed := a.write.ed
	ed.MoveTo(ed.Doc.Len(), false)
	ed.Focus()
	tt.Frame()
	time.Sleep(20 * time.Millisecond)
	tt.Frame()
	if len(f.asked) != 0 {
		t.Fatal("continued unasked")
	}
	tt.Key(ui.Cmd, ui.KeyJ)
	until(t, tt, "the sentence", func() bool { return ed.Ghost() != "" })
	if ed.Ghost() != "，屋里很暗。" {
		t.Fatalf("ghost %q", ed.Ghost())
	}
	if r := f.asked[0]; r.Effort != model.EffortNone {
		t.Fatalf("continuing reasons: %q", r.Effort)
	}
	tt.Key(0, ui.KeyTab)
	if ms := ed.Doc.Marks(); ed.Doc.String() != "她推开门，屋里很暗。" || len(ms) != 1 || ms[0].Kind != editor.MarkAIAccepted || ms[0].Note != "deepseek/deepseek-flash" {
		t.Fatalf("accepted %q %+v", ed.Doc.String(), ms)
	}
}
