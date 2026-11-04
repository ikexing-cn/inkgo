package desk

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"writer/ai"
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/ai/aitext"
	"writer/internal/testshot"
	"writer/model"
	"writer/workspace"
)

func TestAIServiceExistsOnlyWhileExtensionIsOn(t *testing.T) {
	a, _, _ := fixture(t)
	for _, on := range []bool{true, false, true} {
		a.turnAI(on)
		_, ok := a.aiRef.Get()
		if ok != on {
			t.Fatalf("service available=%v, switch=%v", ok, on)
		}
	}
}

func TestAIHasNoBookSwitchAndIgnoresPersistedBookOverride(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := a.extensions.SetBook("ai", false); err == nil {
		t.Fatal("accepted per-book AI switch")
	}
	a.extStore.file(ext.Book{Dir: a.Store.Root()}).Switches = map[string]bool{"ai": false}
	if err := a.extensions.Open(ext.Book{}); err != nil {
		t.Fatal(err)
	}
	if err := a.extensions.Open(ext.Book{Dir: a.Store.Root()}); err != nil {
		t.Fatal(err)
	}
	a.syncExtensions()
	if !a.aiOn() {
		t.Fatal("per-book value disabled prerequisite")
	}
	a.openPrefs("AI")
	tt.Frame()
	if _, ok := tt.Find("ext-more-ai"); ok {
		t.Fatal("AI has a per-book menu")
	}
}

func TestAIToolsPointIsOpenAndFakeToolIsOnContributionLedger(t *testing.T) {
	fake := ext.Module{ID: "faketool", Requires: []string{"ai"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, aiapi.Tools, "lookup", aiapi.Tool{Name: "lookup", Description: "Fake lookup", Schema: func(workspace.Work) json.RawMessage { return json.RawMessage(`{"type":"object"}`) }})
	}}
	a := newTestApp(ai.New(""), fake)
	defer a.Close()
	if !slices.Contains(a.extensions.Ledger("ai").Points, "ai.tools") {
		t.Fatal("missing AI tools point")
	}
	if got := ext.Items(a.extensions, aiapi.Tools); !slices.ContainsFunc(got, func(i ext.Item[aiapi.Tool]) bool { return i.Value.Name == "lookup" }) {
		t.Fatal(got)
	}
	contributions := a.extensions.Ledger("faketool").Contributions
	if len(contributions) != 1 || contributions[0].ID != "faketool.lookup" || contributions[0].Point != "ai.tools" {
		t.Fatal(contributions)
	}
	a.turnAI(false)
	if len(ext.Items(a.extensions, aiapi.Tools)) != 0 || a.extensions.Ledger("faketool").State == ext.On {
		t.Fatal("required consumer survived")
	}
	a.turnAI(true)
	if len(ext.Items(a.extensions, aiapi.Tools)) != 1 {
		t.Fatal("tools did not return")
	}
}

func TestAIErrorTranslationExistsOnlyWhileExtensionIsOn(t *testing.T) {
	a, _, _ := fixture(t)
	err := model.ErrRequestTooLarge
	for _, on := range []bool{true, false, true} {
		a.turnAI(on)
		want := err.Error()
		if on {
			want = aitext.StatusRequestTooLarge()
		}
		if got := a.errText(err); got != want {
			t.Fatalf("on=%v: %q, want %q", on, got, want)
		}
	}
}

func TestAISwitchingBooksRestartsCheaply(t *testing.T) {
	a, _, _ := fixture(t)
	a.extensions.Close()
	a.initExtensions(append([]ext.Module{aimod.New(aimod.Options{ConfigFile: a.configFile})}, testModules()...))
	start := time.Now()
	for range 20 {
		if err := a.extensions.Open(ext.Book{}); err != nil {
			t.Fatal(err)
		}
		if err := a.extensions.Open(ext.Book{Dir: a.Store.Root()}); err != nil {
			t.Fatal(err)
		}
	}
	a.syncExtensions()
	if time.Since(start) > time.Second {
		t.Fatal("book switches exceeded one second")
	}
	if _, ok := a.aiRef.Get(); !ok {
		t.Fatal("missing service after switching")
	}
}

func TestShotExtAISettingsOnOffAndOverview(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	shot(t, tt, "ext-ai-settings-on.png")
	a.turnAI(false)
	shot(t, tt, "ext-ai-settings-off.png")
	a.openPrefs("extensions")
	shot(t, tt, "ext-ai-overview.png")
}

func TestLoreAndOutlineToolsAreContributionsOnTheirOwnersLedgers(t *testing.T) {
	a, _, _ := fixture(t)
	for owner, names := range map[string][]string{"lore": {"look_up_setting", "note_setting"}, "outline": {"read_outline"}} {
		ledger := a.extensions.Ledger(owner)
		for _, name := range names {
			if !slices.ContainsFunc(ledger.Contributions, func(c ext.Contribution) bool { return c.Point == "ai.tools" && c.ID == owner+"."+name }) {
				t.Fatal(owner, name, ledger.Contributions)
			}
		}
	}
}
