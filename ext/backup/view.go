package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

// State holds activation UI and task state; frame thread only.
// Desktop integration and screenshot tests inspect and control this state.
type State struct {
	Running                bool
	Done                   chan struct{}
	Policy                 archive.Policy
	Open, Chip, Ask        bool
	Place                  string
	OnClose, Daily, Hourly bool
	List                   []archive.Info
	Listed                 bool
	KeepChoice             string
	Places                 []Place
}

// Settings is how the open work is backed up from this machine.
func (a *View) Settings() (archive.Settings, bool) {
	if a.book.Get().Store() == nil {
		return archive.Settings{}, false
	}
	return a.setting.Get().Get(a.book.Get().Dir(), a.desktop.Get().SameFolder)
}

// Configure persists this work's local settings on the frame thread.
func (a *View) Configure(s archive.Settings) {
	if a.book.Get().Store() == nil {
		return
	}
	choices := a.setting.Get()
	if choices == nil {
		choices = Choices{}
	}
	choices.Put(a.book.Get().Dir(), s, a.desktop.Get().SameFolder)
	a.setting.Set(choices)
}

// Place is a detected local destination.
type Place struct {
	Dir  string
	Name func() string // in the interface language when drawn
}

// SyncFolders discovers destinations when the dialog opens.
var SyncFolders = func() []Place {
	home, _ := os.UserHomeDir()
	var out []Place
	add := func(name func() string, dir string) {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, Place{Dir: filepath.Join(dir, "写作备份"), Name: name})
		}
	}
	if d := os.Getenv("OneDrive"); d != "" {
		add(func() string { return locales.BackupSyncFolder("OneDrive") }, d)
	}
	if ds, _ := filepath.Glob(filepath.Join(home, "Nutstore", "*", "我的坚果云")); len(ds) > 0 {
		add(locales.BackupNutstore, ds[0])
	}
	add(func() string { return locales.BackupSyncFolder("Dropbox") }, filepath.Join(home, "Dropbox"))
	add(locales.BackupICloud, filepath.Join(home, "iCloudDrive"))
	return out
}

// askBackup opens the dialog that asks where to back the work up, the
// first time it is opened here, or a while after it was put off.
func (a *View) askBackup() {
	if a.book.Get().Store() == nil {
		return
	}
	s, _ := a.Settings()
	if a.State.Policy.Ask(s, time.Now()) {
		a.OpenAsk()
	}
}

// OpenAsk opens the destination question on the frame thread.
func (a *View) OpenAsk() {
	a.State.Ask, a.State.Place = true, ""
	a.State.OnClose, a.State.Daily, a.State.Hourly = true, true, false
	a.State.Places = SyncFolders()
	if len(a.State.Places) > 0 {
		a.State.Place = a.State.Places[0].Dir
	}
}

// startBackups keeps what the dialog chose and backs up once at once.
func (a *View) startBackups(c *ui.Context) {
	s, _ := a.Settings()
	s.Dir, s.OnClose, s.Daily, s.Hourly, s.Later = a.State.Place, a.State.OnClose, a.State.Daily, a.State.Hourly, time.Time{}
	a.Configure(s)
	a.State.Ask, a.State.Listed = false, false
	a.State.Policy.ForgetFailure()
	a.Run(c.Services())
}

// readWork reads the work while the store is held still.
func readWork(st *workspace.Store) (archive.Files, error) {
	var fl archive.Files
	err := st.Hold(func(root string) (err error) {
		fl, err = archive.Read(root)
		return err
	})
	return fl, err
}

// Run backs the work up off the view thread, which sv wakes as it
// is done.
func (a *View) Run(sv ui.Services) {
	s, ok := a.Settings()
	if !ok || s.Dir == "" || a.State.Running {
		return
	}
	a.book.Get().Flush()
	st := a.book.Get().Store()
	root, work := st.Root(), st.Work()
	a.State.Running = true
	done := make(chan struct{})
	a.State.Done = done
	a.jobs.Get().StartJob(sv, func() func() {
		fl, err := readWork(st)
		if err == nil {
			_, _, err = archive.Make(fl, s.Dir, work.Name, fl.Meta(work.ID), s.Kept(), time.Now())
		}
		// Done before the result is handed back: a closing work waits on
		// done from the view thread, which runs no jobs meanwhile.
		close(done)
		return func() {
			if a.State.Done != done {
				return
			}
			a.State.Running = false
			a.State.Done = nil
			if a.book.Get().Store() != nil && a.desktop.Get().SameFolder(a.book.Get().Store().Root(), root) {
				a.RecordResult(s, err)
			}
		}
	})
}

// backupNow backs the work up before it closes, waiting for it.
func (a *View) backupNow() {
	if a.State.Running {
		<-a.State.Done
		a.State.Running = false
		a.State.Done = nil
	}
	s, ok := a.Settings()
	if !ok || s.Dir == "" || !s.OnClose {
		return
	}
	a.book.Get().Flush()
	st := a.book.Get().Store()
	work := st.Work()
	fl, err := readWork(st)
	if err == nil {
		_, _, err = archive.Make(fl, s.Dir, work.Name, fl.Meta(work.ID), s.Kept(), time.Now())
	}
	a.RecordResult(s, err)
}

// RecordResult updates failure state and clears a successful backup's ignore key.
// Call it on the frame thread.
func (a *View) RecordResult(s archive.Settings, err error) {
	a.State.Listed = false
	// The disk is looked for once, here: what says why draws each
	// frame, and a disk that is away may take long to say so.
	a.State.Policy.RecordResult(s.Dir, err, time.Now())
	if err != nil {
		return
	}
	a.pending.Get().ClearIgnored(pendBackup)
	if cur, ok := a.Settings(); ok {
		cur.Last = time.Now()
		a.Configure(cur)
	}
}

// backupDue backs up when the settings say it is time: once a day, and
// every hour of writing.
func (a *View) backupDue(c *ui.Context) {
	now := time.Now()
	if a.book.Get().Store() == nil || a.State.Running {
		return
	}
	s, _ := a.Settings()
	if a.State.Policy.Due(s, a.desktop.Get().Writing(), now) {
		a.Run(c.Services())
	}
}

// pendBackup is the key that ignores a failed backup in the pending list,
// until a backup succeeds.
const pendBackup = "备份/没成功"

// backupPending is the pending list's group for a backup that failed.
func (a *View) backupPending() (hostapi.PendingGroup, bool) {
	s, ok := a.Settings()
	if !ok || !a.State.Policy.Failed() {
		return hostapi.PendingGroup{}, false
	}
	short, _, _ := TroubleText(a.State.Policy.Trouble(s.Dir))
	sub := locales.BackupRetryHint()
	if !s.Last.IsZero() {
		sub = locales.StatusRetryBackup(kit.When(s.Last, time.Now()))
	}
	it := hostapi.PendingItem{Key: pendBackup, Text: locales.StatusBackupWhy(short), Sub: sub, Open: a.Run}
	return hostapi.PendingGroup{ID: "backup", Name: locales.ModBackup(), Color: a.palette.Get().Colors().Amber, Items: []hostapi.PendingItem{it}}, true
}

// lastSaid says when the last backup succeeded, for a failure's popover.
func lastSaid(s archive.Settings, now time.Time) string {
	if s.Last.IsZero() {
		return locales.BackupNoneYet()
	}
	days := int(now.Sub(s.Last).Hours() / 24)
	if days >= 1 {
		return locales.BackupLastDays(days)
	}
	return locales.BackupLastAt(kit.When(s.Last, now))
}

// Kept says what the last of the backups listed holds and how many there
// are, newest first, with the day of the first when there are more.
func Kept(list []archive.Info, now time.Time) string {
	note := locales.BackupSummary(kit.Words(list[0].Words), list[0].Chapters, len(list))
	if len(list) > 1 {
		note = locales.BackupSummaryOldest(note, kit.DayOf(list[len(list)-1].At, now))
	}
	return note
}

// next says when the next backup comes.
func next(s archive.Settings, now time.Time) string {
	switch {
	case s.Hourly:
		return locales.BackupAround(s.Last.Add(time.Hour).Format("15:04"))
	case s.OnClose:
		return locales.BackupWhenClose()
	case s.Daily:
		return locales.BackupNextDay()
	}
	return locales.BackupNextManual()
}

func (a *View) backupText() string {
	s, _ := a.Settings()
	switch {
	case a.State.Running:
		return locales.BackupBusy()
	case a.State.Policy.Failed():
		return locales.BackupFailed()
	case s.Dir == "" || s.Last.IsZero():
		return locales.BackupNone()
	default:
		return locales.StatusBackupOK()
	}
}

// backupLine is the footer's small line of how the backups go; it opens
// what they are and what to do.
func (a *View) backupLine(c *ui.Context) {
	s, _ := a.Settings()
	if a.book.Get().Store() == nil {
		return
	}
	t := a.palette.Get().Colors().Theme
	text, dot := a.backupText(), a.palette.Get().Colors().Faint
	tip := locales.StatusBackedUpAt(kit.When(s.Last, time.Now()))
	if !a.State.Running && (a.State.Policy.Failed() || s.Dir == "" || s.Last.IsZero()) {
		dot = a.palette.Get().Colors().Amber
	}
	if a.State.Running || a.State.Policy.Failed() || s.Dir == "" || s.Last.IsZero() {
		tip = text
	}
	// Set apart from the counts: one is of the work, the other of words.
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Margin(0, 8, 0, 0).Shrink(0).Label("备份状态").Tooltip(tip).KeepFocus()
	a.desktop.Get().SlideEdge(b)
	color := a.palette.Get().Colors().Faint
	if b.Hovered() {
		color = t.Text
	}
	b.Children(func() {
		kit.Dot(c, dot, 6)
		ui.Text(c, text).FontSize(kit.FsNote).TextColor(color).SingleLine()
	})
	if b.Clicked() {
		a.State.Open = !a.State.Open
	}
	kit.Popover(c, b, &a.State.Open, 1, 1, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0)
		a.backupCard(c, panel, s)
	})
}

// backupChip warns on the title bar that a backup failed, unless
// the author said not today.
func (a *View) backupChip(c *ui.Context) {
	s, ok := a.Settings()
	if !ok || !a.State.Policy.Failed() || s.Hushed(time.Now()) {
		return
	}
	short, _, _ := TroubleText(a.State.Policy.Trouble(s.Dir))
	t := a.palette.Get().Colors().Theme
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).Gap(6).AlignItems(ui.Center).Padding(0, 8).Radius(kit.RControl).Shrink(1).MinWidth(40).
		Background(a.palette.Get().Colors().WarnBg).Border(1, a.palette.Get().Colors().WarnBd).Label("备份提示").Tooltip(locales.StatusBackupWarn(short)).KeepFocus()
	if b.Hovered() {
		b.Background(a.palette.Get().Colors().WarnBd.Alpha(0.55))
	}
	b.Children(func() {
		kit.Dot(c, a.palette.Get().Colors().Amber, 6)
		ui.Text(c, locales.StatusBackupWarn(short)).FontSize(kit.FsNote).TextColor(t.Warning).SingleLine().Ellipsis("…").Shrink(1)
	})
	if b.Clicked() {
		a.State.Chip = !a.State.Chip
	}
	kit.Popover(c, b, &a.State.Chip, 1, 0, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0)
		a.backupCard(c, panel, s)
	})
}

// backupCard is what the backups are, or why the last one failed, with
// what to do about it.
func (a *View) backupCard(c *ui.Context, panel ui.Element, s archive.Settings) {
	t := a.palette.Get().Colors().Theme
	now := time.Now()
	bg := a.palette.Get().Colors().Canvas
	if a.State.Policy.Failed() && c.Theme().Dark {
		bg = a.palette.Get().Colors().Bubble
	}
	// As wide as what it says, and no wider than a note reads well.
	a.palette.Get().Colors().Pop.On(panel.MaxWidth(300).Radius(kit.RCard).Padding(12).Gap(8).Background(bg).Border(1, t.Border))
	panel.Children(func() {
		if s.Dir == "" {
			ui.Text(c, locales.BackupNone()).FontSize(kit.FsBody).FontWeight(600)
			ui.Text(c, locales.BackupLocalOnly()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if a.palette.Get().Colors().Primary(c, locales.BackupChoose()).Clicked() {
					a.State.Open = false
					a.OpenAsk()
				}
			})
			return
		}
		if a.State.Policy.Failed() && !a.State.Running {
			_, title, body := TroubleText(a.State.Policy.Trouble(s.Dir))
			ui.Text(c, title).FontSize(kit.FsBody).FontWeight(600)
			ui.Text(c, locales.BackupFailureDetails(body, lastSaid(s, now))).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				if a.palette.Get().Colors().Ghost(c, locales.BackupHushToday(), ui.Color{}).Clicked() {
					s.Hush = now
					a.Configure(s)
					a.State.Open, a.State.Chip = false, false
				}
				if a.dialogs.Get().CanPickFolder() && a.palette.Get().Colors().Secondary(c, locales.BackupChangePlace()).Clicked() {
					a.State.Open, a.State.Chip = false, false
					a.pickBackupFolder(c)
				}
				if a.palette.Get().Colors().Primary(c, locales.BackupRetry()).Disabled(a.State.Running).Clicked() {
					a.State.Open, a.State.Chip = false, false
					a.Run(c.Services())
				}
			})
			return
		}
		title := locales.BackupDone(kit.When(s.Last, now))
		if kit.SameDay(s.Last, now) {
			title = locales.BackupDone(s.Last.Local().Format("15:04"))
		}
		if s.Last.IsZero() {
			title = locales.BackupNone()
		}
		ui.Text(c, title).FontSize(kit.FsBody).FontWeight(600)
		ui.Column(c).Children(func() {
			ui.Text(c, locales.BackupTo(s.Dir)).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
			ui.Text(c, locales.BackupNextAt(next(s, now))).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
		})
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if a.palette.Get().Colors().Secondary(c, locales.BackupSettings()).Clicked() {
				a.State.Open = false
				a.desktop.Get().OpenSettings("backup.settings")
			}
			label := locales.BackupRunNow()
			if a.State.Running {
				label = locales.BackupBusy()
			}
			if a.palette.Get().Colors().Primary(c, label).Label("立即备份").Disabled(a.State.Running).Clicked() {
				a.Run(c.Services())
			}
		})
	})
}

// pickBackupFolder asks for the folder to back up into.
func (a *View) pickBackupFolder(c *ui.Context) {
	sv := c.Services()
	generation := a.generation
	a.dialogs.Get().PickInto(c, locales.DialogOsBackupFolder(), func(path string) {
		if generation != a.generation {
			return
		}
		if a.State.Ask {
			a.State.Place = path
			return
		}
		s, _ := a.Settings()
		s.Dir = path
		a.Configure(s)
		a.State.Policy.ForgetFailure()
		a.State.Listed = false
		a.Run(sv)
	})
}

// askView is the dialog asking where to back the work up.
func (a *View) askView(c *ui.Context) {
	t := a.palette.Get().Colors().Theme
	open := a.State.Ask
	a.palette.Get().Colors().Modal(c, &a.State.Ask, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.palette.Get().Colors().DialogHead(c, locales.BackupDialogTitle(), locales.BackupDialogAbout())
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				places := a.State.Places
				own := true
				for _, p := range places {
					if p.Dir == a.State.Place {
						own = false
					}
					a.placeRow(c, p.Name(), locales.BackupDetected(p.Dir), p.Dir == a.State.Place, func() { a.State.Place = p.Dir })
				}
				if a.dialogs.Get().CanPickFolder() {
					sub := locales.BackupOtherFolderNote()
					if own && a.State.Place != "" {
						sub = a.State.Place
					}
					a.placeRow(c, locales.BackupOtherFolder(), sub, own && a.State.Place != "", func() { a.pickBackupFolder(c) })
				}
			})
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, locales.BackupTiming()).FontSize(kit.FsNote).TextColor(t.TextMuted)
				ui.Column(c).Gap(8).Children(func() {
					for _, o := range whenChoices(&a.State.OnClose, &a.State.Daily, &a.State.Hourly) {
						ui.Checkbox(c, o.on, o.label)
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if a.palette.Get().Colors().Ghost(c, locales.BackupLater(), ui.Color{}).Clicked() {
					a.State.Ask = false
				}
				ui.Spacer(c)
				if a.palette.Get().Colors().Primary(c, locales.BackupStart()).Disabled(a.State.Place == "").Clicked() {
					a.startBackups(c)
				}
			})
		})
	})
	if open && !a.State.Ask {
		// Put off: asked again in a week, not each time the work opens.
		if s, _ := a.Settings(); s.Dir == "" {
			s.Later = time.Now()
			a.Configure(s)
		}
	}
}

// placeRow is one folder to choose in the dialog.
func (a *View) placeRow(c *ui.Context, label, sub string, on bool, pick func()) {
	t := a.palette.Get().Colors().Theme
	bd, ring := t.Border, a.palette.Get().Colors().Faint
	if on {
		bd, ring = a.palette.Get().Colors().Ink, a.palette.Get().Colors().Ink
	}
	b := ui.ButtonBase(c).Row().FillWidth().AlignItems(ui.Start).Gap(10).Padding(10, 12).Radius(kit.RControl).Border(1, bd).Label(label)
	if b.Hovered() && !on {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Size(14, 14).Margin(4, 0, 0, 0).Radius(kit.RRound).Border(1.5, ring).Center().Shrink(0).Children(func() {
			if on {
				kit.Dot(c, a.palette.Get().Colors().Ink, 6)
			}
		})
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			ui.Text(c, label).FontSize(kit.FsBody).LineHeight(kit.LhBody)
			if sub != "" {
				ui.Text(c, sub).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).SingleLine().Ellipsis("…")
			}
		})
	})
	if b.Clicked() {
		pick()
	}
}

type whenChoice struct {
	label string
	on    *bool
}

func whenChoices(onClose, daily, hourly *bool) []whenChoice {
	return []whenChoice{{locales.BackupWhenClose(), onClose}, {locales.BackupTickDaily(), daily}, {locales.BackupTickHourly(), hourly}}
}

// backups are the work's backups in the folder, read when they may have
// changed.
func (a *View) backups(s archive.Settings) []archive.Info {
	if !a.State.Listed {
		a.State.List, _ = archive.List(s.Dir, a.book.Get().Store().Work().ID)
		a.State.Listed = true
	}
	return a.State.List
}

// size says a number of bytes in KB or MB.
func size(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", max(1, (n+512)>>10))
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/(1<<20)), ".0") + " MB"
}

// keepChoices are how many backups the settings offer to keep.
var keepChoices = []int{10, 30, 90}

// backupSettingsView is the settings' 备份.
func (a *View) backupSettingsView(c *ui.Context) {
	t := a.palette.Get().Colors().Theme
	s, _ := a.Settings()
	now := time.Now()
	if s.Dir == "" {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(14, 16).Radius(kit.RCard).Border(1, a.palette.Get().Colors().Line).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, locales.BackupNone()).FontSize(kit.FsGroup).FontWeight(600)
				ui.Text(c, locales.BackupLocalOnly()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
			})
			if a.palette.Get().Colors().Primary(c, locales.BackupChoose()).Clicked() {
				a.OpenAsk()
			}
		})
		return
	}
	list := a.backups(s)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(14, 16).Radius(kit.RCard).Border(1, a.palette.Get().Colors().Line).Children(func() {
		again := locales.BackupRunNow()
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			title, note := locales.BackupDone(kit.When(s.Last, now)), ""
			if len(list) > 0 {
				note = Kept(list, now)
			}
			switch {
			case a.State.Running:
				title = locales.BackupBusy()
			case a.State.Policy.Failed():
				short, _, why := TroubleText(a.State.Policy.Trouble(s.Dir))
				title, note, again = locales.BackupFailed(), locales.BackupFailNote(short, why), locales.BackupRetry()
			case s.Last.IsZero():
				title = locales.BackupNone()
			}
			ui.Text(c, title).FontSize(kit.FsGroup).FontWeight(600)
			if note != "" {
				ui.Text(c, note).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
			}
		})
		if a.palette.Get().Colors().Primary(c, again).Label("立即备份").Disabled(a.State.Running).Clicked() {
			a.Run(c.Services())
		}
	})
	row := prefsRow
	{
		row(c, locales.BackupDestLabel(), func() {
			ui.Text(c, s.Dir).FontSize(kit.FsBody).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.dialogs.Get().CanPickFolder() && a.palette.Get().Colors().Secondary(c, locales.BackupChange()).Clicked() {
				a.pickBackupFolder(c)
			}
		})
		row(c, locales.BackupTiming(), func() {
			ui.Column(c).Gap(8).Children(func() {
				on := []bool{s.OnClose, s.Daily, s.Hourly}
				for _, o := range whenChoices(&on[0], &on[1], &on[2]) {
					if ui.Checkbox(c, o.on, o.label).Changed() {
						s.OnClose, s.Daily, s.Hourly = on[0], on[1], on[2]
						a.Configure(s)
					}
				}
			})
		})
		row(c, locales.BackupKeep(), func() {
			{
				// A number Kept from before these were the choices stays one.
				keeps := keepChoices
				if !slices.Contains(keeps, s.Kept()) {
					keeps = append(slices.Clone(keeps), s.Kept())
					slices.Sort(keeps)
				}
				counts := make([]string, len(keeps))
				for i, k := range keeps {
					counts[i] = strconv.Itoa(k)
				}
				a.State.KeepChoice = strconv.Itoa(s.Kept())
				keepName := func(n string) string { k, _ := strconv.Atoi(n); return locales.BackupKeepCount(k) }
				if a.palette.Get().Colors().SelectNamed(c, &a.State.KeepChoice, counts, keepName).Label("保留份数").Width(140).Changed() {
					s.Keep, _ = strconv.Atoi(a.State.KeepChoice)
					a.Configure(s) // the next backup removes those beyond it
				}
				if len(list) > 0 {
					var all int64
					for _, b := range list {
						all += b.Size
					}
					ui.Text(c, locales.BackupSizes(size(list[0].Size), size(all))).FontSize(kit.FsNote).TextColor(t.TextMuted)
				}
			}
		})
	}
}

func prefsRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).FillWidth().MinHeight(kit.HControl).Gap(8).AlignItems(ui.Center).Children(func() { ui.Text(c, label).FontSize(kit.FsBody).FontWeight(500).Width(120).Shrink(0); fn() })
}
