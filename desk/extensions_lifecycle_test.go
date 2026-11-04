package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"writer/ext/backup/backuptext"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/ext"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
	"writer/shelf"
)

func TestDeskAssemblyIncludesBackupWithoutFirstOpenQuestion(t *testing.T) {
	a, tt, _ := fixture(t)
	if a.extensions.Ledger("backup").State != ext.On || backupOf(a).State.Ask || tt.HasText("作品目前仅保存在本机") {
		t.Fatal("desk test assembly must install quiet backups")
	}
}

func TestBackupConstructorKeepsHostViewsIndependentWithoutAProbeService(t *testing.T) {
	a, _, _ := fixture(t)
	b, _, _ := fixture(t)
	first, second := backupOf(a), backupOf(b)
	if first == second {
		t.Fatal("hosts share a backup view")
	}
	first.OpenAsk()
	if !first.State.Ask || second.State.Ask {
		t.Fatal("view state escaped its assembly")
	}
	if len(a.extensions.Ledger("backup").Provides) != 0 || len(b.extensions.Ledger("backup").Provides) != 0 {
		t.Fatal("backup exposes a test-only service")
	}
}

func TestExtensionSettingsFollowHostPagesThenStableOrderAndCache(t *testing.T) {
	mod := ext.Module{ID: "pages", Setup: func(r *ext.Registrar) {
		for _, p := range []struct {
			id    string
			order int
		}{{"late", 30}, {"first", 10}, {"second", 10}} {
			ext.Contribute(r, hostapi.SettingsPages, p.id, hostapi.SettingsPage{Title: func() string { return p.id }, Label: p.id, Order: p.order})
			ext.Contribute(r, hostapi.SettingsSections, p.id+"-content", hostapi.SettingsSection{Draw: func(c *ui.Context) { ui.Text(c, p.id) }})
		}
	}}
	a := newTestApp(ai.New(""), mod)
	defer a.Close()
	want := []string{"appearance", "extensions", "AI", "pages.first", "pages.second", "reminders", "pages.late"}
	for i, id := range want {
		if a.prefTabs[i].id != id {
			t.Fatal(a.prefTabs)
		}
	}
	first := &a.prefTabs[0]
	pages, sections := &a.extViews.settings[0], &a.extViews.sections[0]
	a.syncExtensions()
	a.syncExtensions()
	if first != &a.prefTabs[0] || pages != &a.extViews.settings[0] || sections != &a.extViews.sections[0] {
		t.Fatal("unchanged host rebuilt derived settings")
	}
	requireExtension(t, a.extensions.Set("pages", false))
	a.syncExtensions()
	if len(a.prefTabs) != len(want) || len(a.extViews.settings) != 4 || len(a.extViews.sections) != 1 {
		t.Fatal("disabled pages disappeared")
	}
	requireExtension(t, a.extensions.Set("pages", true))
	a.syncExtensions()
	if len(a.prefTabs) != len(want) {
		t.Fatal("pages not restored")
	}
}

func TestNewHostPointsIsolatePanicsAndHidePartialDraws(t *testing.T) {
	for _, kind := range []string{"footer", "title", "tick", "leave", "settings", "pending-key", "pending-group", "pending-action", "overlay-open"} {
		t.Run(kind, func(t *testing.T) {
			boom := func() { panic("broken callback") }
			draw := func(c *ui.Context) {
				ui.Column(c).Children(func() { ui.Text(c, "partial failed contribution"); boom() })
			}
			broken := ext.Module{ID: "broken", Setup: func(r *ext.Registrar) {
				switch kind {
				case "footer":
					ext.Contribute(r, hostapi.FooterItems, "item", hostapi.DrawItem{Draw: draw})
				case "title":
					ext.Contribute(r, hostapi.TitleItems, "item", hostapi.DrawItem{Draw: draw})
				case "tick":
					ext.Contribute(r, hostapi.FrameTicks, "item", hostapi.FrameTick{Tick: func(*ui.Context) { boom() }})
				case "leave":
					ext.Contribute(r, hostapi.BeforeLeave, "item", boom)
				case "settings":
					ext.Contribute(r, hostapi.SettingsPages, "item", hostapi.SettingsPage{Title: func() string { return "Broken" }, Label: "broken"})
					ext.Contribute(r, hostapi.SettingsSections, "preferences", hostapi.SettingsSection{Draw: draw})
				case "pending-key":
					ext.Contribute(r, hostapi.PendingGroups, "item", hostapi.PendingSource{Key: func() string { boom(); return "" }})
				case "pending-group":
					ext.Contribute(r, hostapi.PendingGroups, "item", hostapi.PendingSource{Group: func() (hostapi.PendingGroup, bool) { boom(); return hostapi.PendingGroup{}, false }})
				case "pending-action":
					ext.Contribute(r, hostapi.PendingGroups, "item", hostapi.PendingSource{Group: func() (hostapi.PendingGroup, bool) {
						return hostapi.PendingGroup{Items: []hostapi.PendingItem{{Key: "broken.item", Open: func(ui.Services) { boom() }}}}, true
					}})
				case "overlay-open":
					ext.Contribute(r, hostapi.Overlays, "item", hostapi.Overlay{Open: func() bool { boom(); return false }})
				}
			}}
			a := newTestApp(ai.New(""), broken, ext.Module{ID: "healthy"})
			defer a.Close()
			tt := stillTester(func(c *ui.Context) {
				switch kind {
				case "footer":
					a.extensionFooter(c)
				case "title":
					a.extensionTitle(c)
				case "tick":
					a.extensionTicks(c)
				case "leave":
					a.extensionLeave()
				case "settings":
					a.extensionSettings(c, "broken.item")
				case "pending-key":
					a.extensionPendingKey()
				case "pending-group":
					a.extensionPendingGroups()
				case "pending-action":
					for _, g := range a.extensionPendingGroups() {
						for _, it := range g.items {
							it.open(c.Services())
						}
					}
				case "overlay-open":
					a.extensionModalOpen()
				}
			}, 500, 400)
			tt.Frame()
			if a.extensions.Ledger("broken").State != ext.Failed || a.extensions.Ledger("healthy").State != ext.On {
				t.Fatal("failure escaped owner")
			}
			if tt.HasText("partial failed contribution") {
				t.Fatal("failed subtree remained visible")
			}
		})
	}
}

func TestBackupSwitchRemovesAndRestoresEverySurfaceAndLifecycle(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨停了。")
	dir := t.TempDir()
	backupOf(a).Configure(archive.Settings{Dir: dir, OnClose: true, Daily: true})
	for _, on := range []bool{false, true, false, true} {
		requireExtension(t, a.extensions.Set("backup", on))
		tt.Frame()
		a.syncExtensions()
		_, line := tt.Find("备份状态")
		if line != on {
			t.Fatalf("footer on=%v visible=%v", on, line)
		}
		if (len(a.extViews.footer) > 0) != on || (len(a.extViews.title) > 0) != on || (len(a.extViews.pending) > 0) != on || hasOwner(a.extViews.sections, "backup") != on || hasOwner(a.extViews.ticks, "backup") != on || hasOwner(a.extViews.leave, "backup") != on {
			t.Fatalf("stale contribution cache: on=%v", on)
		}
		if on {
			waitBackup(t, a, tt)
			backupOf(a).RecordResult(archive.Settings{Dir: dir}, os.ErrNotExist)
			tt.Frame()
			if len(a.pendingGroups()) != 1 {
				t.Fatal("pending group not restored")
			}
			if _, ok := tt.Find("备份提示"); !ok {
				t.Fatal("warning not restored")
			}
		} else {
			if len(a.pendingGroups()) != 0 {
				t.Fatal("disabled pending group visible")
			}
			if _, ok := tt.Find("备份提示"); ok {
				t.Fatal("disabled warning visible")
			}
			before := zips(t, dir)
			a.extensionLeave()
			if zips(t, dir) != before {
				t.Fatal("disabled leave hook ran")
			}
		}
		a.openPrefs("backup.settings")
		tt.Frame()
		_, tab := tt.Find("设置-备份")
		if !tab {
			t.Fatal("disabled settings page disappeared")
		}
		if tt.HasText(backuptext.BackupTiming()) != on {
			t.Fatal("stale backup settings content", tt.Texts())
		}
		a.closePrefs()
		tt.Frame()
	}
	backupOf(a).State.Policy.ForgetFailure()
	a.extensionLeave()
	if zips(t, dir) != 1 {
		t.Fatal("restored leave hook did not run")
	}
}

func TestBackupFailureIgnoreIsClearedOnlyAfterSuccess(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨停了。")
	s := archive.Settings{Dir: t.TempDir()}
	backupOf(a).Configure(s)
	backupOf(a).RecordResult(s, os.ErrNotExist)
	tt.Frame()
	groups := a.pendingGroups()
	if len(groups) != 1 {
		t.Fatal(groups)
	}
	key := groups[0].items[0].key
	a.write.ignored[key] = true
	a.write.ignoredVer++
	a.savePrefs()
	tt.Frame()
	if len(a.pendingGroups()) != 0 {
		t.Fatal("ignored failure still visible")
	}
	backupOf(a).RecordResult(s, os.ErrPermission)
	if !a.write.ignored[key] {
		t.Fatal("failure cleared ignore")
	}
	backupOf(a).RecordResult(s, nil)
	tt.Frame()
	if a.write.ignored[key] {
		t.Fatal("success retained ignore")
	}
	backupOf(a).RecordResult(s, os.ErrNotExist)
	tt.Frame()
	if len(a.pendingGroups()) != 1 {
		t.Fatal("next failure suppressed")
	}
	prefs, err := os.ReadFile(filepath.Join(a.Store.Root(), ".writer", "desk.json"))
	if err != nil || strings.Contains(string(prefs), key) {
		t.Fatal("ignore not cleared on disk", err)
	}
}

func TestLegacyBookArchiveSettingsMigrateOnceWithoutReplacingExistingFolders(t *testing.T) {
	root := t.TempDir()
	bookA := extensionBook(t)
	bookB := extensionBook(t)
	local := newExtensionStore(root, func(err error) { t.Fatal(err) })
	savedPath := bookA + string(os.PathSeparator) + "."
	existing, _ := json.Marshal(map[string]archive.Settings{savedPath: {Dir: "keep", Keep: 90}})
	local.Save(ext.Book{}, "backup.choices", existing)
	legacy, _ := json.Marshal(archive.Settings{Dir: "old", OnClose: true, Daily: true, Keep: 10, Last: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)})
	old := shelf.Shelf{Books: []shelf.Book{{Path: bookA, MovedBackup: legacy}, {Path: bookB, MovedBackup: legacy}}}
	path := filepath.Join(root, "works.json")
	if err := old.Save(path); err != nil {
		t.Fatal(err)
	}
	a := testApp(ai.New(filepath.Join(root, "providers.yaml")))
	defer a.Close()
	data, ok := a.extStore.Load(ext.Book{}, "backup.choices")
	if !ok {
		t.Fatal("settings missing")
	}
	var choices map[string]archive.Settings
	if err := json.Unmarshal(data, &choices); err != nil {
		t.Fatal(err)
	}
	if choices[savedPath].Dir != "keep" || choices[bookB].Dir != "old" || !choices[bookB].OnClose || !choices[bookB].Daily || choices[bookB].Keep != 10 || choices[bookB].Last.IsZero() {
		t.Fatal(choices)
	}
	cleaned, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(cleaned), "backup") {
		t.Fatal(string(cleaned), err)
	}
	if changed, err := a.shelf.MigrateExtensions(a.extStore.migrate, a.extStore.mergeMigration); err != nil || changed {
		t.Fatal("migration repeated", changed, err)
	}
	requireExtension(t, a.Open(bookB))
	if s, ok := backupOf(a).Settings(); !ok || s.Dir != "old" || s.Keep != 10 {
		t.Fatal(s, ok)
	}
	// The persisted extension setting remains local, outside the book.
	if _, err := os.Stat(filepath.Join(bookB, "ext.json")); !os.IsNotExist(err) {
		t.Fatal("local choices leaked into book", err)
	}
}

func hasOwner[T any](items []ext.Item[T], owner string) bool {
	for _, item := range items {
		if item.Owner == owner {
			return true
		}
	}
	return false
}
