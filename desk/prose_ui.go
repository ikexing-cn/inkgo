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
func (h extensionHost) CloseSelectionTools() { h.a.closeFind() }
func (h extensionHost) PaneFind(c *ui.Context, q *string, hint, add string, more func(*kit.Menu)) bool {
	return h.a.paneFind(c, q, hint, add, more)
}

func (h extensionHost) PaneHidden() bool { return h.a.doc.hidden }

func (h extensionHost) ChapterText(id string) string {
	if c := h.a.write.open[id]; c != nil && c.ed != nil {
		return c.ed.Doc.String()
	}
	ch, err := h.a.Store.ReadChapter(id)
	if err != nil {
		return ""
	}
	return ch.Text
}
func (h extensionHost) EnsureTab(id string) {
	for _, tab := range h.a.write.tabs {
		if tab == id {
			return
		}
	}
	h.a.write.tabs = append(h.a.write.tabs, id)
	h.a.savePrefs()
}
func (h extensionHost) Band(c *ui.Context, top, bottom float32, fn func()) {
	h.a.overProse(c, top, bottom, fn)
}
