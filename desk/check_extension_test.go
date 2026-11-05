package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
	"slices"
	"testing"
	"time"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/workspace"
)

func TestCheckOffGloballyAndPerBookRemovesAllSurfacesAndOnRestoresResults(t *testing.T) {
	for _, book := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "book"}[book], func(t *testing.T) {
			a, tt, id, _ := checkFixture(t)
			click(t, tt, "立即检查")
			until(t, tt, "findings", func() bool { return len(a.checkItems(false)) == 3 })
			before, err := a.Store.ReadCheck(id)
			requireExtension(t, err)
			a.openCheck("")
			tt.Frame()
			a.setExtension("check", false, book)
			tt.Frame()
			tt.Frame()
			if a.page != pageWrite || a.pages[pageCheck].available(a) {
				t.Fatal("disabled page remained")
			}
			for _, p := range []bool{hasOwner(a.extViews.footer, "check"), hasOwner(a.extViews.pending, "check"), hasOwner(a.extViews.sections, "check")} {
				if p {
					t.Fatal("disabled contribution remained")
				}
			}
			for _, command := range a.commandList() {
				if command.name(a) == "检查" {
					t.Fatal("disabled command remained")
				}
			}
			for _, g := range a.pendingGroups() {
				if g.id == "check" {
					t.Fatal("disabled pending group remained")
				}
			}
			reader, ok := ext.Lookup(a.extensions, marksapi.Service)
			if !ok {
				t.Fatal("marks stopped")
			}
			for _, m := range reader.Items(id, shownEditor(a).Doc.String()) {
				if m.Look == editor.MarkCheck {
					t.Fatal("disabled check mark remained")
				}
			}
			a.openPrefs("reminders")
			tt.Frame()
			if tt.HasText("何时检查") {
				t.Fatal("disabled settings remained")
			}
			a.closePrefs()
			a.setExtension("check", true, book)
			tt.Frame()
			tt.Frame()
			after, err := a.Store.ReadCheck(id)
			requireExtension(t, err)
			if !before.Equal(after) || len(a.checkItems(false)) != 3 || !a.pages[pageCheck].available(a) {
				t.Fatal("results did not return")
			}
		})
	}
}

func TestCheckAndRequiredDependenciesOffCancelRunningChecksAndOnRestoresAvailability(t *testing.T) {
	for _, owner := range []string{"check", "ai", "lore", "marks"} {
		for _, book := range []bool{false, true} {
			if book && owner != "check" {
				continue
			}
			t.Run(owner+map[bool]string{false: "/global", true: "/book"}[book], func(t *testing.T) {
				a, tt, id, _ := checkFixture(t)
				p := checkOf(a)
				p.State.Queue.Enqueue(id)
				p.State.Queue.Next()
				started := make(chan context.Context, 1)
				p.Start(checkFunc(func(ctx context.Context, _ string) (workspace.Check, error) {
					started <- ctx
					<-ctx.Done()
					return workspace.Check{}, ctx.Err()
				}), id, ui.Services{})
				ctx := <-started
				a.setExtension(owner, false, book)
				tt.Frame()
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
					t.Fatal("running check was not cancelled")
				}
				if a.extensions.Ledger("check").State == ext.On || p.State.Queue.Running != "" || len(p.State.Queue.Pending) != 0 {
					t.Fatal("check remained active")
				}
				a.setExtension(owner, true, book)
				tt.Frame()
				tt.Frame()
				if a.extensions.Ledger("check").State != ext.On {
					t.Fatal("check did not return")
				}
			})
		}
	}
}

// TestRemindersIsOnePageThatStaysWithEachExtensionsHeaderAndOnlyActiveSettings follows
// the prototype's prefPages: a registered page stays while its extensions are off,
// so the switch in each one's header can turn it back on.
func TestRemindersIsOnePageThatStaysWithEachExtensionsHeaderAndOnlyActiveSettings(t *testing.T) {
	a, tt, _, _ := checkFixture(t)
	for _, state := range []struct{ check, hooks bool }{{true, true}, {false, true}, {true, false}, {false, false}} {
		a.setExtension("check", state.check, false)
		a.setExtension("hooks", state.hooks, false)
		tt.Frame()
		count := 0
		for _, tab := range a.prefTabs {
			if tab.id == "reminders" {
				count++
			}
		}
		if count != 1 {
			t.Fatal("shared reminder page", state, count)
		}
		if owners := a.extViews.pageOwners["reminders"]; !slices.Contains(owners, "check") || !slices.Contains(owners, "hooks") {
			t.Fatal("reminder page owners", state, owners)
		}
		a.openPrefs("reminders")
		tt.Frame()
		if tt.HasText("何时检查") != state.check {
			t.Fatal("check section", state, tt.Texts())
		}
		a.closePrefs()
	}
}

func TestCheckLedgerOwnsItsSurfacesAndHostHasNoCheckMarginSource(t *testing.T) {
	a, _, _, _ := checkFixture(t)
	l := a.extensions.Ledger("check")
	want := map[string]bool{string(hostapi.Commands): false, string(hostapi.Pages): false, string(hostapi.FrameTicks): false, string(hostapi.FooterItems): false, string(hostapi.PendingGroups): false, string(hostapi.SettingsPages): false, string(hostapi.SettingsSections): false, string(hostapi.State): false, string(marksapi.MarginSources): false}
	for _, c := range l.Contributions {
		if _, ok := want[c.Point]; ok {
			want[c.Point] = true
		}
	}
	for point, got := range want {
		if !got {
			t.Fatal("missing ledger contribution", point)
		}
	}
	for _, item := range ext.Declared(a.extensions, marksapi.MarginSources) {
		if item.Owner == ext.HostOwner && item.ID == "host.check" {
			t.Fatal("host still owns check source")
		}
	}
}
