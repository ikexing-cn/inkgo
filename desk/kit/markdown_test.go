package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// The options keep their addresses, and their themes follow the palette.
func TestMarkdownOptionsFollowThePalette(t *testing.T) {
	p := LightPalette()
	m := MarkdownState{}
	reply, digest := m.MarkdownOptions(p, ReplyText), m.MarkdownOptions(p, DigestText)
	rt, dt := reply.Theme, digest.Theme
	if rt.FontSize != FsRead || rt.LineHeight != LhRead || dt.FontSize != FsNote || dt.LineHeight != LhBody {
		t.Fatal("reply and digest typography differ from their tokens")
	}
	if m.MarkdownOptions(p, ReplyText) != reply || m.MarkdownOptions(p, DigestText) != digest || reply.Theme != rt || digest.Theme != dt {
		t.Fatal("options or themes changed within the same palette")
	}
	p = DarkPalette()
	if m.MarkdownOptions(p, ReplyText) != reply || m.MarkdownOptions(p, DigestText) != digest {
		t.Fatal("options moved when the palette changed")
	}
	if reply.Theme == rt || digest.Theme == dt {
		t.Fatal("themes did not follow the palette")
	}
	for _, th := range []*markdown.Theme{reply.Theme, digest.Theme} {
		if th.Link != p.Ink || th.CodeBackground != p.Side || th.CodeBlockBackground != p.Side || th.CodeBlockBorder != p.Line || th.CodeBlockRadius != RControl || th.QuoteBar != p.Line || th.TableBorder != p.Line || th.Muted != p.Theme.TextMuted {
			t.Fatal("Markdown colors differ from the palette")
		}
	}
	if reply.Reveal != nil || digest.Reveal != nil || reply.HTML != nil || digest.HTML != nil {
		t.Fatal("reveal and raw HTML must use the defaults")
	}
}

func TestMarkdownCodeBlockCopy(t *testing.T) {
	p := LightPalette()
	m := MarkdownState{}
	tt := ui.NewTester(func(c *ui.Context) {
		markdown.View(c, "```go\ncode\n```", m.MarkdownOptions(p, ReplyText))
	}, 720, 400)
	if !tt.HasText("go") || !tt.HasText("code") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("复制代码"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "code\n" {
		t.Fatalf("copied %q", got)
	}
}
