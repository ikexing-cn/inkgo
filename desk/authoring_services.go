package desk

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"writer/workspace"
)

// AIEnabled implements the frame-thread host capability.
func (h extensionHost) AIEnabled() bool { return h.a.aiOn() }

// OpenTalk implements the frame-thread host capability.
func (h extensionHost) OpenTalk(id string) { h.a.page = pageTalk; h.a.switchSession(id) }

// TalkTitle implements the frame-thread host capability.
func (h extensionHost) TalkTitle(id string) (string, bool) { return h.a.talkName(id) }

// FindInChapter implements the frame-thread host capability.
func (h extensionHost) FindInChapter(id, quote string) { h.a.findInChapter(id, quote) }

// Flashed implements the frame-thread host capability.
func (h extensionHost) Flashed(id string) (time.Time, bool) { at, ok := h.a.flash[id]; return at, ok }

// ClearFlash implements the frame-thread host capability.
func (h extensionHost) ClearFlash(id string) { delete(h.a.flash, id) }

// Refresh implements the frame-thread host capability.
func (h extensionHost) Refresh() error { return h.a.refreshFromSetting() }

// UndoRevision implements the frame-thread host capability.
func (h extensionHost) UndoRevision(text string, ops ...workspace.Op) { h.a.undoSetting(text, ops...) }

// CountText implements the frame-thread host capability.
func (h extensionHost) CountText(text string) int {
	if text == "" || h.a.loadTree() != nil {
		return 0
	}
	texts, err := h.a.Store.Texts()
	if err != nil {
		return 0
	}
	if ed := h.a.write.cur().ed; ed != nil {
		texts[h.a.write.node] = ed.Doc.String()
	}
	n := 0
	for _, s := range texts {
		n += strings.Count(s, text)
	}
	return n
}

// MenuOpen implements the frame-thread host capability.
func (h extensionHost) MenuOpen(key string) bool { return h.a.menu.MenuOpen(key) }

// RevealPane implements the frame-thread host capability.
func (h extensionHost) RevealPane(tab string) {
	h.a.setDocHidden(false)
	h.a.doc.tab = paneTab(tab)
	if h.a.doc.tab != paneTalk {
		h.a.doc.talks = false
	}
}

// StartTalk implements the temporary conversation navigation capability.
func (h extensionHost) StartTalk(c *ui.Context, words workspace.Event) bool {
	return h.a.startTalk(c, words)
}

// ShowEntry opens a digest reference on the frame thread.
func (h extensionHost) ShowEntry(e workspace.Entry) { h.a.showEntry(e) }

// StopTalk cancels the conversation as leaving the book does. Keep the busy
// marker until the worker returns, so Close cannot close its store underneath it.
func (h extensionHost) StopTalk() {
	a := h.a
	if a.turn.active && a.cancel != nil {
		a.cancel()
		a.dropProps(a.write.props, workspace.FateClosed)
		a.turn.active = false
	}
}
