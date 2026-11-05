package desk

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"testing"
	"time"
	"writer/ext"
	"writer/ext/draft/drafttext"
	"writer/ext/hostapi"
	"writer/internal/testshot"
)

func TestOutlineGlobalAndBookSwitchRemoveEverySurfaceSaveDraftAndRestore(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[book], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			a.book.Timing.SaveAfter = time.Hour
			id := a.write.node
			setChapterOutline(a, "关闭前尚未存盘")
			a.page = pagePlan
			a.setExtension("outline", false, book)
			tt.Frame()
			if a.page != pageWrite {
				t.Fatal("missing page did not return to writing")
			}
			for _, label := range []string{"大纲", "章纲", "章纲为空"} {
				if _, ok := tt.Find(label); ok {
					t.Fatal("disabled surface", label)
				}
			}
			if slices.Contains(a.commandList(), draftCommand(a)) || draftCommand(a).present(a) {
				t.Fatal("draft command survived")
			}
			click(t, tt, "编辑")
			if tt.HasText(drafttext.CmdDraft()) {
				t.Fatal("draft menu survived")
			}
			tt.Key(0, ui.KeyEscape)
			if hasOwner(ext.Items(a.extensions, hostapi.Overlays), "outline") {
				t.Fatal("outline dialogs survived")
			}
			ch, err := a.Store.ReadChapter(id)
			if err != nil || ch.Outline != "关闭前尚未存盘" {
				t.Fatal(ch, err)
			}
			a.setExtension("outline", true, book)
			tt.Frame()
			if !slices.Contains(a.commandList(), draftCommand(a)) || chapterOutline(a) != ch.Outline {
				t.Fatal("outline did not return")
			}
			find(t, tt, "大纲")
		})
	}
}

func TestContextOffCascadesToOutlineAndOutlineOnEnablesContext(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	a.setExtension("context", false, false)
	tt.Frame()
	if a.extensions.Ledger("outline").State != ext.Off || a.hasOutlines() {
		t.Fatal("outline survived dependency")
	}
	a.setExtension("outline", true, false)
	tt.Frame()
	if a.extensions.Ledger("context").State != ext.On || !a.hasOutlines() {
		t.Fatal("required library not enabled")
	}
}

func TestOutlineOpensFreshDiskOutlineWithoutHostPayload(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	id, err := a.book.NewChapter("", "第二章", false)
	requireExtension(t, err)
	requireExtension(t, a.Store.WriteOutline(id, "缓存以外的新章纲"))
	a.openNode(id)
	if got := chapterOutline(a); got != "缓存以外的新章纲" {
		t.Fatal("outline did not read its own chapter", got)
	}
}

func TestDiscussionUsesUnsavedOutlineAndOmitsItWhenOutlineIsOff(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	talkOf(a).Attach = true
	a.book.Timing.SaveAfter = time.Hour
	setChapterOutline(a, "未存盘的章纲")
	ch, err := a.Store.ReadChapter(a.write.node)
	if err != nil || ch.Outline == "未存盘的章纲" {
		t.Fatal("draft already saved", err)
	}
	if got := talkOf(a).Beside(); got == nil || got.Outline != "未存盘的章纲" {
		t.Fatal(got)
	}
	a.setExtension("outline", false, false)
	if got := talkOf(a).Beside(); got == nil || got.Outline != "" || got.VolumeOutline != "" {
		t.Fatal(got)
	}
}

func TestDraftDisappearsWithoutProviderAndEmptyOutlineStaysDisabledWithHint(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	setChapterOutline(a, "")
	if !slices.Contains(a.commandList(), draftCommand(a)) || draftCommand(a).available(a) || draftCommand(a).menuHint(a) != drafttext.CmdDraftNeedsOutline() {
		t.Fatal("empty outline lost disabled command or hint")
	}
	click(t, tt, "编辑")
	hasText(t, tt, drafttext.CmdDraft())
	hasText(t, tt, drafttext.CmdDraftNeedsOutline())
	a.setExtension("outline", false, false)
	if slices.Contains(a.commandList(), draftCommand(a)) || draftCommand(a).available(a) {
		t.Fatal("missing provider kept command")
	}
}

func TestContextAndOutlineBookRestartIsCheapAndClearsSessionState(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	outlinePage(a).Pane = true
	setChapterOutline(a, "跨书不得保留")
	start := time.Now()
	for range 50 {
		a.extensions.Close()
		requireExtension(t, a.extensions.Open(ext.Book{Dir: a.Store.Root()}))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatal("book restart too expensive", elapsed)
	}
	if outlinePage(a).Pane || chapterOutline(a) != "跨书不得保留" {
		t.Fatal("session not reset or draft not saved")
	}
	next := extensionBook(t)
	requireExtension(t, a.Open(next))
	if outlinePage(a).Pane {
		t.Fatal("new book inherited pane state")
	}
	if text, _ := a.outlineValue(""); text == "跨书不得保留" {
		t.Fatal("new book inherited draft")
	}
}

func TestContextSuppliesUnsavedVolumeOutlineFromOutlinePage(t *testing.T) {
	a, tt, f := outlineFixture(t)
	click(t, tt, "卷纲-"+f.volume)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("尚未保存的卷纲")
	text, present := a.outlineValue(f.volume)
	if !present || text != "尚未保存的卷纲" {
		t.Fatal(text, present)
	}
	ch, err := a.Store.ReadChapter(f.volume)
	if err != nil || ch.Outline == text {
		t.Fatal("volume draft already saved", err)
	}
	a.setExtension("outline", false, false)
	ch, err = a.Store.ReadChapter(f.volume)
	if err != nil || ch.Outline != text {
		t.Fatal("volume draft not flushed on stop", ch, err)
	}
}

func TestShotOutlineExtension(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "雨一直没有停。")
	setChapterOutline(a, "林晚沿着路灯寻找旧城的秘密。")
	a.openPrefs("extensions")
	shot(t, tt, "ext-outline-overview.png")
	click(t, tt, "扩展-outline")
	shot(t, tt, "ext-outline-details.png")
	a.closePrefs()
	a.setExtension("outline", false, false)
	shot(t, tt, "ext-outline-off-writing.png")
}
