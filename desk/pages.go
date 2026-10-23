package desk

import "github.com/egoist/mygo/ui"

// The functions are bound once in New, so a page with its own state can
// put its methods here directly. Nil actions do nothing; a nil count is zero.
type pageView struct {
	tabEntry[page]
	enter, leave func()
	view         func(*ui.Context)
}

func (a *App) initPages() {
	a.pages = [...]pageView{
		pageTalk: {
			tabEntry: tabEntry[page]{name: "讨论", icon: icChat, ai: true},
			view:     a.discussion,
		},
		pageWrite: {
			tabEntry: tabEntry[page]{name: "正文", icon: icBook, work: true},
			enter:    a.enterWriting,
			leave: func() {
				a.flush()
				a.leaveCheck()
			},
			view: a.writing,
		},
		pagePlan: {
			tabEntry: tabEntry[page]{name: "大纲", icon: icOutline, work: true},
			enter:    a.plan.enter,
			leave:    a.plan.leave,
			view:     a.plan.view,
		},
		pageCheck: {
			tabEntry: tabEntry[page]{name: "检查", icon: icDone, ai: true, work: true,
				count: func() int { return len(a.listedChecks()) }},
			enter: func() {
				a.loadChecks() // as of the disk, without what was put right meanwhile
				a.pruneStoredChecks()
				a.ck.proseOf = ""
			},
			view: a.checkPage,
		},
		pageCategories: {
			tabEntry: tabEntry[page]{name: "作品分类", icon: icTag, work: true},
			view:     a.categories,
		},
	}
	for id := range a.pages {
		a.pages[id].id = page(id)
	}
}
