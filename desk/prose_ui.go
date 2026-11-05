package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

func (h extensionHost) Bounds() ui.Rect { return h.a.LastProse }
func (h extensionHost) PaneVisible(key string) bool {
	a := h.a
	return a.page == pageWrite && (a.paneOpen() || a.foldLeaving(false)) && !a.focused() && string(a.paneTabs.shown(a, a.doc.tab)) == key
}
func (h extensionHost) PaneOpened() bool     { return h.a.fold.paneOpened }
func (h extensionHost) CloseSelectionTools() { h.a.closeFind(); h.a.write.rewriting = false }
func (h extensionHost) PaneFind(c *ui.Context, q *string, hint, add string, more func(*kit.Menu)) bool {
	return h.a.paneFind(c, q, hint, add, more)
}

func (h extensionHost) PaneHidden() bool { return h.a.doc.hidden }
