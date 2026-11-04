package outline

import (
	"github.com/egoist/mygo/i18n"
	"testing"
	"writer/ext/outline/outlinetext"
)

func TestNewStorylineUsesInterfaceLanguage(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	a, tt, _ := outlineFixture(t)
	click(t, tt, outlinetext.PlanLineAdd())
	lines := outline(a).Lines
	if got := lines[len(lines)-1].Name; got != "New storyline" {
		t.Fatal(got)
	}
}
