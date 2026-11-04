package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

func requireExtension(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOverlaysDrawInRegistrationOrderEachFrameAndIsolatePanics(t *testing.T) {
	var calls []string
	mod := ext.Module{ID: "layers", Setup: func(r *ext.Registrar) {
		for _, id := range []string{"one", "two"} {
			ext.Contribute(r, hostapi.Overlays, id, hostapi.Overlay{Draw: func(*ui.Context) { calls = append(calls, id) }})
		}
	}}
	broken := ext.Module{ID: "broken", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Draw: func(c *ui.Context) {
			ui.Column(c).Children(func() { ui.Text(c, "partial overlay"); panic("overlay failed") })
		}})
	}}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod, broken)...)
	defer a.Close()
	tt := stillTester(a.View, 1266, 780)
	for range 3 {
		calls = nil
		tt.Frame()
		if !reflect.DeepEqual(calls, []string{"one", "two"}) {
			t.Fatal(calls)
		}
		if tt.HasText("partial overlay") {
			t.Fatal("partial failed overlay visible")
		}
	}
	if a.extensions.Ledger("broken").State != ext.Failed || a.extensions.Ledger("layers").State != ext.On {
		t.Fatal("overlay failure not isolated")
	}
}

func TestPhoneSwitchUpdatesEveryHostSurfaceWithoutRestart(t *testing.T) {
	a, tt, _ := writingFixture(t, "灯灭了。")
	for _, on := range []bool{false, true, false, true} {
		requireExtension(t, a.extensions.Set("phone", on))
		tt.Frame()
		m := &kit.Menu{}
		a.viewMenu(m, func(f func()) func() { return f })
		title := false
		for _, r := range m.Rows {
			title = title || r.Label == "手机预览"
		}
		_, system := flattenSystemMenu(a.SystemMenu())["phone.open"]
		command := false
		for _, c := range a.commandList() {
			command = command || c.name(a) == "手机预览"
		}
		_, tab := tt.Find("右栏-预览")
		if title != on || system != on || command != on || tab != on {
			t.Fatalf("on=%v title=%v system=%v command=%v tab=%v", on, title, system, command, tab)
		}
	}
}

func TestPaneDrawPanicHidesPartialFrameAndOnlyStopsItsExtension(t *testing.T) {
	config := filepath.Join(t.TempDir(), "providers.yaml")
	stopped := 0
	mod := ext.Module{ID: "broken", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.PaneTabs, "pane", hostapi.PaneTab{Title: func() string { return "Broken" }, Draw: func(c *ui.Context) {
			ui.Column(c).Children(func() { ui.Text(c, "partial draw"); panic("draw failed") })
		}})
		r.OnStop(func(*ext.Session) { stopped++ })
	}}
	a := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	root := extensionBook(t)
	requireExtension(t, a.Open(root))
	a.page = pageWrite
	stopped = 0
	a.doc.tab = "broken.pane"
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	tt.Frame()
	if a.extensions.Ledger("broken").State != ext.Failed || a.extensions.Ledger("phone").State != ext.On {
		t.Fatal("failure spread or was ignored")
	}
	if !strings.Contains(a.errorText(), "draw failed") {
		t.Fatal("no failure toast", a.errorText())
	}
	if _, ok := tt.Find("右栏-Broken"); ok {
		t.Fatal("failed tab remains")
	}
	if _, ok := tt.Find("右栏-预览"); !ok {
		t.Fatal("unrelated tab removed")
	}
	if tt.HasText("partial draw") {
		t.Fatal("partial draw visible")
	}
	requireExtension(t, a.extensions.Set("phone", false))
	if stopped != 1 {
		t.Fatal("failed cleanup must run exactly once", stopped)
	}
}

func extensionBook(t *testing.T) string {
	t.Helper()
	s, err := workspace.Create(filepath.Join(t.TempDir(), "work"), "书", "网文")
	requireExtension(t, err)
	root := s.Root()
	requireExtension(t, s.Close())
	return root
}

func TestExtensionLocalSwitchAndSettingsPersistAcrossApps(t *testing.T) {
	config := filepath.Join(t.TempDir(), "providers.yaml")
	var pref *ext.Pref[int]
	mod := ext.Module{ID: "prefs", Setup: func(r *ext.Registrar) { pref = ext.Setting(r, "size", ext.Schema[int]{Scope: ext.Local}) }}
	a := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), mod)...)
	pref.Set(17)
	requireExtension(t, a.extensions.Set("phone", false))
	requireExtension(t, a.Close())
	b := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), mod)...)
	defer b.Close()
	if b.extensions.Own("phone") || pref.Get() != 17 {
		t.Fatal("local persistence lost")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(config), "ext.json"))
	requireExtension(t, err)
	var f extensionFile
	requireExtension(t, json.Unmarshal(data, &f))
	if !reflect.DeepEqual(f.Off, []string{"phone"}) || string(f.Settings["prefs.size"]) != "17" {
		t.Fatal(string(data))
	}
}

func TestExtensionBookSwitchAndSettingsPersistInWork(t *testing.T) {
	config := filepath.Join(t.TempDir(), "providers.yaml")
	root := extensionBook(t)
	var pref *ext.Pref[string]
	mod := ext.Module{ID: "prefs", Setup: func(r *ext.Registrar) { pref = ext.Setting(r, "word", ext.Schema[string]{Scope: ext.BookScope}) }}
	a := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), mod)...)
	requireExtension(t, a.Open(root))
	pref.Set("book value")
	requireExtension(t, a.extensions.SetBook("phone", false))
	requireExtension(t, a.Close())
	b := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), mod)...)
	defer b.Close()
	requireExtension(t, b.Open(root))
	if b.extensions.Own("phone") || pref.Get() != "book value" {
		t.Fatal("book persistence lost")
	}
	if on, ok := b.extensions.BookOverride("phone"); !ok || on {
		t.Fatal("override lost")
	}
	data, err := os.ReadFile(filepath.Join(root, "ext.json"))
	requireExtension(t, err)
	if !strings.Contains(string(data), "book value") {
		t.Fatal(string(data))
	}
}

func TestUnreadableExtensionFileIsEmptyReportedAndNotRewritten(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "book"}[book], func(t *testing.T) {
			config := filepath.Join(t.TempDir(), "providers.yaml")
			root := extensionBook(t)
			file := filepath.Join(filepath.Dir(config), "ext.json")
			if book {
				file = filepath.Join(root, "ext.json")
			}
			damaged := []byte("{ broken JSON")
			requireExtension(t, os.WriteFile(file, damaged, 0644))
			a := newTestApp(ai.New(config), testModules()...)
			defer a.Close()
			if book {
				requireExtension(t, a.Open(root))
			}
			tt := stillTester(a.View, 1266, 780)
			tt.Frame()
			if !a.extensions.Own("phone") || a.errorText() == "" {
				t.Fatal("bad file not empty or unreported")
			}
			data, err := os.ReadFile(file)
			requireExtension(t, err)
			if string(data) != string(damaged) {
				t.Fatal("bad file overwritten on read")
			}
			if book {
				requireExtension(t, a.extensions.SetBook("phone", false))
			} else {
				requireExtension(t, a.extensions.Set("phone", false))
			}
			data, err = os.ReadFile(file)
			requireExtension(t, err)
			if !json.Valid(data) {
				t.Fatal("explicit save did not replace damaged file")
			}
		})
	}
}

func TestLegacyPhoneTypesettingMovesOnceWithoutReplacingExistingValue(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "move", true: "keep"}[existing], func(t *testing.T) {
			dir := t.TempDir()
			config := filepath.Join(dir, "providers.yaml")
			legacy := json.RawMessage("{\"mode\": \"scroll\", \"size\": 23, \"unknown\": true}")
			works := filepath.Join(dir, "works.json")
			requireExtension(t, os.WriteFile(works, append(append([]byte("{\"schema\":1,\"phone\":"), legacy...), '}'), 0644))
			old := json.RawMessage("{\"size\":19}")
			if existing {
				s := newExtensionStore(dir, func(err error) { t.Fatal(err) })
				s.Save(ext.Book{}, "phone.typesetting", old)
			}
			a := newApp(config)
			defer a.Close()
			data, ok := a.extStore.Load(ext.Book{}, "phone.typesetting")
			if !ok {
				t.Fatal("migration missing")
			}
			var got, want any
			requireExtension(t, json.Unmarshal(data, &got))
			value := legacy
			if existing {
				value = old
			}
			requireExtension(t, json.Unmarshal(value, &want))
			if !reflect.DeepEqual(got, want) {
				t.Fatal(string(data))
			}
			data, err := os.ReadFile(works)
			requireExtension(t, err)
			var saved map[string]any
			requireExtension(t, json.Unmarshal(data, &saved))
			if _, ok := saved["phone"]; ok {
				t.Fatal("legacy phone field retained")
			}
		})
	}
}

func TestBookSwitchStopsAndStartsEachSessionOnceAndShelfCloses(t *testing.T) {
	var calls []string
	mod := ext.Module{ID: "count", Setup: func(r *ext.Registrar) {
		r.OnStart(func(s *ext.Session) error { calls = append(calls, "+"+s.Book().Dir); return nil })
		r.OnStop(func(s *ext.Session) { calls = append(calls, "-"+s.Book().Dir) })
	}}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	first, second := extensionBook(t), extensionBook(t)
	calls = nil
	requireExtension(t, a.Open(first))
	if !reflect.DeepEqual(calls, []string{"-", "+" + first}) {
		t.Fatal(calls)
	}
	calls = nil
	requireExtension(t, a.Open(second))
	if !reflect.DeepEqual(calls, []string{"-" + first, "+" + second}) {
		t.Fatal(calls)
	}
	calls = nil
	a.closeWork()
	if !reflect.DeepEqual(calls, []string{"-" + second, "+"}) {
		t.Fatal(calls)
	}
	calls = nil
	requireExtension(t, a.Open(second))
	if !reflect.DeepEqual(calls, []string{"-", "+" + second}) {
		t.Fatal("reopening closed book lost session", calls)
	}
}

func TestExtensionCommandsUseKeysHelpMenusAndCallIsolation(t *testing.T) {
	ran := 0
	mod := ext.Module{ID: "keys", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Commands, "act", hostapi.Command{
			Name: func() string { return "Extension action" }, Run: func() { ran++ }, Key: ui.KeyK, Mods: ui.Ctrl, OnMac: true, MacKey: ui.KeyK, MacMods: ui.Super, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView, After: "pane"},
		})
	}}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	mods := ui.Ctrl
	if onMacOS {
		mods = ui.Super
	}
	tt.Key(mods, ui.KeyK)
	tt.Frame()
	if ran != 1 {
		t.Fatal("extension shortcut not dispatched", ran)
	}
	a.QueueSystemMenu("keys.act")
	tt.Frame()
	if ran != 2 {
		t.Fatal("system menu not dispatched", ran)
	}
	a.keysOpen = true
	tt.Frame()
	if !tt.HasText("Extension action") {
		t.Fatal("extension missing from help")
	}
	requireExtension(t, a.extensions.Set("keys", false))
	tt.Frame()
	if tt.HasText("Extension action") {
		t.Fatal("disabled command remains in help")
	}
}

func TestInjectionOrderAndChoicePersistInLocalExtensionFile(t *testing.T) {
	config := filepath.Join(t.TempDir(), "providers.yaml")
	target := ext.HookKey[int, int]("base.value")
	base := ext.Module{ID: "base", Setup: func(r *ext.Registrar) { ext.DeclareHook(r, target) }}
	injector := ext.Module{ID: "injector", Requires: []string{"base"}, Setup: func(r *ext.Registrar) {
		ext.Before(r, target, "first", func(v *int) { *v++ })
		ext.Before(r, target, "second", func(v *int) { *v *= 2 })
		ext.Overwrite(r, target, "one", func(v int) int { return v + 10 })
		ext.Overwrite(r, target, "two", func(v int) int { return v + 20 })
	}}
	a := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), base, injector)...)
	requireExtension(t, a.extensions.SetOrder(string(target), []string{"injector.second", "injector.first"}))
	requireExtension(t, a.extensions.Choose(string(target), "injector.two"))
	requireExtension(t, a.Close())
	b := newTestApp(ai.New(config), append(append([]ext.Module{}, testModules()...), base, injector)...)
	defer b.Close()
	got, err := ext.Call(b.extensions, target, 3, func(v int) int { return v })
	requireExtension(t, err)
	if got != 27 {
		t.Fatal("persisted order or choice lost", got)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(config), "ext.json"))
	requireExtension(t, err)
	var file extensionFile
	requireExtension(t, json.Unmarshal(data, &file))
	p := file.Injections[string(target)]
	if p.Choice != "injector.two" || !reflect.DeepEqual(p.Order, []string{"injector.second", "injector.first"}) {
		t.Fatal(p)
	}
}

func TestExtensionCommandPanicIsReportedAndDoesNotCrash(t *testing.T) {
	mod := ext.Module{ID: "broken", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Commands, "act", hostapi.Command{Name: func() string { return "Broken" }, Run: func() { panic("command failed") }, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView}})
	}}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	tt := stillTester(a.View, 1266, 780)
	tt.Frame()
	a.QueueSystemMenu("broken.act")
	tt.Frame()
	tt.Frame()
	if a.extensions.Ledger("broken").State != ext.Failed || !strings.Contains(a.errorText(), "command failed") {
		t.Fatal("command failure not isolated")
	}
}

func TestContributedPaneTabsKeepAssemblyRegistrationOrderAndUnknownPrefsFallBack(t *testing.T) {
	makeModule := func(id string) ext.Module {
		return ext.Module{ID: id, Setup: func(r *ext.Registrar) {
			for _, tab := range []string{"one", "two"} {
				ext.Contribute(r, hostapi.PaneTabs, tab, hostapi.PaneTab{Title: func() string { return tab }})
			}
		}}
	}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), makeModule("second"), makeModule("first"))...)
	defer a.Close()
	var ids []paneTab
	for _, p := range a.paneTabs {
		if !strings.HasPrefix(string(p.id), "second.") && !strings.HasPrefix(string(p.id), "first.") {
			continue
		}
		ids = append(ids, p.id)
	}
	if !reflect.DeepEqual(ids, []paneTab{"second.one", "second.two", "first.one", "first.two"}) {
		t.Fatal(ids)
	}
	root := extensionBook(t)
	requireExtension(t, a.Open(root))
	a.doc.tab = "first.two"
	requireExtension(t, a.savePrefs())
	a.loadPrefs()
	if a.doc.tab != "first.two" {
		t.Fatal("contributed ID not restored")
	}
	requireExtension(t, a.extensions.Set("first", false))
	a.syncExtensions()
	a.loadPrefs()
	if a.doc.tab != paneMargin {
		t.Fatal("unknown ID did not fall back", a.doc.tab)
	}
}

func TestExtensionPlatformOnlyShortcutsUseEffectiveBinding(t *testing.T) {
	macRuns, otherRuns := 0, 0
	mod := ext.Module{ID: "keys", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, hostapi.Commands, "mac", hostapi.Command{Name: func() string { return "Mac action" }, OnMac: true, MacKey: ui.KeyK, MacMods: ui.Super, Run: func() { macRuns++ }})
		ext.Contribute(r, hostapi.Commands, "other", hostapi.Command{Name: func() string { return "Other action" }, Key: ui.KeyK, Mods: ui.Ctrl, OnMac: true, Run: func() { otherRuns++ }})
	}}
	old := onMacOS
	defer func() { onMacOS = old }()
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	tt := stillTester(a.View, 1266, 780)
	onMacOS = true
	tt.Frame()
	tt.Key(ui.Super, ui.KeyK)
	tt.Key(ui.Ctrl, ui.KeyK)
	if macRuns != 1 || otherRuns != 0 {
		t.Fatal("macOS binding not respected", macRuns, otherRuns)
	}
	onMacOS = false
	tt.Frame()
	tt.Key(ui.Super, ui.KeyK)
	tt.Key(ui.Ctrl, ui.KeyK)
	if macRuns != 1 || otherRuns != 1 {
		t.Fatal("other platform binding not respected", macRuns, otherRuns)
	}
}

func TestUnchangedFramesReuseExtensionTables(t *testing.T) {
	a, tt, _ := writingFixture(t, "灯灭了。")
	commands, tabs, system, view := a.extensionCommands(), a.paneTabs, a.systemCommands(), a.viewCommands()
	list, help := a.commandList(), a.commandHelp()
	file, overlays := a.fileCommands(), a.extViews.overlays
	changes := a.extensions.Changes()
	for range 6 {
		tt.Frame()
		a.SystemMenu()
		if &file[0] != &a.fileCommands()[0] || &overlays[0] != &a.extViews.overlays[0] || a.extensions.Changes() != changes || &commands[0] != &a.extensionCommands()[0] || commands[0].cmd != a.extensionCommands()[0].cmd || &tabs[0] != &a.paneTabs[0] || &system[0] != &a.systemCommands()[0] || &view[0] != &a.viewCommands()[0] || &list[0] != &a.commandList()[0] || &help[0] != &a.commandHelp()[0] {
			t.Fatal("unchanged frame rebuilt extension tables")
		}
	}
	if allocs := testing.AllocsPerRun(50, func() {
		a.extensionCommands()
		a.commandList()
		a.commandHelp()
		a.systemCommands()
		a.viewCommands()
		a.fileCommands()
		a.syncExtensions()
	}); allocs != 0 {
		t.Fatal("cached reads allocated", allocs)
	}
	requireExtension(t, a.extensions.Set("phone", false))
	tt.Frame()
	if len(a.extensionCommands()) != 1 || a.extensionCommands()[0].id != "export.open" {
		t.Fatal("changed revision did not refresh tables")
	}
	requireExtension(t, a.extensions.Set("phone", true))
	tt.Frame()
	if &commands[0] == &a.extensionCommands()[0] {
		t.Fatal("new revision reused stale command table")
	}
}

func TestPhoneMigrationImmediatelyRemovesLegacyShelfField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "works.json")
	requireExtension(t, os.WriteFile(path, []byte("{\"schema\":1,\"phone\":{\"size\":23}}"), 0644))
	a := newApp(filepath.Join(dir, "providers.yaml"))
	defer a.Close()
	data, err := os.ReadFile(path)
	requireExtension(t, err)
	var saved map[string]json.RawMessage
	requireExtension(t, json.Unmarshal(data, &saved))
	if _, ok := saved["phone"]; ok {
		t.Fatal("migration waited for another shelf save")
	}
	if value := savedTypesetting(t, a); value.Size != 23 {
		t.Fatal(value)
	}
}

func TestExtensionFailureToastsUseLocalizedNamesAndOriginalErrors(t *testing.T) {
	for _, lang := range []string{"zh-Hans", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", lang)
			name := func() string { return "阅读预览" }
			mod := ext.Module{ID: "broken", Name: name, Setup: func(r *ext.Registrar) {
				ext.Contribute(r, hostapi.Commands, "fail", hostapi.Command{Name: name, Run: func() { panic("original failure") }, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView}})
			}}
			a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
			defer a.Close()
			tt := stillTester(a.View, 1266, 780)
			tt.Frame()
			a.QueueSystemMenu("broken.fail")
			tt.Frame()
			tt.Frame()
			if a.errorText() != locales.ExtFailed(name(), "original failure") {
				t.Fatal(a.errorText())
			}
			ledger := a.extensions.Ledger("broken")
			if len(ledger.Errors) != 1 || ledger.Errors[0].Target == "" || ledger.Errors[0].Stage == "" {
				t.Fatal("technical location missing from ledger")
			}
		})
	}
}

func TestUnreadableExtensionFileUsesLocalizedDefaultNotice(t *testing.T) {
	for _, lang := range []string{"zh-Hans", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", lang)
			dir := t.TempDir()
			path := filepath.Join(dir, "ext.json")
			data := []byte("{broken")
			requireExtension(t, os.WriteFile(path, data, 0644))
			var raw any
			cause := json.Unmarshal(data, &raw)
			var notices []error
			s := newExtensionStore(dir, func(err error) { notices = append(notices, err) })
			s.Load(ext.Book{}, "phone.typesetting")
			if len(notices) != 1 || notices[0].Error() != locales.ExtFileUnreadable(path+": "+cause.Error()) {
				t.Fatal(notices)
			}
		})
	}
}

func TestTitleAndSystemViewMenusShareExtensionOrdering(t *testing.T) {
	mod := ext.Module{ID: "menus", Setup: func(r *ext.Registrar) {
		for _, value := range []struct{ id, after string }{{"first", "focus"}, {"before", "pane"}, {"second", "focus"}, {"zoom", "zoom-reset"}, {"last", "unknown"}} {
			ext.Contribute(r, hostapi.Commands, value.id, hostapi.Command{Name: func() string { return value.id }, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView, After: value.after}})
		}
	}}
	a := newTestApp(ai.New(""), append(append([]ext.Module{}, testModules()...), mod)...)
	defer a.Close()
	var expected, title, system []string
	known := map[string]bool{}
	for _, row := range a.viewCommands() {
		if row.cmd != nil {
			expected = append(expected, row.cmd.name(a))
			known[row.id] = true
		}
	}
	menu := &kit.Menu{}
	a.viewMenu(menu, func(f func()) func() { return f })
	for _, row := range menu.Rows {
		if row.Kind == kit.RowItem {
			title = append(title, row.Label)
		}
	}
	for _, root := range a.SystemMenu() {
		if root.ID == "view" {
			for _, row := range root.Children {
				if known[row.ID] {
					system = append(system, row.Label)
				}
			}
		}
	}
	if !reflect.DeepEqual(title, expected) || !reflect.DeepEqual(system, expected) {
		t.Fatal("different menu ordering", expected, title, system)
	}
}

func TestOldBookExtensionSettingsAreIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".writer"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".writer", "ext.json"), []byte(`{"switches":{"lore":false},"settings":{"lore.example":42}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store := newExtensionStore(t.TempDir(), func(err error) { t.Fatal(err) })
	b := ext.Book{Dir: root}
	switches, err := (bookExtensions{store}).Load(b)
	if err != nil || len(switches) != 0 {
		t.Fatal(switches, err)
	}
	if data, ok := store.Load(b, "lore.example"); ok {
		t.Fatalf("read legacy settings: %s", data)
	}
}
