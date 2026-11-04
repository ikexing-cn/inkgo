package desk

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

func TestGoalOffRemovesSuffixPopoverAndCountClickGloballyAndPerBook(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[book], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			goalOf(a).SetGoal("2000")
			click(t, tt, "字数")
			a.setExtension("goal", false, book)
			tt.Frame()
			if tt.HasText("每日目标") || tt.HasText("/ 2,000") || len(ext.Items(a.extensions, hostapi.WordPopovers)) != 0 {
				t.Fatal(tt.Texts())
			}
			click(t, tt, "字数")
			tt.Frame()
			if tt.HasText("每日目标") {
				t.Fatal("disabled count opened goal")
			}
			a.setExtension("goal", true, book)
			tt.Frame()
			if goalOf(a).Counter.Goal != 2000 {
				t.Fatal("goal was not restored")
			}
		})
	}
}

func TestFocusOffRemovesControlsAndKeysAndLeavesFocusGloballyAndPerBook(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[book], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			tt.Key(0, ui.KeyF11)
			tt.Frame()
			if !a.write.focus {
				t.Fatal("focus did not start")
			}
			a.setExtension("focus", false, book)
			tt.Frame()
			if a.write.focus || len(ext.Items(a.extensions, hostapi.Tools)) != 0 {
				t.Fatal("focus survived disable")
			}
			for _, row := range a.commandList() {
				if row.name(a) == "专注写作" {
					t.Fatal("focus command survived")
				}
			}
			for _, row := range a.viewCommands() {
				if row.cmd != nil && row.cmd.name(a) == "专注写作" {
					t.Fatal("focus menu survived")
				}
			}
			if _, ok := tt.Find("专注写作"); ok {
				t.Fatal("focus button survived")
			}
			tt.Key(0, ui.KeyF11)
			tt.Key(ui.Ctrl|ui.Super, ui.KeyF)
			tt.Frame()
			if a.write.focus {
				t.Fatal("disabled key enabled focus")
			}
		})
	}
}

func TestCatsOffRemovesPageAndFileRowGloballyAndPerBook(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[book], func(t *testing.T) {
			a, tt, _ := writingFixture(t, "正文")
			clickMenu(tt, "文件", "菜单-作品分类")
			tt.Frame()
			a.setExtension("cats", false, book)
			tt.Frame()
			if a.page != pageWrite || (extensionHost{a}).HasPage("cats.categories") {
				t.Fatal("category page survived")
			}
			for _, row := range a.fileCommands() {
				if row.cmd != nil && row.cmd.name(a) == "作品分类" {
					t.Fatal("category menu survived")
				}
			}
			a.setExtension("cats", true, book)
			tt.Frame()
			if !(extensionHost{a}).HasPage("cats.categories") {
				t.Fatal("category page did not return")
			}
		})
	}
}

func TestGoalCounterUsesLocalExtensionDirectoryAndNeverDeskPreferences(t *testing.T) {
	a, _, _ := writingFixture(t, "正文")
	goalOf(a).SetGoal("2000")
	if savedGoal(t, a).Goal != 2000 {
		t.Fatal("goal not saved")
	}
	requireExtension(t, a.savePrefs())
	b, err := os.ReadFile(a.Store.PrefsPath())
	requireExtension(t, err)
	var prefs map[string]json.RawMessage
	requireExtension(t, json.Unmarshal(b, &prefs))
	for _, key := range []string{"day", "day_start", "day_goal", "days"} {
		if _, ok := prefs[key]; ok {
			t.Fatal("counter in desk prefs", key)
		}
	}
}

func TestTrashingChapterCountsAgainstToday(t *testing.T) {
	a, tt, id := writingFixture(t, "一二三")
	before := goalOf(a).Today(time.Now())
	n, _ := a.book.Tree().Node(id)
	a.trashNode(n)
	tt.Frame()
	if got := goalOf(a).Today(time.Now()); got != before-3 {
		t.Fatalf("today = %d, want %d", got, before-3)
	}
}

func TestSavingAndDroppingCategoriesRefreshesLoreAndCompletionInOpenBook(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚")
	_, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角", Names: []string{"林晚"}}}})
	requireExtension(t, err)
	requireExtension(t, a.refresh())
	page := catsOf(a)
	page.Name, page.Target = "线索", "大纲"
	requireExtension(t, page.Save())
	tt.Frame()
	if !slices.ContainsFunc(a.Store.Work().Categories, func(c workspace.Category) bool { return c.Name == "线索" }) {
		t.Fatal("new category missing")
	}
	if len(a.entities()) == 0 {
		t.Fatal("fixture has no completion names")
	}
	page.Drop("人物")
	tt.Frame()
	for _, entry := range a.book.Setting().Entries {
		if entry.Category == "人物" {
			t.Fatal("lore retained removed category")
		}
	}
	for _, entry := range a.entities() {
		if entry.Kind == "人物" {
			t.Fatal("completion retained removed category")
		}
	}
	// The live lore draw also observes the updated book, without reopening it.
	showTab(a, paneSetting)
	tt.Frame()
	if tt.HasText("删除-人物") {
		t.Fatal("stale lore category")
	}
}

func TestWritingExtensionsLedgerRetainsEveryContributionWhileOff(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文")
	for id, want := range map[string]int{"goal": 4, "focus": 2, "cats": 2} {
		ledger := a.extensions.Ledger(id)
		if len(ledger.Contributions) != want {
			t.Fatal(id, ledger.Contributions)
		}
		a.setExtension(id, false, false)
		tt.Frame()
		if got := a.extensions.Ledger(id); got.State != ext.Off || len(got.Contributions) != want {
			t.Fatal(id, got)
		}
	}
}
