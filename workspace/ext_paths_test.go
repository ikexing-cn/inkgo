package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtDirRejectsBadIDs(t *testing.T) {
	s := testStore(t)
	for _, id := range []string{"", ".", "..", "../lore", "a/b", `a\b`, "/tmp", `C:\tmp`, "a:", "a\x00", "a "} {
		if p, err := s.ExtDir(id); err == nil || p != "" {
			t.Errorf("%q: %q, %v", id, p, err)
		}
	}
	p, err := s.ExtDir("lore")
	if err != nil || p != filepath.Join(s.Root(), "ext", "lore") {
		t.Fatal(p, err)
	}
}

func TestNewBookGitignoreIncludesExtensionData(t *testing.T) {
	s := testStore(t)
	if _, err := os.Stat(filepath.Join(s.Root(), "setting.jsonl")); !os.IsNotExist(err) {
		t.Fatal("root lore file created", err)
	}
	b, err := os.ReadFile(filepath.Join(s.Root(), ".gitignore"))
	if err != nil || string(b) != ".writer/\n" {
		t.Fatalf("gitignore = %q, %v", b, err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable; exact single-rule contents checked")
	}
	run := func(args ...string) ([]byte, error) {
		c := exec.Command("git", args...)
		c.Dir = s.Root()
		return c.CombinedOutput()
	}
	if out, err := run("init", "-q"); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	for _, p := range []string{"ext/lore/setting.jsonl", "ext/outline/chapter.txt", "ext.json"} {
		out, err := run("check-ignore", "--no-index", p)
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			t.Errorf("%s ignored: %s %v", p, out, err)
		}
	}
	if out, err := run("check-ignore", "--no-index", ".writer/desk.json"); err != nil {
		t.Fatalf("local state not ignored: %s %v", out, err)
	}
}

func TestOldBookOpensAndIgnoresLegacyLoreAndOutlines(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "volume", true)
	id := mustChapter(t, s, vol, "chapter", false)
	root := s.Root()
	planOK(t, s.Close())
	planOK(t, os.RemoveAll(filepath.Join(root, "ext")))
	for _, p := range []string{"setting.jsonl", "outline.json", "volume.outline.txt", "chapters/volume.outline.txt", "chapters/volume/chapter.outline.txt"} {
		planOK(t, os.WriteFile(filepath.Join(root, p), []byte("invalid legacy data"), 0600))
	}
	s, err := Open(root)
	planOK(t, err)
	defer s.Close()
	set, err := s.Setting()
	planOK(t, err)
	if len(set.Entries) != 0 || len(set.Revisions) != 0 {
		t.Fatal(set)
	}
	p, err := s.Plan()
	planOK(t, err)
	if len(p.Cards) != 0 || len(p.Lines) != 0 {
		t.Fatal(p)
	}
	outlines, err := s.Outlines()
	planOK(t, err)
	if len(outlines) != 2 || outlines[id] != "" || outlines[vol] != "" {
		t.Fatal(outlines)
	}
	// The first write also works when git did not retain empty directories.
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: s.Work().Categories[0].Name, Text: "new lore"}}})
	planOK(t, s.WriteOutline(id, "new outline"))
}

func TestOutlineFollowsChapterRenameMoveTrashRestoreAndPurge(t *testing.T) {
	for _, folder := range []bool{false, true} {
		name := "chapter"
		if folder {
			name = "volume"
		}
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			parent := mustChapter(t, s, "", "destination", true)
			id := mustChapter(t, s, "", name, folder)
			ids := []string{id}
			if folder {
				ids = append(ids, mustChapter(t, s, id, "child", false))
			}
			for _, n := range ids {
				planOK(t, s.WriteOutline(n, "outline for "+n))
			}
			check := func(live bool) {
				t.Helper()
				for _, n := range ids {
					b, err := os.ReadFile(filepath.Join(s.Root(), "ext", "outline", n+".txt"))
					planOK(t, err)
					if string(b) != "outline for "+n {
						t.Fatal("wrong outline", n, string(b))
					}
					if live {
						ch, err := s.ReadChapter(n)
						planOK(t, err)
						if ch.Outline != string(b) {
							t.Fatal(ch)
						}
					}
				}
			}
			planOK(t, s.RenameChapter(id, "renamed"))
			check(true)
			planOK(t, s.MoveChapter(id, parent, 0))
			check(true)
			planOK(t, s.TrashChapter(id))
			check(false)
			planOK(t, s.RestoreChapter(id))
			check(true)
			planOK(t, s.TrashChapter(id))
			planOK(t, s.PurgeChapter(id))
			for _, n := range ids {
				if _, err := os.Stat(filepath.Join(s.Root(), "ext", "outline", n+".txt")); !os.IsNotExist(err) {
					t.Fatal("outline survived purge", n, err)
				}
			}
		})
	}
}

func TestExternalOutlineChangesDoNotEnterChapterCounts(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "volume", true)
	id := mustChapter(t, s, vol, "chapter", false)
	_, err := s.WriteChapter(id, "one two", nil)
	planOK(t, err)
	for _, n := range []string{vol, id} {
		planOK(t, os.WriteFile(filepath.Join(s.Root(), "ext", "outline", n+".txt"), []byte("external\r\noutline"), 0600))
	}
	out, err := s.Outlines()
	planOK(t, err)
	if out[vol] != "external\noutline" || out[id] != out[vol] {
		t.Fatal(out)
	}
	tree, err := s.Chapters()
	planOK(t, err)
	if len(tree.byID) != 2 {
		t.Fatal("outlines became chapters", tree)
	}
	counts, err := s.Counts()
	planOK(t, err)
	if len(counts) != 1 || counts[id] != 6 {
		t.Fatal(counts)
	}
	pieces, err := s.Pieces()
	planOK(t, err)
	if len(pieces) != 2 || pieces[0].Outline != out[vol] || pieces[1].Outline != out[id] {
		t.Fatal(pieces)
	}
	planOK(t, os.Remove(filepath.Join(s.Root(), "ext", "outline", id+".txt")))
	out, err = s.Outlines()
	planOK(t, err)
	if out[id] != "" {
		t.Fatal("deleted outline still read", out)
	}
}
