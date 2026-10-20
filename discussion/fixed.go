package discussion

import (
	"errors"
	"fmt"
	"strings"

	"writer/workspace"
)

// Fixed is the chapter a talk opened to draft or rewrite keeps to, and
// what the talk's requests carry of it: a piece for each part, as the
// model reads it and as the author is told it.
type Fixed struct {
	Origin  string // workspace.AIDraft or workspace.AIRewrite
	Chapter string
	Title   string // the chapter as the model names it
	Pieces  []Piece
}

// Piece is a part of what goes with a talk: its text for the model and
// what the author is told of it.
type Piece struct {
	Part, Text, Item string
}

// originOf is what the talk of events was opened to do, and the chapter it
// keeps to: its first words say; none for a talk opened to talk.
func originOf(events []workspace.Event) (origin, chapter string) {
	for _, e := range events {
		if e.Role == workspace.RoleUser {
			return e.Origin, e.Chapter
		}
	}
	return "", ""
}

// errGone is what a talk that keeps to a chapter says once the chapter is
// out of the book.
var errGone = errors.New("这段讨论跟着的章已不在书里，不能接着写了")

// fixedOf reads what goes with a talk opened to draft or rewrite chapter
// of tree. A draft takes the chapter's outline and its volume's, the
// outlines of the volumes before and of the chapters before in its
// volume, and the chapter before, whole; a rewrite takes the outlines and
// the chapter itself, whole. Both take the chapter's cards. Outlines left
// empty are left out.
func fixedOf(st *workspace.Store, tree workspace.Tree, origin, chapter string) (*Fixed, error) {
	n, ok := tree.Node(chapter)
	if !ok || n.Folder {
		return nil, errGone
	}
	b := bookOf(tree)
	f := &Fixed{Origin: origin, Chapter: chapter, Title: b.name[n.ID]}
	add := func(part, text, item string) {
		f.Pieces = append(f.Pieces, Piece{part, text, item})
	}
	outline := func(id string) string {
		ch, err := st.ReadChapter(id)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(ch.Outline)
	}
	ch, err := st.ReadChapter(chapter)
	if err != nil {
		return nil, err
	}
	what := map[string]string{workspace.AIDraft: "起草", workspace.AIRewrite: "改写"}[origin]
	head := "【这段讨论" + what + "的章：" + f.Title + "】"
	if o := strings.TrimSpace(ch.Outline); o != "" {
		add(workspace.PartOutline, head+"\n章纲：\n"+o, "章纲："+f.Title)
	} else {
		add(workspace.PartOutline, head+"\n（这一章没有章纲）", what+"的章："+f.Title)
	}
	vol := n.Parent
	if vol != nil && vol.Parent == nil {
		vol = nil // the root holds the chapters of no volume
	}
	if vol != nil {
		if o := outline(vol.ID); o != "" {
			add(workspace.PartOutline, "【所在卷："+b.name[vol.ID]+"】卷纲：\n"+o, "卷纲："+b.name[vol.ID])
		}
	}
	if p, err := st.Plan(); err == nil {
		if rows := chapterCards(p, b, chapter); len(rows) > 0 {
			add(workspace.PartCards, "【本章的卡片】\n"+strings.Join(rows, "\n"), fmt.Sprintf("本章的卡片 %d 张", len(rows)))
		}
	}
	if origin == workspace.AIRewrite {
		add(workspace.PartChapter, "【本章全文】\n"+ch.Text, fmt.Sprintf("本章全文 %d 字", workspace.Count(ch.Text)))
		return f, nil
	}
	// The volumes before, and the chapters before in the volume, in the
	// order of the book.
	var vols, chapters []string
	ahead := true
	tree.Walk(func(m *workspace.Node, _ int) {
		if m == n {
			ahead = false
		}
		switch {
		case !ahead:
		case m.Folder && !holds(m, n):
			if o := outline(m.ID); o != "" {
				vols = append(vols, b.name[m.ID]+"：\n"+o)
			}
		case !m.Folder && vol != nil && m.Parent == vol:
			if o := outline(m.ID); o != "" {
				chapters = append(chapters, b.name[m.ID]+"：\n"+o)
			}
		}
	})
	if len(vols) > 0 {
		add(workspace.PartOutline, "【前面各卷的卷纲】\n"+strings.Join(vols, "\n\n"), fmt.Sprintf("前面各卷的卷纲 %d 卷", len(vols)))
	}
	if len(chapters) > 0 {
		add(workspace.PartOutlines, "【本卷前面各章的章纲】\n"+strings.Join(chapters, "\n\n"), fmt.Sprintf("本卷前面各章的章纲 %d 章", len(chapters)))
	}
	if p, ok := previous(tree, n); ok {
		if pc, err := st.ReadChapter(p.ID); err == nil && strings.TrimSpace(pc.Text) != "" {
			add(workspace.PartPrevious, "【上一章全文："+b.name[p.ID]+"】\n"+pc.Text, fmt.Sprintf("上一章全文：%s %d 字", b.name[p.ID], workspace.Count(pc.Text)))
		}
	}
	// A draft goes on from what the chapter has, or takes its place.
	if strings.TrimSpace(ch.Text) != "" {
		add(workspace.PartChapter, "【本章已有的正文】\n"+ch.Text, fmt.Sprintf("本章已有正文 %d 字", workspace.Count(ch.Text)))
	}
	return f, nil
}

// holds tells whether folder f holds n, at any depth.
func holds(f, n *workspace.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == f {
			return true
		}
	}
	return false
}

// previous is the document before n in the order of the tree.
func previous(t workspace.Tree, n *workspace.Node) (*workspace.Node, bool) {
	var last, found *workspace.Node
	t.Walk(func(m *workspace.Node, _ int) {
		if m == n {
			found = last
		}
		if !m.Folder {
			last = m
		}
	})
	return found, found != nil
}

// Preloaded is about how many tokens a talk opened to draft or rewrite
// chapter carries before anything is said or looked up: the rules, the
// setting as it stands at the chapter, and what goes with the chapter.
// The dialog that opens the talk says it.
func Preloaded(st *workspace.Store, origin, chapter string) (int, error) {
	tree, err := st.Chapters()
	if err != nil {
		return 0, err
	}
	set, err := st.Setting()
	if err != nil {
		return 0, err
	}
	f, err := fixedOf(st, tree, origin, chapter)
	if err != nil {
		return 0, err
	}
	asked, _, err := lay(st.Work(), 1<<30, set, tree, talk{fixed: f})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range asked.Sent.Parts {
		n += p.Tokens
	}
	return n, nil
}

// cleanProse is s as a chapter keeps its prose: a line a paragraph, no
// indent, no blank line, nothing around it. Proposed to follow words in
// their paragraph, it may begin a paragraph of its own: lead keeps a
// line break s begins with.
func cleanProse(s string, lead bool) string {
	var b strings.Builder
	started := false
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if started {
			b.WriteByte('\n')
		}
		b.WriteString(line)
		started = true
	}
	out := b.String()
	if lead && strings.HasPrefix(strings.TrimLeft(s, " \t　\r"), "\n") && out != "" {
		out = "\n" + out
	}
	return out
}
