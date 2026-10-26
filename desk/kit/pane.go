package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// PaneRows is the list under the top of a tab, which scrolls.
func (p *Palette) PaneRows(c *ui.Context, key string, rows func()) {
	p.Hairline(c)
	ui.Scroll(c.Key(key)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 10, 12, 10).Children(rows)
	})
}

// PaneGroup is the small label over the rows of one kind.
func (p *Palette) PaneGroup(c *ui.Context, name string) {
	ui.Text(c, name).FontSize(FsSmall).TextColor(p.Faint).Padding(14, 10, 4, 10).SingleLine()
}

// PaneEmpty says in a list that it has nothing, and what to do about it.
func (p *Palette) PaneEmpty(c *ui.Context, words, how string) {
	ui.Column(c).FillWidth().Gap(6).Padding(20, 10).Children(func() {
		ui.Text(c, words).FontSize(FsBody)
		if how != "" {
			ui.Text(c, how).FontSize(FsNote).LineHeight(LhBody).TextColor(p.Theme.TextMuted).FillWidth()
		}
	})
}

// PaneRow is one thing of a list on one line: its name, a line of what it
// says, what stands at its end, and a dot when it waits on the author. The
// words of find in what it says are darker.
// Give it a key with the context: p.PaneRow(c.Key(id), ...).
func (p *Palette) PaneRow(c *ui.Context, label, name, gist, side, find string, waiting bool) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(HControl).Gap(10).Padding(0, 10).AlignItems(ui.Center).Radius(RControl).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	PressRow(c, b)
	b.Children(func() {
		title := ui.Text(c, name).FontSize(FsBody).SingleLine().Ellipsis("…")
		if gist == "" {
			title.Grow(1).Shrink(1)
		} else {
			title.Shrink(0)
			ui.RichText(c, Marked(gist, find, t.Text)...).FontSize(FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		}
		if side != "" {
			ui.Text(c, side).FontSize(FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1) // a long title is cut, not the name
		}
		if waiting {
			Dot(c, p.Amber, 6)
		}
		Glyph(c, IcChevron, IconSm, p.Faint)
	})
	return b
}

// Marked is text in spans, the words of find in it in color and bold.
func Marked(text, find string, color ui.Color) []ui.Span {
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

// PaneTodo is the first row of a list: what waits on the author, and how
// many.
func (p *Palette) PaneTodo(c *ui.Context, label string, n int) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(HControl).Gap(8).Padding(0, 10).Margin(8, 0, 0, 0).AlignItems(ui.Center).Radius(RControl).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	PressRow(c, b)
	b.Children(func() {
		Dot(c, p.Amber, 6)
		ui.Text(c, label).FontSize(FsBody).SingleLine().Grow(1).Shrink(1)
		ui.Textf(c, "%d", n).FontSize(FsNote).TextColor(t.TextMuted)
		Glyph(c, IcChevron, IconSm, p.Faint)
	})
	return b
}

// PaneBack is the top of a page: the way back to where it was opened
// from, and at its end the thing done to what the page shows, when act
// names one.
func (p *Palette) PaneBack(c *ui.Context, text, act string) (back, acted bool) {
	t := p.Theme
	ui.Row(c).FillWidth().Height(HControl).Gap(8).Padding(0, 20, 8, 20).AlignItems(ui.Center).Shrink(0).Children(func() {
		link := func(label string, lead bool) bool {
			b := ui.ButtonBase(c).Row().Gap(4).AlignItems(ui.Center).Shrink(1).Label(label)
			fg := t.TextMuted
			if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() {
				if lead {
					Glyph(c, IcBack, IconSm, fg)
				}
				ui.Text(c, label).FontSize(FsNote).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
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

// PanePage is the page of one thing under its top, which scrolls.
func (p *Palette) PanePage(c *ui.Context, key string, parts func()) {
	p.Hairline(c)
	ui.Scroll(c.Key(key)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Padding(16, 20, 20, 20).Children(parts)
	})
}

// PaneTitle is the name of what a page shows, a few faint words after it,
// and the button at its end when button names one.
func (p *Palette) PaneTitle(c *ui.Context, name, sub, button string) (clicked bool) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, name).FontSize(FsGroup).FontWeight(600).Shrink(1)
		if sub != "" {
			ui.Text(c, sub).FontSize(FsNote).TextColor(p.Faint).SingleLine().Ellipsis("…").Shrink(1)
		}
		ui.Spacer(c)
		if button != "" {
			clicked = p.PaneButton(c, button, false).Clicked()
		}
	})
	return clicked
}

// PaneButton is a button of a page: solid ink for the one thing the page
// is for, outlined otherwise.
func (p *Palette) PaneButton(c *ui.Context, text string, primary bool) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().Center().Height(HControlSm).Padding(0, 12).Radius(RControl).Shrink(0).Label(text)
	fg := t.Text
	if primary {
		bg := p.Ink
		if b.Hovered() {
			bg = t.AccentHover
		}
		b.Padding(0, 14).Background(bg)
		fg = p.OnInk
	} else {
		b.Background(p.Canvas).Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
	}
	if b.IsDisabled() {
		Pale(b)
	}
	if primary {
		Squeeze(b)
	} else {
		Press(c, b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsControl).TextColor(fg).SingleLine() })
	return b
}

// PaneButtons puts a page's buttons at the right, where the rest of the
// desk has them.
func PaneButtons(c *ui.Context, buttons func()) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Spacer(c)
		buttons()
	})
}

// PaneNote is a line of small grey words in a page.
func (p *Palette) PaneNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(FsNote).LineHeight(LhBody).TextColor(p.Theme.TextMuted).FillWidth()
}

// PaneWhere is a line that says where in the book something is, and the
// link that opens it there.
func (p *Palette) PaneWhere(c *ui.Context, text, label string) (clicked bool) {
	return p.PaneLink(c, text, "查看原文", label)
}

// PaneLink is a line that says where something is, and the link see that
// opens it; label names the link, and empty leaves it out.
func (p *Palette) PaneLink(c *ui.Context, text, see, label string) (clicked bool) {
	t := p.Theme
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if label == "" {
			return
		}
		b := ui.ButtonBase(c).Row().Shrink(0).Label(label)
		fg := t.Text
		if b.Hovered() {
			fg = t.AccentHover
		}
		b.Children(func() { ui.Text(c, see).FontSize(FsNote).TextColor(fg).SingleLine() })
		clicked = b.Clicked()
	})
	return clicked
}

// PaneQuote is a passage of the book quoted in a page, struck through
// when it is gone from the book.
func (p *Palette) PaneQuote(c *ui.Context, text string, gone bool) {
	q := ui.Text(c, text).FontSize(FsBody).LineHeight(LhLoose).FillWidth().Padding(10, 12).Radius(RControl).Background(p.Theme.SurfaceHover).Selectable()
	if gone {
		q.TextColor(p.Faint).Strikethrough()
	}
}

// PaneField is a field of a form under its label.
func (p *Palette) PaneField(c *ui.Context, label string, field func()) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(FsNote).TextColor(p.Theme.TextMuted)
		field()
	})
}

// Labeled stacks a muted caption over a control.
func Labeled(c *ui.Context, caption string, fn func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, caption).FontSize(FsNote).TextColor(c.Theme().TextMuted)
		fn()
	})
}

// PaneTop is the top row of the right pane.
func (p *Palette) PaneTop(c *ui.Context) ui.Element {
	return ui.Row(c).FillWidth().Height(TitleHeight).Padding(0, 16, 0, 10).Gap(2).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).Label("右栏标签行")
}
