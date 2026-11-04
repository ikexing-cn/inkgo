package desk

import (
	"errors"
	"github.com/egoist/mygo/i18n"
	"os"
	"testing"
	"writer/desk/kit"
	"writer/discussion"
	"writer/ext/ai/aiapi"
	"writer/ext/ai/aitext"
	"writer/locales"
	"writer/model"
)

func TestErrorTranslatorsAreCalledInOrder(t *testing.T) {
	sentinel := errors.New("injected")
	calls := 0
	a := &App{ErrorText: []ErrorTranslator{func(err error, _ func(error) string) (string, bool) { calls++; return "ignored", false }, func(err error, _ func(error) string) (string, bool) { calls++; return "translated", err == sentinel }, func(error, func(error) string) (string, bool) { t.Fatal("called after match"); return "", false }}}
	if got := a.errText(&aiapi.SetupError{Err: sentinel}); got != "translated" || calls != 2 {
		t.Fatalf("%q, calls %d", got, calls)
	}
}

func TestErrorWrappersUseTheSameTranslatorChain(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a := &App{ErrorText: []ErrorTranslator{aitext.Translate}}
	inner := errors.New(aitext.SettingsModelNumbers("x"))
	wrapped := &discussion.ReplyError{Err: inner}
	for _, tc := range []struct {
		err  error
		want string
	}{
		{openBookError{book: "Book", err: model.ErrKeyNewline}, locales.AppOpenBookFailed("Book", aitext.SettingsKeyNewline())},
		{extensionFailedError{name: func() string { return "AI" }, err: wrapped}, locales.ExtFailed("AI", aitext.ChatReplyError(aitext.SettingsModelNumbers("x")))},
		{&model.ConfigError{Op: "read", Err: os.ErrPermission}, aitext.SettingsConfigRead(os.ErrPermission.Error())},
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
