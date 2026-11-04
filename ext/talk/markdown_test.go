package talk

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

// A digest names the entries its [N] stand for, as links that open them;
// a number no entry has, and the same text in code, stay as written.
func TestDigestNamesEntries(t *testing.T) {
	entry := workspace.Entry{ID: "entry", Text: "主角怕黑"}
	opened := ""
	a := &Markdown{Palette: kit.LightPalette, Entries: func() []workspace.Entry { return []workspace.Entry{entry} }, ShowEntry: func(e workspace.Entry) { opened = e.ID }}
	src := "提到 [1] 和 [99]。\n\n`[1]`\n\n```\n[1]\n```\n\n[外部链接](https://example.com)"
	tt := stillTester(func(c *ui.Context) { a.Draw(c, src, kit.DigestText) }, 720, 800)
	for _, text := range []string{"「主角怕黑」", "[99]", "[1]"} {
		if !tt.HasText(text) {
			t.Errorf("missing %q in %q", text, tt.Texts())
		}
	}
	if err := tt.Click("外部链接"); err != nil {
		t.Fatal(err)
	}
	if opened != "" {
		t.Fatal("an external link opened an entry")
	}
	if err := tt.Click("主角怕黑"); err != nil {
		t.Fatal(err)
	}
	if opened != entry.ID {
		t.Fatalf("opened entry %q, want %q", opened, entry.ID)
	}
	if urls := tt.OpenedURLs(); len(urls) != 0 {
		t.Fatalf("digest opened %q", urls)
	}
}

// [N] in code of every kind is left as it is.
func TestDigestKeepsCode(t *testing.T) {
	entry := workspace.Entry{ID: "entry", Text: "主角怕黑"}
	a := &Markdown{Palette: kit.LightPalette, Entries: func() []workspace.Entry { return []workspace.Entry{entry} }, ShowEntry: func(workspace.Entry) {}}
	for _, code := range []string{
		"`[1]`", "``[1] ` [1]``", "```\n[1]\n```", "~~~\n[1]\n~~~",
		"    [1]\n", "> ```\n> [1]\n> ```", "- `[1]`",
	} {
		if got := a.Digest(code); got != code {
			t.Errorf("code %q became %q", code, got)
		}
	}
}

// An entry's name is put in as text, whatever Markdown marks it holds.
func TestDigestEscapesNames(t *testing.T) {
	entry := workspace.Entry{ID: "entry", Text: "主角怕黑"}
	a := &Markdown{Palette: kit.LightPalette, Entries: func() []workspace.Entry { return []workspace.Entry{entry} }, ShowEntry: func(workspace.Entry) {}}
	name := "灯[塔]*_`\\|<&~"
	entry.Names = []string{name}
	src := "**[1]**，再提到 [01] 和 [999]。\n\n| 设定 |\n|---|\n| [1] |"
	tt := stillTester(func(c *ui.Context) { a.Draw(c, src, kit.DigestText) }, 720, 800)
	if !tt.HasText("「"+name+"」") || !tt.HasText("[999]") {
		t.Fatal("entry name changed the Markdown", tt.Texts())
	}
	if got := a.Digest(src); strings.Count(got, "inkgo-entry:") != 3 {
		t.Fatalf("references missing from %q", got)
	}
}

// A reply streaming shows its unfinished emphasis without the marks, and
// once done it is drawn as the finished Markdown.
func TestMarkdownStreamFinishes(t *testing.T) {
	entry := workspace.Entry{ID: "entry", Text: "主角怕黑"}
	a := &Markdown{Palette: kit.LightPalette, Entries: func() []workspace.Entry { return []workspace.Entry{entry} }, ShowEntry: func(workspace.Entry) {}}
	src := "开头。\n\n**正在写"
	done := false
	tt := stillTester(func(c *ui.Context) {
		if done {
			a.Draw(c, src, kit.ReplyText)
		} else {
			a.Stream(c, src, kit.ReplyText)
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

func stillTester(draw func(*ui.Context), w, h int) *ui.Tester {
	tt := ui.NewTester(draw, w, h)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}
