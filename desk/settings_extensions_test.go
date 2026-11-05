package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"writer/ext/backup/backuptext"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/ext"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
	"writer/internal/testshot"
	"writer/locales"
)

func extensionSettingsFixture(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	mods := append(testModules(),
		ext.Module{ID: "consumer", Group: hostapi.GroupWrite, Name: func() string { return "使用资料" }, Desc: func() string { return "读取资料。" }, Off: func() string { return "已停止读取。" }, Requires: []string{"source"}},
		ext.Module{ID: "source", Group: hostapi.GroupWrite, Name: func() string { return "前置功能" }, Desc: func() string { return "提供资料。" }},
		ext.Module{ID: "optional", Group: hostapi.GroupLore, Name: func() string { return "配合资料" }, Optional: []string{"source"}},
		ext.Module{ID: "unused", Kind: ext.Library, Group: hostapi.GroupWrite, Name: func() string { return "闲置基础" }},
		ext.Module{ID: "installed", Group: "unknown", Name: func() string { return "另装功能" }},
	)
	a := newTestApp(ai.New(filepath.Join(t.TempDir(), "providers.yaml")), mods...)
	t.Cleanup(func() { a.Close() })
	requireExtension(t, a.Open(extensionBook(t)))
	a.openPrefs("extensions")
	return a, stillTester(a.View, 1266, 780)
}

func extensionDetails(t *testing.T, a *App, tt *ui.Tester, id string) {
	t.Helper()
	a.openPrefs("extensions")
	tt.Frame()
	r := find(t, tt, "扩展-"+id)
	if r.Y > 650 {
		tt.Scroll(750, 500, 0, r.Y-550)
		tt.Frame()
	}
	if r.Y < 180 {
		tt.Scroll(750, 500, 0, r.Y-250)
		tt.Frame()
	}
	click(t, tt, "扩展-"+id)
}

func TestExtensionOverviewListsOnlyAssemblyInGroupOrderAndResetsDetails(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	want := [][]string{{"focus", "goal", "names", "find", "notes", "consumer", "source"}, {"lore", "outline", "hooks", "optional"}, {"talk", "check"}, {"snaps", "trash", "cats", "backup", "export", "phone"}, {"installed"}, {"context", "marks", "unused", "ai"}}
	if len(a.extViews.groups) != len(want) {
		t.Fatal(a.extViews.groups)
	}
	for i, ids := range want {
		if !slices.Equal(a.extViews.groups[i].ids, ids) {
			t.Fatal(a.extViews.groups)
		}
	}
	if _, ok := tt.Find("扩展-missing"); ok {
		t.Fatal(tt.Texts())
	}
	for _, mod := range a.extViews.modules {
		find(t, tt, "扩展-"+mod.ID)
	}
	left, right := find(t, tt, "扩展-focus"), find(t, tt, "扩展-goal")
	if left.Y != right.Y || left.W != right.W || left.X >= right.X {
		t.Fatal("overview is not two equal columns", left, right)
	}
	// Cached host snapshots remain the same across unchanged frames.
	first := &a.extViews.modules[0]
	ledger := a.extViews.moduleViews["consumer"].ledger.Requires
	tt.Frame()
	if first != &a.extViews.modules[0] || &ledger[0] != &a.extViews.moduleViews["consumer"].ledger.Requires[0] {
		t.Fatal("rebuilt snapshots")
	}
	requireExtension(t, a.extensions.Set("source", false))
	tt.Frame()
	if !tt.HasText(locales.ModOff()) || a.extViews.moduleViews["consumer"].active {
		t.Fatal("inactive dependency not reflected")
	}
	click(t, tt, "扩展-consumer")
	if a.settings.ext != "consumer" || a.settings.tab != "extensions" {
		t.Fatal(a.settings)
	}
	hasText(t, tt, "›")
	find(t, tt, "扩展总览")
	click(t, tt, "扩展总览")
	if a.settings.ext != "" {
		t.Fatal(a.settings)
	}
	if _, ok := tt.Find("扩展-consumer"); !ok {
		t.Fatal("breadcrumb did not return to overview")
	}
	click(t, tt, "扩展-phone", "设置-外观", "设置-扩展")
	if a.settings.ext != "" {
		t.Fatal(a.settings)
	}
	click(t, tt, "扩展-phone", locales.SettingsClose())
	a.openPrefs("extensions")
	tt.Frame()
	if a.settings.ext != "" {
		t.Fatal(a.settings)
	}
	click(t, tt, "扩展-phone")
	a.closeWork()
	if a.settings.ext != "" {
		t.Fatal("work detail survived leaving")
	}
}

func TestExtensionHeaderExplainsDescriptionOffRequiredOptionalAndUnusedLibrary(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	extensionDetails(t, a, tt, "consumer")
	hasText(t, tt, "读取资料。")
	hasText(t, tt, locales.ModNeeds("前置功能"))
	if tt.HasText("已停止读取。") {
		t.Fatal("off explanation while on")
	}
	click(t, tt, "扩展开关-consumer")
	hasText(t, tt, "已停止读取。")
	if a.settings.ext != "consumer" || a.settings.askExt {
		t.Fatal(a.settings)
	}
	extensionDetails(t, a, tt, "source")
	hasText(t, tt, locales.ModNeededBy("使用资料"))
	extensionDetails(t, a, tt, "optional")
	hasText(t, tt, locales.ModOptional("前置功能"))
	extensionDetails(t, a, tt, "unused")
	hasText(t, tt, locales.ModNobodyNeeds())
}

func TestExtensionCascadeAsksCancelsPreservesOwnSwitchesAndShadesOnce(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	extensionDetails(t, a, tt, "source")
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		bounds := img.Bounds()
		corner := img.At(3, bounds.Max.Y-3)
		click(t, tt, "扩展开关-source")
		hasText(t, tt, locales.ModAskOffTitle("前置功能"))
		hasText(t, tt, locales.ModAskOffAbout("使用资料"))
		if got := tt.Image().At(3, bounds.Max.Y-3); got != corner {
			t.Fatal("double shade", dark, got, corner)
		}
		click(t, tt, locales.CommonCancel())
		if !a.extensions.Own("source") || a.extensions.Ledger("consumer").State != ext.On {
			t.Fatal("cancel changed switches")
		}
	}
	click(t, tt, "扩展开关-source", "扩展确认开关")
	if a.extensions.Own("source") || !a.extensions.Own("consumer") || a.extensions.Ledger("consumer").State == ext.On {
		t.Fatal("cascade rewrote own choice")
	}
	click(t, tt, "扩展开关-source")
	if a.settings.askExt || a.extensions.Ledger("consumer").State != ext.On {
		t.Fatal("dependency did not resume")
	}
	extensionDetails(t, a, tt, "consumer")
	click(t, tt, "扩展开关-consumer")
	requireExtension(t, a.extensions.Set("source", false))
	tt.Frame()
	click(t, tt, "扩展开关-consumer")
	hasText(t, tt, locales.ModAskOnAbout("前置功能"))
	click(t, tt, locales.CommonCancel())
	if a.extensions.Own("consumer") || a.extensions.Own("source") {
		t.Fatal("cancel enabled dependency")
	}
	click(t, tt, "扩展开关-consumer", "扩展确认开关")
	if a.extensions.Ledger("consumer").State != ext.On || !a.extensions.Own("source") {
		t.Fatal("required dependency not enabled")
	}
}

func TestExtensionMoreScopesSwitchToBookPersistsAndRestoresLocalDefault(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	root := a.Store.Root()
	extensionDetails(t, a, tt, "phone")
	click(t, tt, "ext-more-phone", locales.ModBookOnlyOff())
	if on, ok := a.extensions.BookOverride("phone"); !ok || on {
		t.Fatal("no book override")
	}
	hasText(t, tt, locales.ModBookOff())
	data, err := os.ReadFile(filepath.Join(root, "ext.json"))
	requireExtension(t, err)
	var saved extensionFile
	requireExtension(t, json.Unmarshal(data, &saved))
	if on, ok := saved.Switches["phone"]; !ok || on {
		t.Fatal(string(data))
	}
	// Header changes an existing override, leaving the machine's switch alone.
	click(t, tt, "扩展开关-phone")
	hasText(t, tt, locales.ModBookOn())
	click(t, tt, "扩展开关-phone")
	requireExtension(t, a.Open(extensionBook(t)))
	if !a.extensions.Own("phone") {
		t.Fatal("local switch was changed")
	}
	requireExtension(t, a.Open(root))
	if a.extensions.Own("phone") {
		t.Fatal("book override not reloaded")
	}
	extensionDetails(t, a, tt, "phone")
	click(t, tt, "ext-more-phone", locales.ModFollowLocal())
	if _, ok := a.extensions.BookOverride("phone"); ok || !a.extensions.Own("phone") || a.settings.askExt {
		t.Fatal("did not follow local")
	}
	data, err = os.ReadFile(filepath.Join(root, "ext.json"))
	requireExtension(t, err)
	if strings.Contains(string(data), "phone") {
		t.Fatal("default not persisted", string(data))
	}
	// Book cascade uses the same question and only overrides feature switches.
	extensionDetails(t, a, tt, "source")
	click(t, tt, "ext-more-source", locales.ModBookOnlyOff())
	if !a.settings.askExt || !a.settings.flipBook {
		t.Fatal("book cascade not asked")
	}
	click(t, tt, locales.CommonCancel())
	if _, ok := a.extensions.BookOverride("source"); ok {
		t.Fatal("cancel persisted override")
	}
	click(t, tt, "ext-more-source", locales.ModBookOnlyOff(), "扩展确认开关")
	if a.extensions.Ledger("consumer").State == ext.On || !a.extensions.Own("consumer") {
		t.Fatal("book cascade choices lost")
	}
	extensionDetails(t, a, tt, "unused")
	if _, ok := tt.Find("ext-more-unused"); ok {
		t.Fatal("library has book menu")
	}
	a.closeWork()
	a.openPrefs("extensions")
	tt.Frame()
	click(t, tt, "扩展-phone")
	if _, ok := tt.Find("ext-more-phone"); ok {
		t.Fatal("shelf has book menu")
	}
}

func TestExtensionWaitingKeepsOwnCheckPalesBoxAndNamesMissingRequirement(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	extensionDetails(t, a, tt, "consumer")
	r := find(t, tt, "扩展开关-consumer")
	before := tt.Image().At(int(r.X)+1, int(r.Y+r.H/2))
	requireExtension(t, a.extensions.Set("source", false))
	tt.Frame()
	tt.Frame()
	v := a.extViews.moduleViews["consumer"]
	if !v.own || v.active || !slices.Equal(v.waits, []string{"source"}) {
		t.Fatal(v)
	}
	hasText(t, tt, locales.ModWaiting("前置功能"))
	if tt.HasText("已停止读取。") {
		t.Fatal("own switch confused with waiting")
	}
	if got := tt.Image().At(int(r.X)+1, int(r.Y+r.H/2)); got == before {
		t.Fatal("waiting box not pale")
	}
	// The warning uses the palette's amber, not the ordinary note color.
	img := tt.Image()
	amber := a.pal.Amber
	found := false
	for y := int(r.Y + r.H); y < int(r.Y+r.H)+100; y++ {
		for x := int(r.X); x < int(r.X)+500; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if uint8(cr>>8) == amber.R && uint8(cg>>8) == amber.G && uint8(cb>>8) == amber.B {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("missing amber warning")
	}
}

func TestExtensionBookEnableAsksAndOverridesOnlyThisBooksFeatures(t *testing.T) {
	a, tt := extensionSettingsFixture(t)
	requireExtension(t, a.extensions.Set("consumer", false))
	requireExtension(t, a.extensions.Set("source", false))
	extensionDetails(t, a, tt, "consumer")
	click(t, tt, "ext-more-consumer", locales.ModBookOnlyOn())
	if !a.settings.askExt || !a.settings.flipBook {
		t.Fatal("missing book enable question")
	}
	hasText(t, tt, locales.ModAskOnAbout("前置功能"))
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.settings.askExt || !a.settings.open || a.extensions.Own("consumer") || a.extensions.Own("source") {
		t.Fatal("Escape changed book switches or closed settings")
	}
	click(t, tt, "ext-more-consumer", locales.ModBookOnlyOn(), "扩展确认开关")
	for _, id := range []string{"source", "consumer"} {
		if on, ok := a.extensions.BookOverride(id); !ok || !on || a.extensions.Ledger(id).State != ext.On {
			t.Fatal("book dependency not enabled", id)
		}
	}
	hasText(t, tt, locales.ModBookOn())
	requireExtension(t, a.Open(extensionBook(t)))
	if a.extensions.Own("consumer") || a.extensions.Own("source") {
		t.Fatal("book enable changed local switches")
	}
}

func TestBackupPageKeepsHeaderWhenOffAndHidesWorkSettingsAndBookMenuOnShelf(t *testing.T) {
	a, tt, _ := fixture(t)
	backupOf(a).Configure(archive.Settings{Dir: t.TempDir()})
	a.openPrefs("backup.settings")
	tt.Frame()
	head := find(t, tt, "扩展开关-backup")
	settings := find(t, tt, backuptext.BackupTiming())
	if head.Y >= settings.Y {
		t.Fatal("header below settings")
	}
	click(t, tt, "扩展开关-backup")
	hasText(t, tt, backuptext.ModBackupOff())
	if a.extViews.moduleViews["backup"].own || tt.HasText(backuptext.BackupTiming()) || a.settings.tab != "backup.settings" {
		t.Fatal("disabled header/settings")
	}
	find(t, tt, "扩展开关-backup")
	click(t, tt, "扩展开关-backup")
	a.closeWork()
	a.openPrefs("backup.settings")
	tt.Frame()
	find(t, tt, "扩展开关-backup")
	if tt.HasText(backuptext.BackupTiming()) {
		t.Fatal("work settings on shelf")
	}
	if _, ok := tt.Find("ext-more-backup"); ok {
		t.Fatal("book menu on shelf")
	}
}

func TestShotSettingsExtensions(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	a.openPrefs("extensions")
	shot(t, tt, "settings-ext-overview.png")
	tt.SetDark(true)
	shot(t, tt, "settings-ext-overview-dark.png")
	tt.SetDark(false)
	click(t, tt, "扩展-phone", "ext-more-phone")
	shot(t, tt, "settings-ext-phone-more.png")
	a.showPref("backup.settings")
	tt.Frame()
	click(t, tt, "扩展开关-backup")
	shot(t, tt, "settings-ext-backup-off.png")
	a.closePrefs()
	b, bt := extensionSettingsFixture(t)
	extensionDetails(t, b, bt, "source")
	click(t, bt, "扩展开关-source")
	shot(t, bt, "settings-ext-cascade-ask.png")
	click(t, bt, "扩展确认开关")
	extensionDetails(t, b, bt, "consumer")
	shot(t, bt, "settings-ext-waiting.png")
	bt.SetDark(true)
	shot(t, bt, "settings-ext-waiting-dark.png")
}

func TestBookEnableSaysTheLibrariesItTakesTurnOnForEveryWork(t *testing.T) {
	mods := append(testModules(),
		ext.Module{ID: "base", Kind: ext.Library, Name: func() string { return "底层" }},
		ext.Module{ID: "user", Group: hostapi.GroupWrite, Name: func() string { return "上层" }, Requires: []string{"base"}},
	)
	a := newTestApp(ai.New(filepath.Join(t.TempDir(), "providers.yaml")), mods...)
	t.Cleanup(func() { a.Close() })
	requireExtension(t, a.Open(extensionBook(t)))
	tt := stillTester(a.View, 1266, 780)
	requireExtension(t, a.extensions.Set("user", false))
	requireExtension(t, a.extensions.Set("base", false))
	extensionDetails(t, a, tt, "user")
	hint := locales.ModAskLibraryLocal("底层")
	click(t, tt, "扩展开关-user")
	if !a.settings.askExt || tt.HasText(hint) {
		t.Fatal("turning it on for every work says the libraries go along for every work")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	click(t, tt, "ext-more-user", locales.ModBookOnlyOn())
	hasText(t, tt, hint)
	click(t, tt, "扩展确认开关")
	if on, ok := a.extensions.BookOverride("user"); !ok || !on || !a.extensions.Own("base") {
		t.Fatal("this work's feature or the library of this computer is not on")
	}
}
