package talk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// Markdown runs on the frame thread.
func (a *Panel) Markdown(c *ui.Context, text string, style kit.MdStyle) ui.Element {
	return a.renderer.Draw(c, text, style)
}

// LiveMarkdown runs on the frame thread.
func (a *Panel) LiveMarkdown(c *ui.Context, text string, style kit.MdStyle) ui.Element {
	return a.renderer.Stream(c, text, style)
}

// DigestMarkdown runs on the frame thread.
func (a *Panel) DigestMarkdown(text string) string { return a.renderer.Digest(text) }
