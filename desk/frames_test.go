package desk

import "testing"

func cacheBuilds(a *App) int {
	return a.write.fores.builds + a.write.doubts.builds + a.write.pending.builds +
		a.write.nodeWords.builds + a.write.total.builds +
		a.write.pane.builds + a.write.nameLinks.builds + a.write.margin.notes.builds +
		a.ck.items.builds + a.ck.itemsMeant.builds + a.ck.listed.builds +
		a.doc.ideas.builds + lorePanel(a).CacheBuilds()
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
