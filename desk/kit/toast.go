package kit

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/locales"
)

// A toast is as wide as its words, between toastMin and toastMax.
const (
	toastMin = 200
	toastMax = 300
)

// ToastAction is what can be done about a toast, as a button under it.
type ToastAction struct {
	Label string
	Fn    func()
	Name  string // what tests and screen readers call it, when not the label
}

// SplitToast puts what happened in the title and the rest under it, in
// smaller type: a toast is as wide as its longest line, so one long
// sentence would stretch it.
func SplitToast(s string) (title, detail string) {
	if i := strings.IndexAny(s, "：；。，"); i > 0 {
		cut := i + len("：")
		if rest := strings.TrimSpace(s[cut:]); rest != "" {
			return s[:i], rest
		}
		return s[:i], ""
	}
	return s, ""
}

// ToastCard draws one toast: only its icon carries the colour.
func (p *Palette) ToastCard(c *ui.Context, ts ui.Toast, status *Status, life time.Duration, actions func()) {
	t := p.Theme
	toast := ui.ToastBase(c, ts)
	box := toast.Root.Row().MinWidth(toastMin).MaxWidth(toastMax).Gap(10).Padding(12, 10, 12, 14).AlignItems(ui.Start).Radius(RCard).
		Background(p.Raised).Border(1, t.Border).Clip()
	progress := Arrive(box, 0, MoveTime, 0)
	box.Translate(ToastTravel*(1-progress), 0).Opacity(progress)
	p.Pop.On(box)
	box.Children(func() {
		if ts.Type != "hint" { // a word on what to do is neither done nor wrong
			ui.Box(c).Padding(2, 0, 0, 0).Shrink(0).Children(func() {
				switch ts.Type {
				case "busy":
					ui.Spinner(c).Size(IconMd, IconMd)
				case "error":
					Glyph(c, IcAlert, IconMd, t.Danger)
				default:
					Glyph(c, IcDone, IconMd, t.Success)
				}
			})
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ts.Title).FontSize(FsBody).FontWeight(600).Shrink(1)
				if n := status.Repeated(); ts.Type == "error" && n > 1 {
					ui.Row(c).Padding(0, 6).Radius(RXs).Background(t.SurfaceHover).Shrink(0).Children(func() {
						ui.Text(c, "×"+strconv.Itoa(n)).FontSize(FsSmall).TextColor(t.TextMuted)
					})
				}
			})
			if ts.Description != "" {
				ui.Text(c, ts.Description).FontSize(FsNote).TextColor(t.TextMuted).Selectable()
			}
			actions()
		})
		if ts.Type != "busy" {
			ui.Box(c).Margin(-2, -2, 0, 0).Shrink(0).Children(func() {
				toast.CloseButton().Size(HTag, HTag).Center().Radius(RXs).Label(locales.ToastClose()).Children(func() {
					Glyph(c, IcX, IconSm, p.Faint)
				})
			})
		}
		if left := toast.Left(); left > 0 {
			// The line under it is the time left, grey, as a hint fades too.
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(2).Background(p.Line).Children(func() {
				ui.Box(c).Height(2).WidthPercent(100 * float32(left) / float32(life)).Background(p.Faint)
			})
			c.AnimationFrame()
		}
	})
}

// ToastActions are what can be done about it, the first the likely one.
func (p *Palette) ToastActions(c *ui.Context, acts []ToastAction) {
	if len(acts) == 0 {
		return
	}
	t := p.Theme
	// At the right, as every row of buttons: the one that matters most, the
	// first of them, stands last.
	ui.Row(c).FillWidth().Gap(14).Padding(6, 0, 0, 0).Justify(ui.End).Children(func() {
		for i := len(acts) - 1; i >= 0; i-- {
			x := acts[i]
			b := ui.ButtonBase(c).Shrink(0).KeepFocus()
			if x.Name != "" {
				b.Label(x.Name)
			}
			fg, w := t.TextMuted, 400
			if i == 0 {
				fg, w = t.Text, 600
			}
			if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() { ui.Text(c, x.Label).FontSize(FsNote).FontWeight(w).TextColor(fg) })
			if b.Clicked() {
				x.Fn()
			}
		}
	})
}

// MoreToasts folds the older toasts into one line that opens them.
func (p *Palette) MoreToasts(c *ui.Context, n int, open *bool) {
	t := p.Theme
	b := ui.ButtonBase(c).Padding(4, 10).Radius(RCard).Background(p.Raised).Border(1, t.Border).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Text(c, locales.ToastMore(n)).FontSize(FsSmall).TextColor(t.TextMuted) })
	if b.Clicked() {
		*open = true
	}
}
