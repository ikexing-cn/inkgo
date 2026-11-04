package desk

import (
	"cmp"
	"github.com/egoist/mygo/ui"
	"slices"
	"writer/ext"
	"writer/ext/hostapi"
)

func pageCall(p hostapi.Page) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if p.Draw != nil {
			p.Draw(c)
		}
		return struct{}{}
	}
}

func activityCall(fn func(hostapi.WindowActivity)) func(hostapi.WindowActivity) struct{} {
	return func(v hostapi.WindowActivity) struct{} { fn(v); return struct{}{} }
}

func (a *App) mergePages() {
	if a.pageIDs == nil {
		a.pageIDs = map[string]page{"host.talk": pageTalk, "host.write": pageWrite, "host.check": pageCheck, "host.outline": pagePlan}
	}
	for i := range a.pages {
		if i == int(pagePlan) || i > int(pageCategories) {
			a.pages[i].visible = func() bool { return false }
		}
	}
	a.pageOrder = []page{pageTalk, pageWrite, pageCheck}
	for _, item := range ext.Items(a.extensions, hostapi.Pages) {
		id, ok := a.pageIDs[item.ID]
		if !ok {
			id = page(len(a.pages))
			a.pageIDs[item.ID] = id
			a.pages = append(a.pages, pageView{})
		}
		p := item.Value
		guardBool := func(key string, fn func() bool, fallback bool) func() bool {
			return func() bool {
				v := fallback
				err := ext.Guard(a.extensions, item.Owner, item.ID+key, func() {
					if fn != nil {
						v = fn()
					}
				})
				return err == nil && v
			}
		}
		call := func(phase string, c *ui.Context) {
			if phase == "draw" {
				_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.Page, *ui.Context, struct{}]{Point: hostapi.Pages}, item.ID), c, pageCall(p))
				return
			}
			_ = ext.Guard(a.extensions, item.Owner, item.ID+"."+phase, func() {
				if phase == "enter" && p.Enter != nil {
					p.Enter()
				}
				if phase == "leave" && p.Leave != nil {
					p.Leave()
				}
			})
		}
		a.pages[id] = pageView{tabEntry: tabEntry[page]{id: id, name: func() string {
			var title string
			_ = ext.Guard(a.extensions, item.Owner, item.ID+".title", func() {
				if p.Title != nil {
					title = p.Title()
				}
			})
			return title
		}, icon: p.Icon, label: p.Label, order: p.Order, visible: guardBool(".available", p.Available, true)}, enter: func() { call("enter", nil) }, leave: func() { call("leave", nil) }, view: func(c *ui.Context) { call("draw", c) }, hasPane: guardBool(".has-pane", p.HasPane, false), paneOpen: guardBool(".pane-open", p.PaneOpen, false), togglePane: func() {
			_ = ext.Guard(a.extensions, item.Owner, item.ID+".toggle-pane", func() {
				if p.TogglePane != nil {
					p.TogglePane()
				}
			})
		}}
		a.pageOrder = append(a.pageOrder, id)
	}
	slices.SortStableFunc(a.pageOrder, func(x, y page) int { return cmp.Compare(a.pages[x].order, a.pages[y].order) })
}

func (a *App) beforeChapterChange() {
	for _, item := range ext.Items(a.extensions, hostapi.BeforeChapterChange) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[func(), struct{}, struct{}]{Point: hostapi.BeforeChapterChange}, item.ID), struct{}{}, leaveCall(item.Value))
	}
}

func (a *App) windowActivity(v hostapi.WindowActivity) {
	for _, item := range ext.Items(a.extensions, hostapi.WindowActivities) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[func(hostapi.WindowActivity), hostapi.WindowActivity, struct{}]{Point: hostapi.WindowActivities}, item.ID), v, activityCall(item.Value))
	}
}

func (a *App) chapterHead(c *ui.Context) {
	for _, item := range ext.Items(a.extensions, hostapi.ChapterHeadItems) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.DrawItem, *ui.Context, struct{}]{Point: hostapi.ChapterHeadItems}, item.ID), c, drawItemCall(item.Value))
	}
}

func saveEditsCall(fn func() error) func(struct{}) error { return func(struct{}) error { return fn() } }

func (a *App) saveExtensionEdits() error {
	for _, item := range ext.Items(a.extensions, hostapi.SaveEdits) {
		result, err := ext.Call(a.extensions, ext.HookOf(ext.CallPoint[func() error, struct{}, error]{Point: hostapi.SaveEdits}, item.ID), struct{}{}, saveEditsCall(item.Value))
		if err != nil {
			return err
		}
		if result != nil {
			return result
		}
	}
	return nil
}

func (a *App) chapterEdits(id string, discard bool) bool {
	dirty := false
	for _, item := range ext.Items(a.extensions, hostapi.ChapterEditStates) {
		_ = ext.Guard(a.extensions, item.Owner, item.ID+".state", func() {
			if discard {
				if item.Value.Discard != nil {
					item.Value.Discard(id)
				}
			} else if item.Value.Dirty != nil {
				dirty = dirty || item.Value.Dirty(id)
			}
		})
	}
	return dirty
}

func chapterOpenCall(fn func(hostapi.ChapterOpened)) func(hostapi.ChapterOpened) struct{} {
	return func(v hostapi.ChapterOpened) struct{} { fn(v); return struct{}{} }
}

func (a *App) chapterOpened(id, text string) {
	for _, item := range ext.Items(a.extensions, hostapi.ChapterOpens) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[func(hostapi.ChapterOpened), hostapi.ChapterOpened, struct{}]{Point: hostapi.ChapterOpens}, item.ID), hostapi.ChapterOpened{ID: id, Outline: text}, chapterOpenCall(item.Value))
	}
}
