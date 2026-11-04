package desk

import (
	"testing"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestShellLabelsFollowLocale(t *testing.T) {
	a, _, _ := fixture(t)
	t.Setenv("MYGO_LOCALE", "en")
	if a.pages[pageWrite].title(a) != "Write" || a.paneTabs[0].title(a) != "Notes" || a.snapReason(workspace.SnapOpen) != "On Open" {
		t.Fatal(a.pages[pageWrite].title(a), a.paneTabs[0].title(a), a.snapReason(workspace.SnapOpen))
	}
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	if a.pages[pageWrite].title(a) != "正文" || a.paneTabs[0].title(a) != "批注" || a.snapReason(workspace.SnapOpen) != "打开时" {
		t.Fatal("shell labels retained the previous locale")
	}
}

func TestShotShellLocale(t *testing.T) {
	testshot.Dir(t)
	a, tt, id := writingFixture(t, "雨从傍晚开始下。林晚沿着亮着的灯往旧城走。")
	a.openFind()
	a.write.query, a.write.with, a.write.wide = "雨", "雪", true
	shot(t, tt, "shell-find.png")
	a.write.sure = true
	shot(t, tt, "shell-replace.png")
	a.closeFind()
	a.openSnapshots()
	shot(t, tt, "shell-snapshots.png")
	a.write.snapsOpen = false
	goalOf(a).Open = true
	shot(t, tt, "shell-goal.png")
	goalOf(a).Open = false
	n, _ := a.book.Tree().Node(id)
	a.trashNode(n)
	a.openTrash()
	shot(t, tt, "shell-trash.png")
}

func TestNavigationTables(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ai, work bool
	}{
		{"AI on, work open", true, true},
		{"AI off, work open", false, true},
		{"AI on, no work", true, false},
		{"AI off, no work", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, tt, _ := writingFixture(t, "雨停了。")
			if !tc.work {
				a.closeWork()
			}
			a.turnAI(!(!tc.ai))
			for _, p := range []struct {
				id       page
				name     string
				ai, work bool
			}{
				{pageTalk, "讨论", true, false},
				{pageWrite, "正文", false, true},
				{pagePlan, "大纲", false, true},
				{pageCheck, "检查", true, true},
				{pageCategories, "作品分类", false, true},
			} {
				entry := a.pages[p.id]
				want := (!p.ai || tc.ai) && (!p.work || tc.work)
				if entry.name() != p.name || entry.available(a) != want {
					t.Errorf("page %s: name %q, available %v, want %v", p.name, entry.name(), entry.available(a), want)
				}
				if tc.work {
					count := 0
					if p.id == pageCheck {
						count = len(a.listedChecks())
					}
					if got := entry.number(); got != count {
						t.Errorf("page %s count = %d, want %d", p.name, got, count)
					}
				}
				showPage(a, p.id)
				shown := p.id
				if !tc.work {
					shown = pageTalk
				} else if !want {
					shown = pageWrite
				}
				if got := a.pageShown(); got != shown {
					t.Errorf("page %s shows %s, want %s", p.name, a.pages[got].name(), a.pages[shown].name())
				}
			}
			for i, p := range []struct {
				id   paneTab
				name string
				ai   bool
			}{
				{paneMargin, "批注", false},
				{paneSetting, "设定", false},
				{paneTalk, "讨论", true},
				{paneFore, "伏笔", false},
				{paneTab("phone.preview"), "预览", false},
			} {
				entry := a.paneTabs[i]
				want := !p.ai || tc.ai
				if entry.id != p.id || entry.name() != p.name || entry.available(a) != want {
					t.Errorf("pane tab %s: entry %+v, want available %v", p.name, entry, want)
				}
				if tc.work {
					count := 0
					switch p.id {
					case paneSetting:
						count = a.paneCounts().open
					case paneFore:
						count = a.paneCounts().waiting
					}
					if got := entry.number(); got != count {
						t.Errorf("pane tab %s count = %d, want %d", p.name, got, count)
					}
				}
				showTab(a, p.id)
				shown := p.id
				if !want {
					shown = paneSetting
				}
				if got := a.paneTabs.shown(a, shownTab(a)); got != shown || shownTab(a) != p.id {
					t.Errorf("pane tab %s shows %q, want %q, kept choice %q", p.name, got, shown, shownTab(a))
				}
			}
			for i, p := range []struct {
				name string
				work bool
			}{
				{"appearance", false}, {"extensions", false}, {"AI", false}, {"reminders", true}, {"备份", false},
			} {
				entry := a.prefTabs[i]
				id := p.name
				if id == "备份" {
					id = "backup.settings"
				}
				want := !p.work || tc.work
				if entry.id != id || entry.available(a) != want {
					t.Errorf("settings part %s: entry %+v, want available %v", p.name, entry, want)
				}
				if got := entry.number(); got != 0 {
					t.Errorf("settings part %s count = %d, want 0", p.name, got)
				}
				a.openPrefs(id)
				tt.Frame()
				shown := id
				if !want {
					shown = "AI"
				}
				if a.settings.tab != shown {
					t.Errorf("settings part %s shows %s, want %s", p.name, a.settings.tab, shown)
				}
				name := p.name
				if name == "appearance" {
					name = "外观"
				}
				if name == "extensions" {
					name = "扩展"
				}
				if name == "reminders" {
					name = "提醒"
				}
				if got := entry.title(a); got != name {
					t.Errorf("settings part %s is called %s, want %s", p.name, got, name)
				}
			}
		})
	}
}
