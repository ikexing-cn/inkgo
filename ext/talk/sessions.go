package talk

import (
	"github.com/egoist/mygo/ui"
	"time"
	"writer/desk/kit"
	"writer/ext/talk/talktext"
)

// sideBody lists the chapters, or the talks on the discussion page.
// SideBody runs on the frame thread.
func (a *Panel) SideBody(c *ui.Context) {
	ui.Scroll(c.Key("sessions")).FillWidth().Grow(1).Gap(2).Children(func() {
		put := 0
		for n := len(a.Sessions) - 1; n >= 0; n-- {
			id := a.Sessions[n]
			if a.Blank[id] {
				continue
			}
			if a.Shelved[id] {
				put++
				if !a.ShowShelved && id != a.Session {
					continue
				}
			}
			a.SessionItem(c, id)
		}
		if put > 0 {
			label := talktext.SideFoldedCount(put)
			if a.ShowShelved {
				label = talktext.SideHideFolded()
			}
			if a.palette().Ghost(c, label, ui.Color{}).Label("收起的对话").Margin(6, 0, 0, 4).Clicked() {
				a.ShowShelved = !a.ShowShelved
			}
		}
	})
}

// talkShown tells whether the discussion is on screen, on its page or
// beside the prose: not the list of talks there, nor the prose alone.
// TalkShown runs on the frame thread.
func (a *Panel) TalkShown() bool {
	if a.Store == nil {
		return false
	}
	return a.desktop.Page() == "talk.page" || a.desktop.Writing() && a.talkTab() && !!a.pane.PaneOpen() && !a.Talks && !a.focused()
}

// SessionItem runs on the frame thread.
func (a *Panel) SessionItem(c *ui.Context, id string) {
	t := a.palette().Theme
	words, _ := a.TalkName(id)
	on := id == a.Session && !a.desktop.Writing()
	b := ui.ButtonBase(c.Key("s-"+id)).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(6, 10).Radius(kit.RControl).Label("对话-" + id).Disabled(a.held())
	if on {
		b.Background(t.SurfacePressed)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	kit.PressRow(c, b)
	b.Children(func() {
		fg := t.TextMuted
		if on {
			fg = t.Text
		}
		if a.Shelved[id] {
			fg = a.palette().Faint
		}
		ui.Text(c, words).FontSize(kit.FsControl).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if b.Hovered() || b.FocusWithin() || a.native.MenuOpen("talk-"+id) {
			ui.Text(c, talkDay(id, time.Now())).FontSize(kit.FsSmall).TextColor(a.palette().Faint).SingleLine()
		}
	})
	a.native.MenuState().ContextMenu(c, a.palette(), b, "talk-"+id, func(m *kit.Menu) {
		if a.Shelved[id] {
			m.Item(talktext.SideMoveBack()).Disabled(a.held()).Do(func() {
				delete(a.Shelved, id)
				a.savePrefs()
			})
		} else {
			m.Item(talktext.SideCollapseTalk()).Disabled(a.held()).Do(func() { a.Shelve(id) })
		}
	})
	if b.Clicked() {
		a.desktop.ShowPage("talk.page")
		a.SwitchSession(id)
	}
}

// NewSession runs on the frame thread.
func (a *Panel) NewSession() {
	session, err := a.BlankSession()
	if err != nil {
		a.SetError(err)
		return
	}
	a.desktop.ShowPage("talk.page")
	a.SwitchSession(session)
}

// blankSession is a talk to start: the one open or another nothing was said
// in yet, or else a new one, so that new talks left unused do not pile up.
// One put away stays away.
// BlankSession runs on the frame thread.
func (a *Panel) BlankSession() (string, error) {
	if a.Blank[a.Session] && !a.Shelved[a.Session] {
		return a.Session, nil
	}
	for _, id := range a.Sessions {
		if a.Blank[id] && !a.Shelved[id] {
			return id, nil
		}
	}
	return a.Store.NewSession()
}

// shelve puts a conversation out of the list, keeping its file: the
// setting cites what was said in it. Shelving the one open opens the
// latest left, or a new one.
// Shelve runs on the frame thread.
func (a *Panel) Shelve(id string) {
	if a.Shelved == nil {
		a.Shelved = map[string]bool{}
	}
	a.Shelved[id] = true
	a.savePrefs()
	if id != a.Session {
		return
	}
	for n := len(a.Sessions) - 1; n >= 0; n-- {
		if id := a.Sessions[n]; !a.Shelved[id] && !a.Blank[id] {
			a.SwitchSession(id)
			return
		}
	}
	a.NewSession()
}
