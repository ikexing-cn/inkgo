package desk

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"writer/backup"
	"writer/workspace"
)

// backupView is the state of backing the open work up.
type backupView struct {
	running bool
	done    chan struct{}
	err     error     // why the last backup failed; nil once one succeeds
	gone    bool      // it failed with the disk of its folder not found
	failed  time.Time // when it failed, so it is not retried every frame
	checked time.Time // when it was last seen whether one is due
	open    bool      // the sidebar line's popover
	chip    bool      // the writing toolbar's warning's popover
	// The first-open dialog: whether it shows, the folder chosen, and when.
	ask            bool
	place          string
	onClose, daily bool
	hourly         bool
	asked          bool // asked already this time the work is open
	list           []backup.Info
	listed         bool
	prefs          bool               // the settings dialog
	tab            string             // its part shown: "AI", "外观", "提醒与检查" or "备份"
	records        []workspace.Record // the work's AI record, read as the AI's part shows
	log            bool               // the AI record shown in the AI's part, in place of the rest
	logOnly        int                // the feature it shows: 0 for all, else the one chosen
	logOpen        int                // the line of records open in it, or -1
	askAI          bool               // asking before the AI is turned off or on
	edit           string             // the service open in it: an ID, "+" for a new one
	shown          string             // edit as of the last frame, to bring a form just opened into view
	keepChoice     string
	places         []place
}

// retryAfter is how long a failed backup waits before it is tried again
// unasked.
const retryAfter = 10 * time.Minute

// askAgain is how long after "稍后再说" the author is asked again.
const askAgain = 7 * 24 * time.Hour

// backupSettings is how the open work is backed up from this machine.
func (a *App) backupSettings() (backup.Settings, bool) {
	if a.Store == nil {
		return backup.Settings{}, false
	}
	return a.shelf.Backup(a.Store.Root())
}

func (a *App) setBackup(s backup.Settings) {
	if a.Store == nil {
		return
	}
	a.remember(false) // the work is on the shelf to keep them
	a.shelf.SetBackup(a.Store.Root(), s)
	a.saveShelf()
}

// place is a folder offered to back up into.
type place struct{ label, dir string }

// syncFolders finds the folders a sync service keeps on this machine,
// each with a 写作备份 folder in it to back up into.
var syncFolders = func() []place {
	home, _ := os.UserHomeDir()
	var out []place
	add := func(label, dir string) {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, place{label, filepath.Join(dir, "写作备份")})
		}
	}
	if d := os.Getenv("OneDrive"); d != "" {
		add("OneDrive 同步文件夹", d)
	}
	if ds, _ := filepath.Glob(filepath.Join(home, "Nutstore", "*", "我的坚果云")); len(ds) > 0 {
		add("坚果云同步文件夹", ds[0])
	}
	add("Dropbox 同步文件夹", filepath.Join(home, "Dropbox"))
	add("iCloud 云盘", filepath.Join(home, "iCloudDrive"))
	return out
}

// askBackup opens the dialog that asks where to back the work up, the
// first time it is opened here, or a while after it was put off.
func (a *App) askBackup() {
	if !a.Backups || a.Store == nil || a.bk.asked {
		return
	}
	a.bk.asked = true
	s, ok := a.backupSettings()
	if ok && (s.Dir != "" || time.Since(s.Later) < askAgain) {
		return
	}
	a.openAsk()
}

func (a *App) openAsk() {
	a.bk.ask, a.bk.place = true, ""
	a.bk.onClose, a.bk.daily, a.bk.hourly = true, true, false
	a.bk.places = syncFolders()
	if len(a.bk.places) > 0 {
		a.bk.place = a.bk.places[0].dir
	}
}

// startBackups keeps what the dialog chose and backs up once at once.
func (a *App) startBackups(c *ui.Context) {
	s, _ := a.backupSettings()
	s.Dir, s.OnClose, s.Daily, s.Hourly, s.Later = a.bk.place, a.bk.onClose, a.bk.daily, a.bk.hourly, time.Time{}
	a.setBackup(s)
	a.bk.ask, a.bk.err, a.bk.listed = false, nil, false
	a.runBackup(c.Services())
}

// meta is what a backup of fl says of the work.
func backupMeta(work string, fl backup.Files) backup.Meta {
	m := backup.Meta{Work: work}
	for _, f := range fl.List {
		if strings.HasPrefix(f.Rel, "chapters/") && strings.HasSuffix(f.Rel, ".txt") && !strings.HasSuffix(f.Rel, ".outline.txt") {
			m.Chapters++
			m.Words += workspace.Count(string(f.Data))
		}
	}
	return m
}

// readWork reads the work while the store is held still.
func readWork(st *workspace.Store) (backup.Files, error) {
	var fl backup.Files
	err := st.Hold(func(root string) (err error) {
		fl, err = backup.Read(root)
		return err
	})
	return fl, err
}

// runBackup backs the work up off the view thread, which sv wakes as it
// is done.
func (a *App) runBackup(sv ui.Services) {
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" || a.bk.running {
		return
	}
	a.flush()
	st := a.Store
	root, work := st.Root(), st.Work()
	picks := a.picks
	a.bk.running = true
	done := make(chan struct{})
	a.bk.done = done
	go func() {
		fl, err := readWork(st)
		if err == nil {
			_, _, err = backup.Make(fl, s.Dir, work.Name, backupMeta(work.ID, fl), s.Kept(), time.Now())
		}
		// Done before the result is handed back: a closing work waits on
		// done from the view thread, which takes no picks meanwhile.
		close(done)
		picks <- func() {
			if a.bk.done != done {
				return
			}
			a.bk.running = false
			a.bk.done = nil
			if a.Store != nil && sameFolder(a.Store.Root(), root) {
				a.backupDone(s, err)
			}
		}
		sv.Invalidate()
	}()
}

// backupNow backs the work up before it closes, waiting for it.
func (a *App) backupNow() {
	if a.bk.running {
		<-a.bk.done
		a.bk.running = false
		a.bk.done = nil
	}
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" || !s.OnClose {
		return
	}
	a.flush()
	st := a.Store
	work := st.Work()
	fl, err := readWork(st)
	if err == nil {
		_, _, err = backup.Make(fl, s.Dir, work.Name, backupMeta(work.ID, fl), s.Kept(), time.Now())
	}
	a.backupDone(s, err)
}

func (a *App) backupDone(s backup.Settings, err error) {
	a.bk.listed = false
	if err != nil {
		// The disk is looked for once, here: what says why draws each
		// frame, and a disk that is away may take long to say so.
		a.bk.err, a.bk.failed, a.bk.gone = err, time.Now(), errors.Is(err, fs.ErrNotExist) && volumeGone(s.Dir)
		return
	}
	a.bk.err = nil
	if a.write.ignored[pendBackup] {
		delete(a.write.ignored, pendBackup) // the next failure is pending again
		a.savePrefs()
	}
	if cur, ok := a.backupSettings(); ok {
		cur.Last = time.Now()
		a.setBackup(cur)
	}
}

// backupDue backs up when the settings say it is time: once a day, and
// every hour of writing.
func (a *App) backupDue(c *ui.Context) {
	now := time.Now()
	if a.Store == nil || a.bk.running || now.Sub(a.bk.checked) < time.Minute {
		return
	}
	a.bk.checked = now
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" || a.bk.err != nil && now.Sub(a.bk.failed) < retryAfter {
		return
	}
	daily := s.Daily && s.Last.Format(time.DateOnly) != now.Format(time.DateOnly)
	hourly := s.Hourly && a.page == pageWrite && now.Sub(s.Last) >= time.Hour
	if daily || hourly {
		a.runBackup(c.Services())
	}
}

// pendBackup is the key that ignores a failed backup in the pending list,
// until a backup succeeds.
const pendBackup = "备份/没成功"

// backupPending is the pending list's group for a backup that failed.
func (a *App) backupPending() (pendGroup, bool) {
	s, ok := a.backupSettings()
	if !ok || a.bk.err == nil || a.write.ignored[pendBackup] {
		return pendGroup{}, false
	}
	short, _, _ := trouble(s.Dir, a.bk.err, a.bk.gone)
	sub := "点击重试"
	if !s.Last.IsZero() {
		sub += " · 上次备份 " + when(s.Last, time.Now())
	}
	it := pendItem{key: pendBackup, text: "备份失败，" + short, sub: sub, retry: true}
	return pendGroup{name: "备份", color: a.pal.amber, items: []pendItem{it}}, true
}

// volumeGone says the disk dir is on cannot be found, as one unplugged.
func volumeGone(dir string) bool {
	vol := filepath.VolumeName(dir)
	if vol == "" {
		return false
	}
	_, err := os.Stat(vol + string(filepath.Separator))
	return err != nil
}

// trouble says why a backup into dir failed: shortly, for the line and
// the toolbar, then as the popover's title and what to do. gone is that
// the disk of dir was not found as it failed.
func trouble(dir string, err error, gone bool) (short, title, body string) {
	if errors.Is(err, fs.ErrNotExist) {
		if gone {
			d := strings.TrimSuffix(filepath.VolumeName(dir), ":")
			return "找不到 " + d + " 盘", "找不到 " + d + " 盘，本次备份未完成",
				"作品未受影响，请接入硬盘后重试。"
		}
		return "找不到备份文件夹", "找不到备份文件夹，本次备份未完成",
			"作品未受影响，文件夹可能已被移动或改名。"
	}
	if errors.Is(err, fs.ErrPermission) {
		return "没有权限写入", "备份文件夹无法写入，本次备份未完成",
			"作品未受影响，该文件夹没有写入权限。"
	}
	return "备份失败", "本次备份未完成", "作品未受影响。原因：" + err.Error()
}

// lastSaid says when the last backup succeeded, for a failure's popover.
func lastSaid(s backup.Settings, now time.Time) string {
	if s.Last.IsZero() {
		return "尚无成功的备份。"
	}
	days := int(now.Sub(s.Last).Hours() / 24)
	if days >= 1 {
		return fmt.Sprintf("上次备份于 %d 天前。", days)
	}
	return "上次备份于" + when(s.Last, now) + "。"
}

// kept says what the last of the backups listed holds and how many there
// are, newest first, with the day of the first when there are more.
func kept(list []backup.Info, now time.Time) string {
	note := fmt.Sprintf("%s · %d 章 · 已存 %d 份", words(list[0].Words), list[0].Chapters, len(list))
	if len(list) > 1 {
		note += "，最早 " + dayOf(list[len(list)-1].At, now)
	}
	return note
}

// next says when the next backup comes.
func next(s backup.Settings, now time.Time) string {
	switch {
	case s.Hourly:
		return s.Last.Add(time.Hour).Format("15:04") + " 前后"
	case s.OnClose:
		return "关闭作品时"
	case s.Daily:
		return "明天"
	}
	return "仅在点击「立即备份」时"
}

// backupLine is the sidebar's small line of how the backups go; it opens
// what they are and what to do.
func (a *App) backupLine(c *ui.Context) {
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" {
		return
	}
	t := a.pal.theme
	text, color, weight := "已备份 · "+when(s.Last, time.Now()), a.pal.faint, 400
	switch {
	case a.bk.running:
		text = "正在备份…"
	case a.bk.err != nil:
		text, color, weight = "备份失败", t.Text, 500
	case s.Last.IsZero():
		text = "尚未备份"
	}
	b := ui.ButtonBase(c).Row().FillWidth().Height(hControlSm).Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(0, 10, 0, 14).Radius(rControl).Label("备份状态")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		dot(c, color, 6)
		ui.Text(c, text).FontSize(fsNote).FontWeight(weight).TextColor(color).SingleLine().Ellipsis("…").Shrink(1)
	})
	if b.Clicked() {
		a.bk.open = !a.bk.open
	}
	ui.PopoverBase(c, b, &a.bk.open, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 4, 0)
		a.backupCard(c, panel, s)
	})
}

// backupChip warns on the writing toolbar that a backup failed, unless
// the author said not today.
func (a *App) backupChip(c *ui.Context) {
	s, ok := a.backupSettings()
	if !ok || a.bk.err == nil || s.Hush.Format(time.DateOnly) == time.Now().Format(time.DateOnly) {
		return
	}
	short, _, _ := trouble(s.Dir, a.bk.err, a.bk.gone)
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Height(hControlSm).Margin(0, 0, 0, 6).Gap(6).AlignItems(ui.Center).Padding(0, 8).Radius(rControl).Shrink(0).
		Background(a.pal.warnBg).Border(1, a.pal.warnBd).Label("备份提示")
	if b.Hovered() {
		b.Background(a.pal.warnBd.Alpha(0.55))
	}
	b.Children(func() {
		dot(c, a.pal.amber, 6)
		ui.Text(c, "备份失败 · "+short).FontSize(fsNote).TextColor(t.Warning).SingleLine()
	})
	if b.Clicked() {
		a.bk.chip = !a.bk.chip
	}
	ui.PopoverBase(c, b, &a.bk.chip, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0)
		a.backupCard(c, panel, s)
	})
}

// backupCard is what the backups are, or why the last one failed, with
// what to do about it.
func (a *App) backupCard(c *ui.Context, panel ui.Element, s backup.Settings) {
	t := a.pal.theme
	now := time.Now()
	bg := a.pal.canvas
	if a.bk.err != nil && c.Theme().Dark {
		bg = a.pal.bubble
	}
	// As wide as what it says, and no wider than a note reads well.
	a.pal.pop.on(panel.MaxWidth(300).Radius(rCard).Padding(12).Gap(8).Background(bg).Border(1, t.Border))
	panel.Children(func() {
		if a.bk.err != nil {
			_, title, body := trouble(s.Dir, a.bk.err, a.bk.gone)
			ui.Text(c, title).FontSize(fsBody).FontWeight(600)
			ui.Text(c, body+lastSaid(s, now)).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				if a.pal.ghost(c, "今日不再提醒", ui.Color{}).Clicked() {
					s.Hush = now
					a.setBackup(s)
					a.bk.open, a.bk.chip = false, false
				}
				if a.PickFolder != nil && a.pal.secondary(c, "更换位置").Clicked() {
					a.bk.open, a.bk.chip = false, false
					a.pickBackupFolder(c)
				}
				if a.pal.primary(c, "重试").Disabled(a.bk.running).Clicked() {
					a.bk.open, a.bk.chip = false, false
					a.runBackup(c.Services())
				}
			})
			return
		}
		title := when(s.Last, now) + " 已备份"
		if s.Last.IsZero() {
			title = "尚未备份"
		}
		ui.Text(c, title).FontSize(fsBody).FontWeight(600)
		ui.Column(c).Children(func() {
			ui.Text(c, "备份至 "+s.Dir).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
			ui.Text(c, "下次："+next(s, now)).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
		})
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if a.pal.secondary(c, "备份设置…").Clicked() {
				a.bk.open = false
				a.openPrefs("备份")
			}
			label := "立即备份"
			if a.bk.running {
				label = "正在备份…"
			}
			if a.pal.primary(c, label).Label("立即备份").Disabled(a.bk.running).Clicked() {
				a.runBackup(c.Services())
			}
		})
	})
}

// pickBackupFolder asks for the folder to back up into.
func (a *App) pickBackupFolder(c *ui.Context) {
	sv := c.Services()
	a.pickInto(c, "选择备份文件夹", func(path string) {
		if a.bk.ask {
			a.bk.place = path
			return
		}
		s, _ := a.backupSettings()
		s.Dir = path
		a.setBackup(s)
		a.bk.err, a.bk.listed = nil, false
		a.runBackup(sv)
	})
}

// askView is the dialog asking where to back the work up.
func (a *App) askView(c *ui.Context) {
	t := a.pal.theme
	open := a.bk.ask
	a.modal(c, &a.bk.ask, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "作品目前仅保存在本机", "选择其他位置，定期保存副本。")
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				places := a.bk.places
				own := true
				for _, p := range places {
					if p.dir == a.bk.place {
						own = false
					}
					a.placeRow(c, p.label, "在本机检测到 · "+p.dir, p.dir == a.bk.place, func() { a.bk.place = p.dir })
				}
				if a.PickFolder != nil {
					sub := "移动硬盘、U 盘或其他同步文件夹"
					if own && a.bk.place != "" {
						sub = a.bk.place
					}
					a.placeRow(c, "选择其他文件夹…", sub, own && a.bk.place != "", func() { a.pickBackupFolder(c) })
				}
			})
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "备份时机").FontSize(fsNote).TextColor(t.TextMuted)
				ui.Column(c).Gap(8).Children(func() {
					for _, o := range whenChoices(&a.bk.onClose, &a.bk.daily, &a.bk.hourly) {
						ui.Checkbox(c, o.on, o.label)
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if a.pal.ghost(c, "稍后再说", ui.Color{}).Clicked() {
					a.bk.ask = false
				}
				ui.Spacer(c)
				if a.pal.primary(c, "开始备份").Disabled(a.bk.place == "").Clicked() {
					a.startBackups(c)
				}
			})
		})
	})
	if open && !a.bk.ask {
		// Put off: asked again in a week, not each time the work opens.
		if s, _ := a.backupSettings(); s.Dir == "" {
			s.Later = time.Now()
			a.setBackup(s)
		}
	}
}

// placeRow is one folder to choose in the dialog.
func (a *App) placeRow(c *ui.Context, label, sub string, on bool, pick func()) {
	t := a.pal.theme
	bd, ring := t.Border, a.pal.faint
	if on {
		bd, ring = a.pal.ink, a.pal.ink
	}
	b := ui.ButtonBase(c).Row().FillWidth().AlignItems(ui.Start).Gap(10).Padding(10, 12).Radius(rControl).Border(1, bd).Label(label)
	if b.Hovered() && !on {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Size(14, 14).Margin(4, 0, 0, 0).Radius(rRound).Border(1.5, ring).Center().Shrink(0).Children(func() {
			if on {
				dot(c, a.pal.ink, 6)
			}
		})
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			ui.Text(c, label).FontSize(fsBody).LineHeight(lhBody)
			if sub != "" {
				ui.Text(c, sub).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).SingleLine().Ellipsis("…")
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
	return []whenChoice{{"关闭作品时", onClose}, {"每天一次（当天有写作时）", daily}, {"写作时每小时一次", hourly}}
}

// backups are the work's backups in the folder, read when they may have
// changed.
func (a *App) backups(s backup.Settings) []backup.Info {
	if !a.bk.listed {
		a.bk.list, _ = backup.List(s.Dir, a.Store.Work().ID)
		a.bk.listed = true
	}
	return a.bk.list
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
func (a *App) backupSettingsView(c *ui.Context) {
	t := a.pal.theme
	s, _ := a.backupSettings()
	now := time.Now()
	if s.Dir == "" {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(14, 16).Radius(rCard).Border(1, a.pal.line).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, "尚未备份").FontSize(fsGroup).FontWeight(600)
				ui.Text(c, "作品目前仅保存在本机。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
			})
			if a.pal.primary(c, "选择位置…").Clicked() {
				a.openAsk()
			}
		})
		return
	}
	list := a.backups(s)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(14, 16).Radius(rCard).Border(1, a.pal.line).Children(func() {
		again := "立即备份"
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			title, note := when(s.Last, now)+" 已备份", ""
			if len(list) > 0 {
				note = kept(list, now)
			}
			switch {
			case a.bk.running:
				title = "正在备份…"
			case a.bk.err != nil:
				short, _, why := trouble(s.Dir, a.bk.err, a.bk.gone)
				title, note, again = "备份失败", short+"。"+why, "重试"
			case s.Last.IsZero():
				title = "尚未备份"
			}
			ui.Text(c, title).FontSize(fsGroup).FontWeight(600)
			if note != "" {
				ui.Text(c, note).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
			}
		})
		if a.pal.primary(c, again).Label("立即备份").Disabled(a.bk.running).Clicked() {
			a.runBackup(c.Services())
		}
	})
	row := a.prefsRow
	{
		row(c, "备份到", func() {
			ui.Text(c, s.Dir).FontSize(fsBody).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.PickFolder != nil && a.pal.secondary(c, "更换").Clicked() {
				a.pickBackupFolder(c)
			}
		})
		row(c, "备份时机", func() {
			ui.Column(c).Gap(8).Children(func() {
				on := []bool{s.OnClose, s.Daily, s.Hourly}
				for _, o := range whenChoices(&on[0], &on[1], &on[2]) {
					if ui.Checkbox(c, o.on, o.label).Changed() {
						s.OnClose, s.Daily, s.Hourly = on[0], on[1], on[2]
						a.setBackup(s)
					}
				}
			})
		})
		row(c, "保留份数", func() {
			{
				// A number kept from before these were the choices stays one.
				keeps := keepChoices
				if !slices.Contains(keeps, s.Kept()) {
					keeps = append(slices.Clone(keeps), s.Kept())
					slices.Sort(keeps)
				}
				names := make([]string, len(keeps))
				for i, k := range keeps {
					names[i] = fmt.Sprintf("最近 %d 份", k)
				}
				a.bk.keepChoice = fmt.Sprintf("最近 %d 份", s.Kept())
				if ui.Select(c, &a.bk.keepChoice, names).Label("保留份数").Width(140).Changed() {
					for i, n := range names {
						if n == a.bk.keepChoice {
							s.Keep = keeps[i]
						}
					}
					a.setBackup(s) // the next backup removes those beyond it
				}
				if len(list) > 0 {
					var all int64
					for _, b := range list {
						all += b.Size
					}
					ui.Textf(c, "每份约 %s，共 %s", size(list[0].Size), size(all)).FontSize(fsNote).TextColor(t.TextMuted)
				}
			}
		})
	}
}
