package discussion

import (
	"fmt"
	"strings"
	"writer/aiport"

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
	preload Preload
}

// Piece is a part of what goes with a talk: its text for the model and
// what the author is told of it.
type Piece struct {
	Part, Text, Item string
}

// Preload describes what the fixed context of a talk carries: the tokens
// it takes, the chapter's cards, whether the chapter before goes whole, and
// whether the outlines come from the volume the chapter is in, even when
// they are all empty.
type Preload struct {
	Tokens, Cards int
	Previous      bool
	InVolume      bool
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
var errGone = ErrChapterGone

// fixedOf reads what goes with a talk opened to draft or rewrite chapter
// of tree. A draft takes the chapter's outline and its volume's, the
// outlines of the volumes before and of the chapters before in its
// volume, and the chapter before, whole; a rewrite takes the outlines and
// the chapter itself, whole. Both take the chapter's cards. Outlines left
// empty are left out.
func fixedOf(st *workspace.Store, material aiport.Material, tree workspace.Tree, origin, chapter string) (*Fixed, error) {
	n, ok := tree.Node(chapter)
	if !ok || n.Folder {
		return nil, errGone
	}
	b := bookOf(tree)
	f := &Fixed{Origin: origin, Chapter: chapter, Title: b.name[n.ID]}
	add := func(part, text, item string) {
		f.Pieces = append(f.Pieces, Piece{part, text, item})
	}
	ch, err := st.ReadChapter(chapter)
	if err != nil {
		return nil, err
	}
	what := map[string]string{workspace.AIDraft: "起草", workspace.AIRewrite: "改写"}[origin]
	head := "【这段讨论" + what + "的章：" + f.Title + "】"
	if material.Outline {
		if o := strings.TrimSpace(ch.Outline); o != "" {
			add(workspace.PartOutline, head+"\n章纲：\n"+o, "章纲："+f.Title)
		} else {
			add(workspace.PartOutline, head+"\n（这一章没有章纲）", what+"的章："+f.Title)
		}
	} else {
		add(workspace.PartChapter, head, what+"的章："+f.Title)
	}
	vol := n.Volume()
	f.preload.InVolume = vol != nil
	var ids []string
	if vol != nil {
		ids = append(ids, vol.ID)
	}
	var volumes, chapters []*workspace.Node
	ahead := true
	tree.Walk(func(m *workspace.Node, _ int) {
		if m == n {
			ahead = false
		}
		if origin == workspace.AIRewrite || !ahead {
			return
		}
		switch {
		case m.Folder && (n == m || !n.Within(m)):
			volumes = append(volumes, m)
		case !m.Folder && vol != nil && m.Parent == vol:
			chapters = append(chapters, m)
		default:
			return
		}
		ids = append(ids, m.ID)
	})
	// Outlines that cannot be read are left out, as if there were none.
	var outlines map[string]string
	if material.Outline {
		outlines, _ = st.OutlinesOf(ids...)
	}
	if vol != nil {
		if o := strings.TrimSpace(outlines[vol.ID]); o != "" {
			add(workspace.PartOutline, "【所在卷："+b.name[vol.ID]+"】卷纲：\n"+o, "卷纲："+b.name[vol.ID])
		}
	}
	if material.Outline {
		if p, err := st.Plan(); err == nil {
			if rows := chapterCards(p, b, chapter); len(rows) > 0 {
				f.preload.Cards = len(rows)
				add(workspace.PartCards, "【本章的卡片】\n"+strings.Join(rows, "\n"), fmt.Sprintf("本章的卡片 %d 张", len(rows)))
			}
		}
	}
	if origin == workspace.AIRewrite {
		add(workspace.PartChapter, "【本章全文】\n"+ch.Text, fmt.Sprintf("本章全文 %d 字", workspace.Count(ch.Text)))
		return f, nil
	}
	// The volumes before, and the chapters before in the volume, in the
	// order of the book.
	outlineRows := func(nodes []*workspace.Node) []string {
		var rows []string
		for _, m := range nodes {
			if o := strings.TrimSpace(outlines[m.ID]); o != "" {
				rows = append(rows, b.name[m.ID]+"：\n"+o)
			}
		}
		return rows
	}
	vols, chapterRows := outlineRows(volumes), outlineRows(chapters)
	if len(vols) > 0 {
		add(workspace.PartOutline, "【前面各卷的卷纲】\n"+strings.Join(vols, "\n\n"), fmt.Sprintf("前面各卷的卷纲 %d 卷", len(vols)))
	}
	if len(chapterRows) > 0 {
		add(workspace.PartOutlines, "【本卷前面各章的章纲】\n"+strings.Join(chapterRows, "\n\n"), fmt.Sprintf("本卷前面各章的章纲 %d 章", len(chapterRows)))
	}
	if p, ok := tree.Previous(n); ok {
		if pc, err := st.ReadChapter(p.ID); err == nil && strings.TrimSpace(pc.Text) != "" {
			f.preload.Previous = true
			add(workspace.PartPrevious, "【上一章全文："+b.name[p.ID]+"】\n"+pc.Text, fmt.Sprintf("上一章全文：%s %d 字", b.name[p.ID], workspace.Count(pc.Text)))
		}
	}
	// A draft goes on from what the chapter has, or takes its place.
	if strings.TrimSpace(ch.Text) != "" {
		add(workspace.PartChapter, "【本章已有的正文】\n"+ch.Text, fmt.Sprintf("本章已有正文 %d 字", workspace.Count(ch.Text)))
	}
	return f, nil
}

// Preloaded describes the context a talk opened to draft or rewrite
// chapter carries before anything is said or looked up: the rules, the
// setting as it stands at the chapter, and what goes with the chapter.
// The dialog that opens the talk says it.
func Preloaded(st *workspace.Store, material aiport.Material, origin, chapter string) (Preload, error) {
	tree, err := st.Chapters()
	if err != nil {
		return Preload{}, err
	}
	var set workspace.Setting
	if material.Setting {
		set, err = st.Setting()
	}
	if err != nil {
		return Preload{}, err
	}
	f, err := fixedOf(st, material, tree, origin, chapter)
	if err != nil {
		return Preload{}, err
	}
	asked, _, err := lay(st.Work(), 1<<30, set, tree, talk{fixed: f, material: material})
	if err != nil {
		return Preload{}, err
	}
	out := f.preload
	for _, p := range asked.Sent.Parts {
		out.Tokens += p.Tokens
	}
	return out, nil
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
