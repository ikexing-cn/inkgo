package desk

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/shelf"
)

func declareLifecycle(r *ext.Registrar, a *App) {
	ext.Declare(r, hostapi.FooterItems)
	ext.DeclareCalls(r, hostapi.FooterItems, drawItemCall)
	ext.Declare(r, hostapi.TitleItems)
	ext.DeclareCalls(r, hostapi.TitleItems, drawItemCall)
	ext.Declare(r, hostapi.FrameTicks)
	ext.DeclareCalls(r, hostapi.FrameTicks, tickCall)
	ext.Declare(r, hostapi.BeforeLeave)
	ext.DeclareCalls(r, hostapi.BeforeLeave, leaveCall)
	ext.Declare(r, hostapi.PendingGroups)
	ext.Declare(r, hostapi.SettingsPages)
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
		tab = "扩展"
	}
	h.a.openPrefs(tab)
}

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
	for _, item := range a.extViews.ticks {
		_, _ = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.FrameTick, *ui.Context, struct{}]{Point: hostapi.FrameTicks}, item.ID), c, tickCall(item.Value))
	}
	a.syncExtensions()
}

func (a *App) extensionLeave() {
	a.syncExtensions()
	for _, item := range a.extViews.leave {
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
