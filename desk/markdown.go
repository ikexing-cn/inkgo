package desk

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// The AI writes Markdown. It is parsed once per text into blocks, which
// the view draws with native text: links are styled but not followed, and
// raw HTML shows as text, so a reply cannot run anything.

type mdKind uint8

const (
	mdParagraph mdKind = iota
	mdHeading
	mdList
	mdQuote
	mdCode
	mdRule
	mdTable
)

type mdBlock struct {
	kind    mdKind
	level   int // a heading's
	inline  []mdSpan
	code    string
	ordered bool
	start   int         // an ordered list's first number
	items   [][]mdBlock // a list's
	blocks  []mdBlock   // a quote's
	rows    [][][]mdSpan
}

type mdSpan struct {
	text                       string
	bold, italic, code, strike bool
	link                       bool
}

var md = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))

// mdCache keeps parsed texts; a streaming reply changes each frame, so it
// is bounded rather than keyed by message.
type mdCache map[string][]mdBlock

func (m mdCache) get(s string) []mdBlock {
	if b, ok := m[s]; ok {
		return b
	}
	if len(m) > 256 {
		clear(m)
	}
	src := []byte(s)
	b := mdBlocks(md.Parser().Parse(text.NewReader(src)), src)
	m[s] = b
	return b
}

func mdBlocks(parent ast.Node, src []byte) []mdBlock {
	var out []mdBlock
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			out = append(out, mdBlock{kind: mdParagraph, inline: mdInline(n, src, mdSpan{})})
		case *ast.Heading:
			out = append(out, mdBlock{kind: mdHeading, level: n.Level, inline: mdInline(n, src, mdSpan{})})
		case *ast.List:
			b := mdBlock{kind: mdList, ordered: n.IsOrdered(), start: n.Start}
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				b.items = append(b.items, mdBlocks(item, src))
			}
			out = append(out, b)
		case *ast.Blockquote:
			out = append(out, mdBlock{kind: mdQuote, blocks: mdBlocks(n, src)})
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			var b strings.Builder
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				b.Write(seg.Value(src))
			}
			out = append(out, mdBlock{kind: mdCode, code: strings.TrimRight(b.String(), "\n")})
		case *ast.ThematicBreak:
			out = append(out, mdBlock{kind: mdRule})
		case *east.Table:
			b := mdBlock{kind: mdTable}
			for row := n.FirstChild(); row != nil; row = row.NextSibling() {
				var cells [][]mdSpan
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					style := mdSpan{bold: row.Kind() == east.KindTableHeader}
					cells = append(cells, mdInline(cell, src, style))
				}
				b.rows = append(b.rows, cells)
			}
			out = append(out, b)
		case *ast.HTMLBlock:
			var b strings.Builder
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				b.Write(seg.Value(src))
			}
			out = append(out, mdBlock{kind: mdParagraph, inline: []mdSpan{{text: strings.TrimRight(b.String(), "\n")}}})
		default:
			if n.HasChildren() {
				out = append(out, mdBlocks(n, src)...)
			}
		}
	}
	return out
}

// mdInline flattens a block's inline content into styled runs.
func mdInline(parent ast.Node, src []byte, style mdSpan) []mdSpan {
	var out []mdSpan
	add := func(s string, st mdSpan) {
		if s == "" {
			return
		}
		if n := len(out); n > 0 {
			last := out[n-1]
			if last.bold == st.bold && last.italic == st.italic && last.code == st.code && last.strike == st.strike && last.link == st.link {
				out[n-1].text += s
				return
			}
		}
		st.text = s
		out = append(out, st)
	}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			add(string(n.Segment.Value(src)), style)
			if n.HardLineBreak() || n.SoftLineBreak() {
				add("\n", style)
			}
		case *ast.String:
			add(string(n.Value), style)
		case *ast.CodeSpan:
			st := style
			st.code = true
			var b strings.Builder
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					b.Write(t.Segment.Value(src))
				}
			}
			add(b.String(), st)
		case *ast.Emphasis:
			st := style
			if n.Level >= 2 {
				st.bold = true
			} else {
				st.italic = true
			}
			for _, s := range mdInline(n, src, st) {
				add(s.text, s)
			}
		case *east.Strikethrough:
			st := style
			st.strike = true
			for _, s := range mdInline(n, src, st) {
				add(s.text, s)
			}
		case *ast.Link, *ast.AutoLink:
			st := style
			st.link = true
			if a, ok := n.(*ast.AutoLink); ok {
				add(string(a.URL(src)), st)
				continue
			}
			for _, s := range mdInline(n, src, st) {
				add(s.text, s)
			}
		case *ast.RawHTML:
			segs := n.Segments
			for i := 0; i < segs.Len(); i++ {
				seg := segs.At(i)
				add(string(seg.Value(src)), style)
			}
		default:
			for _, s := range mdInline(n, src, style) {
				add(s.text, s)
			}
		}
	}
	return out
}

// markdown draws s as the body of a reply.
func (a *App) markdown(c *ui.Context, s string, size float32) *ui.Element {
	return ui.Column(c).FillWidth().Gap(size * 0.7).Children(func() {
		a.mdDraw(c, a.md.get(s), size)
	})
}

// liveMarkdown draws a reply still streaming. Laying out one text costs
// as much as its length, so the blocks written stand apart from the last,
// which grows: they keep their text, and their layout, until it is done.
func (a *App) liveMarkdown(c *ui.Context, s string, size float32) *ui.Element {
	return ui.Column(c).FillWidth().Gap(size * 0.7).Children(func() {
		blocks := a.md.get(s)
		n := len(blocks) - 1
		if n <= 0 {
			a.mdDraw(c, blocks, size)
			return
		}
		a.mdDraw(c, blocks[:n], size)
		tail := ui.Column(c).FillWidth()
		if mdFlows(blocks[n-1]) && mdFlows(blocks[n]) {
			// The blank line the two would have between them.
			tail.Padding(size*1.8-size*0.7, 0, 0, 0)
		}
		tail.Children(func() { a.mdDraw(c, blocks[n:], size) })
	})
}

// mdDraw draws blocks. Paragraphs, headings and lists of text run
// together as one text, so the author can select across them; quotes,
// code and tables stand apart, and a selection stops at them.
func (a *App) mdDraw(c *ui.Context, blocks []mdBlock, size float32) {
	t := a.pal.theme
	var flow []ui.Span
	flush := func() {
		if len(flow) > 0 {
			ui.RichText(c, flow...).FillWidth().FontSize(size).LineHeight(1.8).TextColor(t.Text).Selectable()
			flow = nil
		}
	}
	for _, b := range blocks {
		if mdFlows(b) {
			if len(flow) > 0 {
				// A blank line parts blocks: lines are no shorter than
				// the text's line height, so it cannot be a smaller gap.
				flow = append(flow, ui.Span{Text: "\n\n"})
			}
			flow = a.mdFlow(flow, b, size, 0)
			continue
		}
		flush()
		switch b.kind {
		case mdList:
			ui.Column(c).FillWidth().Gap(size * 0.35).Children(func() {
				for n, item := range b.items {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Start).Children(func() {
						ui.Text(c, mdMark(b, n)).FontSize(size).LineHeight(1.8).TextColor(t.TextMuted).Width(size * 1.4).TextAlign(ui.End).Shrink(0)
						ui.Column(c).Grow(1).Shrink(1).Gap(size * 0.35).Children(func() { a.mdDraw(c, item, size) })
					})
				}
			})
		case mdQuote:
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Stretch).Children(func() {
				ui.Box(c).Width(3).Radius(rRound).Background(a.pal.line).Shrink(0)
				ui.Column(c).Grow(1).Shrink(1).Gap(size * 0.5).Opacity(0.85).Children(func() { a.mdDraw(c, b.blocks, size) })
			})
		case mdCode:
			ui.ScrollHorizontal(c).FillWidth().Radius(rControl).Background(a.pal.side).Border(1, a.pal.line).Children(func() {
				ui.Text(c, b.code).Font("monospace").FontSize(size*0.88).LineHeight(1.6).NoWrap().Padding(10, 12).Selectable()
			})
		case mdRule:
			a.pal.hairline(c)
		case mdTable:
			a.pal.group(c).Children(func() {
				for n, row := range b.rows {
					if n > 0 {
						a.pal.hairline(c)
					}
					ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
						for _, cell := range row {
							ui.Column(c).Grow(1).Shrink(1).Basis(0).Padding(8, 10).Children(func() {
								ui.RichText(c, a.mdRuns(nil, cell, size*0.93)...).FillWidth().FontSize(size * 0.93).LineHeight(1.6).TextColor(t.Text).Selectable()
							})
						}
					})
				}
			})
		}
	}
	flush()
}

// mdFlows reports whether b is text that runs with the text around it: a
// list does when its items hold only text, lists of text among it.
func mdFlows(b mdBlock) bool {
	switch b.kind {
	case mdParagraph, mdHeading:
		return true
	case mdList:
		for _, item := range b.items {
			for _, ib := range item {
				if ib.kind != mdParagraph && (ib.kind != mdList || !mdFlows(ib)) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// mdFlow appends b, which flows, to the runs of a text. A list's items
// start lines with their marks, nested lists indented an ideographic
// space a level;
// without hanging indents, an item's wrapped lines start at the margin.
func (a *App) mdFlow(out []ui.Span, b mdBlock, size float32, depth int) []ui.Span {
	switch b.kind {
	case mdParagraph:
		return a.mdRuns(out, b.inline, size)
	case mdHeading:
		scale := map[int]float32{1: 1.35, 2: 1.2, 3: 1.08}[b.level]
		if scale == 0 {
			scale = 1
		}
		from := len(out)
		out = a.mdRuns(out, b.inline, size*scale)
		for i := from; i < len(out); i++ {
			out[i].Size, out[i].Weight = size*scale, 650
		}
		return out
	}
	for n, item := range b.items {
		if n > 0 {
			out = append(out, ui.Span{Text: "\n"})
		}
		out = append(out, ui.Span{Text: strings.Repeat("　", depth) + mdMark(b, n) + " ", Color: a.pal.theme.TextMuted})
		for k, ib := range item {
			if k > 0 {
				out = append(out, ui.Span{Text: "\n"})
			}
			if ib.kind == mdList {
				out = a.mdFlow(out, ib, size, depth+1)
			} else {
				out = a.mdFlow(out, ib, size, depth)
			}
		}
	}
	return out
}

func mdMark(b mdBlock, n int) string {
	if b.ordered {
		return strconv.Itoa(b.start+n) + "."
	}
	return "•"
}

// mdRuns appends styled runs of text at size to out.
func (a *App) mdRuns(out []ui.Span, spans []mdSpan, size float32) []ui.Span {
	for _, s := range spans {
		r := ui.Span{Text: s.text, Italic: s.italic, Strikethrough: s.strike}
		if s.bold {
			r.Weight = 650
		}
		if s.code {
			r.Font, r.Size, r.Background = "monospace", size*0.9, a.pal.side
		}
		if s.link {
			r.Color, r.Underline = a.pal.ink, true
		}
		out = append(out, r)
	}
	return out
}
