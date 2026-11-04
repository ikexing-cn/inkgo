package desk

import (
	"reflect"
	"slices"
	"testing"
	"writer/ext/backup/backuptext"
	"writer/ext/lore/loretext"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/internal/testshot"
	"writer/locales"
	"writer/shelf"
)

func TestAppearanceLanguageBelowColorListsOnlyAppLanguagesAndShowsSavedChoice(t *testing.T) {
	a, tt, _ := fixture(t)
	values, names := languageOptions()
	if !reflect.DeepEqual(values, []string{"", "zh-Hans", "en"}) || !reflect.DeepEqual(names, []string{"跟随系统", "简体中文", "English"}) {
		t.Fatal(values, names)
	}
	a.openPrefs("appearance")
	calls := 0
	a.SetLocale = func(string) { calls++ }
	for _, tag := range values {
		a.shelf.Locale = tag
		tt.Frame()
		color, ok := tt.Find("颜色")
		if !ok {
			t.Fatal("missing color row")
		}
		language, ok := tt.Find("语言")
		if !ok || language.Y <= color.Y {
			t.Fatal("language must follow color")
		}
		texts := tt.Texts()
		at := slices.Index(texts, "English") - 3
		if at < 0 || at+3 >= len(texts) || !slices.Equal(texts[at+1:at+4], names) {
			t.Fatal(texts)
		}
		clickLanguage(t, tt, names[slices.Index(values, tag)])
		if calls != 0 || a.Locale() != tag {
			t.Fatal("saved choice was not selected", tag, calls)
		}
	}
}

func TestLanguageChoiceCallsHostAndPersistsEnglishThenSystem(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("appearance")
	var calls []string
	a.SetLocale = func(tag string) { calls = append(calls, tag) }
	tt.Frame()
	for _, choice := range []struct{ name, tag string }{{"English", "en"}, {"跟随系统", ""}} {
		clickLanguage(t, tt, choice.name)
		kept, fresh, err := shelf.Load(shelf.File(a.ai.ConfigFile()))
		if err != nil || fresh || kept.Locale != choice.tag || a.Locale() != choice.tag {
			t.Fatal(kept.Locale, fresh, err)
		}
	}
	if !slices.Equal(calls, []string{"en", ""}) {
		t.Fatal(calls)
	}
}

func TestLocaleChangedRefreshesSettingsExtensionsMenusAndCommandsBothWays(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a, tt, _ := marginFixture(t, true)
	var menu []SystemMenuItem
	a.MenuChanged = func(rows []SystemMenuItem) { menu = rows }
	for _, next := range []string{"zh-Hans", "en", "zh-Hans"} {
		tag = next
		a.LocaleChanged()
		a.openPrefs("extensions")
		tt.Frame()
		want := locales.In(tag)
		for _, text := range []string{want.ViewLook(), want.SettingsTabExtensions(), backuptext.In(tag).ModBackup(), loretext.In(tag).ModLore()} {
			if !tt.HasText(text) {
				t.Fatalf("%s missing %q", tag, text)
			}
		}
		all := flattenSystemMenu(menu)
		if all["file"].Label != want.MenuFile() || all["new-chapter"].Label != want.CmdNewChapter() {
			t.Fatal(tag, all)
		}
		found := false
		for _, cmd := range a.commandList() {
			if cmd == cmdNewChapter {
				found = cmd.name(a) == want.CmdNewChapter()
			}
		}
		if !found {
			t.Fatal("command list kept old words", tag)
		}
		for _, note := range a.marginNotes() {
			if note.id == "check-conflict" && note.kind != want.NotesKindInconsistent() {
				t.Fatal("margin cache kept old words", tag, note.kind)
			}
		}
	}
}

// The theme picker also says "follow system"; target the language group.
func clickLanguage(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if name != "跟随系统" {
		click(t, tt, name)
		return
	}
	r, ok := tt.Find("语言")
	if !ok {
		t.Fatal("missing language group")
	}
	tt.ClickAt(r.X+r.W+16, r.Y+r.H/2)
	tt.Frame()
}

func TestShotSettingsLookLanguage(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	a.openPrefs("appearance")
	shot(t, tt, "settings-look-language.png")
}
