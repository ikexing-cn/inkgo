package desk

import (
	"errors"
	"github.com/egoist/mygo/i18n"
	"os"
	"testing"
	"writer/ai/aitext"
	"writer/aiport"
	"writer/desk/kit"
	"writer/discussion"
	"writer/locales"
	"writer/model"
)

func TestDeskErrorsTranslateInBothLocales(t *testing.T) {
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a := &App{}
	for _, locale := range []string{"zh-Hans", "en"} {
		tag = locale
		for _, tc := range []struct {
			err  error
			want string
		}{
			{&modelRuleError{id: "x", rule: "numbers"}, locales.SettingsModelNumbers("x")},
			{&modelRuleError{id: "x", rule: "integer"}, locales.SettingsModelAtInt("x")},
			{&modelRuleError{id: "x", rule: "output"}, locales.SettingsModelOutMax("x")},
			{&modelRuleError{id: "x", rule: "threshold"}, locales.SettingsModelAtRange("x")},
			{&providerIDTakenError{name: "x"}, locales.SettingsIDTaken("x")},
		} {
			if got := a.errText(tc.err); got != tc.want || got == tc.err.Error() {
				t.Errorf("%s %T: %q, want %q", tag, tc.err, got, tc.want)
			}
		}
	}
}

func TestErrorTranslatorsAreCalledInOrder(t *testing.T) {
	sentinel := errors.New("injected")
	calls := 0
	a := &App{ErrorText: []ErrorTranslator{func(err error, _ func(error) string) (string, bool) { calls++; return "ignored", false }, func(err error, _ func(error) string) (string, bool) { calls++; return "translated", err == sentinel }, func(error, func(error) string) (string, bool) { t.Fatal("called after match"); return "", false }}}
	if got := a.errText(&aiport.SetupError{Err: sentinel}); got != "translated" || calls != 2 {
		t.Fatalf("%q, calls %d", got, calls)
	}
}

func TestErrorWrappersUseTheSameTranslatorChain(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a := &App{ErrorText: []ErrorTranslator{aitext.Translate}}
	inner := &modelRuleError{id: "x", rule: "numbers"}
	wrapped := &discussion.ReplyError{Err: inner}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{openBookError{book: "Book", err: model.ErrKeyNewline}, locales.AppOpenBookFailed("Book", locales.SettingsKeyNewline())},
		{extensionFailedError{name: func() string { return "AI" }, err: wrapped}, locales.ExtFailed("AI", locales.ChatReplyError(locales.SettingsModelNumbers("x")))},
		{&model.ConfigError{Op: "read", Err: os.ErrPermission}, locales.SettingsConfigRead(os.ErrPermission.Error())},
	} {
		if got := a.errText(tc.err); got != tc.want {
			t.Errorf("%T: %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestUnknownErrorsKeepOriginalText(t *testing.T) {
	a := &App{ErrorText: []ErrorTranslator{aitext.Translate}}
	for _, err := range []error{errors.New("unknown failure"), os.ErrPermission} {
		if a.errText(err) != err.Error() {
			t.Fatal(err)
		}
	}
	if a.errText(nil) != "" {
		t.Fatal("nil error has text")
	}
}

func TestStatusTranslationSurvivesClearAndDeduplicatesShownText(t *testing.T) {
	a := &App{ErrorText: []ErrorTranslator{aitext.Translate}}
	s := kit.Status{Translate: a.errText}
	s.Clear()
	s.Fail(model.ErrKeyNewline)
	s.Fail(model.ErrKeyNewline)
	if s.ErrorText() != a.errText(model.ErrKeyNewline) || s.Repeated() != 2 {
		t.Fatalf("%q repeated %d", s.ErrorText(), s.Repeated())
	}
}
