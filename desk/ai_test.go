package desk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
	"writer/discussion"
	"writer/editor"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// stream is a writing model that streams its pieces; with hold, it waits
// after the first until let through or stopped. With fail, it fails after
// the last.
type stream struct {
	pieces []string
	hold   chan struct{}
	fail   error
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
	return model.Reply{Text: all.String()}, f.fail
}

// proposed is what the editor shows the AI proposing.
func proposed(a *App) string {
	_, _, s, _ := a.write.ed.Proposal()
	return s
}

func penWith(a *App, f *stream) { penWithin(a, f, model.Builtin("deepseek-flash")) }

// penWithin is penWith with a model of the limits m.
func penWithin(a *App, f *stream, m model.Model) {
	aiOf(a).NewWriter = func(s *workspace.Store) (*writing.Writer, error) {
		return &writing.Writer{Store: s, Model: discussion.Model{Model: m, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}}, nil
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

// Drafting needs an outline. The draft streams in beside the text, which
// stays as it was and read-only, on the disk too, until the author takes
// it: then the chapter before it is a snapshot recording what the AI saw,
// and the draft goes in as one undo step with its mark, the keyboard back
// in the prose to undo it.
func TestDraftAChapterFromItsOutline(t *testing.T) {
	a, tt, id := writingFixture(t, "")
	f := &stream{pieces: []string{"雨下了一夜。", "\n她醒了。"}, hold: make(chan struct{})}
	penWith(a, f)
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
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
	until(t, tt, "the first piece", func() bool { return proposed(a) == "雨下了一夜。" })
	if !a.write.ed.ReadOnly() || !tt.HasText("AI 正在按章纲起草 · 已写 6 字 · deepseek/deepseek-flash · High") || !tt.HasText("停止") {
		t.Fatal("not read-only while drafting", tt.Texts())
	}
	tt.Type("不该进去")
	close(f.hold)
	until(t, tt, "the draft to end", func() bool { return a.write.run == nil })
	ed := a.write.ed
	if ed.Doc.String() != "" || proposed(a) != "雨下了一夜。\n她醒了。" || !ed.ReadOnly() || !tt.HasText("起草 10 字，绿色为起草。") {
		t.Fatalf("drafted %q beside %q", proposed(a), ed.Doc.String())
	}
	tt.Key(ui.Cmd, ui.KeyS)
	if !strings.Contains(a.notice, "已保存") {
		t.Fatalf("saving waits for the draft: %q", a.notice)
	}
	if snaps, _ := a.Store.Snapshots(id); len(snaps) > 0 && snaps[0].Reason == workspace.SnapAI {
		t.Fatal("a snapshot before the draft is taken")
	}
	if err := tt.Click("采纳"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ed.Doc.String() != "雨下了一夜。\n她醒了。" || ed.ReadOnly() || !strings.Contains(a.notice, "已采纳起草") || !tt.Focused("正文") {
		t.Fatalf("taken %q %q, the prose focused %v", ed.Doc.String(), a.notice, tt.Focused("正文"))
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
	tt.Key(ui.Cmd, ui.KeyZ)
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
		if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
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

// Stopped, a draft keeps what it wrote, after the text there was, for the
// author to take or not; with nothing written, it is as if there was no
// draft. Drafting again starts it over; failing keeps what came. The
// buttons give the keyboard back to the prose.
func TestStopADraftKeepsWhatCame(t *testing.T) {
	a, tt, _ := writingFixture(t, "她醒了")
	f := &stream{pieces: []string{"雨还在下。", "不该出现。"}, hold: make(chan struct{})}
	penWith(a, f)
	a.write.outline = "她醒来。"
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.draftMode != draftAppend {
		t.Fatal("drafting does not append by default")
	}
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the first piece", func() bool { return proposed(a) == "\n雨还在下。" })
	if err := tt.Click("停止"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to stop", func() bool { return a.write.run == nil })
	tt.Frame()
	if got := a.write.ed.Doc.String(); got != "她醒了" || proposed(a) != "\n雨还在下。" || !tt.HasText("已停止，起草 5 字，绿色为起草。") || !tt.Focused("正文") {
		t.Fatalf("after stop %q beside %q, the prose focused %v", proposed(a), got, tt.Focused("正文"))
	}
	if a.Error != "" {
		t.Fatal(a.Error)
	}

	close(f.hold)
	if err := tt.Click("重新起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft again", func() bool { return a.write.run == nil && a.write.proposed != nil })
	tt.Frame()
	if proposed(a) != "\n雨还在下。不该出现。" || !tt.HasText("起草 10 字，绿色为起草。") {
		t.Fatalf("again %q", proposed(a))
	}
	if err := tt.Click("放弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, _, _, ok := a.write.ed.Proposal(); ok || a.write.ed.ReadOnly() || a.write.ed.Doc.String() != "她醒了" || tt.HasText("采纳") || !tt.Focused("正文") {
		t.Fatal("the draft was not let go", tt.Texts())
	}

	f.pieces, f.hold = []string{"", "不该出现。"}, make(chan struct{})
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to be asked", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.asked) == 3 })
	if err := tt.Click("停止"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to stop", func() bool { return a.write.run == nil })
	tt.Frame()
	if _, _, _, ok := a.write.ed.Proposal(); ok || a.write.proposed != nil || a.write.ed.ReadOnly() || tt.HasText("已停止") || a.Error != "" {
		t.Fatal("a draft that wrote nothing stays", tt.Texts())
	}

	f.pieces, f.hold, f.fail = []string{"雨还在下。"}, nil, errors.New("服务不可用")
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to fail", func() bool { return a.write.run == nil })
	tt.Frame()
	if proposed(a) != "\n雨还在下。" || a.Error != "起草没有写完：服务不可用；已写的部分可以采纳" || !tt.HasText("已停止，起草 5 字") {
		t.Fatalf("failed %q %q", proposed(a), a.Error)
	}
}

// A rewrite is shown beside the words selected; dropping it changes
// nothing, accepting it keeps the words replaced as a snapshot. Asked for,
// taken, or closed, it gives the keyboard back to the prose; dropped, to
// the instruction.
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
	if _, _, text, ok := a.write.ed.Proposal(); !ok || text != "她慢慢走到井边。" || !tt.Focused("正文") {
		t.Fatalf("proposal %q, the prose focused %v", text, tt.Focused("正文"))
	}
	if err := tt.Click("放弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.ed.Doc.String() != "她走到井边。井里有光。" || len(a.write.ed.Doc.Marks()) != 0 || a.write.ed.ReadOnly() {
		t.Fatal("dropping changed the text")
	}
	if !tt.Focused("改写要求") {
		t.Fatal("dropped, the instruction is not where the keyboard is")
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
	if !tt.Focused("正文") {
		t.Fatal("taken, the keyboard is not in the prose")
	}
	snaps, _ := a.Store.Snapshots(id)
	if len(snaps) != len(before)+1 || snaps[0].AI == nil || snaps[0].AI.Op != workspace.AIRewrite || snaps[0].AI.Instruction != "放慢" {
		t.Fatalf("snapshots %+v", snaps)
	}
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "她走到井边。井里有光。" {
		t.Fatalf("snapshot text %q", text)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if ed.Doc.String() != "她走到井边。井里有光。" {
		t.Fatalf("one undo left %q", ed.Doc.String())
	}
	ed.Select(0, 6)
	tt.Frame()
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.write.rewriting || !tt.Focused("正文") {
		t.Fatalf("Escape: rewriting %v, the prose focused %v", a.write.rewriting, tt.Focused("正文"))
	}
	if err := clickMenu(tt, "文件", "菜单-快照"); err != nil {
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

// The writer the editors predict with is made anew when a service is saved
// again: its address or its key may be another.
func TestSavingAServiceRemakesThePredictingWriter(t *testing.T) {
	a, tt, _ := writingFixture(t, "原来的。")
	made := 0
	aiOf(a).NewWriter = func(s *workspace.Store) (*writing.Writer, error) {
		made++
		return nil, writing.ErrNoModel
	}
	a.write.penKey = ""
	tt.Frame()
	tt.Frame()
	if made != 1 {
		t.Fatalf("made %d times before anything changed", made)
	}
	a.provider = aiport.Provider{ID: "local", Name: "本机", Protocol: "chat", BaseURL: "http://127.0.0.1:1234/v1"}
	a.limits = []limitRow{{id: "m1", window: "32K", output: "4096", at: "80"}}
	if err := a.saveProvider(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	was := made
	// The same service saved again, with another address.
	a.provider.BaseURL = "http://127.0.0.1:4321/v1"
	if err := a.saveProvider(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if made != was+1 {
		t.Fatalf("made %d times after the service was saved again, %d before", made, was)
	}
}

// Taking the disk's text while a rewrite is under way ends the rewrite:
// what it would propose is about the text given up.
func TestTakingTheDisksTextEndsTheRewrite(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。井里有光。")
	f := &stream{pieces: []string{"她慢慢", "走到井边。"}, hold: make(chan struct{})}
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
	until(t, tt, "the rewrite to start", func() bool { return a.write.run != nil })
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.lookAtText()
	if a.write.external == nil {
		t.Fatal("the outside change was not seen")
	}
	a.takeDisk()
	close(f.hold)
	for range 20 {
		time.Sleep(5 * time.Millisecond)
		tt.Frame()
	}
	if _, _, _, ok := a.write.ed.Proposal(); ok || a.write.proposed != nil || a.write.run != nil {
		t.Fatal("the rewrite went on over the text taken from the disk")
	}
	if got := a.write.ed.Doc.String(); got != "别处改的。" {
		t.Fatalf("the editor holds %q", got)
	}
}
