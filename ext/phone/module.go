package phone

import (
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/phone/phonetext"
)

// Typesetting keeps named choices, independent of the order of UI controls.
// Empty or unknown choices use the preview defaults. NoFrame and NoLong
// hide the frame and long-paragraph marks; absent, both are shown.
type Typesetting struct {
	Mode    string `json:"mode,omitempty"`    // flip or scroll
	Screen  string `json:"screen,omitempty"`  // narrow, standard or wide
	Size    int    `json:"size,omitempty"`    // 16–24; absent or outside the range uses the default
	Spacing string `json:"spacing,omitempty"` // tight, medium or loose
	Theme   string `json:"theme,omitempty"`   // day, night or eye
	NoFrame bool   `json:"noFrame,omitempty"`
	NoLong  bool   `json:"noLong,omitempty"`
}

// Module registers the phone preview. Each assembly owns a separate panel.
var Module = ext.Module{ID: "phone", Kind: ext.Feature, Group: hostapi.GroupFiles, Name: phonetext.ModPhone, Desc: phonetext.ModPhoneAbout, Setup: func(r *ext.Registrar) {
	p := &Panel{book: ext.Use(r, hostapi.BookService), editing: ext.Use(r, hostapi.EditingService), palette: ext.Use(r, hostapi.PaletteService)}
	pane := ext.Use(r, hostapi.PaneService)
	p.setting = ext.Setting(r, "typesetting", ext.Schema[Typesetting]{Scope: ext.Local, Default: defaultPhone()})
	r.OnStart(func(s *ext.Session) error { p.session = s; return nil })
	r.OnStop(func(*ext.Session) {
		*p = Panel{book: p.book, editing: p.editing, palette: p.palette, setting: p.setting}
	})
	ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: phonetext.CmdPhonePreview,
		Can: func() bool { _, _, open := p.editing.Get().Chapter(); return open },
		Run: func() { pane.Get().Show("phone.preview") }, Menu: hostapi.MenuPosition{Menu: hostapi.MenuView, After: "focus"}})
	ext.Contribute(r, hostapi.PaneTabs, "preview", hostapi.PaneTab{Title: phonetext.PaneTabPreview, Draw: p.View})
}}
