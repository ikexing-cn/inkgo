package desk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
)

type ledgerAdd struct {
	word  ext.Place
	names []string
}
type ledgerData struct {
	path  string
	size  int64
	local bool
}
type ledgerDataCache struct {
	page       string
	generation uint64
	completed  uint64
	rows       map[string][]ledgerData
}

func (a *App) ledgerAdds(id string, l ext.Ledger) []string {
	var groups []ledgerAdd
	add := func(word ext.Place, name string) {
		if word == ext.Hidden {
			return
		}
		for n := range groups {
			if groups[n].word == word {
				if name != "" {
					groups[n].names = append(groups[n].names, name)
				}
				return
			}
		}
		row := ledgerAdd{word: word}
		if name != "" {
			row.names = []string{name}
		}
		groups = append(groups, row)
	}

	for _, c := range l.Contributions {
		place, name := a.extensions.ContributionDisplay(c.ID)
		add(place, name)
	}
	for _, i := range ext.Declared(a.extensions, hostapi.Commands) {
		if i.Owner != id {
			continue
		}
		for _, word := range hostapi.CommandPlaces(i.Value) {
			add(word, "")
		}

	}
	var rows []string
	for _, g := range groups {
		rows = append(rows, locales.ModAdds(string(g.word), strings.Join(g.names, locales.ModListSep())))
	}
	return rows
}

// measureLedgerData runs only in a jobs worker and never follows symlinks.
func measureLedgerData(path string) (int64, bool) {
	if path == "" {
		return 0, false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return 0, false
	}
	var size int64
	err = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err == nil
}

func (a *App) refreshLedgerData(c *ui.Context, id string) {
	cache := &a.ledgerData
	page := id + "\x00" + a.extensions.Book().Dir
	if cache.page == page {
		return
	}
	cache.page = page
	cache.generation++
	generation := cache.generation
	if cache.rows == nil {
		cache.rows = map[string][]ledgerData{}
	}
	host := a.extensions
	store := a.Store
	jobsHost{extensionHost{a}}.StartJob(c.Services(), func() func() {
		book, local := host.DataPaths(id)
		var rows []ledgerData
		if size, ok := measureLedgerData(book); ok {
			rows = append(rows, ledgerData{"ext/" + id, size, false})
		}
		localSize, hasLocal := measureLedgerData(local)
		if store != nil {
			localBook, _ := store.LocalExtDir(id)
			if localBook != local {
				if size, ok := measureLedgerData(localBook); ok {
					localSize += size
					hasLocal = true
				}
			}
		}
		if hasLocal {
			rows = append(rows, ledgerData{"ext/" + id, localSize, true})
		}

		return func() {
			if a.ledgerData.generation == generation {
				a.ledgerData.rows[page] = rows
				a.ledgerData.completed = generation
			}
		}
	})
}

func (a *App) extensionLedger(c *ui.Context, v extensionView) {
	if len(a.extViews.pageOwners[a.settings.tab]) > 1 {
		return
	}
	a.refreshLedgerData(c, v.module.ID)
	part := func(title string, has bool, draw func()) {
		if !has {
			return
		}
		ui.Column(c).FillWidth().Gap(4).Margin(0, 0, 0, kit.TickIndent).Padding(0, 0, 14, 0).Children(func() {
			ui.Text(c, title).FontSize(kit.FsBody).FontWeight(500)
			draw()
		})
	}
	note := func(s string, color ui.Color) {
		ui.Text(c, s).FontSize(kit.FsNote).TextColor(color).LineHeight(kit.LhBody)
	}
	part(locales.ModLedgerAdds(), len(v.adds) > 0, func() {
		for _, row := range v.adds {
			note(row, a.pal.Theme.TextMuted)
		}
	})
	part(locales.ModLedgerInjected(), v.active && len(v.ledger.Injections) > 0, func() {
		for _, i := range v.injected {
			a.injectedRow(c, i)
		}
	})
	rows := a.ledgerData.rows[a.ledgerData.page]
	part(locales.ModLedgerData(), len(rows) > 0, func() {
		for _, r := range rows {
			size := fmt.Sprintf("%.1f KB", float64(r.size)/1024)
			text := locales.ModDataBook(r.path, size)
			if r.local {
				text = locales.ModDataLocal(r.path, size)
			}
			note(text, a.pal.Theme.TextMuted)
		}
	})
	part(locales.ModLedgerErrors(), len(v.ledger.Errors) > 0, func() {
		for n := len(v.ledger.Errors) - 1; n >= 0; n-- {
			e := v.ledger.Errors[n]
			where := e.Stage
			if e.Target != "" {
				where += " · " + a.extensions.TargetName(e.Target)
			}
			if e.Injection != "" {
				where += " · " + e.Injection
			}
			note(locales.ModErrorRow(kit.When(e.At, time.Now()), where, e.Err.Error()), a.pal.Theme.TextMuted)
		}
	})
}

type ledgerInjection struct {
	text           string
	warning        bool
	chosen, target string
	values, names  []string
}

func (a *App) cacheInjectedRow(i ext.Injection) ledgerInjection {
	where := a.extensions.TargetName(i.Target)
	kind := i.Kind.KindWord()
	text := locales.ModInjected(where, kind)
	warning := false
	var rivals []ext.ChainLink
	var others []string
	chosen := ""
	if i.Kind >= ext.OverwriteKind {
		for _, mod := range a.extViews.modules {
			for _, injection := range a.extViews.moduleViews[mod.ID].ledger.Injections {
				r := ext.ChainLink{Injection: injection}
				if r.Target == i.Target && r.Kind >= ext.OverwriteKind && a.extViews.moduleViews[r.Owner].active {
					rivals = append(rivals, r)
					if r.ID != i.ID {
						others = append(others, r.Owner)
					}
					if r.Active {
						chosen = r.ID
					}
				}
			}
		}
	}
	if i.Conflict {
		text = locales.ModInjectConflict(where, a.extensionNames(others))
		warning = true
	}
	if i.Reason.Kind == ext.ReasonNotSelected {
		name := locales.ModUseDefault()
		for _, r := range rivals {
			if r.ID == i.Reason.ID {
				name = a.extViews.moduleViews[r.Owner].name()
			}
		}
		text = locales.ModInjectNotUsed(where, name)
		if chosen == "" {
			chosen = ext.DefaultChoice
		}
	}
	row := ledgerInjection{text: text, warning: warning, chosen: chosen, target: i.Target}
	if len(rivals) > 1 {
		values := []string{ext.DefaultChoice}
		names := []string{locales.ModUseDefault()}
		for _, r := range rivals {
			values = append(values, r.ID)
			names = append(names, a.extViews.moduleViews[r.Owner].name())
		}
		row.values, row.names = values, names
	}
	return row
}

func (a *App) injectedRow(c *ui.Context, row ledgerInjection) {
	color := a.pal.Theme.TextMuted
	if row.warning {
		color = a.pal.Theme.Warning
	}
	ui.Text(c, row.text).FontSize(kit.FsNote).TextColor(color).LineHeight(kit.LhBody)
	if len(row.values) == 0 {
		return
	}
	chosen := row.chosen
	changed := false
	ui.RadioGroup(c, func() {
		for n, value := range row.values {
			if ui.Radio(c, &chosen, value, row.names[n]).Label("injection-choice-" + value).Changed() {
				changed = true
			}
		}
	}).Row().Gap(24).MinHeight(kit.HControl).AlignItems(ui.Center).Label(locales.ModInjectChoose())
	if changed {
		a.warn(a.extensions.Choose(row.target, chosen))
		a.syncExtensions()
	}
}
