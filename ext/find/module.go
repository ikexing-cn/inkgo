// Package find contributes the find interface.
package find

import (
	"github.com/egoist/mygo/ui"
	"runtime"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/find/findapi"
	"writer/ext/find/findtext"
	"writer/ext/hostapi"
	"writer/locales"
)

// Module registers this extension's presentation and command.
var Module = module(nil)

// New creates an independent frame-thread controller.
func New() (ext.Module, *View) { p := &View{}; return module(p), p }
func module(controller *View) ext.Module {
	return ext.Module{ID: "find", Kind: ext.Feature, Group: hostapi.GroupWrite, Name: findtext.ModFind, Desc: findtext.ModFindAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &View{}
		}
		book := ext.Use(r, hostapi.BookService)
		bookEditing := ext.Use(r, hostapi.EditingService)
		pal := ext.Use(r, hostapi.PaletteService)
		notice := ext.Use(r, hostapi.NoticesService)
		desktop := ext.Use(r, hostapi.NavigationService)
		desktopNative := ext.Use(r, hostapi.NativeUIService)
		desktopPaths := ext.Use(r, hostapi.MachinePathsService)
		r.OnStart(func(*ext.Session) error {
			*p = View{Book: book.Get(), Editing: bookEditing.Get(), Palette: pal.Get(), Notices: notice.Get(), Desktop: desktop.Get(), Native: desktopNative.Get(), Paths: desktopPaths.Get()}
			return nil
		})
		ext.Provide(r, findapi.ReplacementService, func(*ext.Session) findapi.Replacement { return p })
		r.OnStop(func(*ext.Session) { p.closeFind(); *p = View{} })
		ext.Contribute(r, hostapi.Commands, "find", hostapi.Command{Name: findtext.CmdFind, Can: func() bool {
			_, ed, ok := bookEditing.Get().Chapter()
			_, lost, _ := desktop.Get().Selected()
			return ok && !ed.ReadOnly() && !lost && !bookEditing.Get().ChapterExternal()
		}, Run: func() {
			if desktop.Get().Writing() {
				p.openFind()
			}
		}, Menu: hostapi.MenuPosition{Menu: hostapi.MenuEdit, After: "edit-find-separator", Order: 25}, Key: ui.KeyF, Mods: ui.Cmd, OnMac: true, MacKey: ui.KeyF, MacMods: ui.Super, HelpGroup: locales.HelpGroupWrite, HelpText: findtext.HelpFind, HelpAfter: "save"})

		ext.Contribute(r, hostapi.Overlays, "replace-dialog", hostapi.Overlay{Open: func() bool { return p.Sure }, Draw: func(c *ui.Context) {
			if p.Sure {
				p.confirmBookReplace(c)
			}
		}})
		ext.Contribute(r, hostapi.ChapterOpens, "chapter", func(hostapi.ChapterOpened) { p.Current = 0 })
		ext.Contribute(r, hostapi.BeforeProse, "bar", hostapi.DrawItem{Draw: func(c *ui.Context) {
			if p.Finding && p.editor() != nil && !p.editor().ReadOnly() {
				p.findBar(c)
			}
		}})
		ext.Contribute(r, hostapi.WritingKeys, "keys", hostapi.WritingKey{Escape: func() bool {
			if !p.Finding {
				return false
			}
			p.closeFind()
			if p.editor() != nil {
				p.editor().Focus()
			}
			return true
		}, HideSelection: func() bool { return p.Finding && p.editor() != nil && !p.editor().Focused() }, Close: p.closeFind})
		ext.Contribute(r, hostapi.Tools, "tool", hostapi.Tool{Order: 0, Draw: func(c *ui.Context) {
			col := ui.Color{}
			if p.Finding {
				col = pal.Get().Colors().Ink
			}
			_, ed, ok := bookEditing.Get().Chapter()
			_, lost, _ := desktop.Get().Selected()
			busy := ok && (ed.ReadOnly() || lost || bookEditing.Get().ChapterExternal())
			key := "Ctrl+F"
			if runtime.GOOS == "darwin" {
				key = "⌘F"
			}
			if kit.IconButton(c, kit.IcSearch, findtext.CmdFind(), col).Tooltip(findtext.TabFindTip(key)).Disabled(!ok || busy).KeepFocus().Clicked() {
				if p.Finding {
					p.closeFind()
				} else {
					p.openFind()
				}
			}
		}})

	}}
}
