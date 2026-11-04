package desk

import (
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext/backup/archive"
	"writer/internal/testshot"
	"writer/workspace"
)

// The three rows below the title bar end on the same line. The paper
// and the history move together when either column is put away.
func TestTheRowsShareThePaper(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 80))
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		for _, width := range []int{1100, 1266, 2200} {
			tt.SetSize(width, 780)
			for _, hidden := range []bool{false, true} {
				a.sideHidden, a.doc.hidden = hidden, hidden
				tt.Frame()
				tt.Frame()
				row := find(t, tt, "标签行")
				if row.Y != kit.HBar || row.H != kit.HBar {
					t.Fatalf("tabs are not the second row: %+v", row)
				}
				if !hidden {
					side, pane := find(t, tt, "侧栏标题行"), find(t, tt, "右栏标签行")
					if side.Y+side.H != row.Y+row.H || pane.Y+pane.H != row.Y+row.H {
						t.Fatalf("the three rules do not meet: %+v %+v %+v", side, row, pane)
					}
					if width < narrow && (side.W != 200 || a.LastSetting.W != 320) {
						t.Fatalf("narrow columns: %+v %+v", side, a.LastSetting)
					}
				}
				box, ok := shownEditor(a).RangeBox(0, shownEditor(a).Doc.Len())
				arrow := find(t, tt, "上一个打开的章节图形")
				// The chevron starts at x=9 in its 24-unit SVG viewbox.
				if !ok || math.Abs(float64(arrow.X+arrow.W*9/24-a.LastProse.X-box.X)) > 3 {
					t.Fatalf("%d wide: tabs at %v, prose at %v", width, arrow.X, a.LastProse.X+box.X)
				}
				if box.W > shownEditor(a).FontSize()*36+1 {
					t.Fatalf("prose exceeds 36 characters: %v", box.W)
				}
				// These points contain no controls or words: all are paper,
				// including the title bar's bottom edge.
				img := tt.Image()
				paper := img.At(width-2, 600)
				for _, p := range [][2]int{{300, 39}, {8, 600}} {
					if img.At(p[0], p[1]) != paper {
						t.Fatalf("%v is not the same paper (dark %v)", p, dark)
					}
				}
				if !hidden {
					for _, x := range []int{5, int(row.X + 2), width - 5} {
						if img.At(x, 79) == paper || img.At(x, 80) != paper {
							t.Fatalf("the horizontal rule steps at x=%d (dark %v)", x, dark)
						}
					}
				}
			}
		}
	}
}

// The rows over and under the prose start where it does in the frame
// that lays it out, as on the first frame after a resize: built under a
// fresh key each time, so nothing has bounds from a frame before, the
// tabs are where they settle in the window as the author has it.
func TestTheRowsMoveWithThePaperAtOnce(t *testing.T) {
	a, settled, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 80))
	build := 0
	fresh := stillTester(func(c *ui.Context) {
		ui.Column(c.Key(build)).Fill().Children(func() { a.View(c) })
		build++
	}, 1266, 780)
	for _, width := range []int{1100, 1266, 2200} {
		for _, hidden := range []bool{false, true} {
			a.sideHidden, a.doc.hidden = hidden, hidden
			for _, tt := range []*ui.Tester{settled, fresh} {
				tt.SetSize(width, 780)
				tt.Frame()
			}
			want, got := find(t, settled, "上一个打开的章节图形").X, find(t, fresh, "上一个打开的章节图形").X
			if math.Abs(float64(got-want)) > 0.5 {
				t.Fatalf("%d wide, columns away %v: the tabs start at %v before they settle at %v", width, hidden, got, want)
			}
		}
	}
}

// The two switches stay on the title bar on every page; only the pages
// with a pane act on its switch, and the outline keeps its own choice.
func TestThePaneSwitchStaysOnEveryPage(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	tt.SetTitleBar(ui.TitleBar{Height: kit.HBar, Right: 138})
	tt.Frame()
	place := find(t, tt, "收起右栏")
	for _, page := range []page{pageTalk, pageWrite, pagePlan, pageCheck, pageCategories} {
		a.page = page
		tt.Frame()
		tt.Frame()
		name := cmdPane.name(a)
		if got := find(t, tt, name); got != place {
			t.Fatalf("page %d moved its switch: %+v, was %+v", page, got, place)
		}
		before, doc, plan := a.paneOpen(), a.doc.hidden, outlinePage(a).Pane
		click(t, tt, name)
		if a.hasPane() {
			if a.paneOpen() == before {
				t.Fatalf("page %d did not toggle its pane", page)
			}
			if page == pagePlan && a.doc.hidden != doc || page != pagePlan && outlinePage(a).Pane != plan {
				t.Fatalf("page %d changed the other pane's choice", page)
			}
		} else if a.doc.hidden != doc || outlinePage(a).Pane != plan {
			t.Fatalf("page %d acted on a disabled switch", page)
		}
	}
	a.closeWork()
	tt.Frame()
	click(t, tt, "展开右栏")
	if _, ok := tt.Find("换页"); ok || a.Store != nil {
		t.Fatal("the shelf has page navigation or an active pane switch")
	}
}

// Changing notices never move the find and focus buttons. With no chapter
// find is still there, disabled; the empty tab row keeps the history.
func TestWritingToolsKeepTheirPlaces(t *testing.T) {
	a, tt, _ := writingFixture(t, "井边有一把钥匙。")
	tt.SetTitleBar(ui.TitleBar{Height: kit.HBar, Right: 138})
	tt.Frame()
	search, focus := find(t, tt, "查找替换"), find(t, tt, "专注写作")
	ed := shownEditor(a)
	ed.Select(0, 2)
	ed.MarkSelection(editor.MarkForeshadow, "")
	a.doc.hidden = true
	backupOf(a).Configure(archive.Settings{Dir: t.TempDir()})
	backupOf(a).State.Policy.RecordResult("", os.ErrNotExist, time.Now())
	tt.Frame()
	tt.Frame()
	hasText(t, tt, "本章伏笔 1")
	hasText(t, tt, "备份提示")
	hasText(t, tt, "待处理 1")
	if find(t, tt, "查找替换") != search || find(t, tt, "专注写作") != focus {
		t.Fatal("notices moved the fixed tools")
	}
	click(t, tt, "本章伏笔")
	if a.doc.hidden || a.doc.tab != paneFore {
		t.Fatalf("the chapter's foreshadowings did not open: hidden %v, tab %s, page %d", a.doc.hidden, a.doc.tab, a.page)
	}
	a.shutTabs(append([]string(nil), a.write.tabs...))
	tt.Frame()
	tt.Frame()
	click(t, tt, "查找替换")
	if a.write.finding || find(t, tt, "查找替换") != search {
		t.Fatal("find moved or acted without a chapter")
	}
	find(t, tt, "标签行")
	find(t, tt, "上一个打开的章节")
}

// The sidebar only lists objects; counts and dates appear under the
// pointer. A chapter clicked from another page comes back to writing.
func TestTheSidebarOnlyListsObjects(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	for _, page := range []page{pageWrite, pagePlan, pageCheck, pageCategories} {
		a.page = page
		tt.Frame()
		tt.Frame()
		find(t, tt, "章节-第一章")
		for _, label := range []string{"章节", "回收站", "备份状态", "设置", "新建文件夹"} {
			if r, ok := tt.Find(label); ok && r.X < a.LastWrite.X {
				t.Fatalf("page %d sidebar still has %s", page, label)
			}
		}
		click(t, tt, "章节-第一章")
		if a.page != pageWrite {
			t.Fatalf("page %d did not return to writing", page)
		}
	}
	r := find(t, tt, "章节-第一章")
	shownEditor(a).Focus()
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	if slices.Contains(tt.Texts(), "8") {
		t.Fatal("the chapter count is always visible", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Frame()
	if !slices.Contains(tt.Texts(), "8") {
		t.Fatal("the hovered chapter has no count")
	}
	click(t, tt, "讨论")
	if _, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "修路灯"}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	date := talkDay(a.session, time.Now())
	if slices.Contains(tt.Texts(), date) {
		t.Fatal("the discussion date is always visible")
	}
	r = find(t, tt, "对话-"+a.session)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Frame()
	if !slices.Contains(tt.Texts(), date) {
		t.Fatal("the hovered discussion has no date")
	}
}

func TestShotPaperSkeleton(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下，一直没有停。", 40))
	tt.SetTitleBar(ui.TitleBar{Height: kit.HBar, Right: 138})
	shot(t, tt, "paper-writing.png")
	tt.SetSize(1100, 780)
	shot(t, tt, "paper-narrow.png")
	a.sideHidden = true
	shot(t, tt, "paper-sidebar-away.png")
	a.doc.hidden = true
	shot(t, tt, "paper-both-away.png")
	a.write.focus = true
	shot(t, tt, "paper-focus.png")
	tt.SetDark(true)
	shot(t, tt, "paper-focus-dark.png")
}

// Blank margins belong to the editor, including when the text is centered
// far from the viewport edges. The scroll bar stays at the page's edge.
func TestTheEditorFillsTheWritingArea(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width         int
		hidden, focus bool
	}{
		{"wide", 2200, false, false},
		{"narrow", 1100, false, false},
		{"both columns away", 2200, true, false},
		{"focus", 2200, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下，一直没有停。\n", 200))
			a.sideHidden, a.doc.hidden, a.write.focus = tc.hidden, tc.hidden, tc.focus
			tt.SetSize(tc.width, 780)
			tt.Frame()
			tt.Frame()
			prose := a.LastProse
			if prose.X != a.LastWrite.X || prose.W != a.LastWrite.W {
				t.Fatalf("editor %+v does not fill writing area %+v", prose, a.LastWrite)
			}
			ed := shownEditor(a)
			for _, x := range []float32{prose.X + 2, prose.X + prose.W - 20} {
				before, _, _ := ed.ScrollAnchor()
				tt.Scroll(x, prose.Y+prose.H/2, 0, 100)
				after, _, _ := ed.ScrollAnchor()
				if after <= before {
					t.Fatalf("blank margin at %v did not scroll: %v -> %v", x, before, after)
				}
				tt.ClickAt(x, prose.Y+prose.H/2)
				if !ed.Focused() {
					t.Fatalf("blank margin at %v did not give the caret to the editor", x)
				}
			}
			ed.MoveTo(0, false)
			tt.Frame()
			x, y := prose.X+prose.W-6, prose.Y+12
			tt.Move(x, y)
			tt.Frame()
			img := tt.Image()
			if tt.Cursor() != ui.CursorDefault || img.At(int(x), int(y)) == img.At(int(x)-8, int(y)) {
				t.Fatal("the scroll bar is missing at the writing area's right edge")
			}
			// A short thumb is at the top for this long chapter.
			tt.Press(x, y)
			tt.Move(x, y+80)
			tt.Release(x, y+80)
			if fraction, _, _ := ed.ScrollAnchor(); fraction == 0 {
				t.Fatal("the scroll bar at the writing area's edge did not drag")
			}
		})
	}
}

// A remembered wide pane yields as the window narrows, and can use the
// space released by hiding the sidebar.
func TestThePaneLeavesRoomForThePage(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	for _, width := range []int{1000, 1100, 1266, 1800} {
		for _, hidden := range []bool{false, true} {
			a.sideHidden, a.doc.hidden = hidden, false
			a.doc.width = 600
			tt.SetSize(width, 780)
			tt.Frame()
			tt.Frame()
			if a.LastWrite.W < pageMin || a.LastSetting.W < panelMin || a.LastSetting.W > float32(width)/2 {
				t.Fatalf("%d wide, sidebar hidden %v: page %+v, pane %+v", width, hidden, a.LastWrite, a.LastSetting)
			}
			r := a.LastSetting
			tt.Press(r.X, r.Y+300)
			tt.Move(r.X-200, r.Y+300)
			tt.Release(r.X-200, r.Y+300)
			tt.Frame()
			if a.LastWrite.W < pageMin || a.doc.width > a.panelLimit(float32(width)) {
				t.Fatalf("dragging at %d wide left page %+v and remembered pane %v", width, a.LastWrite, a.doc.width)
			}
		}
	}
	a.sideHidden, a.doc.width = false, 0
	tt.SetSize(1100, 780)
	tt.Frame()
	if a.LastSetting.W != 320 {
		t.Fatalf("the narrow default is %v, want 320", a.LastSetting.W)
	}
}

// Every build gets fresh element keys: a previous width is unavailable,
// as on the first frame and after changing which page names are shown.
func TestThePagesAreCenteredWithoutPreviousBounds(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	build := 0
	tt := stillTester(func(c *ui.Context) {
		a.titleBar(c.Key(build))
		build++
	}, 1266, 80)
	for _, noAI := range []bool{false, true, false} {
		a.shelf.NoAI = noAI
		tt.Frame()
		first, last := "讨论", "检查"
		if noAI {
			first, last = "正文", "大纲"
		}
		left, right := find(t, tt, first), find(t, tt, last)
		if math.Abs(float64((left.X+right.X+right.W)/2-1266/2)) > 1 {
			t.Fatalf("AI off %v: page switch not centered: %+v %+v", noAI, left, right)
		}
	}
	// The real title bar's pass-through row leaves its buttons clickable.
	a, tt, _ = writingFixture(t, "雨从傍晚开始下。")
	for _, page := range []page{pageTalk, pagePlan, pageCheck, pageWrite} {
		click(t, tt, a.pages[page].name())
		if a.page != page {
			t.Fatalf("page %d could not be clicked through the centered row", page)
		}
	}
}

func TestThePageSwitchPassesBlankSpace(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	held := false
	under := stillTester(func(c *ui.Context) {
		base := ui.ButtonBase(c).Fill().Label("下面的空白")
		base.Children(func() { a.pageSwitch(c, 1266) })
		if base.Clicked() {
			held = true
		}
	}, 1266, 40)
	under.ClickAt(300, 20)
	if !held {
		t.Fatal("the centered pass-through layer takes blank title bar space")
	}
}

// Secondary numbers stay visible while the keyboard is in their row or
// in its context menu, after the pointer has moved elsewhere.
func TestListNumbersFollowKeyboardAndMenus(t *testing.T) {
	a, tt, id := writingFixture(t, "雨从傍晚开始下。")
	testRow := func(label, number, owner string) {
		t.Helper()
		r := find(t, tt, label)
		// Cancel a press to start focus traversal without opening the row.
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(0, 0)
		tt.Release(0, 0)
		tt.Key(ui.Shift, ui.KeyTab)
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if !tt.Focused(label) || !slices.Contains(tt.Texts(), number) {
			t.Fatalf("keyboard row %s has no %s: %v", label, number, tt.Texts())
		}
		tt.Key(ui.Shift, ui.KeyF10)
		tt.Key(0, ui.KeyDown)
		tt.Move(0, 0)
		tt.Frame()
		if !a.menu.MenuOpen(owner) || !slices.Contains(tt.Texts(), number) {
			t.Fatalf("open menu %s has no %s: %v", owner, number, tt.Texts())
		}
		tt.Key(0, ui.KeyEscape)
	}
	testRow("章节-第一章", "8", "chapter-"+id)
	a.page = pageTalk
	if _, err := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "修路灯"}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	testRow("对话-"+a.session, talkDay(a.session, time.Now()), "talk-"+a.session)
}

// The search controls have room above them, and a standalone setting
// heading starts on the same line as the writing page's first tab.
func TestThePaneSearchAndHeadingHaveRoom(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	for _, tc := range []struct {
		tab   paneTab
		query string
	}{{paneSetting, "搜索设定"}, {paneFore, "搜索伏笔"}, {paneTalk, "搜索讨论"}} {
		a.doc.tab, a.doc.talks = tc.tab, true
		tt.Frame()
		tt.Frame()
		top, query := find(t, tt, "右栏标签行"), find(t, tt, tc.query+"框")
		if query.Y != top.Y+top.H+12 || query.H != kit.HControl {
			t.Fatalf("%s has no top padding: %+v, header %+v", tc.query, query, top)
		}
	}
	a.doc.tab = paneSetting
	tt.Frame()
	tab := find(t, tt, "右栏-批注")
	for _, page := range []page{pageTalk, pagePlan} {
		a.page, outlinePage(a).Pane = page, true
		tt.Frame()
		tt.Frame()
		heading := find(t, tt, "右栏设定标题")
		if heading.X != tab.X+10 {
			t.Fatalf("page %d setting heading at %v, tab at %v", page, heading.X, tab.X+10)
		}
	}
}
