package desk

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
	"writer/ext"
	"writer/ext/draft/drafttext"
	"writer/ext/propose/proposetext"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/internal/testshot"
	"writer/locales"
)

func TestTalkCascadeNamesDraftAndProposeAndRestoresOwnSwitches(t *testing.T) {
	for _, book := range []bool{false, true} {
		for _, draftOff := range []bool{false, true} {
			a, tt, _ := fixture(t)
			if draftOff {
				a.setExtension("draft", false, book)
			}
			a.openPrefs("extensions")
			a.flipExtension("talk", false, book)
			tt.Frame()
			if !a.settings.askExt || !tt.HasText(locales.ModAskOffAbout(a.extensionNames(a.extViews.moduleViews["talk"].stops))) {
				t.Fatal("missing cascade question", tt.Texts())
			}
			about := a.extensionNames(a.extViews.moduleViews["talk"].stops)
			if !strings.Contains(about, proposetext.ModPropose()) || (!draftOff && !strings.Contains(about, drafttext.ModDraft())) {
				t.Fatal(about)
			}
			click(t, tt, "扩展确认开关")
			for _, id := range []string{"talk", "draft", "propose"} {
				if a.extensions.Enabled(id) {
					t.Fatal(id, "remained enabled")
				}
			}
			a.setExtension("talk", true, book)
			tt.Frame()
			if !a.extensions.Enabled("propose") || a.extensions.Enabled("draft") == draftOff {
				t.Fatal("own switches lost")
			}
		}
	}
}

func TestTalkEmptyPageWithoutLore(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en"} {
		t.Run(locale, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return locale })
			defer i18n.SetLocaleFunc(testLocale)
			a, tt, _ := fixture(t) // This book has lore: disabling it must still use the starting page.
			showPage(a, pageTalk)
			tt.Frame()
			if !tt.HasText(talktext.TalkContinueTitle()) || !tt.HasText(talktext.TalkContinueAbout()) {
				t.Fatal("lore-on page changed", tt.Texts())
			}
			a.setExtension("lore", false, false)
			tt.Frame()
			for _, text := range []string{talktext.TalkStartTitle(), talktext.TalkStartAboutNoLore(), talktext.TalkStart1(), talktext.TalkStart2(), talktext.TalkStart3()} {
				if !tt.HasText(text) {
					t.Fatal("missing", text, tt.Texts())
				}
			}
			before, after, _ := strings.Cut(talktext.TalkLoreSuggest("\x01"), "\x01")
			if !tt.HasText(before) || !tt.HasText(after) || a.hasPane() || a.paneOpen() {
				t.Fatal("suggestion or pane", tt.Texts())
			}
			name, ok := (extensionHost{a}).Name("lore")
			if !ok {
				t.Fatal("missing lore name")
			}
			click(t, tt, name)
			if !a.settings.open || a.settings.tab != "extensions" || a.settings.ext != "lore" {
				t.Fatal("wrong settings destination", a.settings)
			}
			a.closePrefs()
			tt.Frame()
			click(t, tt, talktext.TalkStart1())
			if talkOf(a).Input != talktext.TalkStart1() {
				t.Fatal("starter did not fill composer")
			}
			a.setExtension("lore", true, false)
			tt.Frame()
			if !tt.HasText(talktext.TalkContinueTitle()) || tt.HasText(talktext.TalkStartAboutNoLore()) {
				t.Fatal("lore-on page did not return")
			}
		})
	}
}

func TestShotTalkWithoutLoreAndDependencies(t *testing.T) {
	testshot.Dir(t)
	for _, locale := range []string{"zh-Hans", "en"} {
		i18n.SetLocaleFunc(func() string { return locale })
		a, tt, _ := fixture(t)
		showPage(a, pageTalk)
		a.setExtension("lore", false, false)
		shot(t, tt, "talk-no-lore-"+locale+".png")
		a.openPrefs("extensions")
		for _, id := range []string{"draft", "propose", "talk"} {
			a.settings.ext = id
			shot(t, tt, "settings-ext-"+id+"-"+locale+".png")
		}
	}
	i18n.SetLocaleFunc(testLocale)
}

func TestTalkOwnsCollaborationDeclarations(t *testing.T) {
	a, _, _ := fixture(t)
	ledger := a.extensions.Ledger("talk")
	if len(ledger.Calls) < 7 {
		t.Fatal(ledger.Calls)
	}
	for _, id := range []string{"draft", "propose"} {
		ledger := a.extensions.Ledger(id)
		if len(ledger.Reads) != 0 {
			t.Fatal(id, ledger.Reads)
		}
		found := false
		for _, service := range ledger.Uses {
			if service.Key == string(talkapi.Service) {
				found = true
			}
		}
		if !found {
			t.Fatal(id, "does not use talk service", ledger)
		}
	}
	if len(ext.Items(a.extensions, talkapi.Proposals)) != 1 {
		t.Fatal("proposal handoff missing")
	}
}

// Services must enter the declared operations, including when an injector
// replaces the action before it can navigate or start a model request.
func TestTalkServiceDispatchesThroughOwnedOperations(t *testing.T) {
	base, _, _ := fixture(t)
	var starts int
	var opened string
	injector := ext.Module{ID: "observer", Requires: []string{"talk"}, Setup: func(r *ext.Registrar) {
		start := ext.CallPoint[talkapi.TalkStarter, talkapi.StartTalk, bool]{Point: talkapi.TalkStarters, Operation: "Start"}
		ext.Overwrite(r, ext.HookOf(start, "talk.starter"), "start", func(talkapi.StartTalk) bool { starts++; return true })
		open := ext.CallPoint[talkapi.TalkStarter, string, struct{}]{Point: talkapi.TalkStarters, Operation: "Open"}
		ext.Overwrite(r, ext.HookOf(open, "talk.starter"), "open", func(id string) struct{} { opened = id; return struct{}{} })
	}}
	a := newTestApp(base.ai, append(testModules(), injector)...)
	defer a.Close()
	service, ok := ext.Lookup(a.extensions, talkapi.Service)
	if !ok {
		t.Fatal("talk service missing")
	}
	if !service.Start(talkapi.StartTalk{}) || starts != 1 {
		t.Fatal("Start bypassed injection")
	}
	service.Open("saved-talk")
	if opened != "saved-talk" {
		t.Fatal("Open bypassed injection")
	}
}
