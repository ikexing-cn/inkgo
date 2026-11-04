package desk

import (
	"slices"
	"testing"
	"writer/ext/backup/backuptext"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/ext"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
)

func TestSettingsSidebarOrdersAppearanceExtensionsThenHostAndRegisteredPages(t *testing.T) {
	a, tt, _ := fixture(t)
	for _, work := range []bool{true, false} {
		if !work {
			a.closeWork()
		}
		a.openPrefs("")
		tt.Frame()
		if a.settings.tab != "AI" {
			t.Fatal("default page is not AI")
		}
		var labels []string
		for _, tab := range a.prefTabs {
			if tab.available(a) {
				labels = append(labels, tab.label)
				if _, ok := tt.Find(tab.label); !ok {
					t.Fatal("missing sidebar button", tab.label)
				}
			}
		}
		want := []string{"设置-外观", "设置-扩展", "设置-AI"}
		if work {
			want = append(want, "设置-提醒")
		}
		want = append(want, "设置-备份")
		if !slices.Equal(labels, want) {
			t.Fatal(labels, want)
		}
	}
}

func TestBackupSettingsPageRemainsOnShelfWithoutWorkSettings(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("backup.settings")
	tt.Frame()
	if !tt.HasText(backuptext.BackupLocalOnly()) {
		t.Fatal("work settings missing")
	}
	a.closeWork()
	(extensionHost{a}).OpenSettings("backup.settings")
	tt.Frame()
	if _, ok := tt.Find("设置-备份"); !ok {
		t.Fatal("shelf registration missing")
	}
	if a.settings.tab != "backup.settings" || tt.HasText(backuptext.BackupLocalOnly()) || tt.HasText(backuptext.BackupTiming()) {
		t.Fatal(a.settings, tt.Texts())
	}
}

func TestSettingsSharedLabelKeepsOnePageAndOrdersActiveSections(t *testing.T) {
	var drawn []string
	makeModule := func(id string, order int) ext.Module {
		return ext.Module{ID: id, Setup: func(r *ext.Registrar) {
			ext.Contribute(r, hostapi.SettingsPages, "settings", hostapi.SettingsPage{Title: func() string { return id }, Label: "shared", Order: order})
			ext.Contribute(r, hostapi.SettingsSections, "preferences", hostapi.SettingsSection{Draw: func(c *ui.Context) { drawn = append(drawn, id); ui.Text(c, id) }})
		}}
	}
	a := New(ai.New(""), makeModule("late", 30), makeModule("early", 5), makeModule("equal", 5))
	defer a.Close()
	for _, id := range []string{"late.settings", "early.settings", "equal.settings"} {
		(extensionHost{a}).OpenSettings(id)
		if a.settings.tab != "early.settings" {
			t.Fatal("shared ID did not resolve", a.settings)
		}
	}
	tt := stillTester(func(c *ui.Context) { a.extensionSettings(c, a.settings.tab) }, 500, 400)
	for _, on := range []bool{true, false, true} {
		requireExtension(t, a.extensions.Set("early", on))
		a.syncExtensions()
		count := 0
		for _, tab := range a.prefTabs {
			if tab.label == "shared" {
				count++
			}
		}
		if count != 1 || a.prefTabs[2].id != "early.settings" {
			t.Fatal(a.prefTabs)
		}
		drawn = nil
		tt.Frame()
		want := []string{"equal", "late"}
		if on {
			want = append([]string{"early"}, want...)
		}
		if !slices.Equal(drawn, want) {
			t.Fatal(drawn, want)
		}
	}
}

func TestOpenSettingsResolvesBackupAndUnknownIDsAndFooterCard(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨停了。")
	for _, test := range []struct{ id, want string }{{"backup.settings", "backup.settings"}, {"unknown", "extensions"}} {
		(extensionHost{a}).OpenSettings(test.id)
		tt.Frame()
		if a.settings.tab != test.want || !a.settings.open {
			t.Fatal(a.settings)
		}
	}
	a.closePrefs()
	backupOf(a).Configure(archive.Settings{Dir: t.TempDir()})
	tt.Frame()
	click(t, tt, "备份状态")
	click(t, tt, backuptext.BackupSettings())
	if a.settings.tab != "backup.settings" || !a.settings.open {
		t.Fatal(a.settings)
	}
}
