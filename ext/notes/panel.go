package notes

import (
	"cmp"
	"math"
	"slices"
	"time"
	"writer/ext/notes/notestext"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/workspace"
)

type noteAction struct {
	label string
	run   func()
}
type marginNote struct {
	idea                  *workspace.Idea
	id, kind, title, text string
	start, end            int
	warn                  bool
	open                  func()
	actions               []noteAction
}

type marginKey struct {
	book              *book.Book
	version, doc      int
	ed                *editor.Editor
	chapter, language string
	changes           uint64
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
func (a *Panel) marginNotes() []marginNote {
	m := &a.margin
	chapter, ed, _ := a.services.Book.Chapter()
	if ed == nil {
		return nil
	}
	key := marginKey{a.services.Book.Content(), a.services.Book.Content().Version(), ed.Doc.Version(), ed, chapter, notestext.NotesNone(), a.reader.Get().Revision()}
	return m.notes.get(key, func() []marginNote {
		var out []marginNote
		for _, v := range a.reader.Get().Items(chapter, ed.Doc.String()) {
			if v.Kind == "" {
				continue
			}
			n := marginNote{idea: v.Idea, id: v.ID, kind: v.Kind, title: v.Title, text: v.Text, start: v.Start, end: v.End, warn: v.Warn, open: v.Open}
			for _, act := range v.Actions {
				n.actions = append(n.actions, noteAction{act.Label, act.Run})
			}
			out = append(out, n)
		}
		slices.SortStableFunc(out, func(x, y marginNote) int { return cmp.Compare(x.start, y.start) })
		return out
	})
}

func (a *Panel) openMarginNote(n marginNote) {
	if n.open != nil {
		n.open()
	}
}

func (a *Panel) marginPanel(c *ui.Context) {
	m := &a.margin
	list := a.marginNotes()
	fresh := !m.visible || m.chapter != a.chapter()
	m.visible, m.chapter = true, a.chapter()
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
		if a.services.Prose.PaneOpened() {
			m.runs[n.id] = 0 // the pane brings these notes in together
		}
	}
	boxes := make(map[string]ui.Rect, len(list))
	ed := a.editor()
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
			text := notestext.NotesNone()
			if ed == nil {
				text = notestext.NotesOpenChapter()
			}
			ui.Text(c, text).FontSize(kit.FsNote).TextColor(a.pal().Faint).Padding(24, 20)
			return
		}
		ha, hb := ed.HoveredLine()
		for i := range drawing {
			n := drawing[i]
			leaving := i >= len(list)
			rangeBox, _ := ed.RangeBoxUnclipped(n.start, n.end)
			y := max(rangeBox.Y+a.services.Prose.Bounds().Y-m.top-3, floor)
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
			bg := a.pal().Theme.SurfaceHover.Alpha(0)
			if !leaving && (lit || row.Hovered()) {
				m.lit = n.id
				bg = a.pal().Theme.SurfaceHover
				ed.SetNoteHighlight(n.start, n.end)
			}
			row.Background(row.AnimateColor("hover", bg, ui.Animation{Duration: kit.HoverTime, Ease: kit.Out}))
			rows = append(rows, row)
			acted := false
			row.Children(func() {
				ui.Box(c).Absolute().Left(0).Top(16).Width(14).Height(1).Background(a.pal().Faint)
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					if n.warn {
						kit.Dot(c, a.pal().Amber, 5)
					}
					ui.Text(c, n.kind).FontSize(kit.FsSmall).TextColor(a.pal().Theme.TextMuted).LineHeight(kit.LhBody)
				})
				ui.Text(c, n.title).FontSize(kit.FsControl).FontWeight(600).TextColor(a.pal().Ink).LineHeight(kit.LhBody).FillWidth()
				if n.text != "" {
					ui.Text(c, n.text).FontSize(kit.FsNote).TextColor(a.pal().Theme.TextMuted).LineHeight(kit.LhBody).FillWidth()
				}
				var acts []noteAction
				acts = n.actions
				if len(acts) > 0 {
					ui.Row(c).FillWidth().Gap(16).Margin(6, 0, 0, 0).Justify(ui.End).Children(func() {
						for j, act := range acts {
							fg := a.pal().Theme.TextMuted
							if j == len(acts)-1 {
								fg = a.pal().Ink
							}
							button := a.pal().Ghost(c, act.label, fg).Label("批注-" + n.id + "-" + act.label)
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
			top := max(anchor.Y+a.services.Prose.Bounds().Y-3, floor)
			floor = top + actual.H + 8
			if math.Abs(float64(actual.Y-top-m.shifts[n.id]*still)) > 0.5 {
				services.Invalidate()
			}
		}
		m.boxes = boxes
	})
	m.previous = slices.Clone(list)
}
