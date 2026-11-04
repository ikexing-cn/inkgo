package desk

import (
	"writer/workspace"
)

type paneTab string

const (
	paneMargin  paneTab = "margin"
	paneSetting paneTab = "setting"
	paneTalk    paneTab = "talk"
	paneFore    paneTab = "fore"
)

type docView struct {
	ideas  cached[ideasKey, []workspace.Idea]
	hidden bool
	// The right column: its width as dragged, and, on the writing page,
	// its selected tab and whether a talk sends the chapter with it.
	width    float32
	resizing bool
	tab      paneTab
}

// setSideHidden puts the sidebar away, on every page, or brings it back.
func (a *App) setSideHidden(hidden bool) {
	a.sideHidden = hidden
	a.savePrefs()
}

// setDocHidden folds the panel away, for the talk alone, or brings it back.
func (a *App) setDocHidden(hidden bool) {
	if p := a.pages[a.page]; p.togglePane != nil && a.page != pageWrite && a.page != pageTalk {
		if a.paneOpen() == hidden {
			p.togglePane()
		}
		return
	}
	a.doc.hidden = hidden
	a.savePrefs()
}
