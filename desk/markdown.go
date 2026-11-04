package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

func (a *App) markdown(c *ui.Context, text string, style kit.MdStyle) ui.Element {
	return a.talkAPI.Renderer().Draw(c, text, style)
}
func (a *App) liveMarkdown(c *ui.Context, text string, style kit.MdStyle) ui.Element {
	return a.talkAPI.Renderer().Stream(c, text, style)
}
func (a *App) digestMarkdown(text string) string { return a.talkAPI.Renderer().Digest(text) }
