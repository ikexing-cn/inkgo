package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// moreButton is the toolbar's 「⋯」: continuing and drafting, the chapter's
// snapshots, export, and the appearance. chapter says a chapter is shown.
func (a *App) moreButton(c *ui.Context, chapter bool) {
	w := &a.write
	b := iconButton(c, icMore, "更多", ui.Color{}).Tooltip("续写、起草、快照、导出和外观")
	if b.Clicked() {
		w.moreOpen = !w.moreOpen
	}
	ui.Popover(c, b, &w.moreOpen, func() {
		ui.Column(c).Width(250).Gap(2).Children(func() {
			if chapter {
				busy := w.ed.ReadOnly() || w.lost || w.external != nil
				if a.menuRow(c, icSparkle, "续写一句", modKey+"J", busy).Clicked() {
					w.moreOpen = false
					a.continueText()
				}
				hint := ""
				if strings.TrimSpace(w.outline) == "" {
					hint = "先写章纲"
				}
				if a.menuRow(c, icDraft, "起草本章", hint, busy || hint != "").Clicked() {
					w.moreOpen = false
					a.openDraft()
				}
				a.menuLine(c)
				if a.menuRow(c, icHistory, "快照", "", busy).Clicked() {
					w.moreOpen = false
					a.openSnapshots()
				}
			}
			if a.menuRow(c, icDownload, "导出…", "", false).Clicked() {
				w.moreOpen = false
				w.exportOpen, w.exportDone, w.exportFrom = true, false, ""
			}
			a.menuLine(c)
			a.themeChoice(c)
		})
	})
}

// menuRow is a row of a menu drawn in a popover: an icon, what it does,
// and on the right a shortcut, or why it cannot be done now.
func (a *App) menuRow(c *ui.Context, s *ui.SVG, text, hint string, off bool) *ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(hControl).Padding(0, 10).Gap(10).AlignItems(ui.Center).Radius(rControl).Label(strings.TrimSuffix(text, "…")).Disabled(off)
	fg := t.AccentHover
	if off {
		fg = a.pal.faint
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, s, iconMd, fg)
		ui.Text(c, text).FontSize(fsControl).TextColor(fg).Grow(1).SingleLine()
		if hint != "" {
			ui.Text(c, hint).FontSize(fsSmall).TextColor(a.pal.faint).SingleLine()
		}
	})
	return b
}

func (a *App) menuLine(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Margin(6, 4).Background(a.pal.line).Shrink(0)
}

// themeChoice picks the appearance of the whole app on this machine:
// following the system, light or dark.
func (a *App) themeChoice(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(6).Padding(6, 10, 8, 10).Children(func() {
		ui.Text(c, "外观").FontSize(fsNote).TextColor(a.pal.faint)
		a.themeSegments(c)
	})
}

// themeSegments are the three appearances side by side.
func (a *App) themeSegments(c *ui.Context) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Gap(2).Padding(2).Radius(rControl).Background(t.SurfaceHover).Children(func() {
		for _, m := range []struct {
			v, n string
			s    *ui.SVG
		}{{"", "跟随系统", icMonitor}, {"light", "浅色", icSun}, {"dark", "深色", icMoon}} {
			on := a.shelf.Theme == m.v
			b := ui.ButtonBase(c).Row().Grow(1).Height(hSeg).Center().Gap(6).Radius(rControl).Label("外观-" + m.n)
			fg := t.TextMuted
			if on {
				a.pal.raise.on(b.Background(a.pal.raised))
				fg = t.Text
			} else if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() {
				glyph(c, m.s, iconSm, fg)
				ui.Text(c, m.n).FontSize(fsNote).TextColor(fg).SingleLine()
			})
			if b.Clicked() {
				a.setTheme(m.v)
			}
		}
	})
}

// setTheme keeps the appearance chosen in the shelf, which is this
// machine's, and applies it.
func (a *App) setTheme(v string) {
	if a.shelf.Theme == v {
		return
	}
	a.shelf.Theme = v
	a.saveShelf()
	if a.Appearance != nil {
		a.Appearance(v)
	}
}

// Theme is the appearance chosen on this machine: "light", "dark", or
// empty to follow the system.
func (a *App) Theme() string { return a.shelf.Theme }

// selectionBar floats over the start of the selection with what acts on
// it: the AI's rewrite, and taking the words into the settings.
func (a *App) selectionBar(c *ui.Context) {
	w := &a.write
	r, ok := w.ed.SelectionTop()
	if !ok || w.ed.ReadOnly() || w.lost || w.external != nil || w.rewriting || w.run != nil || w.proposed != nil ||
		w.takeOpen || w.ed.Predicting() || w.fore.noting || w.fore.paying {
		return
	}
	t := a.pal.theme
	const height, width = 34, 190
	top := r.Y - height - 6
	if top < 4 {
		top = r.Y + r.H + 6 // under the line when there is no room above
	}
	left := max(8, min(r.X-12, w.ed.Width()-width-8))
	bar := ui.Row(c).Absolute().Left(left).Top(top).Height(height).Padding(4).Gap(2).AlignItems(ui.Center).Radius(rControl).
		Background(a.pal.raised).Border(1, t.Border)
	a.pal.pop.on(bar)
	bar.Children(func() {
		if a.selButton(c, icPen, "AI 改写").Tooltip("让 AI 改写选中的部分").Clicked() {
			a.openRewrite()
		}
		if a.selButton(c, icBookmark, "加入设定").Tooltip("把选中的原文加入设定").Clicked() {
			a.openTake()
		}
	})
}

func (a *App) selButton(c *ui.Context, s *ui.SVG, text string) *ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Height(hSeg).Padding(0, 8).Gap(6).AlignItems(ui.Center).Radius(rControl).Label(text)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, s, iconSm, t.AccentHover)
		ui.Text(c, text).FontSize(fsNote).TextColor(t.AccentHover).SingleLine()
	})
	return b
}
