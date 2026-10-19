package main

import (
	"strings"
	"testing"

	"writer/workspace"
	"writer/writing"
)

// Both books are written as works the app opens: fifteen chapters each,
// long enough to check, in two volumes, with a setting in all three
// places, and foreshadowings paid off, open and given up.
func TestBooksAreWritten(t *testing.T) {
	dirs, err := writeAll(t.TempDir())
	if err != nil || len(dirs) != 2 {
		t.Fatal(err, dirs)
	}
	for _, dir := range dirs {
		s, err := workspace.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		set, err := s.Setting()
		if err != nil {
			t.Fatal(err)
		}
		places := map[string]int{}
		for _, e := range set.Entries {
			places[e.Place]++
		}
		if places[workspace.PlaceBody] < 10 || places[workspace.PlaceOpen] == 0 || places[workspace.PlaceDropped] == 0 {
			t.Fatal(dir, places)
		}
		tree, err := s.Chapters()
		if err != nil {
			t.Fatal(err)
		}
		if len(tree.Root.Children) != 2 {
			t.Fatal(dir, len(tree.Root.Children), "volumes")
		}
		plan, err := s.Plan()
		if err != nil {
			t.Fatal(err)
		}
		loose, untimed := 0, 0
		for _, c := range plan.Cards {
			if c.Chapter == "" {
				loose++
			} else if _, ok := tree.Node(c.Chapter); !ok {
				t.Fatal("card of unknown chapter", c)
			}
			if c.When == "" {
				untimed++
			}
		}
		if len(plan.Lines) != 3 || len(plan.Cards) != 22 || loose != 3 || untimed != 2 {
			t.Fatal("sample outline", plan, loose, untimed)
		}
		if first, ok := tree.Node(plan.Cards[0].Chapter); !ok || first == tree.Root.Children[0].Children[0] {
			t.Fatal("sample has no flashback")
		}
		chapters, paid, dropped, buried := 0, 0, 0, map[string]bool{}
		var marks []workspace.ChapterMark
		for _, v := range tree.Root.Children {
			if !v.Folder {
				t.Fatal(dir, v.Title)
			}
			for _, n := range v.Children {
				ch, err := s.ReadChapter(n.ID)
				if err != nil {
					t.Fatal(err)
				}
				if ch.Outline == "" || workspace.Count(ch.Text) < writing.MinCheck || strings.ContainsAny(ch.Text, "{}|") {
					t.Fatal(dir, n.Title, workspace.Count(ch.Text))
				}
				chapters++
				for _, m := range ch.Marks {
					if m.Orphaned || m.Quote == "" || string([]rune(ch.Text)[m.Start:m.End]) != m.Quote {
						t.Fatal(dir, n.Title, m)
					}
					if m.Kind == workspace.MarkForeshadow {
						buried[m.ID] = true
					}
					if m.Dropped {
						dropped++
					}
				}
				marks = append(marks, ch.Marks...)
			}
		}
		for _, m := range marks {
			if m.Kind == workspace.MarkPayoff {
				if !buried[m.Of] {
					t.Fatal(dir, "pays off nothing", m)
				}
				paid++
			}
		}
		if chapters != 15 || paid != 3 || len(buried) < 5 {
			t.Fatal(dir, chapters, paid, len(buried))
		}
		t.Log(dir, len(set.Entries), "entries", len(buried), "foreshadowings", dropped, "given up")
	}
}
