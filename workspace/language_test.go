package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageRoundTripsDefaultsAndRejectsUnknownOnOpen(t *testing.T) {
	for _, language := range []Language{Chinese, English, ""} {
		t.Run(string(language), func(t *testing.T) {
			s, err := Create(t.TempDir(), "Book", "novel", language)
			if err != nil {
				t.Fatal(err)
			}
			dir := s.Root()
			s.Close()
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			want := language
			if want == "" {
				want = "zh-Hans"
			}
			if s.Work().Language != want {
				t.Fatal(s.Work())
			}
			s.Close()
			p := filepath.Join(dir, "work.yaml")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			text := string(b)
			if language == "" {
				text = strings.Replace(text, "language: zh-Hans\n", "", 1)
			}
			if language == "" {
				if err = os.WriteFile(p, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				legacy, err := Open(dir)
				if err != nil || legacy.Work().Language != "zh-Hans" {
					t.Fatal(err)
				}
				legacy.Close()
			}
			text = "language: xx\n" + strings.Join(strings.Split(text, "\n")[1:], "\n")
			if err = os.WriteFile(p, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			_, err = Open(dir)
			var bad *LanguageError
			if !errors.As(err, &bad) || bad.Value != "xx" {
				t.Fatal(err)
			}
		})
	}
}
func TestChineseAndEnglishBooksCountMixedTextByTheirOwnRules(t *testing.T) {
	for _, tc := range []struct {
		text   string
		en, zh int
	}{
		{"Hello, world!", 2, 12}, {"don't re-enter 123", 3, 16}, {"Hello世界 42中文", 6, 11}, {"a--b 'word' -", 3, 11}, {" \n\t", 0, 0},
	} {
		for _, language := range []Language{Chinese, English} {
			w := Work{Language: language}
			want := tc.zh
			if language == "en" {
				want = tc.en
			}
			if got := w.Language.Count(tc.text); got != want {
				t.Errorf("%s %q: %d want %d", language, tc.text, got, want)
			}
		}
	}
}
func TestNewEnglishBookDefaultCategoriesAreEnglish(t *testing.T) {
	s, err := Create(t.TempDir(), "Book", "novel", "en")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cats := s.Work().Categories
	if cats[0].Name != "People" || cats[0].Target != "Lore" || cats[len(cats)-1].Name != "Other" {
		t.Fatal(cats)
	}
	if err = s.RemoveCategory("Other"); err == nil {
		t.Fatal("catch-all removed")
	}
}

func TestReadingMissingOrEmptyLanguageNormalizesBeforeUse(t *testing.T) {
	for _, field := range []string{"", "language: \"\"\n"} {
		t.Run(field, func(t *testing.T) {
			st, err := Create(t.TempDir(), "Book", "novel", Chinese)
			if err != nil {
				t.Fatal(err)
			}
			dir := st.Root()
			st.Close()
			path := filepath.Join(dir, "work.yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := field + strings.TrimPrefix(string(data), "language: zh-Hans\n")
			if err = os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			w, err := ReadWork(dir)
			if err != nil || w.Language != Chinese {
				t.Fatal(w, err)
			}
			opened, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.Close()
			if opened.Work().Language != Chinese {
				t.Fatal(opened.Work())
			}
		})
	}
}
