package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func planCard(t *testing.T, s *Store, c Card, after string) Card {
	t.Helper()
	c, err := s.SaveCard(c, after)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func readPlan(t *testing.T, s *Store) Plan {
	t.Helper()
	p, err := s.Plan()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func planLine(t *testing.T, s *Store, name string) PlotLine {
	t.Helper()
	l, err := s.SaveLine(PlotLine{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func planOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOutlineCardOrderAndStorage(t *testing.T) {
	s := testStore(t)
	one := mustChapter(t, s, "", "一", false)
	two := mustChapter(t, s, "", "二", false)
	l := planLine(t, s, "查灯")
	other := planLine(t, s, "母亲")
	a := planCard(t, s, Card{Title: "后发生", Chapter: two, Line: l.ID, When: "夜"}, "")
	b := planCard(t, s, Card{Title: "先发生", Chapter: one, Line: other.ID, When: "昼"}, "")
	c := planCard(t, s, Card{Title: "未定", When: "夜"}, "")
	d := planCard(t, s, Card{Title: "接着记", Chapter: one, When: "昼"}, b.ID)
	if got := readPlan(t, s).Cards; !slices.Equal(got, []Card{b, d, a, c}) {
		t.Fatal(got)
	}
	planOK(t, s.PlaceCard(c.ID, one, &l.ID))
	if got := readPlan(t, s).Cards; !slices.Equal(got, []Card{b, d, cWith(c, one, l.ID), a}) {
		t.Fatal(got)
	}
	planOK(t, s.PlaceCard(c.ID, "", nil)) // tray keeps line and position
	p := readPlan(t, s)
	if p.Cards[2].Chapter != "" || p.Cards[2].Line != l.ID {
		t.Fatal(p)
	}
	planOK(t, s.DropCard(b.ID, 1, l.ID))
	p = readPlan(t, s)
	if p.Cards[len(p.Cards)-1].ID != b.ID || p.Cards[2].ID != a.ID || p.Cards[3].When != "夜" {
		t.Fatal(p)
	}
	planOK(t, s.DropCard(b.ID, -1, other.ID))
	p = readPlan(t, s)
	if p.Cards[len(p.Cards)-1].ID != b.ID {
		t.Fatal(p)
	}
	b = p.Cards[len(p.Cards)-1]
	b.Title = "改标题"
	planCard(t, s, b, "")
	planOK(t, s.SaveTime(1, "昼"))
	p = readPlan(t, s)
	if len(p.Columns()) != 1 || len(p.Columns()[0].Cards) != 4 {
		t.Fatal(p.Columns())
	}
	planOK(t, s.MoveLine(other.ID, -1))
	other.Name = "失踪"
	_, err := s.SaveLine(other)
	planOK(t, err)
	planOK(t, s.DeleteLine(l.ID))
	p = readPlan(t, s)
	if len(p.Lines) != 1 || p.Lines[0].Name != "失踪" || slices.ContainsFunc(p.Cards, func(c Card) bool { return c.Line == l.ID }) {
		t.Fatal(p)
	}
	planOK(t, s.DeleteCard(d.ID))
	p = readPlan(t, s)
	root := s.Root()
	planOK(t, s.Close())
	reopened, err := Open(root)
	planOK(t, err)
	defer reopened.Close()
	if got := readPlan(t, reopened); !reflect.DeepEqual(got, p) {
		t.Fatalf("reopened: %+v != %+v", got, p)
	}
}

func cWith(c Card, chapter, line string) Card { c.Chapter, c.Line = chapter, line; return c }

func TestOutlineChapterLifecycle(t *testing.T) {
	s := testStore(t)
	v := mustChapter(t, s, "", "卷", true)
	one := mustChapter(t, s, v, "一", false)
	two := mustChapter(t, s, "", "二", false)
	c := planCard(t, s, Card{Title: "事件", Chapter: one}, "")
	planOK(t, s.RenameChapter(one, "改名"))
	planOK(t, s.MoveChapter(one, "", 0))
	if got, _ := readPlan(t, s).Card(c.ID); got != c {
		t.Fatal(got)
	}
	planOK(t, s.TrashChapter(one))
	if got, _ := readPlan(t, s).Card(c.ID); got.Chapter != one {
		t.Fatal(got)
	}
	before := readPlan(t, s).Cards
	c.Title = "回收站里的章仍可改卡片"
	planCard(t, s, c, "")
	after := readPlan(t, s).Cards
	if before[0].ID != after[0].ID {
		t.Fatal("editing a trashed chapter's card moved it")
	}
	planOK(t, s.RestoreChapter(one))
	if got, _ := readPlan(t, s).Card(c.ID); got.Chapter != one {
		t.Fatal(got)
	}
	planOK(t, s.MoveChapter(one, v, 0))
	planOK(t, s.TrashChapter(v))
	planOK(t, s.PurgeChapter(v))
	if got, _ := readPlan(t, s).Card(c.ID); got.Chapter != "" {
		t.Fatal(got)
	}
	c = planCard(t, s, Card{Title: "第二件", Chapter: two}, "")
	planOK(t, s.TrashChapter(two))
	planOK(t, s.PurgeChapter(two))
	if got, _ := readPlan(t, s).Card(c.ID); got.Chapter != "" {
		t.Fatal(got)
	}
}

func TestOutlineMalformedFile(t *testing.T) {
	for _, bad := range []string{`{`, `{"schema":2,"cards":[{"id":"a","title":""}]}`, `{"schema":2,"lines":[{"id":"l","name":"线"},{"id":"l","name":"重"}]}`, `{"schema":2,"cards":[{"id":"a","title":"事","line":"unknown"}]}`, `{"schema":2,"surprise":true}`, `{"schema":2} {}`} {
		bad = strings.NewReplacer(`"a"`, `"20261019-0000-0001"`, `"l"`, `"20261019-0000-0002"`, `"unknown"`, `"20261019-0000-0003"`).Replace(bad)
		t.Run(bad, func(t *testing.T) {
			s := testStore(t)
			id := mustChapter(t, s, "", "正文", false)
			p := filepath.Join(s.Root(), "outline.json")
			planOK(t, os.WriteFile(p, []byte(bad), 0600))
			if _, err := s.Plan(); err == nil || !strings.Contains(err.Error(), "outline.json") {
				t.Fatal(err)
			}
			if _, err := s.SaveLine(PlotLine{Name: "不能覆盖"}); err == nil {
				t.Fatal("overwrote invalid file")
			}
			root := s.Root()
			planOK(t, s.Close())
			s, err := Open(root)
			planOK(t, err)
			defer s.Close()
			planOK(t, s.WriteOutline(id, "正文的章纲仍可保存"))
			_, err = s.WriteChapter(id, "大纲损坏后仍可写正文。", nil)
			planOK(t, err)
			if _, err := s.ReadChapter(id); err != nil {
				t.Fatal("bad plan blocked prose", err)
			}
			if _, err := s.Plan(); err == nil {
				t.Fatal("invalid plan was accepted after reopening")
			}
			for _, write := range []func() error{
				func() error { _, err := s.SaveCard(Card{Title: "不能覆盖"}, ""); return err },
				func() error { return s.DeleteCard("a") },
				func() error { return s.PlaceCard("a", id, nil) },
				func() error { return s.DropCard("a", 0, "") },
				func() error { return s.SaveTime(0, "时间") },
				func() error { return s.MoveLine("l", 1) },
				func() error { return s.DeleteLine("l") },
			} {
				if err := write(); err == nil {
					t.Fatal("mutation accepted damaged plan")
				}
			}
			got, _ := os.ReadFile(p)
			if string(got) != bad {
				t.Fatal("damaged file overwritten")
			}
		})
	}
}

func TestPurgeChapterWithMalformedOutline(t *testing.T) {
	s := testStore(t)
	v := mustChapter(t, s, "", "卷", true)
	id := mustChapter(t, s, v, "章", false)
	c := planCard(t, s, Card{Title: "旧事", Chapter: id}, "")
	file := filepath.Join(s.Root(), "outline.json")
	good, err := os.ReadFile(file)
	planOK(t, err)
	planOK(t, s.TrashChapter(v))
	bad := []byte("{ broken outline")
	planOK(t, os.WriteFile(file, bad, 0600))
	planOK(t, s.PurgeChapter(v))
	trash, err := s.Trash()
	planOK(t, err)
	if len(trash) != 0 {
		t.Fatal("damaged outline blocked trash cleanup", trash)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != string(bad) {
		t.Fatal("damaged outline changed", string(data), err)
	}
	planOK(t, os.WriteFile(file, good, 0600))
	got, _ := readPlan(t, s).Card(c.ID)
	if got.Chapter != id {
		t.Fatal("unreadable outline should be left intact", got)
	}
	tree, err := s.Chapters()
	planOK(t, err)
	if _, ok := tree.Node(got.Chapter); ok {
		t.Fatal("purged chapter still in tree")
	}
}

func TestOutlineTimeColumnsAndDrops(t *testing.T) {
	s := testStore(t)
	a := planCard(t, s, Card{Title: "a", When: "一"}, "")
	b := planCard(t, s, Card{Title: "b", When: "二"}, "")
	c := planCard(t, s, Card{Title: "c", When: "二"}, "")
	d := planCard(t, s, Card{Title: "d"}, "")
	planCard(t, s, Card{Title: "e"}, "")
	if len(readPlan(t, s).Columns()) != 4 {
		t.Fatal(readPlan(t, s))
	}
	planOK(t, s.SaveTime(1, "一"))
	p := readPlan(t, s)
	if len(p.Columns()) != 3 || len(p.Columns()[0].Cards) != 3 {
		t.Fatal(p)
	}
	planOK(t, s.DropCard(d.ID, 1, ""))
	if got := readPlan(t, s).Cards; got[3].ID != d.ID {
		t.Fatal(got)
	} // sole card stays
	planOK(t, s.DropCard(a.ID, 0, ""))
	if got := readPlan(t, s).Cards; got[0].ID != b.ID || got[1].ID != c.ID || got[2].ID != a.ID {
		t.Fatal(got)
	}
	if s.SaveTime(99, "bad") == nil || s.DropCard("missing", 0, "") == nil {
		t.Fatal("invalid mutation accepted")
	}
}
