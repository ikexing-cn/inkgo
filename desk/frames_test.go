package desk

import (
	"testing"
	"writer/editor"
	"writer/workspace"

	"github.com/egoist/mygo/ui"
)

func cacheBuilds(a *App) int {
	return hooksOf(a).CacheBuilds() + a.write.pending.builds +
		a.write.names.builds + a.write.prose.marks.builds +
		a.write.nodeWords.builds + a.write.total.builds +
		notesOf(a).Inspect().Builds +
		checkOf(a).State.Items.Builds + checkOf(a).State.ItemsMeant.Builds + checkOf(a).State.Listed.Builds +
		a.doc.ideas.builds + lorePanel(a).CacheBuilds()
}

func TestNameFeedSteadyFramesKeepCompletionAndNewEditorGetsNamesOnce(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "names", Names: []string{"林晚", "林舟"}})
	tt.Frame()
	ed := shownEditor(a)
	ed.Focus()
	tt.Frame()
	tt.Type("林")
	tt.Frame()
	before := a.write.names.builds
	tt.Key(0, ui.KeyDown)
	for range 10 {
		tt.Frame()
	}
	if a.write.names.builds != before {
		t.Fatal("steady frames rebuilt name feed")
	}
	// The popup keeps the selected second candidate; feeding it again resets it.
	tt.Key(0, ui.KeyEnter)
	if ed.Doc.String() != "林舟" {
		t.Fatal("steady frames reset the completion selection", ed.Doc.String())
	}
	a.feedNames()
	before = a.write.names.builds
	for _, on := range []bool{false, true} {
		a.extensions.Ledger("names").Settings[0].Write(on)
		a.feedNames()
		if a.write.names.builds != before+1 || (len(a.write.namedValues.Links) != 0) != on {
			t.Fatal("highlights must rebuild once")
		}
		before++
		for range 10 {
			a.feedNames()
		}
		if a.write.names.builds != before {
			t.Fatal("unchanged highlights rebuilt")
		}
	}
	for _, on := range []bool{false, true} {
		a.setExtension("names", on, false)
		a.feedNames()
		if a.write.names.builds != before+1 || (len(a.write.namedValues.Entities) != 0) != on {
			t.Fatal("extension switch must rebuild names once")
		}
		before++
		for range 10 {
			a.feedNames()
		}
		if a.write.names.builds != before {
			t.Fatal("unchanged extension switch rebuilt names")
		}
	}
	newEditor := editor.New(editor.NewDoc(""), editor.Options{})
	a.write.cur().ed = newEditor
	a.feedNames()
	if a.write.named != newEditor || a.write.names.builds != before {
		t.Fatal("a new editor must receive cached names without rebuilding")
	}
	newEditor.Focus()
	tt.Frame()
	tt.Type("林")
	if len(newEditor.Completions()) != 2 {
		t.Fatal("new editor did not receive candidates", newEditor.Completions())
	}
}

func TestMarginCacheSteadyFramesBuildNothingAndSourceChangesRebuildOnce(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	for _, change := range []func(){
		func() { shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: 0, End: 2, Key: "new"}) },
		func() { a.setExtension("hooks", false, false) },
		func() { a.setExtension("hooks", true, false) },
	} {
		before := notesOf(a).Inspect().Builds
		change()
		tt.Frame()
		if notesOf(a).Inspect().Builds != before+1 {
			t.Fatal("source change must rebuild margin once")
		}
		for range 10 {
			tt.Frame()
		}
		if notesOf(a).Inspect().Builds != before+1 {
			t.Fatal("steady frames rebuilt margin")
		}
	}
}

func TestProseFeedSteadyFramesBuildNothingAndEachSourceChangeRebuildsOnce(t *testing.T) {
	a, tt, chapter := marginFixture(t, true)
	for _, change := range []struct {
		name string
		run  func()
	}{
		{"new foreshadowing", func() { shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkForeshadow, Start: 0, End: 2, Key: "new"}) }},
		{"check result", func() {
			check := a.book.Checks()[chapter]
			check.Findings = append(check.Findings, workspace.Finding{ID: "fresh", Kind: workspace.FindNew, Quote: "雨", Name: "雨", Title: "fresh"})
			if err := a.book.SaveCheck(chapter, check); err != nil {
				t.Fatal(err)
			}
		}},
		{"setting rename", func() {
			e := a.book.Setting().Entries[len(a.book.Setting().Entries)-1]
			a.revise(workspace.Op{Op: workspace.OpName, Entry: e.ID, Names: []string{"林晴", "晚晚"}})
		}},
		{"lore idea", func() {
			e := a.book.Setting().Entries[len(a.book.Setting().Entries)-1]
			u, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "a fresh idea"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.book.Revise(workspace.Revision{By: workspace.ByTalk, Session: talkOf(a).Session, Turn: u.ID, Ops: []workspace.Op{{Op: workspace.OpChange, Entry: e.ID, From: chapter, Place: workspace.PlaceOpen, Text: "a fresh idea"}}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"AI mark", func() { shownEditor(a).AddMark(editor.Mark{Kind: editor.MarkAI, Start: 0, End: 1}) }},
		{"hooks off", func() { a.setExtension("hooks", false, false) }},
		{"hooks on", func() { a.setExtension("hooks", true, false) }},
		{"AI off", func() { a.setExtension("ai", false, false) }},
		{"AI on", func() { a.setExtension("ai", true, false) }},
		{"marks off", func() { a.setExtension("marks", false, false) }},
		{"marks on", func() { a.setExtension("marks", true, false) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			before := a.write.prose.marks.builds
			change.run()
			tt.Frame()
			if a.write.prose.marks.builds != before+1 {
				t.Fatal("change must rebuild once", a.write.prose.marks.builds-before)
			}
			for range 10 {
				tt.Frame()
			}
			if a.write.prose.marks.builds != before+1 {
				t.Fatal("steady frames rebuilt prose marks", a.write.prose.marks.builds-before-1)
			}
		})
	}
}

func TestLiveTextFollowsTheWritingPage(t *testing.T) {
	a, _, id := writingFixture(t, "正文。")
	if _, ok := a.liveText(); !ok {
		t.Fatal("the writing page has no live text")
	}
	showTab(a, paneTalk) // beside a talk on the writing page
	if _, ok := a.liveText(); !ok {
		t.Fatal("the talk beside the prose has no live text")
	}
	showPage(a, pageCheck) // after leaving the writing page
	if _, ok := a.liveText(); ok {
		t.Fatal("the check page has live text")
	}
	showPage(a, pageTalk)
	a.findInChapter(id, "正文") // a jump to the writing page
	if _, ok := a.liveText(); !ok {
		t.Fatal("a jump has no live text")
	}
}

// Ten frames with nothing changing build no derived value again. The
// writing page is entered by writingFixture, whose quick clock puts the
// 2-second look an hour away.
func TestUnchangedFramesBuildNoCache(t *testing.T) {
	t.Run("writing page, pending list open", func(t *testing.T) {
		a, tt, _ := writingFixture(t, "正文。")
		a.write.pendOpen = true
		tt.Frame()
		before := cacheBuilds(a)
		for range 10 {
			tt.Frame()
		}
		if got := cacheBuilds(a); got != before {
			t.Fatalf("built %d caches again in 10 frames", got-before)
		}
	})

	t.Run("outline page", func(t *testing.T) {
		a, tt, _ := outlineFixture(t)
		tt.Frame()
		before := cacheBuilds(a)
		for range 10 {
			tt.Frame()
		}
		if got := cacheBuilds(a); got != before {
			t.Fatalf("built %d caches again in 10 frames", got-before)
		}
	})

	t.Run("check page", func(t *testing.T) {
		a, tt, _, _ := checkFixture(t)
		showPage(a, pageCheck)
		tt.Frame()
		before := cacheBuilds(a)
		for range 10 {
			tt.Frame()
		}
		if got := cacheBuilds(a); got != before {
			t.Fatalf("built %d caches again in 10 frames", got-before)
		}
	})
}
