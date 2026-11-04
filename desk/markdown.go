package desk

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"writer/desk/kit"
	"writer/workspace"
)

// markdownOptions are kit's options for the kind, with a digest's links
// opening the entries they name.
func (a *App) markdownOptions(st kit.MdStyle) *markdown.Options {
	m := &a.md
	if m.Digest.OnLink == nil {
		m.Digest.OnLink = func(url string) {
			if number, ok := strings.CutPrefix(url, "inkgo-entry:"); ok {
				n, err := strconv.Atoi(number)
				if err == nil && n > 0 && n <= len(a.entries()) {
					a.showEntry(a.entries()[n-1])
				}
			}
		}
	}
	return m.MarkdownOptions(a.pal, st)
}

// markdown draws finished Markdown, selectable across its blocks.
func (a *App) markdown(c *ui.Context, s string, st kit.MdStyle) ui.Element {
	if st == kit.DigestText {
		s = a.digestMarkdown(s)
	}
	return markdown.View(c, s, a.markdownOptions(st)).FillWidth()
}

// liveMarkdown draws Markdown still being written: the block not yet
// ended shows as it stands, and what comes in fades in.
func (a *App) liveMarkdown(c *ui.Context, s string, st kit.MdStyle) ui.Element {
	return markdown.Stream(c, s, a.markdownOptions(st)).FillWidth()
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
			for _, m := range (workspace.Setting{Entries: a.entries()}).Mentions(s[in.Start:in.End]) {
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
