// Package focus contributes the focus command and writing tool.
package focus

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"

	"runtime"
	"writer/ext/focus/focustext"
)

// Module registers focus controls while the host owns the layout state.
var Module = ext.Module{ID: "focus", Kind: ext.Feature, Group: hostapi.GroupWrite, Name: focustext.ModFocus, Desc: focustext.ModFocusAbout, Setup: func(r *ext.Registrar) {
	desktop := ext.Use(r, hostapi.DesktopService)
	book := ext.Use(r, hostapi.BookService)
	toggle := func() { desktop.Get().SetFocus(!desktop.Get().Focused()) }
	r.OnStop(func(*ext.Session) { desktop.Get().SetFocus(false) })
	ext.Contribute(r, hostapi.Commands, "focus", hostapi.Command{HelpGroup: locales.HelpGroupWrite, HelpText: focustext.HelpFocus, HelpAfter: locales.HelpCloseTab, Name: focustext.CmdFocus, Can: func() bool { return book.Get().Store() != nil && desktop.Get().Writing() }, Run: toggle, Key: ui.KeyF11, OnMac: true, MacKey: ui.KeyF, MacMods: ui.Ctrl | ui.Super, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView, After: "pane"}})
	palette := ext.Use(r, hostapi.PaletteService)
	ext.Contribute(r, hostapi.Tools, "tool", hostapi.Tool{Order: 1, Draw: func(c *ui.Context) {
		pal := palette.Get().Colors()
		key := "F11"
		if runtime.GOOS == "darwin" {
			key = "⌃⌘F"
		}
		if desktop.Get().Focused() {
			if kit.IconButton(c, kit.IcShrink, focustext.TabExitFocus(), pal.Ink).Tooltip(focustext.TabExitFocusTip(key)).KeepFocus().Clicked() {
				toggle()
			}
		} else if kit.IconButton(c, kit.IcExpand, focustext.CmdFocus(), ui.Color{}).Tooltip(focustext.TabFocusTip(key)).KeepFocus().Clicked() {
			toggle()
		}
	}})
}}
