package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestNewBookLanguageDefaultsToSupportedInterfaceLanguage(t *testing.T) {
	defer i18n.SetLocaleFunc(testLocale)
	a, _, _ := fixture(t)
	for _, tc := range []struct{ locale, want string }{{"zh-Hans", "zh-Hans"}, {"en", "en"}, {"fr", "zh-Hans"}, {"", "zh-Hans"}} {
		i18n.SetLocaleFunc(func() string { return tc.locale })
		a.openCreate()
		if a.books.language != tc.want {
			t.Fatalf("%q: %q", tc.locale, a.books.language)
		}
	}
}

func TestNewBookLanguageChoicePersistsBeforeFirstOpen(t *testing.T) {
	defer i18n.SetLocaleFunc(testLocale)
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	for _, language := range []workspace.Language{workspace.English, workspace.Chinese} {
		t.Run(string(language), func(t *testing.T) {
			a, tt, _ := fixture(t)
			a.closeWork()
			a.openCreate()
			a.name = "Book"
			a.books.at = filepath.Join(t.TempDir(), "Book")
			tt.Frame()
			tt.Frame()
			clickBookLanguage(t, tt, a.books.language)
			name := "English"
			if language == workspace.Chinese {
				name = "简体中文"
			}
			if err := tt.Click(name); err != nil {
				t.Fatal(err, tt.Texts())
			}
			tt.Frame()
			if a.books.language != string(language) {
				t.Fatal(a.books.language)
			}
			click(t, tt, "创建作品")
			if a.Store == nil {
				t.Fatal(a.errorText())
			}
			w := a.Store.Work()
			if w.Language != language || w.Categories[len(w.Categories)-1].Name != language.OtherCategory() {
				t.Fatal(w)
			}
			data, err := os.ReadFile(filepath.Join(a.Store.Root(), "work.yaml"))
			if err != nil || !strings.Contains(string(data), "language: "+string(language)+"\n") {
				t.Fatal(string(data), err)
			}
		})
	}
}

func TestShotNewBookLanguageChineseAndEnglish(t *testing.T) {
	testshot.Dir(t)
	defer i18n.SetLocaleFunc(testLocale)
	for _, locale := range []string{"zh-Hans", "en"} {
		t.Run(locale, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return locale })
			a, tt, _ := fixture(t)
			a.closeWork()
			a.openCreate()
			a.name = "The Last Harbor"
			a.books.at = filepath.Join("/tmp", "The Last Harbor")
			a.PickFolder = func(string) (string, error) { return "", nil }
			shot(t, tt, "new-book-language-"+locale+".png")
			clickBookLanguage(t, tt, a.books.language)
			shot(t, tt, "new-book-language-options-"+locale+".png")
		})
	}
}

func clickBookLanguage(t *testing.T, tt *ui.Tester, language string) {
	t.Helper()
	// Target the select value rather than the field caption.
	name := "简体中文"
	if language == "en" {
		name = "English"
	}
	click(t, tt, name)
}
