package discussion

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"writer/ext/ai/aiapi"

	"writer/workspace"
)

// Preloaded reports what fixedOf carried, from what it read: the tokens,
// the chapter's cards, whether the chapter before goes whole, and the
// outline source, which holds even when every outline is empty.
func TestPreloadedDescribesTheChapterContext(t *testing.T) {
	for _, inVolume := range []bool{false, true} {
		for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
			t.Run(origin+map[bool]string{false: "/book", true: "/volume"}[inVolume], func(t *testing.T) {
				e, _ := setup(t)
				parent := ""
				if inVolume {
					var err error
					parent, err = e.Store.NewChapter("", "空卷纲", true)
					if err != nil {
						t.Fatal(err)
					}
				}
				one, err := e.Store.NewChapter(parent, "第一章", false)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := e.Store.WriteChapter(one, "上一章的正文。", nil); err != nil {
					t.Fatal(err)
				}
				two, err := e.Store.NewChapter(parent, "第二章", false)
				if err != nil {
					t.Fatal(err)
				}
				writePlan(t, e.Store, workspace.Plan{Cards: []workspace.Card{
					{Title: "别章", Chapter: one}, {Title: "本章", Chapter: two}, {Title: "本章另一张", Chapter: two},
				}})
				got, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, origin, two)
				previous := origin == workspace.AIDraft
				if err != nil || got.Tokens <= 0 || got.Cards != 2 || got.InVolume != inVolume || got.Previous != previous {
					t.Fatal(got, err)
				}
				first, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, origin, one)
				if err != nil || first.Previous || first.Cards != 1 || first.InVolume != inVolume {
					t.Fatal(first, err)
				}
			})
		}
	}
}

// The chapter before goes whole only with prose that can be read: blank or
// unreadable, it is left out and Preloaded says it does not carry it.
func TestPreloadedLeavesOutThePreviousChapterItCannotCarry(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "blank", true: "unreadable"}[broken], func(t *testing.T) {
			e, _ := setup(t)
			_, one, two, _ := shelf(t, e)
			if broken {
				path := filepath.Join(e.Store.Root(), "chapters", "第一卷", "第一章.marks.json")
				if err := os.WriteFile(path, []byte(`broken`), 0600); err != nil {
					t.Fatal(err)
				}
			} else if _, err := e.Store.WriteChapter(one, " \n\t", nil); err != nil {
				t.Fatal(err)
			}
			got, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, workspace.AIDraft, two)
			if err != nil || got.Previous {
				t.Fatal(got, err)
			}
			tree, err := e.Store.Chapters()
			if err != nil {
				t.Fatal(err)
			}
			fixed, err := fixedOf(e.Store, aiapi.Material{Setting: true, Outline: true}, tree, workspace.AIDraft, two)
			if err != nil || slices.ContainsFunc(fixed.Pieces, func(p Piece) bool { return p.Part == workspace.PartPrevious }) {
				t.Fatal(fixed, err)
			}
		})
	}
}

func TestPreloadedFailsWithoutAChapterContext(t *testing.T) {
	e, _ := setup(t)
	if got, err := Preloaded(e.Store, aiapi.Material{Setting: true, Outline: true}, workspace.AIDraft, "missing"); err == nil || got != (Preload{}) {
		t.Fatal(got, err)
	}
}
