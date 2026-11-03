package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"writer/locales"
)

// MdStyle is which text the Markdown is: a reply in a talk, read at the
// size of the prose, or the smaller digest, whose [N] name entries.
type MdStyle bool

const (
	ReplyText  MdStyle = false
	DigestText MdStyle = true
)

// MarkdownState keeps options at stable addresses, shared by views of a kind.
type MarkdownState struct {
	pal           *Palette
	Reply, Digest markdown.Options
}

// MarkdownOptions are the options a view of the kind is drawn with,
// remade when the palette changes. A reply's links do nothing; a digest's
// open the entries they name.
func (m *MarkdownState) MarkdownOptions(p *Palette, st MdStyle) *markdown.Options {
	if m.Reply.OnLink == nil {
		m.Reply.CodeBlock, m.Digest.CodeBlock = m.codeBlock, m.codeBlock
		m.Reply.OnLink = func(string) {}
	}
	if m.pal != p {
		m.pal = p
		m.Reply.Theme = p.markdownTheme(FsRead, LhRead)
		m.Digest.Theme = p.markdownTheme(FsNote, LhBody)
	}
	if st == DigestText {
		return &m.Digest
	}
	return &m.Reply
}

// markdownTheme sets mygo's Markdown theme in the app's sizes and colors.
func (p *Palette) markdownTheme(size, lh float32) *markdown.Theme {
	t := markdown.DefaultTheme(p.Theme)
	t.FontSize, t.LineHeight = size, lh
	t.Link, t.StrongWeight = p.Ink, 650
	t.CodeFont, t.CodeSize, t.CodeLineHeight = "monospace", 0.9, LhBody
	t.CodeBackground, t.CodeBlockBackground = p.Side, p.Side
	t.CodeBlockBorder, t.CodeBlockRadius = p.Line, RControl
	t.BlockSpacing, t.ItemSpacing = 0.7, 0.35
	t.QuoteBar, t.QuoteText = p.Line, p.Theme.Text.Alpha(0.85)
	t.Muted, t.Rule, t.TableBorder = p.Theme.TextMuted, p.Line, p.Line
	t.TableHeader = p.Raised
	for i, scale := range [...]float32{1.35, 1.2, 1.08, 1, 1, 1} {
		t.Headings[i] = markdown.HeadingStyle{Size: scale, Weight: 650, Color: t.Text, Top: 0.7, Bottom: 0.7}
	}
	return t
}

// codeBlock is a code block with its language and a button copying it,
// in the app's words: mygo's own says Copy.
func (m *MarkdownState) codeBlock(c *ui.Context, b *markdown.CodeBlockView) {
	ui.Column(c).Background(m.pal.Side).Border(1, m.pal.Line).Radius(RControl).AlignSelf(ui.Stretch).Clip().Label("代码").Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(4, 4, 0, 12).Children(func() {
			ui.Text(c, b.Language).FontSize(FsSmall).TextColor(m.pal.Faint).Unselectable()
			ui.Spacer(c)
			if IconButton(c, IcCopy, locales.CommonCopyCode(), ui.Color{}).Disabled(b.Open).Clicked() {
				c.WriteClipboard(b.Code())
			}
		})
		b.Body(c)
	})
}
