package desk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/aiport"
	"writer/editor"
	"writer/inference"
	"writer/locales"
	"writer/model"
	"writer/workspace"
)

func TestServiceModel(t *testing.T) {
	a := newApp(filepath.Join(t.TempDir(), "providers.yaml"))
	defer a.Close()
	for name, want := range map[string]string{
		"deepseek/deepseek-flash":        "DeepSeek/deepseek-flash",
		"deepseek/vendor/deepseek-flash": "DeepSeek/vendor/deepseek-flash",
		"missing/vendor/id":              "missing/vendor/id",
		"deepseek/":                      "DeepSeek/", "/id": "/id", "bare": "bare", "": "",
	} {
		if got := a.serviceModel(name); got != want {
			t.Fatal(name, got)
		}
	}
}

func TestDraftNoteKeepsItsOutlineSource(t *testing.T) {
	for _, inVolume := range []bool{false, true} {
		for _, previous := range []string{"", " \n\t", "前章正文。"} {
			t.Run(fmt.Sprint(inVolume, previous), func(t *testing.T) {
				a, _, one := writingFixture(t, previous)
				parent, outlines := "", "各卷卷纲"
				if inVolume {
					var err error
					parent, err = a.Store.NewChapter("", "第一卷", true)
					if err != nil {
						t.Fatal(err)
					}
					if err := a.Store.MoveChapter(one, parent, 0); err != nil {
						t.Fatal(err)
					}
					outlines = "本卷各章章纲"
				}
				two, err := a.Store.NewChapter(parent, "第二章", false)
				if err != nil {
					t.Fatal(err)
				}
				if err := a.loadTree(); err != nil {
					t.Fatal(err)
				}
				a.openNode(two)
				a.openDraft()
				prefix := "将带上" + outlines + "等，约 "
				if workspace.Count(previous) > 0 {
					prefix = "将带上上一章全文、" + outlines + "等，约 "
				}
				if !strings.HasPrefix(a.write.draftNote, prefix) || !strings.HasSuffix(a.write.draftNote, " token；AI 需要时会自己再查。") {
					t.Fatal(a.write.draftNote)
				}
			})
		}
	}
}

type failedPreload struct{ aiport.Service }

func (f failedPreload) Preloaded(*workspace.Store, string, string) (aiport.Preload, error) {
	return aiport.Preload{}, errors.New("preload failed")
}

func TestDraftCardsSurvivePreloadFailure(t *testing.T) {
	a, tt, id := writingFixture(t, "")
	if _, err := a.Store.SaveCard(workspace.Card{Title: "拧开灯座", Chapter: id}, ""); err != nil {
		t.Fatal(err)
	}
	a.ai = failedPreload{a.ai}
	a.openDraft()
	tt.Frame()
	head := "AI 仅将章纲和本章的卡片扩写为正文，不添加这些之外的重大情节。起草在一段新的讨论里进行：写好的先在正文里显示为改稿建议，采纳后才写入，写入前保存历史版本；想改哪里，在讨论里接着说。"
	if a.write.draftCards != 1 || a.write.draftNote != "" || !tt.HasText(head) {
		t.Fatal(a.write.draftCards, a.write.draftNote, tt.Texts())
	}
}

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

// script is a talk's model: each request takes the next of its steps,
// a reply, or a change proposed in the prose, given as the pieces its
// input streams in. With hold, a proposal waits before its last piece
// until let through or stopped.
type script struct {
	mu    sync.Mutex
	steps [][]string
	hold  chan struct{}
	asked []model.Request
}

// calls numbers the calls of every script, as a provider does.
var calls atomic.Int64

// then adds steps to f: each a reply, or the pieces of a proposal.
func (f *script) then(steps ...[]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps = append(f.steps, steps...)
}

// holding has the proposals of f wait before their last piece.
func (f *script) holding() chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hold = make(chan struct{})
	return f.hold
}

// last is the last request f was sent.
func (f *script) last() model.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[len(f.asked)-1]
}

func (f *script) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	if len(f.steps) == 0 {
		f.mu.Unlock()
		return model.Reply{}, errors.New("unexpected model call")
	}
	step, hold := f.steps[0], f.hold
	f.steps = f.steps[1:]
	f.mu.Unlock()
	if !strings.HasPrefix(step[0], "{") {
		on(model.Delta{Text: step[0]})
		return model.Reply{Text: step[0]}, nil
	}
	id := fmt.Sprintf("call-%d", calls.Add(1))
	for i, p := range step {
		if i == len(step)-1 && hold != nil {
			select {
			case <-hold:
			case <-ctx.Done():
				return model.Reply{}, ctx.Err()
			}
		}
		name := ""
		if i == 0 {
			name = workspace.ProposeTool
		}
		on(model.Delta{Call: model.CallDelta{ID: id, Name: name, Input: p}})
	}
	return model.Reply{Calls: []model.Call{{ID: id, Name: workspace.ProposeTool, Input: json.RawMessage(strings.Join(step, ""))}}}, nil
}

// proposal is a change proposed in chapter at where, in place of or after
// quote, as the pieces of its input: the text comes in two.
func proposal(chapter, at, quote, text string) []string {
	r := []rune(text)
	head := fmt.Sprintf(`{"chapter":%q,"at":%q,"quote":%q,"text":"`, chapter, at, quote)
	return []string{head + string(r[:len(r)/2]), strings.ReplaceAll(string(r[len(r)/2:]), "\n", `\n`) + `"}`}
}

// penWith has the work talk with f, and predict and check with it.
func penWith(a *App, f model.Completer) { penWithin(a, f, model.Builtin("deepseek-flash")) }

// penWithin is penWith with a model of the limits m.
func penWithin(a *App, f model.Completer, m model.Model) {
	w := a.Store.Work()
	w.Discussion = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash", Effort: string(model.EffortHigh)}
	if err := a.Store.SaveWork(w); err != nil {
		panic(err)
	}
	a.loadWork()
	aiOf(a).Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		return inference.Model{Model: m, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}, nil
	}
	a.write.penKey = nil // build the editors' writer again
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

// replied waits for the turn in flight to end.
func replied(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	until(t, tt, "the reply", func() bool { return a.busy == "" })
	tt.Frame()
}

// shownProposals are the texts the editor shows proposed.
func shownProposals(a *App) []string {
	var out []string
	for _, p := range shownEditor(a).Proposals() {
		out = append(out, p.Text)
	}
	return out
}

// Drafting needs an outline. The draft is a talk opened beside the prose,
// its first words what the author asked in the dialog, keeping to the
// chapter. Its AI proposes the draft in the prose, as it comes, while the
// text stays as it was, on the disk too. Taken, after a snapshot naming
// the talk, it goes in as one step to undo, marked as the AI's, the
// keyboard in the prose; a click as it came does not take it.
func TestDraftAChapterThroughATalk(t *testing.T) {
	a, tt, id := writingFixture(t, "")
	f := &script{}
	penWith(a, f)
	hold := f.holding()
	f.then(proposal("第一章", "end", "", "雨下了一夜。\n她醒了。"), []string{"按章纲写了两段。"})
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.write.draftOpen {
		t.Fatal("drafting opened with no outline")
	}
	setChapterOutline(a, "她在雨夜醒来。")
	tt.Frame()
	if err := tt.Click("起草本章"); err != nil { // the menu stayed open
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("用讨论模型 deepseek-flash · High；可在设置中「AI」配置里更换。") || !strings.HasPrefix(a.write.draftNote, "将带上各卷卷纲等，约 ") {
		t.Fatal("the dialog does not say what goes and to whom", a.write.draftNote, tt.Texts())
	}
	// Words not a count are said under the field, and drafting waits.
	a.write.words = "很多"
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("目标字数须为正整数，或留空") || a.busy != "" || a.errorText() != "" {
		t.Fatal("drafted with words not a count", a.errorText(), tt.Texts())
	}
	a.write.words = "800"
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the draft to come", func() bool { return slices.Equal(shownProposals(a), []string{"雨下了一夜"}) })
	ed := shownEditor(a)
	if ed.Doc.String() != "" || disk(t, a, "第一章.txt") != "" || ed.ReadOnly() || !tt.HasText("AI 正在写改稿建议…") || !tt.HasText("正在写…") {
		t.Fatal("the draft is not shown beside the text as it comes", tt.Texts())
	}
	if shownTab(a) != paneTalk || a.doc.hidden || talkNameOf(a) != "起草：第一章" || !tt.HasText("按章纲起草本章，约 800 字") || !tt.HasText("跟着「第一章」") {
		t.Fatal("the talk is not beside the prose", talkNameOf(a), tt.Texts())
	}
	if !strings.Contains(f.last().System, "【这段讨论起草的章：第一章】") || !strings.Contains(f.last().System, "她在雨夜醒来。") {
		t.Fatal("the talk does not keep to the chapter")
	}
	// As it comes there is no 采纳, and 全部采纳 is grey; neither 放弃
	// moves once it is done, so a click meant for one as it ends is not on
	// a 采纳.
	pid := a.write.props[0].id()
	after := func() float32 { // where 放弃 is after the proposal's end
		r, _ := tt.Find("放弃")
		g, _ := ed.ProposalGap(pid)
		return r.X - g.X
	}
	letGo := after()
	letAllGo, ok := tt.Find("全部放弃")
	if !ok || tt.Click("采纳") == nil {
		t.Fatal("the buttons as it comes", tt.Texts())
	}
	if err := tt.Click("全部采纳"); err == nil {
		tt.Frame()
	}
	if ed.Doc.String() != "" {
		t.Fatal("taken by a click as it came")
	}
	close(hold)
	replied(t, a, tt)
	if x := after(); x != letGo {
		t.Fatalf("放弃 moved from %v after the proposal to %v", letGo, x)
	}
	if r, _ := tt.Find("全部放弃"); r != letAllGo {
		t.Fatalf("全部放弃 moved from %v to %v", letAllGo, r)
	}
	if !slices.Equal(shownProposals(a), []string{"雨下了一夜。\n她醒了。"}) || !tt.HasText("本章有 1 处改稿建议") || !tt.HasText("接在章末 · 10 字 · 等你决定") {
		t.Fatal("the draft did not come whole", shownProposals(a), tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyS)
	if snaps, _ := a.Store.Snapshots(id); len(snaps) > 0 && snaps[0].Reason == workspace.SnapAI {
		t.Fatal("a snapshot before the draft is taken")
	}
	if err := tt.Click("采纳"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ed.Doc.String() != "雨下了一夜。\n她醒了。" || !strings.Contains(noticeOf(a), "已采纳改稿建议") || !tt.Focused("正文") || ed.Caret() != ed.Doc.Len() {
		t.Fatalf("taken %q %q, the prose focused %v", ed.Doc.String(), noticeOf(a), tt.Focused("正文"))
	}
	if ms := ed.Doc.Marks(); len(ms) != 1 || ms[0].Kind != editor.MarkAI || ms[0].End != ed.Doc.Len() || ms[0].Note != "deepseek/deepseek-flash" {
		t.Fatalf("marks %+v", ms)
	}
	if got := disk(t, a, "第一章.txt"); got != "雨下了一夜。\n她醒了。" {
		t.Fatalf("on disk %q", got)
	}
	if len(ed.Proposals()) != 0 || !tt.HasText("接在章末 · 10 字 · 已采纳") {
		t.Fatal("the talk does not say it was taken", tt.Texts())
	}
	snaps, _ := a.Store.Snapshots(id)
	if s := snaps[0]; s.Reason != workspace.SnapAI || !slices.Equal(s.Talks, []string{a.session}) {
		t.Fatalf("snapshot %+v", s)
	}
	if text, _ := a.Store.SnapshotText(id, snaps[0].Name); text != "" {
		t.Fatalf("snapshot text %q", text)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if ed.Doc.String() != "" {
		t.Fatalf("one undo left %q", ed.Doc.String())
	}
	if err := clickMenu(tt, "文件", "菜单-历史版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("采纳讨论「起草：第一章」里的建议") {
		t.Fatal("the snapshot does not name the talk", tt.Texts())
	}
}

// What becomes of the words already there is chosen in the dialog, asked
// afresh each time it opens, and said in the talk's first words.
func TestDraftOverWhatIsThere(t *testing.T) {
	a, tt, _ := writingFixture(t, "她醒了")
	f := &script{}
	penWith(a, f)
	setChapterOutline(a, "她醒来。")
	tt.Frame()
	for _, want := range []string{draftReplace, draftAppend} {
		if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if want == draftReplace {
			if err := tt.Click(locales.DraftOverwrite()); err != nil {
				t.Fatal(err, tt.Texts())
			}
			tt.Frame()
		}
		tt.Frame()
		if a.write.draftMode != want {
			t.Fatalf("mode %q, want %q", a.write.draftMode, want)
		}
		if want == draftAppend {
			break
		}
		a.write.draftOpen = false
		tt.Frame()
	}
	f.then([]string{"先问一句：她是怎么醒的？"})
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	if !tt.HasText("按章纲起草本章，接在已有内容之后") || !strings.Contains(f.last().System, "【本章已有的正文】\n她醒了") {
		t.Fatal("the draft does not go on from what is there", tt.Texts())
	}
}

// A rewrite is a talk opened on the words selected, quoting them in its
// first words with what the author asked. Its proposal strikes the words
// through with the new ones after them, and the author writes on
// elsewhere. Asked again in the talk, the AI's new proposal takes the old
// one's place, which the talk says; let go, the text is as it was, and the
// AI is told the next time the author speaks. The box that asks gives the
// keyboard back to the prose, closed by Escape.
func TestRewriteASelectionThroughATalk(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。井里有光。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "replace", "她走到井边。", "她慢慢走到井边。"), []string{"放慢了。"})
	ed := shownEditor(a)
	ed.Select(0, 6)
	tt.Frame()
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.busy != "" {
		t.Fatal("a rewrite went with nothing asked")
	}
	a.write.instruction = "放慢"
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	ps := ed.Proposals()
	if len(ps) != 1 || ps[0].A != 0 || ps[0].B != 6 || ps[0].Keep || ps[0].Text != "她慢慢走到井边。" || !tt.Focused("正文") {
		t.Fatalf("proposed %+v, the prose focused %v", ps, tt.Focused("正文"))
	}
	if a.write.rewriting || talkNameOf(a) != "改写：她走到井边。" || !tt.HasText("换掉一句 · 8 字 · 等你决定") || !tt.HasText("放慢") {
		t.Fatal("the talk is not of the rewrite", talkNameOf(a), tt.Texts())
	}
	if first := a.events[0]; first.Origin != workspace.AIRewrite || len(first.Quotes) != 1 || first.Quotes[0].Text != "她走到井边。" {
		t.Fatalf("first words %+v", first)
	}
	ed.MoveTo(ed.Doc.Len(), false)
	ed.Insert("后来")
	tt.Frame()
	if len(ed.Proposals()) != 1 {
		t.Fatal("writing elsewhere let the proposal go")
	}

	f.then(proposal("第一章", "replace", "她走到井边。", "她缓缓走到井边。"), []string{"再慢一点。"})
	a.input = "再慢一点"
	tt.Frame()
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	if !slices.Equal(shownProposals(a), []string{"她缓缓走到井边。"}) || !tt.HasText("换掉一句 · 8 字 · 已被新的建议替换") {
		t.Fatal("the new proposal did not take the old one's place", shownProposals(a), tt.Texts())
	}
	if err := tt.Click("放弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ed.Doc.String() != "她走到井边。井里有光。后来" || len(ed.Proposals()) != 0 || !tt.HasText("换掉一句 · 8 字 · 已放弃") || !tt.Focused("正文") {
		t.Fatal("letting go changed the text, or is not said", ed.Doc.String(), tt.Texts())
	}
	f.then([]string{"好。"})
	a.input = "算了"
	tt.Frame()
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	if m := f.last().Messages; !strings.Contains(m[len(m)-1].Content, "【之前的提议】「第一章」换掉「她走到井边。」的提议：被作者放弃。") {
		t.Fatalf("the AI is not told: %q", m[len(m)-1].Content)
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
}

// A proposal whose words the author changes is void, which the talk says;
// changing other words leaves it. One let go as it comes stops the AI.
func TestAProposalVoidAndOneLetGo(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。井里有光。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "replace", "井里有光。", "井里亮着。"), []string{"换了。"})
	ed := shownEditor(a)
	ed.Select(6, 11)
	tt.Frame()
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.instruction = "换个说法"
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	ed.MoveTo(0, false)
	ed.Insert("那天")
	tt.Frame()
	if len(ed.Proposals()) != 1 {
		t.Fatal("other words changed let the proposal go")
	}
	ed.Select(10, 12) // 有光
	ed.Insert("亮")
	tt.Frame()
	if len(ed.Proposals()) != 0 || hintOf(a) != "1 处改稿建议已作废；它要换掉的原文改动了" || !tt.HasText("正文已改动，建议已作废") {
		t.Fatal("the proposal is not void", hintOf(a), tt.Texts())
	}

	hold := f.holding()
	f.then(proposal("第一章", "replace", "那天", "那一天"), []string{"不该来。"})
	ed.Select(0, 2)
	tt.Frame()
	if err := tt.Click("AI 改写"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.instruction = "长一点"
	tt.Frame()
	if err := tt.Click("改写"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the proposal to come", func() bool { return len(ed.Proposals()) == 1 })
	if err := tt.Click("放弃"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	close(hold)
	tt.Frame()
	if len(ed.Proposals()) != 0 || len(a.write.props) != 0 || ed.Doc.String() != "那天她走到井边。井里亮。" || !tt.HasText("该消息尚未收到回复") {
		t.Fatal("letting go of a proposal as it came left the AI writing", ed.Doc.String(), tt.Texts())
	}
}

// Leaving the work with proposals not decided on asks first; let go, what
// came of them is in their talk.
func TestLeavingTheWorkAsksAboutProposals(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "end", "", "井里有光。"), []string{"接了一句。"})
	setChapterOutline(a, "井里有光。")
	a.openDraft()
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	session := a.session
	leave := func() {
		t.Helper()
		if err := tt.Click("切换作品"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click("新建作品…"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	leave()
	if !tt.HasText("还有 1 处改稿建议没有处理") || !tt.HasText("关闭后将丢弃。") || a.books.creating {
		t.Fatal("left without asking", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("还有 1 处改稿建议没有处理") || len(a.write.props) != 1 || !tt.HasText("本章有 1 处改稿建议") {
		t.Fatal("staying let the proposal go")
	}
	leave()
	if err := tt.Click("丢弃并关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.books.creating || len(a.write.props) != 0 {
		t.Fatal("did not leave")
	}
	events, _ := a.Store.Events(session)
	if last := events[len(events)-1]; last.Role != workspace.RoleFate || last.Fate != workspace.FateClosed {
		t.Fatalf("the talk does not say: %+v", last)
	}
}

// A talk may propose in a chapter not open: it opens in a tab beside the
// one shown, which stays. The talk's card shows it there.
func TestAProposalInAnotherChapterOpensBeside(t *testing.T) {
	a, tt, one := writingFixture(t, "第一章的字。")
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(two, "档案室白天也要开灯。", nil); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	f := &script{}
	penWith(a, f)
	f.then(proposal("第二章", "replace", "白天也要开灯。", "白天也得开着灯。"), []string{"顺了一下。"})
	showTab(a, paneTalk)
	a.doc.hidden = false
	a.input = "第二章那句顺一下"
	tt.Frame()
	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	replied(t, a, tt)
	if shownChapter(a) != one || !slices.Contains(openTabs(a), two) || !tt.HasText("建议修改「第二章」") || !tt.HasText("换掉一句 · 8 字 · 等你决定") {
		t.Fatal("the chapter is not open beside", openTabs(a), tt.Texts())
	}
	if err := tt.Click("在正文中查看"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if shownChapter(a) != two || !tt.HasText("本章有 1 处改稿建议") || !slices.Equal(shownProposals(a), []string{"白天也得开着灯。"}) {
		t.Fatal("the proposal is not shown", tt.Texts())
	}
	if err := tt.Click("全部放弃"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("换掉一句 · 8 字 · 已放弃") || tt.HasText("本章有 1 处改稿建议") {
		t.Fatal("letting go is not said", tt.Texts())
	}
}

// Proposals of two talks taken at once go in after one snapshot, which
// names both talks.
func TestTakingTheProposalsOfTwoTalks(t *testing.T) {
	a, tt, id := writingFixture(t, "她走到井边。井里有光。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "replace", "她走到井边。", "她慢慢走到井边。"), []string{"慢了一点。"},
		proposal("第一章", "end", "", "雨停了。"), []string{"接了一句。"})
	showTab(a, paneTalk)
	a.doc.hidden = false
	var talks []string
	for _, said := range []string{"开头慢一点", "末尾接一句"} {
		a.newSession()
		a.input = said
		tt.Frame()
		if err := tt.Click("发送"); err != nil {
			t.Fatal(err)
		}
		replied(t, a, tt)
		talks = append(talks, a.session)
	}
	showPage(a, pageWrite)
	tt.Frame()
	if talks[0] == talks[1] || !tt.HasText("本章有 2 处改稿建议") {
		t.Fatal("two talks did not each propose", talks, tt.Texts())
	}
	if err := tt.Click("全部采纳"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := shownEditor(a).Doc.String(); got != "她慢慢走到井边。井里有光。\n雨停了。" {
		t.Fatalf("taken %q", got)
	}
	snaps, _ := a.Store.Snapshots(id)
	if s := snaps[0]; s.Reason != workspace.SnapAI || !slices.Equal(s.Talks, talks) {
		t.Fatalf("snapshot %+v, talks %v", s, talks)
	}
	if err := clickMenu(tt, "文件", "菜单-历史版本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("采纳讨论「开头慢一点」「末尾接一句」里的建议") {
		t.Fatal("the snapshot does not name both talks", tt.Texts())
	}
}

// Asked to draft with no model to talk with, or while another reply is
// in flight, the author is told why nothing happens.
func TestADraftWaitsForAModelAndItsTurn(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	setChapterOutline(a, "她醒来。")
	a.openDraft()
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.errorText() != "请先配置讨论模型" || !a.write.draftOpen {
		t.Fatalf("drafted with no model: %q", a.errorText())
	}
	a.SetError(nil)
	penWith(a, &script{})
	a.busy = "AI 正在思考…"
	tt.Frame()
	if err := tt.Click("开始起草"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if hintOf(a) != "AI 正在回复别的讨论；等它回完或取消后再起草" || !a.write.draftOpen {
		t.Fatalf("drafted over another reply: %q", hintOf(a))
	}
	a.busy = ""
}

// Cmd+J asks for one sentence; Tab keeps it, marked with the model.
func TestContinueOnDemand(t *testing.T) {
	a, tt, _ := writingFixture(t, "她推开门")
	f := &stream{pieces: []string{"，屋里", "很暗。她", "停住了。"}}
	penWith(a, f)
	ed := shownEditor(a)
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
	if ms := ed.Doc.Marks(); ed.Doc.String() != "她推开门，屋里很暗。" || len(ms) != 1 || ms[0].Kind != editor.MarkAI || ms[0].Note != "deepseek/deepseek-flash" {
		t.Fatalf("accepted %q %+v", ed.Doc.String(), ms)
	}
}

// The writer the editors predict with is made anew when a service is saved
// again: its address or its key may be another.
func TestSavingAServiceRemakesThePredictingWriter(t *testing.T) {
	a, tt, _ := writingFixture(t, "原来的。")
	penWith(a, &stream{})
	made := 0
	aiOf(a).Open = func(workspace.Selection, *workspace.Store) (inference.Model, error) {
		made++
		return inference.Model{}, errors.New("没有模型")
	}
	a.write.penKey = nil
	tt.Frame()
	tt.Frame()
	if made != 1 {
		t.Fatalf("made %d times before anything changed", made)
	}
	a.serviceForm.provider = aiport.Provider{ID: "local", Name: "本机", Protocol: "chat", BaseURL: "http://127.0.0.1:1234/v1"}
	a.serviceForm.limits = []limitRow{{id: "m1", window: "32K", output: "4096", at: "80"}}
	if err := a.saveProvider(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	was := made
	// The same service saved again, with another address.
	a.serviceForm.provider.BaseURL = "http://127.0.0.1:4321/v1"
	if err := a.saveProvider(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if made != was+1 {
		t.Fatalf("made %d times after the service was saved again, %d before", made, was)
	}
}

// Taking the disk's text while a proposal is shown leaves the proposal to
// the disk's text: there, its words are gone, and it is void.
func TestTakingTheDisksTextVoidsWhatItChanged(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。井里有光。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "replace", "她走到井边。", "她慢慢走到井边。"), []string{"放慢了。"})
	shownEditor(a).Select(0, 6)
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
	replied(t, a, tt)
	p := filepath.Join(a.Store.Root(), "chapters", "第一章.txt")
	if err := os.WriteFile(p, []byte("别处改的。"), 0644); err != nil {
		t.Fatal(err)
	}
	a.lookAtText()
	if !diskChanged(a) {
		t.Fatal("the outside change was not seen")
	}
	a.takeDisk()
	tt.Frame()
	if len(shownEditor(a).Proposals()) != 0 || len(a.write.props) != 0 || shownEditor(a).Doc.String() != "别处改的。" {
		t.Fatalf("the proposal stayed over the text taken from the disk: %q", shownEditor(a).Doc.String())
	}
}

func TestKeepPenBuildsOnlyInChangedFrames(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	penWith(a, &stream{})
	tt.Frame()
	s := &spy{Service: a.ai}
	a.ai = s
	frames := func(want int) {
		t.Helper()
		tt.Frame()
		tt.Frame()
		if got := len(s.heard()); got != want {
			t.Fatalf("built %d times, want %d: %v", got, want, s.heard())
		}
	}
	frames(0)
	a.modelsChanged()
	if len(s.heard()) != 0 {
		t.Fatal("invalidation built the pen")
	}
	frames(1)
	w := a.Store.Work()
	for i, change := range []func(){
		func() { w.Writing = workspace.Selection{Provider: "deepseek", Model: "deepseek-flash"} },
		func() { w.Writing.Model = "other" },
		func() { w.Writing.Provider = "other" },
		func() { w.Writing.Effort = "high" },
		func() { w.Discussion.Model = "other" },
		func() { w.Discussion.Provider = "other" },
		func() { w.Discussion.Effort = "low" },
	} {
		change()
		if err := a.Store.SaveWork(w); err != nil {
			t.Fatal(err)
		}
		frames(i + 2)
	}
	if err := s.Service.SaveProvider(aiport.Provider{ID: "local", Name: "Local", BaseURL: "http://127.0.0.1:1/v1", Protocol: "chat"}); err != nil {
		t.Fatal(err)
	}
	frames(9)
	a.shelf.NoAI = true
	frames(9)
	if a.write.pen.Load() != nil || a.write.penErr != nil {
		t.Fatal("the disabled pen survived")
	}
	a.shelf.NoAI = false
	frames(10)
}

func TestKeepPenBuildsAnEmptyConfigurationOnce(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	s := &spy{Service: ai.New("")}
	a.ai = s
	a.modelsChanged()
	tt.Frame()
	tt.Frame()
	if len(s.heard()) != 1 || a.write.pen.Load() != nil || a.write.penErr == nil {
		t.Fatal(s.heard(), a.write.penErr)
	}
	a.modelsChanged()
	if len(s.heard()) != 1 {
		t.Fatal("invalidation rebuilt the failed pen")
	}
	tt.Frame()
	tt.Frame()
	if len(s.heard()) != 2 {
		t.Fatal(s.heard())
	}
	a.shelf.NoAI = true
	tt.Frame()
	if a.write.penErr != nil || a.write.pen.Load() != nil {
		t.Fatal("the disabled pen survived", a.write.penErr)
	}
	a.shelf.NoAI = false
	tt.Frame()
	tt.Frame()
	if len(s.heard()) != 3 {
		t.Fatal(s.heard())
	}
}
