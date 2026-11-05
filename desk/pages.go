package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"

	"writer/locales"
)

// The functions are bound once in New, so a page with its own state can
// put its methods here directly. Nil actions do nothing; a nil count is zero.
type pageView struct {
	tabEntry[page]
	enter, leave           func()
	view                   func(*ui.Context)
	sidebar, sidebarAction func(*ui.Context)
	hasPane, paneOpen      func() bool
	togglePane             func()
}

func (a *App) initPages() {
	a.pages = []pageView{
		pageTalk: {},
		pageWrite: {
			tabEntry: tabEntry[page]{name: locales.PageWrite, icon: kit.IcBook, work: true},
			enter:    a.enterWriting,
			leave: func() {
				a.flush()
				a.beforeChapterChange()
			},
			view: a.writing,
		},
		pagePlan:       {},
		pageCheck:      {},
		pageCategories: {},
	}
	for id := range a.pages {
		a.pages[id].id = page(id)
		a.pages[id].order = (id + 1) * 10
	}
	a.pages[pageTalk].order, a.pages[pageWrite].order, a.pages[pageCheck].order = 10, 20, 40
	a.pages[pageWrite].hasPane = func() bool { return len(a.paneTabs) > 0 }
	a.pages[pageWrite].paneOpen = func() bool { return !a.doc.hidden }
	a.pages[pageWrite].togglePane = func() { a.setDocHidden(!a.doc.hidden) }
}
