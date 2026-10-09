package desk

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"writer/backup"
	"writer/workspace"
)

// backupView is the state of backing the open work up.
type backupView struct {
	running bool
	err     error     // why the last backup failed; nil once one succeeds
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
	prefs          bool   // the settings dialog
	tab            string // its part shown: "模型", "外观" or "备份"
	edit           string // the service open in it: an ID, "+" for a new one
	keepChoice     string
}

// retryAfter is how long a failed backup waits before it is tried again
// unasked.
const retryAfter = 10 * time.Minute

// askAgain is how long after "以后再说" the author is asked again.
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
	if ps := syncFolders(); len(ps) > 0 {
		a.bk.place = ps[0].dir
	}
}

// startBackups keeps what the dialog chose and backs up once at once.
func (a *App) startBackups(c *ui.Context) {
	s, _ := a.backupSettings()
	s.Dir, s.OnClose, s.Daily, s.Hourly, s.Later = a.bk.place, a.bk.onClose, a.bk.daily, a.bk.hourly, time.Time{}
	a.setBackup(s)
	a.bk.ask, a.bk.err, a.bk.listed = false, nil, false
	a.runBackup(c)
}

// meta is what a backup of fl says of the work.
func (a *App) meta(fl backup.Files) backup.Meta {
	m := backup.Meta{Work: a.Store.Work().ID}
	for _, f := range fl.List {
		if strings.HasPrefix(f.Rel, "chapters/") && strings.HasSuffix(f.Rel, ".txt") && !strings.HasSuffix(f.Rel, ".outline.txt") {
			m.Chapters++
			m.Words += workspace.Count(string(f.Data))
		}
	}
	return m
}

// readWork saves what is being written and reads the work as it is then.
func (a *App) readWork() (backup.Files, error) {
	a.flush()
	var fl backup.Files
	err := a.Store.Hold(func(root string) (err error) {
		fl, err = backup.Read(root)
		return err
	})
	return fl, err
}

// runBackup backs the work up off the view thread.
func (a *App) runBackup(c *ui.Context) {
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" || a.bk.running {
		return
	}
	fl, err := a.readWork()
	if err != nil {
		a.backupDone(s, err)
		return
	}
	a.bk.running = true
	root, name, meta := a.Store.Root(), a.Store.Work().Name, a.meta(fl)
	go func() {
		_, _, err := backup.Make(fl, s.Dir, name, meta, s.Kept(), time.Now())
		a.picks <- func() {
			a.bk.running = false
			if a.Store != nil && sameFolder(a.Store.Root(), root) {
				a.backupDone(s, err)
			}
		}
		c.Invalidate()
	}()
}

// backupNow backs the work up before it closes, waiting for it.
func (a *App) backupNow() {
	s, ok := a.backupSettings()
	if !ok || s.Dir == "" || !s.OnClose {
		return
	}
	fl, err := a.readWork()
	if err == nil {
		_, _, err = backup.Make(fl, s.Dir, a.Store.Work().Name, a.meta(fl), s.Kept(), time.Now())
	}
	a.backupDone(s, err)
}

func (a *App) backupDone(s backup.Settings, err error) {
	a.bk.listed = false
	if err != nil {
		a.bk.err, a.bk.failed = err, time.Now()
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
	hourly := s.Hourly && a.page == 3 && now.Sub(s.Last) >= time.Hour
	if daily || hourly {
		a.runBackup(c)
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
	short, _, _ := trouble(s.Dir, a.bk.err)
	sub := "点一下重试"
	if !s.Last.IsZero() {
		sub += " · 上次成功 " + when(s.Last, time.Now())
	}
	it := pendItem{key: pendBackup, text: "备份未完成，" + short, sub: sub, retry: true}
	return pendGroup{name: "备份", color: a.pal.amber, items: []pendItem{it}}, true
}

// when says when t was, from now: 今天 10:32, 昨天 23:15, 10-06 21:40.
func when(t, now time.Time) string {
	switch y, m, d := t.Date(); {
	case now.Format(time.DateOnly) == t.Format(time.DateOnly):
		return "今天 " + t.Format("15:04")
	case now.AddDate(0, 0, -1).Format(time.DateOnly) == t.Format(time.DateOnly):
		return "昨天 " + t.Format("15:04")
	case y == now.Year():
		return fmt.Sprintf("%d-%02d %s", m, d, t.Format("15:04"))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// trouble says why a backup into dir failed: shortly, for the line and
// the toolbar, then as the popover's title and what to do.
func trouble(dir string, err error) (short, title, body string) {
	vol := filepath.VolumeName(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if vol != "" {
			if _, e := os.Stat(vol + string(filepath.Separator)); e != nil {
				d := strings.TrimSuffix(vol, ":")
				return "找不到 " + d + " 盘", "找不到 " + d + " 盘，这次没备份上",
					"移动硬盘可能没插。稿子在这台电脑上好好的，插上以后点「再试一次」就行。"
			}
		}
		return "找不到备份文件夹", "找不到备份文件夹，这次没备份上",
			"它可能被移走或改了名。稿子在这台电脑上好好的，换个地方或者把文件夹放回去再试一次。"
	}
	if errors.Is(err, fs.ErrPermission) {
		return "没有权限写入", "备份文件夹不让写，这次没备份上",
			"稿子在这台电脑上好好的。换个地方，或者让这个文件夹可以写入。"
	}
	return "没写成功", "这次没备份上", "稿子在这台电脑上好好的。原因：" + err.Error()
}

// lastSaid says when the last backup succeeded, for a failure's popover.
func lastSaid(s backup.Settings, now time.Time) string {
	if s.Last.IsZero() {
		return "还没有成功过。"
	}
	days := int(now.Sub(s.Last).Hours() / 24)
	if days >= 1 {
		return fmt.Sprintf("上次成功是 %d 天前。", days)
	}
	return "上次成功是" + when(s.Last, now) + "。"
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
	return "只在点「现在备份一次」时"
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
		text, color, weight = "备份没成功", t.Text, 500
	case s.Last.IsZero():
		text = "还没备份过"
	}
	b := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(4, 10, 4, 14).Radius(6).Label("备份状态")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		dot(c, color, 6)
		ui.Text(c, text).FontSize(12).FontWeight(weight).TextColor(color).SingleLine().Ellipsis("…").Shrink(1)
	})
	if b.Clicked() {
		a.bk.open = !a.bk.open
	}
	ui.PopoverBase(c, b, &a.bk.open, func(panel *ui.Element) {
		panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 8, 0)
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
	short, _, _ := trouble(s.Dir, a.bk.err)
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Margin(0, 0, 0, 6).Gap(7).AlignItems(ui.Center).Padding(4, 10).Radius(6).Shrink(0).
		Background(a.pal.warnBg).Border(1, a.pal.warnBd).Label("备份未完成")
	if b.Hovered() {
		b.Background(a.pal.warnBd.Alpha(0.5))
	}
	b.Children(func() {
		dot(c, a.pal.amber, 6)
		ui.Text(c, "备份未完成 · "+short).FontSize(12.5).TextColor(t.Warning).SingleLine()
	})
	if b.Clicked() {
		a.bk.chip = !a.bk.chip
	}
	ui.PopoverBase(c, b, &a.bk.chip, func(panel *ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(10, 0, 0, 0)
		a.backupCard(c, panel, s)
	})
}

// backupCard is what the backups are, or why the last one failed, with
// what to do about it.
func (a *App) backupCard(c *ui.Context, panel *ui.Element, s backup.Settings) {
	t := a.pal.theme
	now := time.Now()
	bg := a.pal.canvas
	if a.bk.err != nil && c.Theme().Dark {
		bg = a.pal.bubble
	}
	panel.Width(300).Radius(8).Padding(14).Gap(10).Background(bg).Border(1, t.Border).Shadow(0, 8, 28, 0, ui.RGBA(0, 0, 0, 0.12))
	panel.Children(func() {
		small := func(text string, primary bool) *ui.Element {
			b := ui.ButtonBase(c).Row().Padding(4, 10).Radius(6).Shrink(0)
			fg := t.Text
			if primary {
				b.Background(a.pal.ink)
				fg = a.pal.onInk
			} else {
				b.Border(1, t.Border)
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
			}
			b.Children(func() { ui.Text(c, text).FontSize(12.5).TextColor(fg).SingleLine() })
			return b
		}
		if a.bk.err != nil {
			_, title, body := trouble(s.Dir, a.bk.err)
			ui.Text(c, title).FontSize(13.5).FontWeight(600).FillWidth()
			ui.Text(c, body+lastSaid(s, now)).FontSize(12.5).TextColor(t.TextMuted).LineHeight(1.7).FillWidth()
			ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
				if small("再试一次", true).Disabled(a.bk.running).Clicked() {
					a.bk.open, a.bk.chip = false, false
					a.runBackup(c)
				}
				if a.PickFolder != nil && small("换个地方", false).Clicked() {
					a.bk.open, a.bk.chip = false, false
					a.pickBackupFolder(c)
				}
				if ghost(c, "今天别提醒", ui.Color{}).Clicked() {
					s.Hush = now
					a.setBackup(s)
					a.bk.open, a.bk.chip = false, false
				}
			})
			return
		}
		title := when(s.Last, now) + " 备份过"
		if s.Last.IsZero() {
			title = "还没备份过"
		}
		ui.Text(c, title).FontSize(13.5).FontWeight(600)
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			ui.Text(c, "备份在 "+s.Dir).FontSize(12.5).TextColor(t.TextMuted).FillWidth()
			ui.Text(c, "下次："+next(s, now)).FontSize(12.5).TextColor(t.TextMuted)
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			label := "现在备份一次"
			if a.bk.running {
				label = "正在备份…"
			}
			if small(label, true).Disabled(a.bk.running).Clicked() {
				a.runBackup(c)
			}
			if small("备份设置…", false).Clicked() {
				a.bk.open = false
				a.openPrefs("备份")
			}
		})
	})
}

// pickBackupFolder asks for the folder to back up into.
func (a *App) pickBackupFolder(c *ui.Context) {
	pick := a.PickFolder
	go func() {
		path, err := pick("备份到哪个文件夹")
		a.picks <- func() {
			switch {
			case err != nil:
				a.Error = err.Error()
			case path == "":
			case a.bk.ask:
				a.bk.place = path
			default:
				s, _ := a.backupSettings()
				s.Dir = path
				a.setBackup(s)
				a.bk.err, a.bk.listed = nil, false
				a.runBackup(c)
			}
		}
		c.Invalidate()
	}()
}

// askView is the dialog asking where to back the work up.
func (a *App) askView(c *ui.Context) {
	t := a.pal.theme
	open := a.bk.ask
	ui.Modal(c, &a.bk.ask, func() {
		ui.Column(c).Width(520).Gap(18).Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "稿子现在只存在这台电脑上").FontSize(18).FontWeight(700)
				ui.Text(c, "电脑坏了稿子就没了。选一个别的地方，我们定时复制一份过去。").FontSize(13.5).TextColor(t.TextMuted).LineHeight(1.7).FillWidth()
			})
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				places := syncFolders()
				own := true
				for _, p := range places {
					if p.dir == a.bk.place {
						own = false
					}
					a.placeRow(c, p.label, "在这台电脑上找到了 · "+p.dir, p.dir == a.bk.place, func() { a.bk.place = p.dir })
				}
				if a.PickFolder != nil {
					sub := ""
					if own && a.bk.place != "" {
						sub = a.bk.place
					}
					a.placeRow(c, "自己选一个文件夹…", sub, own && a.bk.place != "", func() { a.pickBackupFolder(c) })
				}
			})
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "多久备份一次").FontSize(13).TextColor(t.TextMuted)
				a.whenMenu(c, &a.bk.onClose, &a.bk.daily, &a.bk.hourly)
			})
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if ghost(c, "以后再说", ui.Color{}).Clicked() {
					a.bk.ask = false
				}
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
	b := ui.ButtonBase(c).Row().FillWidth().AlignItems(ui.Center).Gap(12).Padding(12, 14).Radius(8).Border(1, bd).Label(label)
	if b.Hovered() && !on {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Size(14, 14).Radius(7).Border(1.5, ring).Center().Shrink(0).Children(func() {
			if on {
				dot(c, a.pal.ink, 6)
			}
		})
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(14).FontWeight(500)
			if sub != "" {
				ui.Text(c, sub).FontSize(12.5).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			}
		})
	})
	if b.Clicked() {
		pick()
	}
}

// whenMenu picks when to back up: a button saying it, opening the three
// choices to tick.
func (a *App) whenMenu(c *ui.Context, onClose, daily, hourly *bool) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().AlignItems(ui.Center).Gap(6).Padding(4, 10).Radius(6).Border(1, t.Border).Label("多久备份一次")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, whenSaid(*onClose, *daily, *hourly)).FontSize(13).SingleLine()
		glyph(c, icChevronDown, 12, a.pal.faint)
	})
	b.Menu(func(m *ui.Menu) {
		for _, o := range whenChoices(onClose, daily, hourly) {
			if m.Item(o.label).Checked(*o.on).Chosen() {
				*o.on = !*o.on
			}
		}
	})
}

type whenChoice struct {
	label string
	on    *bool
}

func whenChoices(onClose, daily, hourly *bool) []whenChoice {
	return []whenChoice{{"关闭作品时", onClose}, {"每天一次（如果当天写过）", daily}, {"写作时每小时一次", hourly}}
}

// whenSaid says when the backups come.
func whenSaid(onClose, daily, hourly bool) string {
	var parts []string
	if onClose {
		parts = append(parts, "每次关闭作品时")
	}
	if daily {
		parts = append(parts, "每天一次")
	}
	if hourly {
		parts = append(parts, "写作时每小时一次")
	}
	switch len(parts) {
	case 0:
		return "只在我点的时候"
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], "、") + "，再加" + parts[len(parts)-1]
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

var keepChoices = []int{10, 30, 60, 100}

// backupSettingsView is the settings' 备份.
func (a *App) backupSettingsView(c *ui.Context) {
	t := a.pal.theme
	s, _ := a.backupSettings()
	now := time.Now()
	a.prefsHead(c, "备份", "把整部作品复制一份到别处。快照只防改错，备份防电脑坏。")
	if s.Dir == "" {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Padding(16, 18).Radius(10).Border(1, t.Border).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
				ui.Text(c, "还没有备份").FontSize(14.5).FontWeight(600)
				ui.Text(c, "稿子现在只存在这台电脑上。").FontSize(12.5).TextColor(t.TextMuted)
			})
			if a.pal.primary(c, "选一个地方…").Clicked() {
				a.openAsk()
			}
		})
		return
	}
	list := a.backups(s)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Padding(16, 18).Radius(10).Border(1, t.Border).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(3).Children(func() {
			title := when(s.Last, now) + " 备份过"
			switch {
			case a.bk.running:
				title = "正在备份…"
			case a.bk.err != nil:
				_, title, _ = trouble(s.Dir, a.bk.err)
			case s.Last.IsZero():
				title = "还没备份过"
			}
			ui.Text(c, title).FontSize(14.5).FontWeight(600)
			if len(list) > 0 {
				first := list[len(list)-1].At
				ui.Textf(c, "%s · %d 章 · 共保留 %d 份，最早一份是 %d 月 %d 日", words(list[0].Words), list[0].Chapters, len(list), first.Month(), first.Day()).
					FontSize(12.5).TextColor(t.TextMuted)
			}
		})
		if a.pal.primary(c, "现在备份一次").Disabled(a.bk.running).Clicked() {
			a.runBackup(c)
		}
	})
	row := func(label string, fn func()) {
		ui.Row(c).FillWidth().Gap(24).Padding(16, 0).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(14).FontWeight(500).Width(180).Shrink(0)
			ui.Column(c).Grow(1).Shrink(1).Children(fn)
		})
		a.pal.hairline(c)
	}
	ui.Column(c).FillWidth().Children(func() {
		a.pal.hairline(c)
		row("备份到", func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Grow(1).Shrink(1).Height(34).AlignItems(ui.Center).Padding(0, 12).Radius(6).Border(1, t.Border).Children(func() {
					ui.Text(c, s.Dir).FontSize(13.5).SingleLine().Ellipsis("…")
				})
				if a.PickFolder != nil && ui.Button(c, "换一个").Clicked() {
					a.pickBackupFolder(c)
				}
			})
		})
		row("什么时候备份", func() {
			ui.Column(c).Gap(10).Children(func() {
				on := []bool{s.OnClose, s.Daily, s.Hourly}
				for _, o := range whenChoices(&on[0], &on[1], &on[2]) {
					if ui.Checkbox(c, o.on, o.label).Changed() {
						s.OnClose, s.Daily, s.Hourly = on[0], on[1], on[2]
						a.setBackup(s)
					}
				}
			})
		})
		row("留几份", func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				names := make([]string, len(keepChoices))
				for i, k := range keepChoices {
					names[i] = fmt.Sprintf("最近 %d 份", k)
				}
				a.bk.keepChoice = fmt.Sprintf("最近 %d 份", s.Kept())
				if ui.Select(c, &a.bk.keepChoice, names).Label("留几份").Width(140).Changed() {
					for i, n := range names {
						if n == a.bk.keepChoice {
							s.Keep = keepChoices[i]
						}
					}
					a.setBackup(s) // the next backup removes those beyond it
				}
				if len(list) > 0 {
					var all int64
					for _, b := range list {
						all += b.Size
					}
					ui.Textf(c, "每份约 %s，现在一共占 %s", size(list[0].Size), size(all)).FontSize(12.5).TextColor(t.TextMuted)
				}
			})
		})
	})
}
