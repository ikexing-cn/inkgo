package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func trim(s string) string { return strings.TrimSpace(s) }

// narrow collapses the sidebar to icons so the chat keeps a readable width.
const narrow = 1180

func (a *App) View(c *ui.Context) {
	a.poll()
	if a.Store != nil && a.worker == nil {
		a.startWorker(c)
	}
	a.pal = a.light
	if c.Theme().Dark {
		a.pal = a.dark
	}
	c.SetTheme(a.pal.theme)
	if a.Store == nil && a.page == 2 {
		a.page = 0
	}
	if a.busy == "" {
		a.talking = false
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.pal.canvas).Children(func() {
		if a.Store != nil {
			a.sidebar(c)
			ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		}
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Children(func() {
			if a.page != 0 || a.Store == nil || !a.talking {
				a.banners(c)
			}
			switch {
			case a.page == 1:
				a.settings(c)
			case a.page == 2:
				a.categories(c)
			case a.Store == nil:
				a.welcome(c)
			default:
				a.discussion(c)
			}
		})
	})
	if a.sourceOpen && a.source != nil {
		a.extraction(c)
	}
}

// sessionParts splits a session title into its date and its words.
func (a *App) sessionParts(id string) (when, words string) {
	title := a.titles[id]
	if n := strings.Index(title, " · "); n >= 0 {
		return title[:n], title[n+len(" · "):]
	}
	return "", title
}

func (a *App) sidebar(c *ui.Context) {
	t := a.pal.theme
	w, _ := c.Size()
	slim := w < narrow
	width := float32(232)
	if slim {
		width = 60
	}
	busy := a.busy != ""
	ui.Column(c).Width(width).FillHeight().Background(a.pal.side).Padding(14, 10).Gap(2).Shrink(0).Children(func() {
		work := a.Store.Work()
		head := ui.ButtonBase(c).Row().FillWidth().AlignItems(ui.Center).Gap(10).Padding(6, 8).Radius(8).Label("切换作品").Tooltip("切换作品").Disabled(busy)
		if head.Hovered() {
			head.Background(t.SurfaceHover)
		}
		head.Children(func() {
			ui.Box(c).Size(26, 26).Radius(7).Background(a.pal.ink).Center().Shrink(0).Children(func() {
				ui.Text(c, string([]rune(work.Name + " ")[:1])).FontSize(13).Bold().TextColor(a.pal.onInk)
			})
			if !slim {
				ui.Column(c).Grow(1).Shrink(1).Gap(1).Children(func() {
					ui.Text(c, work.Name).FontSize(13.5).FontWeight(600).SingleLine().Ellipsis("…")
					ui.Text(c, work.Genre+" · 切换作品").FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				})
			}
		})
		if head.Clicked() {
			a.closeWork()
			return
		}
		ui.Box(c).Height(10)
		a.navItem(c, icChat, "讨论", 0, slim)
		a.navItem(c, icTag, "作品分类", 2, slim)
		a.navItem(c, icGear, "模型设置", 1, slim)
		ui.Box(c).Height(16)
		if slim {
			if iconButton(c, icPlus, "新对话", ui.Color{}).Disabled(busy).Clicked() {
				a.newSession()
			}
			return
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 4, 0, 8).Children(func() {
			ui.Text(c, "总讨论").FontSize(11.5).FontWeight(600).TextColor(a.pal.faint).Grow(1)
			if iconButton(c, icPlus, "新对话", ui.Color{}).Disabled(busy).Clicked() {
				a.newSession()
			}
		})
		ui.Scroll(c).Key("sessions").FillWidth().Grow(1).Gap(1).Children(func() {
			for n := len(a.sessions) - 1; n >= 0; n-- {
				a.sessionItem(c, a.sessions[n])
			}
		})
	})
}

func (a *App) navItem(c *ui.Context, s *ui.SVG, label string, page int, slim bool) {
	t := a.pal.theme
	on := a.page == page
	b := ui.ButtonBase(c).Row().FillWidth().AlignItems(ui.Center).Gap(10).Padding(7, 10).Radius(7).Label(label).Disabled(a.busy != "")
	if slim {
		b.Tooltip(label).Justify(ui.Center)
	} else {
		b.Justify(ui.Start)
	}
	fg := t.TextMuted
	switch {
	case on:
		b.Background(t.SurfacePressed)
		fg = t.Text
	case b.Hovered():
		b.Background(t.SurfaceHover)
		fg = t.Text
	}
	b.Children(func() {
		glyph(c, s, 16, fg)
		if !slim {
			ui.Text(c, label).FontSize(13.5).TextColor(fg).FontWeight(map[bool]int{true: 600, false: 400}[on])
		}
	})
	if b.Clicked() {
		a.page = page
	}
}

func (a *App) sessionItem(c *ui.Context, id string) {
	t := a.pal.theme
	when, words := a.sessionParts(id)
	on := id == a.session && a.page == 0
	b := ui.ButtonBase(c).Key("s-"+id).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(6, 10).Radius(7).Label("对话-" + id).Disabled(a.busy != "")
	if on {
		b.Background(t.SurfacePressed)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		fg := t.TextMuted
		if on {
			fg = t.Text
		}
		ui.Text(c, words).FontSize(13).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		ui.Text(c, when).FontSize(11).TextColor(a.pal.faint).SingleLine()
	})
	if b.Clicked() {
		a.page = 0
		a.switchSession(id)
	}
}

func (a *App) newSession() {
	session, err := a.Store.NewSession()
	if err != nil {
		a.setError(err)
		return
	}
	a.page = 0
	a.switchSession(session)
}

// banners float a slim line for an error, a running request and a quiet
// confirmation. A request can always be cancelled from here.
func (a *App) banners(c *ui.Context) {
	t := a.pal.theme
	bar := func(s *ui.SVG, color ui.Color, text string, fn func()) {
		ui.Row(c).FillWidth().Padding(10, 20, 0, 20).Children(func() {
			ui.Row(c).FillWidth().Padding(7, 8, 7, 12).Gap(10).Radius(9).AlignItems(ui.Center).Background(color.Alpha(0.08)).Border(1, color.Alpha(0.18)).Children(func() {
				if s != nil {
					glyph(c, s, 15, color)
				} else {
					ui.Spinner(c)
				}
				ui.Text(c, text).FontSize(13).TextColor(color.Mix(t.Text, 0.35)).Selectable().Grow(1).Shrink(1)
				fn()
			})
		})
	}
	if a.Error != "" {
		bar(icAlert, t.Danger, a.Error, func() {
			if iconButton(c, icX, "关闭提示", t.Danger).Clicked() {
				a.Error = ""
			}
		})
	}
	if a.busy != "" {
		bar(nil, t.TextMuted, a.busy, func() {
			if ghost(c, "取消请求", ui.Color{}).Clicked() && a.cancel != nil {
				a.cancel()
			}
		})
	} else if a.notice != "" {
		bar(icCheck, t.Success, a.notice, func() {
			if iconButton(c, icX, "知道了", t.Success).Clicked() {
				a.notice = ""
			}
		})
	}
}
