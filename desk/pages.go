package desk

import "github.com/egoist/mygo/ui"

// The functions are bound once in New, so a page with its own state can
// put its methods here directly. Nil actions do nothing; a nil count is zero.
type pageView struct {
	enter, leave func()
	view         func(*ui.Context)
	count        func() int
}

func (a *App) initPages() {
	a.pages = [...]pageView{
		pageTalk: {view: a.discussion},
		pageWrite: {
			enter: a.enterWriting,
			leave: func() {
				a.flush()
				a.leaveCheck()
			},
			view: a.writing,
		},
		pagePlan: {
			enter: func() { a.plan.loaded = false },
			leave: func() {
				a.finishPlanOutline()
				a.finishPlanLine()
			},
			view: a.outlinePage,
		},
		pageCheck: {
			enter: func() {
				a.loadChecks() // as of the disk, without what was put right meanwhile
				a.pruneStoredChecks()
				a.ck.proseOf = ""
			},
			view:  a.checkPage,
			count: func() int { return len(a.listedChecks()) },
		},
		pageCategories: {view: a.categories},
	}
}
