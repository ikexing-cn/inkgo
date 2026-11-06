package desk

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/internal/testshot"
	"writer/locales"
)

func ledgerFixture(t *testing.T, extra ...ext.Module) (*App, *ui.Tester, *int, *int) {
	t.Helper()
	first, second := new(int), new(int)
	base := ext.Module{ID: "ledgerbase", Name: locales.ModInjectDocx, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Commands, "docx", hostapi.Command{Name: locales.ModInjectDocx, Run: func() {}})
		ext.NameTarget(r, string(ext.HookOf(ext.CallPoint[hostapi.Command, struct{}, struct{}]{Point: hostapi.Commands}, "ledgerbase.docx")), locales.ModInjectDocx)
	}}
	rival := func(id, name string, n *int) ext.Module {
		return ext.Module{ID: id, Installed: true, Name: func() string { return name }, Requires: []string{"ledgerbase"}, Setup: func(r *ext.Registrar) {
			ext.Replace(r, hostapi.Commands, "ledgerbase.docx", "replace", hostapi.Command{Name: locales.ModInjectDocx, Run: func() {}})
			ext.Contribute(r, hostapi.Commands, "key", hostapi.Command{Name: func() string { return name }, Key: ui.KeyA, Mods: ui.Ctrl | ui.Shift, HelpBefore: "new-chapter", HelpGroup: locales.HelpGroupWrite, Run: func() { *n++ }})
		}}
	}
	saveRival := ext.Module{ID: "saverival", Installed: true, Name: locales.ModClassic, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Commands, "save", hostapi.Command{Name: locales.ModClassic, Key: ui.KeyS, Mods: modCmd, Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile}, Run: func() { *second++ }})
	}}
	mods := append(testModules(), saveRival, base, rival("vertical", "Vertical", first), rival("classic", "Classic", second), ext.Module{ID: "brokenledger", Name: func() string { return "Broken" }})
	mods = append(mods, extra...)
	a := newTestApp(ai.New(filepath.Join(t.TempDir(), "providers.yaml")), mods...)
	t.Cleanup(func() { a.Close() })
	requireExtension(t, a.Open(extensionBook(t)))
	tt := stillTester(a.View, 1266, 780)
	return a, tt, first, second
}

func showLedger(a *App, tt *ui.Tester, id string) {
	a.openPrefs("extensions")
	a.settings.ext = id
	tt.Frame()
}
func waitLedger(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		tt.Frame()
		if a.ledgerData.completed == a.ledgerData.generation {
			tt.Frame()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ledger worker did not finish")
}

func TestLedgerPartsOffSharedDataAndErrors(t *testing.T) {
	a, tt, _, _ := ledgerFixture(t)
	book, _ := a.extensions.DataPaths("lore")
	requireExtension(t, os.MkdirAll(book, 0755))
	requireExtension(t, os.WriteFile(filepath.Join(book, "ledger-fixture"), make([]byte, 12800), 0600))
	showLedger(a, tt, "lore")
	waitLedger(t, a, tt)
	if !tt.HasText(locales.ModAdds("marks", "")) || !tt.HasText(locales.ModLedgerAdds()) || !tt.HasText(locales.ModLedgerData()) || tt.HasText(locales.ModLedgerErrors()) || tt.HasText(locales.ModLedgerInjected()) {
		t.Fatal(tt.Texts())
	}
	requireExtension(t, a.extensions.Set("lore", false))
	tt.Frame()
	if !tt.HasText(locales.ModAdds("pane", a.extViews.moduleViews["lore"].name())) {
		t.Fatal(tt.Texts())
	}
	localBook, err := a.Store.LocalExtDir("goal")
	requireExtension(t, err)
	requireExtension(t, os.MkdirAll(localBook, 0755))
	requireExtension(t, os.WriteFile(filepath.Join(localBook, "ledger-fixture"), make([]byte, 400), 0600))
	_, global := a.extensions.DataPaths("goal")
	requireExtension(t, os.MkdirAll(global, 0755))
	requireExtension(t, os.WriteFile(filepath.Join(global, "ledger-fixture"), make([]byte, 500), 0600))
	showLedger(a, tt, "goal")
	waitLedger(t, a, tt)
	found := false
	locals := 0
	for _, row := range a.ledgerData.rows[a.ledgerData.page] {
		if row.local {
			locals++
		}
		if row.path == "ext/goal" && row.local && row.size >= 900 {
			found = true
		}
	}
	if !found || locals != 1 {
		t.Fatal("machine data must be totaled into one row")
	}
	showLedger(a, tt, "brokenledger")
	if tt.HasText(locales.ModLedgerAdds()) || tt.HasText(locales.ModLedgerData()) {
		t.Fatal(tt.Texts())
	}
	_ = ext.Guard(a.extensions, "brokenledger", "first", func() { panic(errors.New("first failure")) })
	requireExtension(t, a.extensions.Set("brokenledger", false))
	requireExtension(t, a.extensions.Set("brokenledger", true))
	_ = ext.Guard(a.extensions, "brokenledger", "second", func() { panic("second failure") })
	tt.Frame()
	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "first failure") || !strings.Contains(texts, "second failure") || strings.Index(texts, "second failure") > strings.Index(texts, "first failure") || !strings.Contains(texts, locales.TimeTodayDay()) {
		t.Fatal(texts)
	}
	a.openPrefs("reminders")
	tt.Frame()
	if tt.HasText(locales.ModLedgerAdds()) || tt.HasText(locales.ModLedgerErrors()) {
		t.Fatal(tt.Texts())
	}
}

func TestLedgerReplaceChoicesAndShortcutLoser(t *testing.T) {
	a, tt, first, second := ledgerFixture(t)
	showLedger(a, tt, "vertical")
	want := locales.ModInjectConflict(locales.ModInjectDocx(), "Classic")
	if !tt.HasText(want) {
		t.Fatal(tt.Texts())
	}
	click(t, tt, "injection-choice-vertical.replace")
	tt.Frame()
	if !tt.HasText(locales.ModInjected(locales.ModInjectDocx(), "overwrite")) {
		t.Fatal(tt.Texts())
	}
	showLedger(a, tt, "classic")
	if !tt.HasText(locales.ModInjectNotUsed(locales.ModInjectDocx(), "Vertical")) {
		t.Fatal(tt.Texts())
	}
	a.closePrefs()
	a.keysOpen = true
	tt.Frame()
	if !tt.HasText(locales.KeysConflict("Classic", "Vertical")) {
		t.Fatal(tt.Texts())
	}
	a.keysOpen = false
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
	tt.Frame()
	if *first != 1 || *second != 0 {
		t.Fatal(*first, *second)
	}
}

func TestShotExtensionLedger(t *testing.T) {
	testshot.Dir(t)
	for _, lang := range []string{"zh-Hans", "en"} {
		t.Run(lang, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return lang })
			t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
			a, tt, _, _ := ledgerFixture(t)
			book, _ := a.extensions.DataPaths("lore")
			requireExtension(t, os.MkdirAll(book, 0755))
			requireExtension(t, os.WriteFile(filepath.Join(book, "ledger-fixture"), make([]byte, 12800), 0600))
			showLedger(a, tt, "lore")
			waitLedger(t, a, tt)
			shot(t, tt, "ledger-lore-"+lang+".png")
			_ = ext.Guard(a.extensions, "brokenledger", "draw", func() { panic("draw failed") })
			showLedger(a, tt, "brokenledger")
			shot(t, tt, "ledger-error-"+lang+".png")
			a.SetError(nil)
			showLedger(a, tt, "vertical")
			shot(t, tt, "ledger-conflict-"+lang+".png")
			click(t, tt, "injection-choice-vertical.replace")
			tt.Frame()
			shot(t, tt, "ledger-picked-"+lang+".png")
			showLedger(a, tt, "classic")
			shot(t, tt, "ledger-unused-"+lang+".png")
			a.closePrefs()
			requireExtension(t, a.extensions.Set("saverival", false))
			a.keysOpen = true
			tt.Frame()
			shot(t, tt, "ledger-keys-"+lang+".png")
			requireExtension(t, a.extensions.Set("saverival", true))
			tt.Frame()
			shot(t, tt, "ledger-keys-save-"+lang+".png")
		})
	}
}

func TestLedgerDataWorkerCachesAndRefreshesOnOpening(t *testing.T) {
	dir := t.TempDir()
	local := t.TempDir()
	file := filepath.Join(dir, "data")
	requireExtension(t, os.WriteFile(file, make([]byte, 12800), 0600))
	requireExtension(t, os.MkdirAll(filepath.Join(local, "ext", "sample"), 0755))
	requireExtension(t, os.WriteFile(filepath.Join(local, "ext", "sample", "data"), make([]byte, 400), 0600))
	goroutine := func() string {
		var b [80]byte
		n := runtime.Stack(b[:], false)
		return strings.Fields(string(b[:n]))[1]
	}
	frameID := goroutine()
	var calls atomic.Int32
	var onFrame atomic.Bool
	h, err := ext.New(ext.Options{LocalRoot: local, BookDir: func(string, string) (string, error) {
		calls.Add(1)
		if goroutine() == frameID {
			onFrame.Store(true)
		}
		return dir, nil
	}}, ext.Module{ID: "sample"})
	requireExtension(t, err)
	defer h.Close()
	requireExtension(t, h.Open(ext.Book{Dir: dir}))
	a := &App{extensions: h}
	tt := stillTester(func(c *ui.Context) { a.jobs.poll(); a.refreshLedgerData(c, "sample") }, 500, 400)
	waitLedger(t, a, tt)
	rows := a.ledgerData.rows[a.ledgerData.page]
	if onFrame.Load() || len(rows) != 2 || rows[0].size != 12800 || rows[1].size != 400 || !rows[1].local {
		t.Fatal(onFrame.Load(), rows)
	}
	before := calls.Load()
	requireExtension(t, os.WriteFile(file, make([]byte, 1024), 0600))
	for range 5 {
		tt.Frame()
	}
	if calls.Load() != before || a.ledgerData.rows[a.ledgerData.page][0].size != 12800 {
		t.Fatal("steady frame did filesystem work")
	}
	a.ledgerData.page = ""
	tt.Frame()
	waitLedger(t, a, tt)
	if a.ledgerData.rows[a.ledgerData.page][0].size != 1024 {
		t.Fatal("opening did not refresh")
	}
}

func TestLedgerDisabledPaneListsDeclaredTitle(t *testing.T) {
	mod := ext.Module{ID: "named", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.PaneTabs, "pane", hostapi.PaneTab{Title: func() string { return "Declared pane" }})
	}}
	a := newTestApp(ai.New(""), mod)
	defer a.Close()
	requireExtension(t, a.extensions.Set("named", false))
	a.syncExtensions()
	a.settings.tab = "extensions"
	a.settings.ext = "named"
	tt := stillTester(func(c *ui.Context) { a.extensionSection(c, "named") }, 600, 400)
	if !tt.HasText(locales.ModAdds("pane", "Declared pane")) {
		t.Fatal(tt.Texts())
	}
}

func TestShortcutWinnerOnlyHostSaveAndLanguageSwitch(t *testing.T) {
	a, tt, _, second := ledgerFixture(t)
	a.keysOpen = true
	tt.Frame()
	for _, lang := range []string{"zh-Hans", "en"} {
		i18n.SetLocaleFunc(func() string { return lang })
		a.localeRevision.Add(1)
		tt.Frame()
		want := locales.KeysConflict("Classic", "Vertical")
		texts := strings.Join(paintedTexts(tt), "\n")
		if strings.Count(texts, want) != 1 {
			t.Fatal("clash must occur once", texts)
		}
		saveClash := locales.KeysConflict(locales.ModClassic(), locales.CmdSave())
		if !strings.Contains(texts, saveClash) {
			t.Fatal("host save clash missing", texts, saveClash)
		}
		for _, cmd := range a.extensionCommands() {
			if cmd.id == "classic.key" || cmd.id == "saverival.save" {
				if cmd.cmd.keys() != "" || systemAccelerator(cmd.cmd) != "" {
					t.Fatal("loser retained accelerator")
				}
			}
		}
	}
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a.keysOpen = false
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyS)
	tt.Frame()
	if *second != 0 {
		t.Fatal("host save loser fired")
	}
}

func TestInjectionRivalsFollowRegistrationDespiteChainOrder(t *testing.T) {
	third := ext.Module{ID: "thirdrival", Name: func() string { return "Third" }, Requires: []string{"ledgerbase"}, Setup: func(r *ext.Registrar) {
		ext.Replace(r, hostapi.Commands, "ledgerbase.docx", "replace", hostapi.Command{Name: locales.ModInjectDocx, Run: func() {}})
	}}
	a, tt, _, _ := ledgerFixture(t, third)
	target := "ledgerbase.docx@host.commands"
	for _, i := range a.extensions.Ledger("vertical").Injections {
		target = i.Target
	}
	requireExtension(t, a.extensions.SetOrder(target, []string{"thirdrival.replace", "classic.replace", "vertical.replace"}))
	showLedger(a, tt, "vertical")
	row := a.extViews.moduleViews["vertical"].injected[0]
	if len(row.names) != 4 || row.names[1] != "Vertical" || row.names[2] != "Classic" || row.names[3] != "Third" || row.text != locales.ModInjectConflict(locales.ModInjectDocx(), "Classic"+locales.ModListSep()+"Third") {
		t.Fatal(row)
	}
}
