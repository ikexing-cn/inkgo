package desk

import (
	"cmp"
	"encoding/json"
	"slices"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/shelf"
)

func declareLifecycle(r *ext.Registrar, a *App, mods []ext.Module) {
	ext.Declare(r, hostapi.NameHighlights)
	ext.DeclareCalls(r, hostapi.NameHighlights, func(p hostapi.NameProvider) func(struct{}) hostapi.Names { return p })
	ext.Declare(r, hostapi.BeforeProse)
	ext.DeclareCalls(r, hostapi.BeforeProse, drawItemCall)
	ext.Declare(r, hostapi.WritingKeys)
	ext.DeclareCalls(r, hostapi.WritingKeys, func(k hostapi.WritingKey) func(func(hostapi.WritingKey)) struct{} {
		return func(fn func(hostapi.WritingKey)) struct{} { fn(k); return struct{}{} }
	})

	h := extensionHost{a}
	ext.Provide(r, hostapi.ProseUIService, func(*ext.Session) hostapi.ProseUI { return h })
	ext.Declare(r, hostapi.ToolHints)
	ext.DeclareCalls(r, hostapi.ToolHints, drawItemCall)
	ext.Declare(r, hostapi.ProseOverlays)
	ext.DeclareCalls(r, hostapi.ProseOverlays, drawItemCall)
	ext.Declare(r, hostapi.WordSuffix)
	ext.Declare(r, hostapi.WordPopovers)
	ext.DeclareCalls(r, hostapi.WordPopovers, wordPopoverCall)
	ext.Declare(r, hostapi.Tools)
	ext.DeclareCalls(r, hostapi.Tools, func(v hostapi.Tool) func(*ui.Context) struct{} {
		return func(c *ui.Context) struct{} { v.Draw(c); return struct{}{} }
	})
	ext.Provide(r, hostapi.SuggestionsService, func(*ext.Session) hostapi.Suggestions { return h })
	ext.Declare(r, hostapi.State)
	ext.Provide(r, hostapi.SearchService, func(*ext.Session) hostapi.Search { return h })
	ext.Provide(r, hostapi.AIService, func(*ext.Session) hostapi.AI { return h })
	ext.Provide(r, hostapi.RevisionsService, func(*ext.Session) hostapi.Revisions { return h })
	ext.Declare(r, hostapi.SelectionMenus)
	ext.DeclareCalls(r, hostapi.SelectionMenus, selectionMenuCall)
	ext.Declare(r, hostapi.BesideItems)
	ext.DeclareCalls(r, hostapi.BesideItems, drawItemCall)
	ext.Declare(r, hostapi.ProseMarks)
	ext.DeclareCalls(r, hostapi.ProseMarks, proseMarksCall)
	for _, mod := range mods {
		if mod.ID == "marks" {
			registerLegacyMarks(r, a)
			break
		}
	}

	ext.Declare(r, hostapi.ChapterOpens)
	ext.DeclareCalls(r, hostapi.ChapterOpens, chapterOpenCall)
	ext.Declare(r, hostapi.ChapterEditStates)
	ext.Declare(r, hostapi.SaveEdits)
	ext.DeclareCalls(r, hostapi.SaveEdits, saveEditsCall)
	ext.Declare(r, hostapi.Pages)
	ext.DeclareCalls(r, hostapi.Pages, pageCall)
	ext.Declare(r, hostapi.WindowActivities)
	ext.DeclareCalls(r, hostapi.WindowActivities, activityCall)
	ext.Declare(r, hostapi.FooterItems)
	ext.DeclareCalls(r, hostapi.FooterItems, drawItemCall)
	ext.Declare(r, hostapi.TitleItems)
	ext.DeclareCalls(r, hostapi.TitleItems, drawItemCall)
	ext.Declare(r, hostapi.ChapterHeadItems)
	ext.DeclareCalls(r, hostapi.ChapterHeadItems, drawItemCall)
	ext.Declare(r, hostapi.BeforeChapterChange)
	ext.DeclareCalls(r, hostapi.BeforeChapterChange, leaveCall)
	ext.Declare(r, hostapi.FrameTicks)
	ext.DeclareCalls(r, hostapi.FrameTicks, tickCall)
	ext.Declare(r, hostapi.BeforeLeave)
	ext.DeclareCalls(r, hostapi.BeforeLeave, leaveCall)
	ext.Declare(r, hostapi.PendingGroups)
	ext.Declare(r, hostapi.SettingsPages)
	registerCheckSettings(r, a)
	ext.Declare(r, hostapi.SettingsSections)
	ext.DeclareCalls(r, hostapi.SettingsSections, settingsCall)
	ext.Provide(r, hostapi.DesktopService, func(*ext.Session) hostapi.Desktop { return extensionHost{a} })
	ext.Provide(r, hostapi.PendingService, func(*ext.Session) hostapi.Pending { return extensionHost{a} })
}

func (h extensionHost) Writing() bool { return h.a.page == pageWrite }

func (h extensionHost) SlideEdge(e ui.Element) { h.a.slideEdge(e) }

func (h extensionHost) OpenSettings(id string) {
	h.a.syncExtensions()
	tab, ok := h.a.extViews.settingsTabs[id]
	if !ok {
		tab = "extensions"
	}
	h.a.openPrefs(tab)
}

func (h extensionHost) ShowPage(id string) {
	h.a.syncExtensions()
	p, ok := h.a.pageIDs[id]
	if ok && h.a.pages[p].available(h.a) {
		h.a.page = p
	} else {
		h.a.page = pageWrite
	}
}

func (h extensionHost) OpenWriting(id string) { (appHost{h.a}).OpenWriting(id) }

func (h extensionHost) DropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	h.a.dropMenu(c, b, key, place, build)
}

func (h extensionHost) SettingColumn(c *ui.Context) { h.a.settingColumn(c) }

func (h extensionHost) SameFolder(a, b string) bool { return shelf.SameFolder(a, b) }

func (h extensionHost) ClearIgnored(key string) {
	if h.a.Store != nil && h.a.write.ignored[key] {
		delete(h.a.write.ignored, key)
		h.a.write.ignoredVer++
		h.a.savePrefs()
	}
}

func drawItemCall(v hostapi.DrawItem) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if v.Draw != nil {
			v.Draw(c)
		}
		return struct{}{}
	}
}

func tickCall(v hostapi.FrameTick) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if v.Tick != nil {
			v.Tick(c)
		}
		return struct{}{}
	}
}

func leaveCall(v func()) func(struct{}) struct{} {
	return func(struct{}) struct{} {
		if v != nil {
			v()
		}
		return struct{}{}
	}
}

func settingsCall(v hostapi.SettingsSection) func(*ui.Context) struct{} {
	return func(c *ui.Context) struct{} {
		if v.Draw != nil {
			v.Draw(c)
		}
		return struct{}{}
	}
}

func (a *App) extensionDraw(c *ui.Context, point ext.PointKey[hostapi.DrawItem], items []ext.Item[hostapi.DrawItem]) {
	for _, item := range items {
		body := ui.Box(c)
		var err error
		body.Children(func() {
			_, err = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.DrawItem, *ui.Context, struct{}]{Point: point}, item.ID), c, drawItemCall(item.Value))
		})
		body.Hidden(err != nil)
	}
	a.syncExtensions()
}

func (a *App) extensionFooter(c *ui.Context) {
	a.syncExtensions()
	a.extensionDraw(c, hostapi.FooterItems, a.extViews.footer)
}

func (a *App) extensionTitle(c *ui.Context) {
	a.syncExtensions()
	a.extensionDraw(c, hostapi.TitleItems, a.extViews.title)
}

func (a *App) extensionTicks(c *ui.Context) {
	a.syncExtensions()
	for _, item := range append(append([]ext.Item[hostapi.FrameTick]{}, a.extViews.hostTicks...), a.extViews.ticks...) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.FrameTick, *ui.Context, struct{}]{Point: hostapi.FrameTicks}, item.ID), c, tickCall(item.Value))
	}
	a.syncExtensions()
}

func (a *App) extensionLeave() {
	a.syncExtensions()
	for _, item := range append(append([]ext.Item[func()]{}, a.extViews.hostLeave...), a.extViews.leave...) {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[func(), struct{}, struct{}]{Point: hostapi.BeforeLeave}, item.ID), struct{}{}, leaveCall(item.Value))
	}
	a.syncExtensions()
}

func (a *App) extensionSettings(c *ui.Context, id string) {
	a.syncExtensions()
	ui.Column(c).FillWidth().Children(func() {
		for i, owner := range a.extViews.pageOwners[id] {
			if i > 0 {
				a.pal.Hairline(c)
			}
			a.extensionSection(c, owner)
		}
	})
	a.syncExtensions()
}

func (a *App) extensionModalOpen() bool {
	a.syncExtensions()
	for _, item := range a.extViews.overlays {
		var open bool
		err := ext.Guard(a.extensions, item.Owner, item.ID+".open", func() {
			if item.Value.Open != nil {
				open = item.Value.Open()
			}
		})
		if err == nil && open {
			return true
		}
	}
	return false
}

func (a *App) extensionPendingKey() string {
	a.syncExtensions()
	parts := []any{a.extViews.changes}
	for _, item := range a.extViews.pending {
		var key string
		err := ext.Guard(a.extensions, item.Owner, item.ID+".key", func() {
			if item.Value.Key != nil {
				key = item.Value.Key()
			}
		})
		if err == nil {
			parts = append(parts, item.ID, key)
		}
	}
	data, _ := json.Marshal(parts)
	return string(data)
}

func (a *App) extensionPendingGroups() []pendGroup {
	a.syncExtensions()
	var groups []pendGroup
	for _, item := range a.extViews.pending {
		var value hostapi.PendingGroup
		var present bool
		err := ext.Guard(a.extensions, item.Owner, item.ID+".group", func() {
			if item.Value.Group != nil {
				value, present = item.Value.Group()
			}
		})
		if err != nil || !present {
			continue
		}
		g := pendGroup{id: value.ID, name: value.Name, color: value.Color}
		for _, it := range value.Items {
			if a.write.ignored[it.Key] {
				continue
			}
			action := it.Open
			g.items = append(g.items, pendItem{key: it.Key, text: it.Text, sub: it.Sub, here: it.Here, open: func(sv ui.Services) {
				_ = ext.Guard(a.extensions, item.Owner, item.ID+".open/"+it.Key, func() {
					if action != nil {
						action(sv)
					}
				})
			}})
		}
		if len(g.items) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

func (h extensionHost) Selected() (string, bool, bool) {
	return h.a.write.node, h.a.write.cur().lost, len(h.a.write.closing) > 0
}

// Focused reports the host layout state on the frame thread.
func (h extensionHost) Focused() bool { return h.a.write.focus }

// SetFocus changes the host layout state on the frame thread.
func (h extensionHost) SetFocus(v bool) { h.a.write.focus = v }

// CategoriesChanged refreshes setting data and open-book consumers on the frame thread.
func (h extensionHost) CategoriesChanged() error {
	if err := h.a.book.ReloadSetting(); err != nil {
		return err
	}
	return h.a.refreshFromSetting()
}

func (a *App) extensionTools(c *ui.Context) {
	items := ext.Items(a.extensions, hostapi.Tools)
	slices.SortStableFunc(items, func(a, b ext.Item[hostapi.Tool]) int { return cmp.Compare(a.Value.Order, b.Value.Order) })
	for _, item := range items {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.Tool, *ui.Context, struct{}]{Point: hostapi.Tools}, item.ID), c, func(c *ui.Context) struct{} { item.Value.Draw(c); return struct{}{} })
	}
}

// HasPage reports a contributed page's availability on the frame thread.
func (h extensionHost) HasPage(id string) bool {
	h.a.syncExtensions()
	p, ok := h.a.pageIDs[id]
	return ok && h.a.pages[p].available(h.a)
}
