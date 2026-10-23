package desk

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"writer/desk/kit"
)

// mdStyle is which text the Markdown is: a reply in a talk, read at the
// size of the prose, or the smaller digest, whose [N] name entries.
type mdStyle bool

const (
	replyText  mdStyle = false
	digestText mdStyle = true
)

// markdownState keeps options at stable addresses, shared by views of a kind.
type markdownState struct {
	pal           *kit.Palette
	reply, digest markdown.Options
}

// markdownOptions are the options a view of the kind is drawn with,
// remade when the palette changes. A reply's links do nothing; a digest's
// open the entries they name.
func (a *App) markdownOptions(st mdStyle) *markdown.Options {
	m := &a.md
	if m.reply.OnLink == nil {
		m.reply.CodeBlock, m.digest.CodeBlock = a.codeBlock, a.codeBlock
		m.reply.OnLink = func(string) {}
		m.digest.OnLink = func(url string) {
			if number, ok := strings.CutPrefix(url, "inkgo-entry:"); ok {
				n, err := strconv.Atoi(number)
				if err == nil && n > 0 && n <= len(a.book.Setting().Entries) {
					a.showEntry(a.book.Setting().Entries[n-1])
				}
			}
		}
	}
	if m.pal != a.pal {
		m.pal = a.pal
		m.reply.Theme = a.markdownTheme(kit.FsRead, kit.LhRead)
		m.digest.Theme = a.markdownTheme(kit.FsNote, kit.LhBody)
	}
	if st == digestText {
		return &m.digest
	}
	return &m.reply
}

// markdownTheme sets mygo's Markdown theme in the app's sizes and colors.
func (a *App) markdownTheme(size, lh float32) *markdown.Theme {
	t := markdown.DefaultTheme(a.pal.Theme)
	t.FontSize, t.LineHeight = size, lh
	t.Link, t.StrongWeight = a.pal.Ink, 650
	t.CodeFont, t.CodeSize, t.CodeLineHeight = "monospace", 0.9, kit.LhBody
	t.CodeBackground, t.CodeBlockBackground = a.pal.Side, a.pal.Side
	t.CodeBlockBorder, t.CodeBlockRadius = a.pal.Line, kit.RControl
	t.BlockSpacing, t.ItemSpacing = 0.7, 0.35
	t.QuoteBar, t.QuoteText = a.pal.Line, a.pal.Theme.Text.Alpha(0.85)
	t.Muted, t.Rule, t.TableBorder = a.pal.Theme.TextMuted, a.pal.Line, a.pal.Line
	t.TableHeader = a.pal.Raised
	for i, scale := range [...]float32{1.35, 1.2, 1.08, 1, 1, 1} {
		t.Headings[i] = markdown.HeadingStyle{Size: scale, Weight: 650, Color: t.Text, Top: 0.7, Bottom: 0.7}
	}
	return t
}

// markdown draws finished Markdown, selectable across its blocks.
func (a *App) markdown(c *ui.Context, s string, st mdStyle) ui.Element {
	if st == digestText {
		s = a.digestMarkdown(s)
	}
	return markdown.View(c, s, a.markdownOptions(st)).FillWidth()
}

// liveMarkdown draws Markdown still being written: the block not yet
// ended shows as it stands, and what comes in fades in.
func (a *App) liveMarkdown(c *ui.Context, s string, st mdStyle) ui.Element {
	return markdown.Stream(c, s, a.markdownOptions(st)).FillWidth()
}

// codeBlock is a code block with its language and a button copying it,
// in the app's words: mygo's own says Copy.
func (a *App) codeBlock(c *ui.Context, b *markdown.CodeBlockView) {
	ui.Column(c).Background(a.pal.Side).Border(1, a.pal.Line).Radius(kit.RControl).AlignSelf(ui.Stretch).Clip().Label("代码").Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(4, 4, 0, 12).Children(func() {
			ui.Text(c, b.Language).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Unselectable()
			ui.Spacer(c)
			if kit.IconButton(c, kit.IcCopy, "复制代码", ui.Color{}).Disabled(b.Open).Clicked() {
				c.WriteClipboard(b.Code())
			}
		})
		b.Body(c)
	})
}

// digestMarkdown names entry references in text, leaving code and link
// destinations as written. Source offsets keep the rest of the Markdown intact.
func (a *App) digestMarkdown(s string) string {
	d := parser.Parse(s)
	var out strings.Builder
	at := 0
	for i := 1; i < d.Len(); i++ {
		inlines := d.Inlines(i)
		for j := 0; j < len(inlines); j++ {
			in := inlines[j]
			// Links cannot contain another link.
			if in.Kind == parser.Link || in.Kind == parser.Image {
				j = in.Next - 1
				continue
			}
			if in.Kind != parser.Text {
				continue
			}
			for _, m := range a.book.Setting().Mentions(s[in.Start:in.End]) {
				if m.Entry == nil {
					continue
				}
				start, end := in.Start+m.Start, in.Start+m.End
				out.WriteString(s[at:start])
				out.WriteString("「[")
				for _, r := range m.Text {
					if strings.ContainsRune("\\`*_{}[]()#+-.!|<>&~", r) {
						out.WriteString("&#")
						out.WriteString(strconv.Itoa(int(r)))
						out.WriteByte(';')
					} else {
						out.WriteRune(r)
					}
				}
				out.WriteString("](inkgo-entry:")
				out.WriteString(s[start+1 : end-1])
				out.WriteString(")」")
				at = end
			}
		}
	}
	if at == 0 {
		return s
	}
	out.WriteString(s[at:])
	return out.String()
}
