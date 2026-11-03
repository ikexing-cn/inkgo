package export

import (
	"writer/ext"
	"writer/ext/export/format"
	"writer/ext/hostapi"
	"writer/locales"
)

// Choices keeps the export options and fallback pen name on this machine.
type Choices struct {
	format.Options
	Pen string `json:"pen,omitempty"`
}

// Module registers the export dialog, command and local choices.
var Module = ext.Module{ID: "export", Kind: ext.Feature, Name: locales.ModExport, Desc: locales.ModExportAbout, Setup: func(r *ext.Registrar) {
	d := registerDialog(r)
	r.OnStop(func(*ext.Session) {
		d.generation++
		d.Open = false
		d.pieces = nil
		d.plan = format.Plan{}
		d.planned = planKey{}
	})
	ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: locales.CmdExport,
		Can: func() bool { return d.book.Get().Store() != nil }, Run: d.Show,
		Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile, After: "snapshots"}})
	ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Draw: d.View})
}}

func registerDialog(r *ext.Registrar) *Dialog {
	return &Dialog{book: ext.Use(r, hostapi.BookService), palette: ext.Use(r, hostapi.PaletteService),
		notices: ext.Use(r, hostapi.NoticesService), jobs: ext.Use(r, hostapi.JobsService), dialogs: ext.Use(r, hostapi.DialogsService),
		setting: ext.Setting(r, "choices", ext.Schema[Choices]{Scope: ext.Local, Default: Choices{Options: format.Options{Prose: true}}})}
}
