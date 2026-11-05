package find

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/ext/find/findtext"
	"writer/ext/hostapi"
)

// Keep the host's real editor and inject each safety signal independently.
// This lets the test reach the panel even when the host would cover it with
// a close decision or an external-change prompt.
type safetyNavigation struct {
	hostapi.Navigation
	lost, paused bool
}

func (n *safetyNavigation) Selected() (string, bool, bool) {
	id, lost, paused := n.Navigation.Selected()
	return id, lost || n.lost, paused || n.paused
}

type safetyEditing struct {
	hostapi.Editing
	external bool
}

func (e *safetyEditing) ChapterExternal() bool {
	return e.external || e.Editing.ChapterExternal()
}

func TestFindReplacementSafetyGates(t *testing.T) {
	for _, state := range []string{"lost", "paused", "external"} {
		for _, wide := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/book-%t", state, wide), func(t *testing.T) {
				const original = "林晚说，林晚走了。"
				a, tt, id := writingFixture(t, original)
				v := findOf(a)
				nav := &safetyNavigation{Navigation: v.Desktop}
				editing := &safetyEditing{Editing: v.Editing}
				v.Desktop, v.Editing = nav, editing
				v.Finding, v.Wide, v.Query, v.With = true, wide, "林晚", "苏晴"
				tt.Frame()
				tt.Frame()
				click := func(label string) {
					t.Helper()
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				// Enter starts with the replacement field focused, before the state changes.
				click(findtext.FindReplaceWith())
				if !tt.Focused(findtext.FindReplaceWith()) {
					t.Fatal("replacement field is not focused")
				}
				switch state {
				case "lost":
					nav.lost = true
				case "paused":
					nav.paused = true
				case "external":
					editing.external = true
				}
				tt.Frame()
				if v.lost() != (state == "lost") || v.paused() != (state != "lost") {
					t.Fatal("safety signals are not isolated", v.lost(), v.paused())
				}
				if len(shownEditor(a).Doc.Find(v.Query)) != 2 || wide && v.hitTotal != 2 {
					t.Fatal("gate must be tested with hits")
				}
				version := shownEditor(a).Doc.Version()
				start, end := shownEditor(a).Selection()
				unchanged := func() {
					t.Helper()
					if shownEditor(a).Doc.Version() != version || shownEditor(a).Doc.String() != original || v.Sure {
						t.Fatal("blocked action changed the editor or opened replacement review")
					}
					if s, e := shownEditor(a).Selection(); s != start || e != end {
						t.Fatal("blocked action moved the selection", s, e)
					}
					ch, err := a.Store.ReadChapter(id)
					if err != nil || ch.Text != original {
						t.Fatal("blocked action changed disk", ch, err)
					}
				}
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				unchanged()
				if wide {
					click(findtext.FindReplaceBook())
					unchanged()
				} else if state == "lost" {
					for _, label := range []string{findtext.FindReplace(), findtext.FindReplaceAll()} {
						click(label)
						unchanged()
						if tt.Focused(label) {
							t.Fatal("disabled replacement button took focus", label)
						}
					}
				}
				// Clearing the signal restores the same controls without reopening find.
				nav.lost, nav.paused, editing.external = false, false, false
				tt.Frame()
				if wide {
					click(findtext.FindReplaceBook())
					if !v.Sure {
						t.Fatal("replacement review did not recover")
					}
					click("取消")
				}
				click(findtext.FindReplaceWith())
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if got := shownEditor(a).Doc.String(); got != "苏晴说，林晚走了。" {
					t.Fatal("Enter did not recover after the safety signal cleared", got)
				}
			})
		}
	}
}
