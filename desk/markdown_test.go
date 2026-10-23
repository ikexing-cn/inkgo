package desk

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// A digest names the entries its [N] stand for, as links that open them;
// a number no entry has, and the same text in code, stay as written.
func TestDigestNamesEntries(t *testing.T) {
	a, _, entry := fixture(t)
	src := "提到 [1] 和 [99]。\n\n`[1]`\n\n```\n[1]\n```\n\n[外部链接](https://example.com)"
	tt := ui.NewTester(func(c *ui.Context) { a.markdown(c, src, kit.DigestText) }, 720, 800)
	for _, text := range []string{"「主角怕黑」", "[99]", "[1]"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("外部链接"); err != nil {
		t.Fatal(err)
	}
	if shownEntry(a) != "" {
		t.Fatal("an external link opened an entry")
	}
	if err := tt.Click("主角怕黑"); err != nil {
		t.Fatal(err)
	}
	if shownEntry(a) != entry.ID {
		t.Fatalf("opened entry %q, want %q", shownEntry(a), entry.ID)
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
	tt := ui.NewTester(func(c *ui.Context) { a.markdown(c, src, kit.DigestText) }, 720, 800)
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
			a.markdown(c, src, kit.ReplyText)
		} else {
			a.liveMarkdown(c, src, kit.ReplyText)
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
