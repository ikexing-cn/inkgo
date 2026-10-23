package desk

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
)

// noticeFor is how long a confirmation stays. The time stops while the
// pointer rests on it.
var noticeFor = 3 * time.Second

// A toast is as wide as its words, between toastMin and toastMax; toastsShown
// is how many show at once, the rest fold into one line that opens them.
const (
	toastMin    = 200
	toastMax    = 300
	toastsShown = 2
)

// toasts float the error, the request running, what was done and what to
// do first at the top right, over every page and dialog: none of them pushes
// the page down.
// The fields stay the state; a toast shows each while it is set, and
// closing the toast clears it.
func (a *App) toasts(c *ui.Context) {
	busy := a.busy
	if a.replying() && a.talkShown() || !a.aiOn() {
		busy = "" // the turn shows its own progress in its talk; all that runs is the AI's
	}
	errText := a.status.ErrorText()
	errKey := errText
	if n := a.status.Repeated(); n > 1 {
		errKey += "×" + strconv.Itoa(n)
	}
	a.syncToast(c, "busy", busy, busy, -1, func() {})
	a.syncToast(c, "error", errText, errKey, -1, a.status.CloseError)
	notice := a.status.NoticeText()
	a.syncToast(c, "notice", notice, notice, noticeFor, func() {
		a.status.CloseNotice(notice)
	})
	hint := a.status.HintText()
	a.syncToast(c, "hint", hint, hint, noticeFor, func() {
		a.status.CloseHint(hint)
	})
	ui.ToastViewportBase(c, func(v ui.Element, list []ui.Toast) {
		v.Justify(ui.Start).AlignItems(ui.End).Gap(8).Padding(2*kit.TitleHeight+12, 16, 0, 0) // under the title bar and the page's top row
		if len(list) <= toastsShown {
			a.toastsOpen = false
		}
		shown := 0
		for i := len(list) - 1; i >= 0; i-- { // the newest on top
			if shown == toastsShown && !a.toastsOpen {
				a.moreToasts(c, i+1)
				break
			}
			a.toastCard(c, list[i])
			shown++
		}
	})
}

// syncToast shows text as the toast id while it is set, again when key
// changes, and closes the toast once text is cleared. done runs when the
// toast closes by itself or by the author.
func (a *App) syncToast(c *ui.Context, id, text, key string, life time.Duration, done func()) {
	if a.toastShown == nil {
		a.toastShown = map[string]string{}
	}
	switch {
	case text == "" && a.toastShown[id] != "":
		delete(a.toastShown, id)
		c.CloseToast(id)
	case text != "" && a.toastShown[id] != key:
		a.toastShown[id] = key
		title, detail := splitToast(text)
		c.AddToast(ui.Toast{ID: id, Type: id, Title: title, Description: detail, Timeout: life, OnClose: func() {
			if a.toastShown[id] == key {
				delete(a.toastShown, id)
				done()
			}
		}})
	}
}

// splitToast puts what happened in the title and the rest under it, in
// smaller type: a toast is as wide as its longest line, so one long
// sentence would stretch it.
func splitToast(s string) (title, detail string) {
	if i := strings.IndexAny(s, "：；。，"); i > 0 {
		cut := i + len("：")
		if rest := strings.TrimSpace(s[cut:]); rest != "" {
			return s[:i], rest
		}
		return s[:i], ""
	}
	return s, ""
}

// toastCard draws one toast: only its icon carries the colour.
func (a *App) toastCard(c *ui.Context, ts ui.Toast) {
	t := a.pal.Theme
	p := ui.ToastBase(c, ts)
	box := p.Root.Row().MinWidth(toastMin).MaxWidth(toastMax).Gap(10).Padding(12, 10, 12, 14).AlignItems(ui.Start).Radius(kit.RCard).
		Background(a.pal.Raised).Border(1, t.Border).Clip()
	a.pal.Pop.On(box)
	box.Children(func() {
		if ts.Type != "hint" { // a word on what to do is neither done nor wrong
			ui.Box(c).Padding(2, 0, 0, 0).Shrink(0).Children(func() {
				switch ts.Type {
				case "busy":
					ui.Spinner(c).Size(kit.IconMd, kit.IconMd)
				case "error":
					kit.Glyph(c, kit.IcAlert, kit.IconMd, t.Danger)
				default:
					kit.Glyph(c, kit.IcDone, kit.IconMd, t.Success)
				}
			})
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ts.Title).FontSize(kit.FsBody).FontWeight(600).Shrink(1)
				if n := a.status.Repeated(); ts.Type == "error" && n > 1 {
					ui.Row(c).Padding(0, 6).Radius(kit.RXs).Background(t.SurfaceHover).Shrink(0).Children(func() {
						ui.Text(c, "×"+strconv.Itoa(n)).FontSize(kit.FsSmall).TextColor(t.TextMuted)
					})
				}
			})
			if ts.Description != "" {
				ui.Text(c, ts.Description).FontSize(kit.FsNote).TextColor(t.TextMuted).Selectable()
			}
			a.toastActions(c, ts)
		})
		if ts.Type != "busy" {
			ui.Box(c).Margin(-2, -2, 0, 0).Shrink(0).Children(func() {
				p.CloseButton().Size(kit.HTag, kit.HTag).Center().Radius(kit.RXs).Label("关闭提示").Children(func() {
					kit.Glyph(c, kit.IcX, kit.IconSm, a.pal.Faint)
				})
			})
		}
		if left := p.Left(); left > 0 {
			// The line under it is the time left, grey, as a hint fades too.
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(2).Background(a.pal.Line).Children(func() {
				ui.Box(c).Height(2).WidthPercent(100 * float32(left) / float32(noticeFor)).Background(a.pal.Faint)
			})
			c.AnimationFrame()
		}
	})
}

// toastActions are what can be done about it, the first the likely one.
func (a *App) toastActions(c *ui.Context, ts ui.Toast) {
	type act struct {
		label string
		fn    func()
		name  string // what tests and screen readers call it, when not the label
	}
	var acts []act
	switch ts.Type {
	case "busy":
		acts = append(acts, act{"取消", func() {
			if a.cancel != nil {
				a.cancel()
			}
		}, "取消请求"})
	case "error":
		if again := a.status.RetryAction(); again != nil {
			acts = append(acts, act{"重试", func() { a.SetError(nil); again() }, ""})
		}
		var setup *aiport.SetupError
		if errors.As(a.status.Err(), &setup) {
			// What it says is done by going there: it goes, as in the prototype.
			acts = append(acts, act{"打开设置", func() { a.SetError(nil); a.openPrefs("AI") }, ""})
		}
		text := a.status.ErrorText()
		acts = append(acts, act{"复制详情", func() { c.WriteClipboard(text) }, ""})
	}
	if len(acts) == 0 {
		return
	}
	t := a.pal.Theme
	// At the right, as every row of buttons: the one that matters most, the
	// first of them, stands last.
	ui.Row(c).FillWidth().Gap(14).Padding(6, 0, 0, 0).Justify(ui.End).Children(func() {
		for i := len(acts) - 1; i >= 0; i-- {
			x := acts[i]
			b := ui.ButtonBase(c).Shrink(0).KeepFocus()
			if x.name != "" {
				b.Label(x.name)
			}
			fg, w := t.TextMuted, 400
			if i == 0 {
				fg, w = t.Text, 600
			}
			if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() { ui.Text(c, x.label).FontSize(kit.FsNote).FontWeight(w).TextColor(fg) })
			if b.Clicked() {
				x.fn()
			}
		}
	})
}

// moreToasts folds the older toasts into one line that opens them.
func (a *App) moreToasts(c *ui.Context, n int) {
	t := a.pal.Theme
	b := ui.ButtonBase(c).Padding(4, 10).Radius(kit.RCard).Background(a.pal.Raised).Border(1, t.Border).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, "还有 "+strconv.Itoa(n)+" 条").FontSize(kit.FsSmall).TextColor(t.TextMuted) })
	if b.Clicked() {
		a.toastsOpen = true
	}
}
