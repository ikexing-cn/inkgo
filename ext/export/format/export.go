// Package format lays a work out to be sent away, as a manuscript is: its
// prose, its outline or its setting, as plain text or as a Word document.
package format

import (
	"strings"

	"writer/workspace"
)

// Style is how a paragraph is set: Word's own styles, so that a change to
// one in Word changes the whole document.
type Style int

const (
	Body    Style = iota // 正文 (Normal)
	Heading              // 标题 1, 2, 3 by Level: a volume, a chapter, a category
)

// Para is a paragraph. Lead is said first and set bold: an entry's name.
// NewPage starts it on a page of its own.
type Para struct {
	Style   Style
	Level   int // of a Heading, from 1
	Lead    string
	Text    string
	NewPage bool
}

// Cover is the first page of a manuscript, on its own and unnumbered.
type Cover struct {
	Title string
	Pen   string // the author's name, or empty for none
	Words string // how long what is sent is, as the author reads it
}

// Doc is what is exported. Title is said first as plain text; as a Word
// document the Cover is, when there is one, and the Title otherwise.
type Doc struct {
	// Language selects the manuscript typography; empty means Chinese.
	Language workspace.Language
	Title    string
	Cover    *Cover
	Paras    []Para
}

// level is a heading's level for a node: a folder by its depth, a
// document one below the top, so a chapter outside any volume is still
// a chapter; Word's styles go to 标题 3 here.
func level(p workspace.Piece) int {
	l := p.Depth + 1
	if !p.Folder {
		l = max(l, 2)
	}
	return min(l, 3)
}

// paragraphs are the lines of a chapter's prose: a blank one inside it
// stays, as the author may mean a break between scenes by it.
func paragraphs(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// lines are the paragraphs of a text, the blank ones left out.
func lines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// Within is the pieces of chapters, in their order, each under the
// folders holding it, which come once; nil is all of the pieces.
func Within(pieces []workspace.Piece, chapters map[string]bool) []workspace.Piece {
	if chapters == nil {
		return pieces
	}
	type folder struct {
		workspace.Piece
		out bool
	}
	var out []workspace.Piece
	var above []folder
	for _, p := range pieces {
		for len(above) > 0 && above[len(above)-1].Depth >= p.Depth {
			above = above[:len(above)-1]
		}
		if p.Folder {
			above = append(above, folder{Piece: p})
			continue
		}
		if !chapters[p.ID] {
			continue
		}
		for i := range above {
			if !above[i].out {
				out, above[i].out = append(out, above[i].Piece), true
			}
		}
		out = append(out, p)
	}
	return out
}

// Prose is the prose of pieces under their titles, each document from a
// new page; a volume's title heads the page of its first chapter. whole
// says it is the whole work, which plain text opens with the work's name.
func Prose(cover Cover, pieces []workspace.Piece, whole bool) Doc {
	d := Doc{Cover: &cover}
	if whole {
		d.Title = cover.Title
	}
	under := false // the paragraph before is a folder's title
	for i, p := range pieces {
		d.Paras = append(d.Paras, Para{Style: Heading, Level: level(p), Text: p.Title, NewPage: i > 0 && !under})
		under = p.Folder
		for _, l := range paragraphs(p.Text) {
			d.Paras = append(d.Paras, Para{Style: Body, Text: l})
		}
	}
	return d
}

// Outline is the outline of pieces under their titles, one after the
// other; a title without an outline shows which are not written yet.
func Outline(name string, pieces []workspace.Piece) Doc {
	d := Doc{Title: name + " · 大纲"}
	for _, p := range pieces {
		d.Paras = append(d.Paras, Para{Style: Heading, Level: level(p), Text: p.Title})
		for _, l := range lines(p.Outline) {
			d.Paras = append(d.Paras, Para{Style: Body, Text: l})
		}
	}
	return d
}

// Setting is what the author decided, by category in the work's order:
// each entry its name in bold, what else it is called, and what it says;
// then what later says of it, each the way it reads from a chapter on.
func Setting(name string, categories []workspace.Category, entries []workspace.Entry, later func(workspace.Entry) []string) Doc {
	d := Doc{Title: name + " · 设定"}
	for _, cat := range categories {
		head := false
		for _, e := range entries {
			if e.Place != workspace.PlaceBody || e.Category != cat.Name {
				continue
			}
			if !head {
				d.Paras = append(d.Paras, Para{Style: Heading, Level: 1, Text: cat.Name})
				head = true
			}
			for i, l := range lines(e.Text) {
				p := Para{Style: Body, Text: l}
				if i == 0 && len(e.Names) > 0 {
					p.Lead = e.Names[0]
					p.Text = "：" + l
					if len(e.Names) > 1 {
						p.Text = "（" + strings.Join(e.Names[1:], "、") + "）" + p.Text
					}
				}
				d.Paras = append(d.Paras, p)
			}
			if later == nil {
				continue
			}
			for _, c := range later(e) {
				for _, l := range lines(c) {
					d.Paras = append(d.Paras, Para{Style: Body, Text: l})
				}
			}
		}
	}
	return d
}

// Settled says whether entries hold anything Setting would export.
func Settled(entries []workspace.Entry) bool {
	for _, e := range entries {
		if e.Place == workspace.PlaceBody {
			return true
		}
	}
	return false
}

// Text is d as plain text: titles each on a line with a blank line about
// them, the paragraphs under them one to a line; indent puts two
// full-width spaces before each paragraph of the body.
func (d Doc) Text(indent bool) string {
	if d.Language == "en" {
		indent = false
		d = d.english()
	}
	var b strings.Builder
	first, body := true, false
	head := func(s string) {
		if !first {
			b.WriteString("\n")
		}
		b.WriteString(s + "\n")
		first, body = false, false
	}
	if d.Title != "" {
		head(d.Title)
	}
	for _, p := range d.Paras {
		if p.Style != Body {
			head(p.Text)
			continue
		}
		if !body && !first {
			b.WriteString("\n")
		}
		if indent && strings.TrimSpace(p.Lead+p.Text) != "" {
			b.WriteString("　　")
		}
		b.WriteString(p.Lead + p.Text + "\n")
		first, body = false, true
	}
	return b.String()
}

// english localizes generated manuscript labels without rewriting the prose.
func (d Doc) english() Doc {
	d.Paras = append([]Para(nil), d.Paras...)
	d.Title = strings.TrimSuffix(strings.TrimSuffix(d.Title, " · 大纲"), " · 设定") + func() string {
		if strings.HasSuffix(d.Title, " · 大纲") {
			return " · Outline"
		}
		if strings.HasSuffix(d.Title, " · 设定") {
			return " · Lore"
		}
		return ""
	}()
	for i, p := range d.Paras {
		if p.Lead != "" {
			p.Text = strings.NewReplacer("：", ": ", "（", " (", "）", ")", "、", ", ").Replace(p.Text)
			d.Paras[i] = p
		}
	}
	return d
}
