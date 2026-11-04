package desk

import (
	"writer/desk/lore"
	"writer/ext"
)

func (a *App) registerLore(r *ext.Registrar) {
	h := extensionHost{a}
	lore.Register(r, lore.Services{Book: h, Palette: h, Notices: h, Desktop: h, Talk: h, Search: h, AI: h, Edits: h, Pane: h})
}
