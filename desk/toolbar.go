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
		ui.Column(c).MinWidth(230).Children(func() {
			if chapter {
				busy := w.ed.ReadOnly() || w.lost || w.external != nil
				if a.menuRow(c, icSparkle, "续写一句", modKey+"J", busy).Clicked() {
					w.moreOpen = false
					a.continueText()
				}
				hint := ""
				if strings.TrimSpace(w.outline) == "" {
					hint = "请先填写章纲"
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
				w.exportOpen, w.exportDone, w.exportFrom, w.exportAt = true, false, "", 0
			}
			a.menuLine(c)
			a.themeChoice(c)
		})
	})
}

// menuRow is a row of a menu drawn in a popover: an icon, what it does,
// and on the right a shortcut, or why it cannot be done now.
func (a *App) menuRow(c *ui.Context, s *ui.SVG, text, hint string, off bool) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(hRow).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(rControl).Label(strings.TrimSuffix(text, "…")).Disabled(off)
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
	ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.pal.line).Shrink(0)
}

// themeChoice picks the appearance of the whole app on this machine:
// following the system, light or dark.
func (a *App) themeChoice(c *ui.Context) {
	ui.Text(c, "外观").FontSize(fsSmall).TextColor(a.pal.faint).Padding(6, 10, 2, 10)
	// As wide as its three words: the menu grows to hold it, it does not
	// squeeze into the menu.
	ui.Row(c).Padding(2, 6, 6, 6).Children(func() { a.themeSegments(c, false) })
}

// themeSegments are the three appearances side by side.
func (a *App) themeSegments(c *ui.Context, fill bool) {
	t := a.pal.theme
	bar := ui.Row(c).Gap(2).Padding(2).Radius(rControl).Background(t.SurfacePressed).Shrink(0)
	if fill {
		bar.FillWidth()
	}
	bar.Children(func() {
		for _, m := range []struct {
			v, n string
			s    *ui.SVG
		}{{"", "跟随系统", icMonitor}, {"light", "浅色", icSun}, {"dark", "深色", icMoon}} {
			on := a.shelf.Theme == m.v
			b := ui.ButtonBase(c).Row().Height(hSeg).Center().Gap(6).Padding(0, 10).Radius(rXs).Shrink(0).Label("外观-" + m.n)
			if fill {
				b.Grow(1)
			}
			fg, weight := t.TextMuted, 400
			if on {
				a.pal.raise.on(b.Background(a.pal.canvas))
				fg, weight = t.Text, 600
			} else if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() {
				glyph(c, m.s, iconSm, fg)
				ui.Text(c, m.n).FontSize(fsControl).FontWeight(weight).TextColor(fg).SingleLine()
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
		if a.selButton(c, icPen, "AI 改写").Tooltip("由 AI 改写选中内容").Clicked() {
			a.openRewrite()
		}
		if a.selButton(c, icBookmark, "加入设定").Tooltip("将选中的原文加入设定").Clicked() {
			a.openTake()
		}
	})
}

func (a *App) selButton(c *ui.Context, s *ui.SVG, text string) ui.Element {
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
