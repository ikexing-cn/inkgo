package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"

	"writer/locales"
)

// The functions are bound once in New, so a page with its own state can
// put its methods here directly. Nil actions do nothing; a nil count is zero.
type pageView struct {
	tabEntry[page]
	enter, leave      func()
	view              func(*ui.Context)
	hasPane, paneOpen func() bool
	togglePane        func()
}

func (a *App) initPages() {
	a.pages = []pageView{
		pageTalk: {
			tabEntry: tabEntry[page]{name: locales.PageTalk, icon: kit.IcChat, ai: true},
			view:     a.discussion,
		},
		pageWrite: {
			tabEntry: tabEntry[page]{name: locales.PageWrite, icon: kit.IcBook, work: true},
			enter:    a.enterWriting,
			leave: func() {
				a.flush()
				a.leaveCheck()
			},
			view: a.writing,
		},
		pagePlan: {},
		pageCheck: {
			tabEntry: tabEntry[page]{name: locales.PageCheck, visible: a.hasEntries, icon: kit.IcDone, ai: true, work: true,
				count: func() int { return len(a.listedChecks()) }},
			enter: func() {
				a.loadChecks() // as of the disk, without what was put right meanwhile
				a.pruneStoredChecks()
				a.ck.proseOf = ""
			},
			view: a.checkPage,
		},
		pageCategories: {
			tabEntry: tabEntry[page]{name: locales.CmdCategories, icon: kit.IcTag, work: true},
			view:     a.categories,
		},
	}
	for id := range a.pages {
		a.pages[id].id = page(id)
		a.pages[id].order = (id + 1) * 10
	}
	a.pages[pageTalk].order, a.pages[pageWrite].order, a.pages[pageCheck].order = 10, 20, 40
	for _, id := range []page{pageTalk, pageWrite} {
		a.pages[id].hasPane = func() bool {
			if id == pageTalk {
				return len(ext.Items(a.extensions, hostapi.BesideItems)) > 0
			}
			return len(a.paneTabs) > 0
		}
		a.pages[id].paneOpen = func() bool { return !a.doc.hidden }
		a.pages[id].togglePane = func() { a.setDocHidden(!a.doc.hidden) }
	}
}
