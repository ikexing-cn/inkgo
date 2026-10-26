package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// How things move, after the prototype's motion styles: each is done
// within a quarter second. While the system asks for less motion, mygo
// puts every one of them where it ends at once.
const (
	PressIn      = 60 * time.Millisecond  // a button going down
	PressOut     = 120 * time.Millisecond // and springing back
	PopTime      = 140 * time.Millisecond // menus, drop-downs, popovers, floating bars
	DialogTime   = 200 * time.Millisecond
	ScrimTime    = 180 * time.Millisecond
	MoveTime     = 200 * time.Millisecond // a column folding, a note leaving, a toast coming in
	NoteTime     = 160 * time.Millisecond // one margin note coming in
	NoteDelay    = 30 * time.Millisecond  // after the note above it, up to the fourth
	HoverTime    = 100 * time.Millisecond // a margin note's fill under the pointer
	PressScale   = float32(0.97)
	PopScale     = float32(0.96)
	DialogScale  = float32(0.98)
	ToastTravel  = float32(12) // toasts come in from the right
	NoteTravel   = float32(8)  // margin notes from the prose
	NoteExit     = float32(16) // and leave toward the window's edge
	ColumnTravel = float32(24) // a folding column toward its own side
)

// The prototype's curves, CSS's ease-out and ease-in: what comes in or
// moves slows as it arrives, what goes away speeds up as it leaves. mygo's
// own EaseOut is steeper, and covers most of the way in the first frame.
var (
	Out = ui.CubicBezier(0, 0, 0.58, 1)
	In  = ui.CubicBezier(0.42, 0, 1, 1)
)

// Press is a boxed button held down: the pressed fill and a squeeze.
// Filled buttons keep their fill and only Squeeze; list rows only take
// the fill (PressRow); words and icons do neither.
func Press(c *ui.Context, b ui.Element) {
	PressRow(c, b)
	Squeeze(b)
}

// Squeeze shrinks a button a little while it is held, and springs it back.
func Squeeze(b ui.Element) {
	target, duration := float32(1), PressOut
	if b.Pressed() && !b.IsDisabled() {
		target, duration = PressScale, PressIn
	}
	s := b.AnimateWith("press", target, duration, Out)
	b.Scale(s, s)
}

// PressRow fills a held row as a current row is filled.
func PressRow(c *ui.Context, b ui.Element) {
	if b.Pressed() && !b.IsDisabled() {
		b.Background(c.Theme().SurfacePressed)
	}
}

// Arrive goes from 0 to 1 once, when e is first built, and again each
// time run changes (a margin note brought back by an undo).
func Arrive(e ui.Element, run uint64, duration, delay time.Duration) float32 {
	return e.Timeline("arrive", ui.TimelineOptions{Run: run, Delay: delay}).Tween(ui.Tween{From: 0, To: 1, Duration: duration, Ease: Out})
}

// PopIn opens a menu or popover from (x, y), 0 to 1 across it: the
// corner or edge nearest what opened it.
func PopIn(panel ui.Element, x, y float32) {
	p := Arrive(panel, 0, PopTime, 0)
	s := PopScale + (1-PopScale)*p
	panel.TransformOrigin(x, y).Scale(s, s).Opacity(p)
}

// Popover is ui.PopoverBase with its panel popping in from (x, y); fn may
// set another TransformOrigin.
func Popover(c *ui.Context, anchor ui.Element, open *bool, x, y float32, fn func(ui.Element)) ui.Element {
	return ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		PopIn(panel, x, y)
		fn(panel)
	})
}

// DialogIn fades the scrim in and grows the dialog into place.
func DialogIn(back, panel ui.Element, scrim ui.Color) {
	// The scrim's fill fades on its own: fading back would fade the panel
	// in it too and stretch the panel's entrance.
	p := Arrive(back, 0, ScrimTime, 0)
	back.Background(scrim.Alpha(p))
	q := Arrive(panel, 0, DialogTime, 0)
	s := DialogScale + (1-DialogScale)*q
	panel.Scale(s, s).Opacity(q)
}
