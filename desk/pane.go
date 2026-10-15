package desk

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// The right column's tabs share one shape: a list, a row to a thing, and
// the page of the thing a row was clicked for. These are their parts.

// paneFind is the top of a list: a box that only searches, the button that
// adds beside it when add names one, and what is rarely looked at under
// the dots when more lists it.
func (a *App) paneFind(c *ui.Context, query *string, hint, add string, more func(m *menu)) (added bool) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Gap(8).Padding(0, 20, 12, 20).AlignItems(ui.Center).Shrink(0).Children(func() {
		box := ui.Row(c).Grow(1).Shrink(1).Height(hControl).Gap(8).Padding(0, 10).AlignItems(ui.Center).Radius(rControl).Background(a.pal.canvas).Border(1, t.Border)
		if box.FocusWithin() {
			box.Border(1, a.pal.ink)
		}
		box.Children(func() {
			glyph(c, icSearch, iconSm, a.pal.faint)
			ui.TextInputBase(c, query).Label(hint).Placeholder(hint).Grow(1).Shrink(1).FontSize(fsBody)
		})
		if add != "" {
			b := ui.ButtonBase(c).Row().Height(hControl).Gap(6).Padding(0, 12).AlignItems(ui.Center).Radius(rControl).Background(a.pal.canvas).Border(1, t.Border).Shrink(0).Label(add)
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			if b.IsDisabled() {
				pale(b)
			}
			b.Children(func() {
				glyph(c, icPlus, iconSm, t.Text)
				ui.Text(c, add).FontSize(fsControl).SingleLine()
			})
			added = b.Clicked()
		}
		if more != nil {
			b := ui.ButtonBase(c).Size(hControl, hControl).Center().Radius(rControl).Shrink(0).Label("更多").Tooltip("更多")
			if b.Hovered() || a.menuOpen(hint) {
				b.Background(t.SurfacePressed)
			}
			b.Children(func() { glyph(c, icMore, iconMd, t.TextMuted) })
			a.dropMenu(c, b, hint, func(panel ui.Element) {
				panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0).Width(160)
			}, more)
		}
	})
	return added
}

// count is how many a menu's row leads to, at its right, when any.
func count(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// paneRows is the list under the top of a tab, which scrolls.
func (a *App) paneRows(c *ui.Context, key string, rows func()) {
	a.pal.hairline(c)
	ui.Scroll(c.Key(key)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 10, 12, 10).Children(rows)
	})
}

// paneGroup is the small label over the rows of one kind.
func (a *App) paneGroup(c *ui.Context, name string) {
	ui.Text(c, name).FontSize(fsSmall).TextColor(a.pal.faint).Padding(14, 10, 4, 10).SingleLine()
}

// paneEmpty says in a list that it has nothing, and what to do about it.
func (a *App) paneEmpty(c *ui.Context, words, how string) {
	ui.Column(c).FillWidth().Gap(6).Padding(20, 10).Children(func() {
		ui.Text(c, words).FontSize(fsBody)
		if how != "" {
			ui.Text(c, how).FontSize(fsNote).LineHeight(lhBody).TextColor(a.pal.theme.TextMuted).FillWidth()
		}
	})
}

// paneRow is one thing of a list on one line: its name, a line of what it
// says, what stands at its end, and a dot when it waits on the author. The
// words of find in what it says are darker.
// Give it a key with the context: a.paneRow(c.Key(id), ...).
func (a *App) paneRow(c *ui.Context, label, name, gist, side, find string, waiting bool) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(hControl).Gap(10).Padding(0, 10).AlignItems(ui.Center).Radius(rControl).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		title := ui.Text(c, name).FontSize(fsBody).SingleLine().Ellipsis("…")
		if gist == "" {
			title.Grow(1).Shrink(1)
		} else {
			title.Shrink(0)
			ui.RichText(c, marked(gist, find, t.Text)...).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		}
		if side != "" {
			ui.Text(c, side).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1) // a long title is cut, not the name
		}
		if waiting {
			dot(c, a.pal.amber, 6)
		}
		glyph(c, icChevron, iconSm, a.pal.faint)
	})
	return b
}

// marked is text in spans, the words of find in it in color and bold.
func marked(text, find string, color ui.Color) []ui.Span {
	if find == "" {
		return []ui.Span{{Text: text}}
	}
	var out []ui.Span
	for {
		i := strings.Index(text, find)
		if i < 0 {
			break
		}
		if i > 0 {
			out = append(out, ui.Span{Text: text[:i]})
		}
		out = append(out, ui.Span{Text: find, Weight: 600, Color: color})
		text = text[i+len(find):]
	}
	if text != "" {
		out = append(out, ui.Span{Text: text})
	}
	return out
}

// nearFind is text from a little before find in it, so that a line cut
// short still shows what was found.
func nearFind(text, find string) string {
	text = strings.Join(strings.Fields(text), " ")
	i := strings.Index(text, find)
	if find == "" || i < 0 {
		return text
	}
	before := []rune(text[:i])
	if len(before) <= 8 {
		return text
	}
	return "…" + string(before[len(before)-6:]) + text[i:]
}

// paneTodo is the first row of a list: what waits on the author, and how
// many.
func (a *App) paneTodo(c *ui.Context, label string, n int) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(hControl).Gap(8).Padding(0, 10).Margin(8, 0, 0, 0).AlignItems(ui.Center).Radius(rControl).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		dot(c, a.pal.amber, 6)
		ui.Text(c, label).FontSize(fsBody).SingleLine().Grow(1).Shrink(1)
		ui.Textf(c, "%d", n).FontSize(fsNote).TextColor(t.TextMuted)
		glyph(c, icChevron, iconSm, a.pal.faint)
	})
	return b
}

// paneBack is the top of a page: the way back to where it was opened
// from, and at its end the thing done to what the page shows, when act
// names one.
func (a *App) paneBack(c *ui.Context, text, act string) (back, acted bool) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Height(hControl).Gap(8).Padding(0, 20, 8, 20).AlignItems(ui.Center).Shrink(0).Children(func() {
		link := func(label string, lead bool) bool {
			b := ui.ButtonBase(c).Row().Gap(4).AlignItems(ui.Center).Shrink(1).Label(label)
			fg := t.TextMuted
			if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() {
				if lead {
					glyph(c, icBack, iconSm, fg)
				}
				ui.Text(c, label).FontSize(fsNote).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
			})
			return b.Clicked()
		}
		back = link(text, true)
		ui.Spacer(c)
		if act != "" {
			acted = link(act, false)
		}
	})
	return back, acted
}

// panePage is the page of one thing under its top, which scrolls.
func (a *App) panePage(c *ui.Context, key string, parts func()) {
	a.pal.hairline(c)
	ui.Scroll(c.Key(key)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Padding(16, 20, 20, 20).Children(parts)
	})
}

// paneTitle is the name of what a page shows, a few faint words after it,
// and the button at its end when button names one.
func (a *App) paneTitle(c *ui.Context, name, sub, button string) (clicked bool) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, name).FontSize(fsGroup).FontWeight(600).Shrink(1)
		if sub != "" {
			ui.Text(c, sub).FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Ellipsis("…").Shrink(1)
		}
		ui.Spacer(c)
		if button != "" {
			clicked = a.paneButton(c, button, false).Clicked()
		}
	})
	return clicked
}

// paneButton is a button of a page: solid ink for the one thing the page
// is for, outlined otherwise.
func (a *App) paneButton(c *ui.Context, text string, primary bool) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Center().Height(hControlSm).Padding(0, 12).Radius(rControl).Shrink(0).Label(text)
	fg := t.Text
	if primary {
		bg := a.pal.ink
		if b.Hovered() {
			bg = t.AccentHover
		}
		b.Padding(0, 14).Background(bg)
		fg = a.pal.onInk
	} else {
		b.Background(a.pal.canvas).Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
	}
	if b.IsDisabled() {
		pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(fsControl).TextColor(fg).SingleLine() })
	return b
}

// paneButtons puts a page's buttons at the right, where the rest of the
// desk has them.
func paneButtons(c *ui.Context, buttons func()) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Spacer(c)
		buttons()
	})
}

// paneNote is a line of small grey words in a page.
func (a *App) paneNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(fsNote).LineHeight(lhBody).TextColor(a.pal.theme.TextMuted).FillWidth()
}

// paneWhere is a line that says where in the book something is, and the
// link that opens it there.
func (a *App) paneWhere(c *ui.Context, text, label string) (clicked bool) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if label == "" {
			return
		}
		b := ui.ButtonBase(c).Row().Shrink(0).Label(label)
		fg := t.Text
		if b.Hovered() {
			fg = t.AccentHover
		}
		b.Children(func() { ui.Text(c, "查看原文").FontSize(fsNote).TextColor(fg).SingleLine() })
		clicked = b.Clicked()
	})
	return clicked
}

// paneQuote is a passage of the book quoted in a page, struck through
// when it is gone from the book.
func (a *App) paneQuote(c *ui.Context, text string, gone bool) {
	q := ui.Text(c, text).FontSize(fsBody).LineHeight(lhLoose).FillWidth().Padding(10, 12).Radius(rControl).Background(a.pal.theme.SurfaceHover).Selectable()
	if gone {
		q.TextColor(a.pal.faint).Strikethrough()
	}
}

// paneField is a field of a form under its label.
func (a *App) paneField(c *ui.Context, label string, field func()) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		field()
	})
}

// chapterTitle is the title of chapter node, if the book has it.
func (a *App) chapterTitle(node string) (string, bool) {
	title, ok := "", false
	a.write.tree.Walk(func(nd *workspace.Node, _ int) {
		if !nd.Folder && nd.ID == node {
			title, ok = nd.Title, true
		}
	})
	return title, ok
}
