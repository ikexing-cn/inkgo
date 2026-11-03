// Package backup contributes local archives and their desktop controls.
package backup

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
	"writer/locales"
)

// Choices stores per-folder settings on this machine only.
type Choices map[string]archive.Settings

// Get finds a folder using the host's platform identity rules; frame thread only.
func (v Choices) Get(dir string, same func(string, string) bool) (archive.Settings, bool) {
	for path, s := range v {
		if same(path, dir) {
			return s, true
		}
	}
	return archive.Settings{}, false
}

// Put updates a folder without duplicating equivalent paths; frame thread only.
func (v Choices) Put(dir string, s archive.Settings, same func(string, string) bool) {
	for path := range v {
		if same(path, dir) {
			v[path] = s
			return
		}
	}
	v[dir] = s
}

// View owns backup state for one host; its methods require the frame thread.
type View struct {
	// State is read and controlled by desktop integration and screenshot tests.
	State      State
	generation uint64
	book       *ext.Ref[hostapi.Book]
	palette    *ext.Ref[hostapi.Palette]
	jobs       *ext.Ref[hostapi.Jobs]
	dialogs    *ext.Ref[hostapi.Dialogs]
	desktop    *ext.Ref[hostapi.Desktop]
	pending    *ext.Ref[hostapi.Pending]
	setting    *ext.Pref[Choices]
}

// New constructs a backup module and its view for one host assembly.
// ask enables the first-open destination question. Integration tests may read
// the returned view on the frame thread; create a fresh module for each host.
func New(ask bool) (ext.Module, *View) {
	a := &View{}
	return ext.Module{ID: "backup", Kind: ext.Feature, Name: locales.ModBackup, Desc: locales.ModBackupAbout, Setup: func(r *ext.Registrar) {
		*a = View{book: ext.Use(r, hostapi.BookService), palette: ext.Use(r, hostapi.PaletteService), jobs: ext.Use(r, hostapi.JobsService), dialogs: ext.Use(r, hostapi.DialogsService), desktop: ext.Use(r, hostapi.DesktopService), pending: ext.Use(r, hostapi.PendingService), setting: ext.Setting(r, "choices", ext.Schema[Choices]{Scope: ext.Local})}
		r.OnStart(func(s *ext.Session) error {
			if ask && s.Book().Dir != "" {
				a.askBackup()
			}
			return nil
		})
		r.OnStop(func(*ext.Session) {
			a.generation++
			if a.State.Running {
				<-a.State.Done
			}
			a.State = State{}
		})
		ext.Contribute(r, hostapi.FooterItems, "status", hostapi.DrawItem{Draw: a.backupLine})
		ext.Contribute(r, hostapi.TitleItems, "warning", hostapi.DrawItem{Draw: a.backupChip})
		ext.Contribute(r, hostapi.FrameTicks, "due", hostapi.FrameTick{Tick: a.backupDue})
		ext.Contribute(r, hostapi.BeforeLeave, "save", a.backupNow)
		ext.Contribute(r, hostapi.SettingsPages, "settings", hostapi.SettingsPage{Title: locales.ModBackup, Label: "设置-备份", Order: 30, Work: true, Draw: a.backupSettingsView})
		ext.Contribute(r, hostapi.PendingGroups, "failure", hostapi.PendingSource{Key: func() string {
			s, ok := a.Settings()
			short := ""
			minute := int64(0)
			if a.State.Policy.Failed() {
				short, _, _ = a.State.Policy.Trouble(s.Dir)
				minute = time.Now().Unix() / 60
			}
			return fmt.Sprint(ok, s, a.State.Policy.Failed(), short, minute)
		}, Group: a.backupPending})
		ext.Contribute(r, hostapi.Overlays, "question", hostapi.Overlay{Open: func() bool { return a.State.Ask && a.book.Get().Store() != nil }, Draw: func(c *ui.Context) {
			if a.State.Ask && a.book.Get().Store() != nil {
				a.askView(c)
			}
		}})
	}}, a
}
