package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
)

// extensionView keeps allocating host snapshots out of the frame path.
type extensionView struct {
	adds              []string
	injected          []ledgerInjection
	module            ext.Module
	ledger            ext.Ledger
	own, active, mine bool
	waits             []string
	starts, stops     []string
	meta, waiting     string
}

type extensionGroup struct {
	title   func() string
	library bool
	ids     []string
}

func (a *App) cacheExtensionViews(cache *extensionViews) {
	cache.modules = a.extensions.Modules()
	cache.moduleViews = make(map[string]extensionView, len(cache.modules))
	groups := []extensionGroup{
		{title: locales.ModGroupWrite}, {title: locales.ModGroupLore},
		{title: locales.ModGroupAI}, {title: locales.ModGroupFiles},
		{title: locales.ModGroupInstalled}, {title: locales.ModGroupLibraries, library: true},
	}
	for _, mod := range cache.modules {
		l := a.extensions.Ledger(mod.ID)
		_, mine := a.extensions.BookOverride(mod.ID)
		v := extensionView{module: mod, ledger: l, own: a.extensions.Own(mod.ID), active: l.State == ext.On, mine: mine}
		v.starts, v.stops = a.extensions.Starts(mod.ID), a.extensions.Stops(mod.ID)
		if v.own {
			for _, dep := range l.Requires {
				if !dep.Active {
					v.waits = append(v.waits, dep.ID)
				}
			}
		}
		cache.moduleViews[mod.ID] = v
		group := 4
		if mod.Kind == ext.Library {
			group = 5
		} else {
			switch mod.Group {
			case hostapi.GroupWrite:
				group = 0
			case hostapi.GroupLore:
				group = 1
			case hostapi.GroupAI:
				group = 2
			case hostapi.GroupFiles:
				group = 3
			}
		}
		groups[group].ids = append(groups[group].ids, mod.ID)
	}
	for _, group := range groups {
		if len(group.ids) > 0 {
			cache.groups = append(cache.groups, group)
		}
	}
}

func (v extensionView) name() string {
	if v.module.Name != nil {
		return v.module.Name()
	}
	return v.module.ID
}

func (a *App) extensionNames(ids []string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, a.extViews.moduleViews[id].name())
	}
	return strings.Join(names, locales.ModListSep())
}

func (a *App) cacheExtensionNotes() {
	deps := func(list []ext.Dependency) string {
		ids := make([]string, 0, len(list))
		for _, dep := range list {
			ids = append(ids, dep.ID)
		}
		return a.extensionNames(ids)
	}
	for id, v := range a.extViews.moduleViews {
		var meta []string
		if len(v.ledger.Requires) > 0 {
			meta = append(meta, locales.ModNeeds(deps(v.ledger.Requires)))
		}
		if len(v.ledger.Optional) > 0 {
			meta = append(meta, locales.ModOptional(deps(v.ledger.Optional)))
		}
		if len(v.ledger.NeededBy) > 0 {
			meta = append(meta, locales.ModNeededBy(a.extensionNames(v.ledger.NeededBy)))
		} else if v.module.Kind == ext.Library {
			meta = append(meta, locales.ModNobodyNeeds())
		}
		if v.mine {
			text := locales.ModBookOff()
			if v.own {
				text = locales.ModBookOn()
			}
			meta = append(meta, text)
		}
		v.adds = a.ledgerAdds(id, v.ledger)
		for _, i := range v.ledger.Injections {
			v.injected = append(v.injected, a.cacheInjectedRow(i))
		}
		v.meta = strings.Join(meta, " · ")
		if len(v.waits) > 0 {
			v.waiting = locales.ModWaiting(a.extensionNames(v.waits))
		}
		a.extViews.moduleViews[id] = v
	}
}

func (a *App) extensionPrefTitle(c *ui.Context) {
	if a.settings.tab != "extensions" || a.settings.ext == "" {
		ui.Text(c, a.prefTitle(a.settings.tab)).FontSize(kit.FsDialog).FontWeight(700)
		return
	}
	v := a.extViews.moduleViews[a.settings.ext]
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).Label("扩展总览").KeepFocus()
		fg := a.pal.Theme.TextMuted
		if b.Hovered() {
			fg = a.pal.Theme.Text
		}
		b.Children(func() {
			ui.Text(c, locales.SettingsTabExtensions()).FontSize(kit.FsDialog).FontWeight(400).TextColor(fg)
		})
		ui.Text(c, "›").FontSize(kit.FsDialog).FontWeight(400).TextColor(a.pal.Faint).Margin(0, 8)
		ui.Text(c, v.name()).FontSize(kit.FsDialog).FontWeight(700)
		if b.Clicked() {
			a.settings.ext = ""
			a.closeExtensionMenu()
		}
	})
}

func (a *App) extensionOverview(c *ui.Context) {
	if a.ledgerData.page != "" {
		a.ledgerData.page = ""
		a.ledgerData.generation++
	}
	for _, group := range a.extViews.groups {
		caption := ""
		if group.library {
			caption = locales.ModLibrariesAbout()
		}
		a.pal.Subhead(c, group.title(), caption)
		ui.Column(c).FillWidth().Border(1, a.pal.Line).Radius(kit.RCard).Clip().Children(func() {
			for i := 0; i < len(group.ids); i += 2 {
				if i > 0 {
					a.pal.Hairline(c)
				}
				ui.Row(c).FillWidth().Children(func() {
					a.extensionCell(c, group.ids[i])
					ui.Box(c).Width(1).FillHeight().Background(a.pal.Line)
					if i+1 < len(group.ids) {
						a.extensionCell(c, group.ids[i+1])
					} else {
						extensionCellBox(ui.Box(c))
					}
				})
			}
		})
	}
}

// extensionCellBox sizes a cell of the overview, a blank one too, so that
// the line between the two columns runs straight.
func extensionCellBox(e ui.Element) ui.Element {
	return e.Grow(1).Basis(0).Height(kit.HRow).Padding(4, 12, 4, 14)
}

func (a *App) extensionCell(c *ui.Context, id string) {
	v := a.extViews.moduleViews[id]
	b := ui.ButtonBase(c).Row().Gap(8).AlignItems(ui.Center).Label("扩展-" + id)
	extensionCellBox(b)
	if b.Hovered() {
		b.Background(a.pal.Theme.SurfaceHover)
	}
	kit.PressRow(c, b)
	b.Children(func() {
		fg := a.pal.Theme.Text
		if !v.active {
			fg = a.pal.Theme.TextMuted
		}
		ui.Text(c, v.name()).FontSize(kit.FsBody).TextColor(fg).Grow(1).Shrink(1).SingleLine()
		if !v.active {
			ui.Text(c, locales.ModOff()).FontSize(kit.FsNote).TextColor(a.pal.Faint)
		}
		kit.Glyph(c, kit.IcChevron, kit.IconXs, a.pal.Faint)
	})
	if b.Clicked() {
		a.settings.ext = id
	}
}

func (a *App) extensionSection(c *ui.Context, id string) {
	v, ok := a.extViews.moduleViews[id]
	if !ok {
		return
	}
	ui.Column(c).FillWidth().Gap(14).Padding(16, 0).Children(func() {
		a.extensionHead(c, v)
		if !v.active {
			return
		}
		hasSettings := len(a.simpleSettings(id)) > 0
		for _, item := range a.extViews.sections {
			if item.Owner == id && (!item.Value.Work || a.Store != nil) {
				hasSettings = true
				break
			}
		}
		if !hasSettings {
			return
		}
		ui.Column(c).FillWidth().Gap(10).Margin(0, 0, 0, kit.TickIndent).Children(func() {
			a.drawSimpleSettings(c, id)
			for _, item := range a.extViews.sections {
				if item.Owner != id || item.Value.Work && a.Store == nil {
					continue
				}
				body := ui.Column(c).FillWidth().Gap(10)
				var err error
				body.Children(func() {
					_, err = ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.SettingsSection, *ui.Context, struct{}]{Point: hostapi.SettingsSections}, item.ID), c, settingsCall(item.Value))
				})
				body.Hidden(err != nil)
			}
		})
	})
	if a.settings.tab == "extensions" && a.settings.ext == id {
		a.extensionLedger(c, v)
	}
}

func (a *App) extensionHead(c *ui.Context, v extensionView) {
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		on := v.own
		// The tick box of every other setting; waiting on what it needs,
		// it pales, words and all.
		b := ui.Checkbox(c, &on, v.name()).FontSize(kit.FsGroup).FontWeight(600).Label("扩展开关-" + v.module.ID)
		if len(v.waits) > 0 {
			b.Opacity(kit.OpacityDisabled)
		}
		if b.Changed() {
			a.flipExtension(v.module.ID, on, v.mine)
		}
		note := func(text string, color ui.Color) {
			ui.Text(c, text).FontSize(kit.FsNote).TextColor(color).LineHeight(kit.LhBody).Margin(0, 0, 0, kit.TickIndent)
		}
		if v.module.Desc != nil {
			note(v.module.Desc(), a.pal.Theme.TextMuted)
		}
		if !v.own && v.module.Off != nil {
			note(v.module.Off(), a.pal.Theme.TextMuted)
		}
		if v.meta != "" {
			note(v.meta, a.pal.Faint)
		}
		if v.waiting != "" {
			note(v.waiting, a.pal.Amber)
		}
		if a.Store != nil && v.module.Kind != ext.Library {
			a.extensionMore(c, v)
		}
	})
}

func (a *App) extensionMore(c *ui.Context, v extensionView) {
	key := "ext-more-" + v.module.ID
	more := kit.IconButton(c, kit.IcMore, key, ui.Color{}).Tooltip(locales.CommonMoreActions()).Absolute().Top(-4).Right(0)
	a.menu.DropMenu(c, a.pal, more, key, func(panel ui.Element) {
		panel.AttachTo(more, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0)
	}, func(m *kit.Menu) {
		if v.mine {
			m.Item(locales.ModFollowLocal()).Do(func() { a.warn(a.extensions.ClearBook(v.module.ID)); a.syncExtensions() })
		} else {
			label := locales.ModBookOnlyOn()
			if v.own {
				label = locales.ModBookOnlyOff()
			}
			m.Item(label).Do(func() { a.flipExtension(v.module.ID, !v.own, true) })
		}
	})
}

func (a *App) closeExtensionMenu() {
	if strings.HasPrefix(a.menu.Owner, "ext-more-") {
		a.menu = kit.MenuState{}
	}
}

func (a *App) flipExtension(id string, on, book bool) {
	if id == "ai" {
		a.settings.askAI = true
		return
	}
	a.syncExtensions()
	v := a.extViews.moduleViews[id]
	also := v.stops
	if on {
		also = v.starts
	}
	if len(also) > 0 {
		a.settings.askExt, a.settings.flip, a.settings.flipOn, a.settings.flipBook = true, id, on, book
		return
	}
	a.setExtension(id, on, book)
}

func (a *App) setExtension(id string, on, book bool) {
	if book {
		a.warn(a.extensions.SetBook(id, on))
	} else {
		a.warn(a.extensions.Set(id, on))
	}
	a.syncExtensions()
}

func (a *App) askExtension(c *ui.Context) {
	if !a.settings.askExt {
		return
	}
	id, on, book := a.settings.flip, a.settings.flipOn, a.settings.flipBook
	v := a.extViews.moduleViews[id]
	also := v.stops
	title, about, verb := locales.ModAskOffTitle(a.extViews.moduleViews[id].name()), locales.ModAskOffAbout(a.extensionNames(also)), locales.ModTurnOff()
	if on {
		also = v.starts
		title, about, verb = locales.ModAskOnTitle(a.extViews.moduleViews[id].name()), locales.ModAskOnAbout(a.extensionNames(also)), locales.ModTurnOn()
		// For this work alone, the libraries taken along have no switch of
		// the work's: they turn on for every work, which the question says.
		var libs []string
		for _, dep := range also {
			if book && a.extViews.moduleViews[dep].module.Kind == ext.Library {
				libs = append(libs, dep)
			}
		}
		if len(libs) > 0 {
			about += "\n" + locales.ModAskLibraryLocal(a.extensionNames(libs))
		}
	}
	turn := false
	a.pal.Modal(c, &a.settings.askExt, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, title, about)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
					a.settings.askExt = false
				}
				if a.pal.Primary(c, verb).Label("扩展确认开关").Clicked() {
					turn = true
				}
			})
		})
	})
	if turn {
		a.settings.askExt = false
		a.setExtension(id, on, book)
	}
	if !a.settings.askExt {
		a.settings.flip = ""
	}
}
