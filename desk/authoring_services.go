package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

// AIEnabled implements the frame-thread host capability.
func (h extensionHost) AIEnabled() bool { return h.a.aiOn() && h.a.extensions.Own("ai") }

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
	h.a.paneChanged(tab)

}

// HasBeside implements frame-thread pane availability.
func (h extensionHost) HasBeside() bool {
	return len(ext.Items(h.a.extensions, hostapi.BesideItems)) > 0
}

// PaneOpen implements frame-thread pane state.
func (h extensionHost) PaneOpen() bool { return !h.a.doc.hidden }

// TogglePane implements frame-thread pane navigation.
func (h extensionHost) TogglePane() { h.a.setDocHidden(!h.a.doc.hidden) }

// Selected implements frame-thread pane selection.
func (h extensionHost) SelectedTab() string { return string(h.a.paneTabs.shown(h.a, h.a.doc.tab)) }

// Flash marks changed book content on the frame thread.
func (h extensionHost) Flash(id string) { h.a.flash[id] = time.Now() }

// Request starts a nonblocking author operation on the frame thread.
func (h extensionHost) Request(sv ui.Services, label string, fn func(context.Context) (func(), error)) {
	h.a.nonblocking = true
	h.a.work(sv, label, fn)
}

// Post dispatches from any goroutine onto the frame thread.
func (h extensionHost) Post(sv ui.Services, fn func()) { h.a.jobs.post(sv, fn) }

// SavePreferences saves contributed view state on the frame thread.
func (h extensionHost) SavePreferences() error { return h.a.savePrefs() }

// Page returns the host's selected page identity on the frame thread.
func (h extensionHost) Page() string {
	for id, p := range h.a.pageIDs {
		if p == h.a.page {
			return id
		}
	}
	return ""
}

// EntryAvailable reports optional entry navigation on the frame thread.
func (h extensionHost) EntryAvailable() bool { return h.a.entryNavigationAvailable() }

// ShowEntry delegates optional entry navigation on the frame thread.
func (h extensionHost) ShowEntry(e workspace.Entry) { h.a.showEntry(e) }

// CancelRequest stops the current author operation on the frame thread.
func (h extensionHost) CancelRequest() {
	if h.a.cancel != nil {
		h.a.cancel()
	}
}
func (a *App) paneChanged(tab string) {
	for _, item := range ext.Items(a.extensions, hostapi.State) {
		if item.Value.PaneChanged != nil {
			_ = ext.Guard(a.extensions, item.Owner, item.ID+".pane", func() { item.Value.PaneChanged(tab) })
		}
	}
}

// MenuState exposes persistent menu state on the frame thread.
func (h extensionHost) MenuState() *kit.MenuState { return &h.a.menu }
