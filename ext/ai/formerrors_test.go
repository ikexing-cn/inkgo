package ai

import (
	"github.com/egoist/mygo/i18n"
	"testing"
	"writer/ext/ai/aitext"
)

func TestFormErrorsTranslateInBothLocales(t *testing.T) {
	original := i18n.Locale()
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return original }) })
	for _, locale := range []string{"zh-Hans", "en"} {
		tag = locale
		for _, tc := range []struct {
			err  error
			want string
		}{
			{&modelRuleError{id: "x", rule: "numbers"}, aitext.SettingsModelNumbers("x")},
			{&modelRuleError{id: "x", rule: "integer"}, aitext.SettingsModelAtInt("x")},
			{&modelRuleError{id: "x", rule: "output"}, aitext.SettingsModelOutMax("x")},
			{&modelRuleError{id: "x", rule: "threshold"}, aitext.SettingsModelAtRange("x")},
			{&providerIDTakenError{name: "x"}, aitext.SettingsIDTaken("x")},
		} {
			if got := formText(tc.err); got != tc.want || got == tc.err.Error() {
				t.Errorf("%s %T: %q, want %q", tag, tc.err, got, tc.want)
			}
		}
	}
}

func formText(err error) string {
	text, ok := formErrorText(err, nil)
	if !ok {
		return err.Error()
	}
	return text
}
