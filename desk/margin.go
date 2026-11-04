package desk

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"writer/ext/lore/loreapi"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

type noteAction struct {
	label string
	run   func()
}

func (a *App) checkActions(it checkItem) []noteAction {
	leave := noteAction{locales.CheckNoChange(), func() { a.setMeant(it) }}
	prose := noteAction{locales.CheckEditText(), func() { a.findInChapter(it.node, it.f.Quote) }}
	switch it.f.Kind {
	case workspace.FindNew:
		leave.label = locales.CheckIgnore()
		return []noteAction{leave, {locales.CheckAddToSettings(), func() { a.takeFound(it) }}}
	case workspace.FindConflict:
		if !a.entryNavigationAvailable() {
			return []noteAction{leave, prose}
		}
		return []noteAction{leave, {locales.CheckEditSetting(), func() {
			a.findInChapter(it.node, it.f.Quote)
			a.entryNavigation(func(n loreapi.EntryNavigation) { n.Edit(it.entry) })
			a.savePrefs()
		}}, prose}
	}
	return []noteAction{leave, prose}
}

type marginNote struct {
	idea                  *workspace.Idea
	id, kind, title, text string
	start, end            int
	warn                  bool
	check                 *checkItem
	fore                  string
	open                  func()
	actions               []noteAction
}

type marginKey struct {
	doubtKey
	changes uint64
}

type marginState struct {
	notes    cached[marginKey, []marginNote]
	boxes    map[string]ui.Rect
	top      float32
	lit      string
	visible  bool
	chapter  string
	runs     map[string]uint64
	serial   uint64
	previous []marginNote
	action   bool
	departed []marginDeparture
	shifts   map[string]float32
	shifting bool
}

type marginDeparture struct {
	note marginNote
	top  float32
}

// marginNotes resolves quotes only when the document or the book changes.
// Positions on each frame then cost one lookup per note, not per character.
func (a *App) marginNotes() []marginNote {
	m := &a.write.margin
	return m.notes.get(marginKey{doubtKey{a.checkKey(), a.checksOn()}, a.extensions.Changes()}, func() []marginNote {
		ed := a.write.cur().ed
		if ed == nil {
			return nil
		}
		text, _ := a.liveText()
		var out []marginNote
		span := func(quote string) (int, int, bool) {
			at := strings.Index(text, quote)
			if quote == "" || at < 0 {
				return 0, 0, false
			}
			start := utf8.RuneCountInString(text[:at])
			return start, start + utf8.RuneCountInString(quote), true
		}
		if a.checksOn() {
			for _, it := range a.checkItems(false) {
				if !it.here {
					continue
				}
				start, end, ok := span(it.f.Quote)
				if !ok {
					continue
				}
				n := marginNote{id: "check-" + it.f.ID, kind: kindLabel(it.f.Kind), title: it.f.Title, start: start, end: end, warn: true, check: &it}
				switch it.f.Kind {
				case workspace.FindConflict:
					n.kind, n.text = locales.NotesKindInconsistent(), locales.NotesSays(it.f.Says)
				case workspace.FindNew:
					n.text = it.f.Text
				}
				out = append(out, n)
			}
		}
		for _, f := range a.foreshadows() {
			if f.Status() != workspace.ForeOpen && f.Status() != workspace.ForeDone {
				continue
			}
			if f.Here && f.End > f.Start {
				kind := locales.NotesKindHook()
				if f.Status() == workspace.ForeDone {
					kind = locales.NotesKindHookPaid()
				}
				out = append(out, marginNote{id: "fore-" + f.ID, kind: kind, title: foreName(f), start: f.Start, end: f.End, fore: f.ID})
			}
			for _, p := range f.Payoffs {
				if p.Here && p.End > p.Start {
					out = append(out, marginNote{id: "pay-" + p.ID, kind: locales.NotesKindPayoff(), title: foreName(f), text: locales.NotesPlanted(f.Title), start: p.Start, end: p.End, fore: f.ID})
				}
			}
		}
		for _, item := range ext.Items(a.extensions, hostapi.MarginSources) {
			var notes []hostapi.MarginItem
			if ext.Guard(a.extensions, item.Owner, item.ID, func() { notes = item.Value.Notes(a.write.node, text) }) != nil {
				continue
			}
			for _, v := range notes {
				v := v
				owner := item.Owner
				n := marginNote{idea: v.Idea, id: v.ID, kind: v.Kind, title: v.Title, text: v.Text, start: v.Start, end: v.End, warn: v.Warn}
				if v.Open != nil {
					n.open = func() { _ = ext.Guard(a.extensions, owner, v.ID, v.Open) }
				}
				for _, action := range v.Actions {
					action := action
					n.actions = append(n.actions, noteAction{action.Label, func() { _ = ext.Guard(a.extensions, owner, v.ID, action.Run) }})
				}
				out = append(out, n)
			}
		}
		slices.SortStableFunc(out, func(x, y marginNote) int { return cmp.Compare(x.start, y.start) })
		return out
	})
}

func (a *App) openMarginNote(n marginNote) {
	switch {
	case n.check != nil:
		a.ck.tab = 0
		a.openCheck(n.check.f.ID)
	case n.fore != "":
		a.showFore(n.fore)
	case n.open != nil:
		n.open()

	}
}

func (a *App) marginPanel(c *ui.Context) {
	a.pal.PaneTop(c).Children(func() { a.panelTabs(c) })
	m := &a.write.margin
	list := a.marginNotes()
	fresh := !m.visible || m.chapter != a.write.node
	m.visible, m.chapter = true, a.write.node
	if fresh {
		m.previous, m.departed, m.shifts, m.runs = nil, nil, nil, nil
		m.shifting, m.action = false, false
	}
	if m.runs == nil {
		m.runs = map[string]uint64{}
	}
	for _, n := range list {
		if !slices.ContainsFunc(m.previous, func(old marginNote) bool { return old.id == n.id }) {
			m.serial++
			m.runs[n.id] = m.serial
		}
		if a.fold.paneOpened {
			m.runs[n.id] = 0 // the pane brings these notes in together
		}
	}
	boxes := make(map[string]ui.Rect, len(list))
	ed := a.write.cur().ed
	services := c.Services()
	box := ui.Box(c.Key("margin")).FillWidth().Grow(1).Shrink(1).Clip().Label("页边批注")
	m.top = box.Bounds().Y
	starting := false
	if m.action {
		for _, n := range m.previous {
			if !slices.ContainsFunc(list, func(now marginNote) bool { return now.id == n.id }) && !c.Preferences().ReduceMotion {
				m.departed = append(m.departed, marginDeparture{n, m.boxes[n.id].Y - m.top})
				starting = true
			}
		}
		m.action = false
	}
	if starting {
		m.shifts = map[string]float32{}
		m.shifting = true
		box.Jump("remove", 0)
	}
	progress := float32(1)
	if m.shifting {
		progress = box.AnimateWith("remove", 1, kit.MoveTime, ui.Linear)
	}
	if progress == 1 {
		m.departed, m.shifts, m.shifting = nil, nil, false
	}
	// The notes below move up as one arriving; the note dealt with leaves.
	still, gone := 1-kit.Out(progress), kit.In(progress)
	m.departed = slices.DeleteFunc(m.departed, func(gone marginDeparture) bool {
		return slices.ContainsFunc(list, func(n marginNote) bool { return n.id == gone.note.id })
	})
	drawing := slices.Clone(list)
	for _, gone := range m.departed {
		drawing = append(drawing, gone.note)
	}
	floor := float32(math.Inf(-1))
	var rows []ui.Element
	box.Draw(func(_ *ui.Painter, r ui.Rect) { m.top = r.Y })
	m.lit = ""
	box.Children(func() {
		if len(drawing) == 0 {
			text := locales.NotesNone()
			if ed == nil {
				text = locales.NotesOpenChapter()
			}
			ui.Text(c, text).FontSize(kit.FsNote).TextColor(a.pal.Faint).Padding(24, 20)
			return
		}
		ha, hb := ed.HoveredLine()
		for i := range drawing {
			n := drawing[i]
			leaving := i >= len(list)
			rangeBox, _ := ed.RangeBoxUnclipped(n.start, n.end)
			y := max(rangeBox.Y+a.LastProse.Y-m.top-3, floor)
			if leaving {
				y = m.departed[i-len(list)].top
			} else {
				floor = y + m.boxes[n.id].H + 8
				if starting {
					m.shifts[n.id] = m.boxes[n.id].Y - m.top - y
				}
				y += m.shifts[n.id] * still
			}
			row := ui.ButtonBase(c.Key(n.id)).Column().Absolute().Left(0).Right(16).Top(y).Gap(2).Padding(6, 10, 8, 20).Radius(kit.RControl).Label("批注-" + n.id)
			if leaving {
				row.Disabled(true).Translate(kit.NoteExit*gone, 0).Opacity(1 - gone)
			} else if m.runs[n.id] != 0 {
				p := kit.Arrive(row, m.runs[n.id], kit.NoteTime, time.Duration(min(i, 3))*kit.NoteDelay)
				row.Translate(-kit.NoteTravel*(1-p), 0).Opacity(p)
			}
			lit := hb > ha && ha < n.end && n.start < hb && m.lit == ""
			bg := a.pal.Theme.SurfaceHover.Alpha(0)
			if !leaving && (lit || row.Hovered()) {
				m.lit = n.id
				bg = a.pal.Theme.SurfaceHover
				ed.SetNoteHighlight(n.start, n.end)
			}
			row.Background(row.AnimateColor("hover", bg, ui.Animation{Duration: kit.HoverTime, Ease: kit.Out}))
			rows = append(rows, row)
			acted := false
			row.Children(func() {
				ui.Box(c).Absolute().Left(0).Top(16).Width(14).Height(1).Background(a.pal.Faint)
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					if n.warn {
						kit.Dot(c, a.pal.Amber, 5)
					}
					ui.Text(c, n.kind).FontSize(kit.FsSmall).TextColor(a.pal.Theme.TextMuted).LineHeight(kit.LhBody)
				})
				ui.Text(c, n.title).FontSize(kit.FsControl).FontWeight(600).TextColor(a.pal.Ink).LineHeight(kit.LhBody).FillWidth()
				if n.text != "" {
					ui.Text(c, n.text).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).LineHeight(kit.LhBody).FillWidth()
				}
				var acts []noteAction
				if n.check != nil {
					acts = a.checkActions(*n.check)
				} else {
					acts = n.actions
				}
				if len(acts) > 0 {
					ui.Row(c).FillWidth().Gap(16).Margin(6, 0, 0, 0).Justify(ui.End).Children(func() {
						for j, act := range acts {
							fg := a.pal.Theme.TextMuted
							if j == len(acts)-1 {
								fg = a.pal.Ink
							}
							button := a.pal.Ghost(c, act.label, fg).Label("批注-" + n.id + "-" + act.label)
							if button.Clicked() {
								acted = true
								m.action = true
								act.run()
							}
						}
					})
				}
			})
			if !leaving && row.Clicked() && !acted {
				a.openMarginNote(n)
			}
		}
	})
	box.DrawOver(func(_ *ui.Painter, r ui.Rect) {
		floor := float32(math.Inf(-1))
		for i, row := range rows {
			if i >= len(list) {
				break
			}
			n := list[i]
			actual := row.Bounds()
			boxes[n.id] = actual
			anchor, _ := ed.RangeBoxUnclipped(n.start, n.end)
			top := max(anchor.Y+a.LastProse.Y-3, floor)
			floor = top + actual.H + 8
			if math.Abs(float64(actual.Y-top-m.shifts[n.id]*still)) > 0.5 {
				services.Invalidate()
			}
		}
		m.boxes = boxes
	})
	m.previous = slices.Clone(list)
}
