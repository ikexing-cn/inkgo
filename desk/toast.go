package desk

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
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

// toasts float the error, the request running and the confirmation at the
// top right, over every page and dialog: none of them pushes the page down.
// The fields stay the state; a toast shows each while it is set, and
// closing the toast clears it.
func (a *App) toasts(c *ui.Context) {
	busy := a.busy
	if a.talking && a.talkShown() {
		busy = "" // the turn shows its own progress
	}
	errKey := a.Error
	if a.Error != "" && a.Error == a.failed {
		errKey += "×" + strconv.Itoa(a.fails)
	}
	a.syncToast(c, "busy", busy, busy, -1, func() {})
	a.syncToast(c, "error", a.Error, errKey, -1, func() {
		a.Error, a.failed, a.fails = "", "", 0
	})
	notice := a.notice
	a.syncToast(c, "notice", notice, notice, noticeFor, func() {
		if a.notice == notice {
			a.notice = ""
		}
	})
	ui.ToastViewportBase(c, func(v ui.Element, list []ui.Toast) {
		v.Justify(ui.Start).AlignItems(ui.End).Gap(8).Padding(titleHeight+12, 16, 0, 0)
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
	t := a.pal.theme
	p := ui.ToastBase(c, ts)
	box := p.Root.Row().MinWidth(toastMin).MaxWidth(toastMax).Gap(10).Padding(12, 10, 12, 14).AlignItems(ui.Start).Radius(rCard).
		Background(a.pal.raised).Border(1, t.Border).Clip()
	a.pal.pop.on(box)
	box.Children(func() {
		ui.Box(c).Padding(2, 0, 0, 0).Shrink(0).Children(func() {
			switch ts.Type {
			case "busy":
				ui.Spinner(c).Size(iconMd, iconMd)
			case "error":
				glyph(c, icAlert, iconMd, t.Danger)
			default:
				glyph(c, icDone, iconMd, t.Success)
			}
		})
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ts.Title).FontSize(fsBody).FontWeight(600).Shrink(1)
				if ts.Type == "error" && a.fails > 1 && a.failed == a.Error {
					ui.Row(c).Padding(0, 6).Radius(rXs).Background(t.SurfaceHover).Shrink(0).Children(func() {
						ui.Text(c, "×"+strconv.Itoa(a.fails)).FontSize(fsSmall).TextColor(t.TextMuted)
					})
				}
			})
			if ts.Description != "" {
				ui.Text(c, ts.Description).FontSize(fsNote).TextColor(t.TextMuted).Selectable()
			}
			a.toastActions(c, ts)
		})
		if ts.Type != "busy" {
			ui.Box(c).Margin(-2, -2, 0, 0).Shrink(0).Children(func() {
				p.CloseButton().Size(hTag, hTag).Center().Radius(rXs).Label("关闭提示").Children(func() {
					glyph(c, icX, iconSm, a.pal.faint)
				})
			})
		}
		if left := p.Left(); left > 0 {
			// The line under it is the time left.
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(2).Background(a.pal.line).Children(func() {
				ui.Box(c).Height(2).WidthPercent(100 * float32(left) / float32(noticeFor)).Background(t.Success.Alpha(0.55))
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
		if a.again != nil && a.failed == a.Error {
			again := a.again
			acts = append(acts, act{"重试", func() { a.Error = ""; again() }, ""})
		}
		head := a.Error // what went wrong, without the advice after it
		if i := strings.IndexAny(head, "：；。"); i > 0 {
			head = head[:i]
		}
		if strings.Contains(head, "模型") || strings.Contains(head, "AI") {
			acts = append(acts, act{"打开设置", func() { a.openPrefs("模型") }, ""})
		}
		text := a.Error
		acts = append(acts, act{"复制详情", func() { c.WriteClipboard(text) }, ""})
	}
	if len(acts) == 0 {
		return
	}
	t := a.pal.theme
	ui.Row(c).Gap(14).Padding(6, 0, 0, 0).Children(func() {
		for i, x := range acts {
			b := ui.ButtonBase(c).Shrink(0)
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
			b.Children(func() { ui.Text(c, x.label).FontSize(fsNote).FontWeight(w).TextColor(fg) })
			if b.Clicked() {
				x.fn()
			}
		}
	})
}

// moreToasts folds the older toasts into one line that opens them.
func (a *App) moreToasts(c *ui.Context, n int) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Padding(4, 10).Radius(rCard).Background(a.pal.raised).Border(1, t.Border).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, "还有 "+strconv.Itoa(n)+" 条").FontSize(fsSmall).TextColor(t.TextMuted) })
	if b.Clicked() {
		a.toastsOpen = true
	}
}
