package desk

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// A digest names the entries its [N] stand for, as links that open them;
// a number no entry has, and the same text in code, stay as written.
func TestDigestNamesEntries(t *testing.T) {
	a, _, entry := fixture(t)
	src := "提到 [1] 和 [99]。\n\n`[1]`\n\n```\n[1]\n```\n\n[外部链接](https://example.com)"
	tt := ui.NewTester(func(c *ui.Context) { a.markdown(c, src, digestText) }, 720, 800)
	for _, text := range []string{"「主角怕黑」", "[99]", "[1]"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("外部链接"); err != nil {
		t.Fatal(err)
	}
	if a.doc.shown != "" {
		t.Fatal("an external link opened an entry")
	}
	if err := tt.Click("主角怕黑"); err != nil {
		t.Fatal(err)
	}
	if a.doc.shown != entry.ID {
		t.Fatalf("opened entry %q, want %q", a.doc.shown, entry.ID)
	}
	if urls := tt.OpenedURLs(); len(urls) != 0 {
		t.Fatalf("digest opened %q", urls)
	}
}

// [N] in code of every kind is left as it is.
func TestDigestKeepsCode(t *testing.T) {
	a, _, _ := fixture(t)
	for _, code := range []string{
		"`[1]`", "``[1] ` [1]``", "```\n[1]\n```", "~~~\n[1]\n~~~",
		"    [1]\n", "> ```\n> [1]\n> ```", "- `[1]`",
	} {
		if got := a.digestMarkdown(code); got != code {
			t.Errorf("code %q became %q", code, got)
		}
	}
}

// An entry's name is put in as text, whatever Markdown marks it holds.
func TestDigestEscapesNames(t *testing.T) {
	a, _, _ := fixture(t)
	name := "灯[塔]*_`\\|<&~"
	setting(a).Entries[0].Names = []string{name}
	src := "**[1]**，再提到 [01] 和 [999]。\n\n| 设定 |\n|---|\n| [1] |"
	tt := ui.NewTester(func(c *ui.Context) { a.markdown(c, src, digestText) }, 720, 800)
	if !tt.HasText("「"+name+"」") || !tt.HasText("[999]") {
		t.Fatal("entry name changed the Markdown", tt.Texts())
	}
	if got := a.digestMarkdown(src); strings.Count(got, "inkgo-entry:") != 3 {
		t.Fatalf("references missing from %q", got)
	}
}

// A reply streaming shows its unfinished emphasis without the marks, and
// once done it is drawn as the finished Markdown.
func TestMarkdownStreamFinishes(t *testing.T) {
	a, _, _ := fixture(t)
	src := "开头。\n\n**正在写"
	done := false
	tt := ui.NewTester(func(c *ui.Context) {
		if done {
			a.markdown(c, src, replyText)
		} else {
			a.liveMarkdown(c, src, replyText)
		}
	}, 720, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("正在写") || tt.HasText("**") {
		t.Fatal("unfinished emphasis did not show", tt.Texts())
	}
	src += "完**。\n\n[链接](https://example.com)"
	tt.Frame()
	done = true
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("正在写完") || !tt.HasText("链接") || tt.HasText("**") {
		t.Fatal("finished reply did not show", tt.Texts())
	}
	if err := tt.Click("链接"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 0 {
		t.Fatalf("stream opened %q", urls)
	}
}

// The options keep their addresses, and their themes follow the palette.
func TestMarkdownOptionsFollowThePalette(t *testing.T) {
	a, _, _ := fixture(t)
	reply, digest := a.markdownOptions(replyText), a.markdownOptions(digestText)
	rt, dt := reply.Theme, digest.Theme
	if rt.FontSize != fsRead || rt.LineHeight != lhRead || dt.FontSize != fsNote || dt.LineHeight != lhBody {
		t.Fatal("reply and digest typography differ from their tokens")
	}
	if a.markdownOptions(replyText) != reply || a.markdownOptions(digestText) != digest || reply.Theme != rt || digest.Theme != dt {
		t.Fatal("options or themes changed within the same palette")
	}
	a.pal = a.dark
	if a.markdownOptions(replyText) != reply || a.markdownOptions(digestText) != digest {
		t.Fatal("options moved when the palette changed")
	}
	if reply.Theme == rt || digest.Theme == dt {
		t.Fatal("themes did not follow the palette")
	}
	for _, th := range []*markdown.Theme{reply.Theme, digest.Theme} {
		if th.Link != a.pal.ink || th.CodeBackground != a.pal.side || th.CodeBlockBackground != a.pal.side || th.CodeBlockBorder != a.pal.line || th.CodeBlockRadius != rControl || th.QuoteBar != a.pal.line || th.TableBorder != a.pal.line || th.Muted != a.pal.theme.TextMuted {
			t.Fatal("Markdown colors differ from the palette")
		}
	}
	if reply.Reveal != nil || digest.Reveal != nil || reply.HTML != nil || digest.HTML != nil {
		t.Fatal("reveal and raw HTML must use the defaults")
	}
}
