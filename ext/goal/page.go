// Package goal provides local daily word goals and history.
package goal

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"writer/desk/kit"
	"writer/ext/goal/goaltext"
	"writer/ext/hostapi"
	"writer/internal/atomicfile"
	"writer/locales"
)

// Page owns the counter and count popover for a book session.
type Page struct {
	Book    hostapi.Book
	Editing hostapi.Editing
	Palette hostapi.Palette
	Notices hostapi.Notices
	Counter Counter
	Text    string
	Open    bool
}

func (a *Page) total() int {
	n := 0
	for _, k := range a.Book.Content().Counts() {
		n += k
	}
	return n
}
func (a *Page) unit() string { return a.Book.Store().Work().Language.Unit() }
func (a *Page) path() (string, error) {
	dir, err := a.Book.Store().LocalExtDir("goal")
	return filepath.Join(dir, "counter.json"), err
}
func (a *Page) load() error {
	path, err := a.path()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &a.Counter); err != nil {
		return err
	}
	if a.Counter.Goal > 0 {
		a.Text = strconv.Itoa(a.Counter.Goal)
	}
	return nil
}
func (a *Page) persist() {
	path, err := a.path()
	if err == nil {
		err = a.Counter.save(path)
	}
	if err != nil {
		a.Notices.Notices().SetError(err)
	}
}
func (c Counter) save(path string) error {
	b, err := json.Marshal(c)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0755)
	}
	if err == nil {
		err = atomicfile.Write(path, b, 0644)
	}
	return err
}

// Today returns today's change without rolling an unread book.
func (a *Page) Today(now time.Time) int {
	if a.Book.Content().Counts() == nil {
		return 0 // not read yet: a start of nothing would count the whole book
	}
	return a.Counter.Today(a.total(), now)
}

// LastDays returns daily changes ending today.
func (a *Page) LastDays(now time.Time, n int) []int {
	days := a.Counter.LastDays(a.total(), now, n)
	if a.Book.Content().Counts() == nil {
		days[n-1] = 0
	}
	return days
}

// SetGoal updates and persists a valid goal.
func (a *Page) SetGoal(text string) {
	if a.Counter.SetGoal(text) {
		a.persist()
	}
}

// RollDay rolls the baseline only when book counts are known.
func (a *Page) RollDay(now time.Time) bool {
	if a.Book.Content().Counts() == nil {
		return false
	}
	return a.Counter.Roll(a.total(), now)
}

// lastWeek lists how much was written on each of the last seven days, today
// first.
func (a *Page) lastWeek(c *ui.Context) {
	now := time.Now()
	days := a.LastDays(now, 7)
	ui.Text(c, goaltext.StatusLast7()).FontSize(kit.FsSmall).TextColor(a.Palette.Colors().Faint).Padding(6, 10, 2, 10)
	for i := len(days) - 1; i >= 0; i-- {
		day, k := now.AddDate(0, 0, i-len(days)+1), days[i]
		got := locales.StatusWords(0, a.unit())
		if k != 0 {
			got = goaltext.StatusSignedWords(k, kit.Signed(k), a.unit())
		}
		ui.Row(c.Key(i)).FillWidth().Padding(4, 10).Label(goaltext.StatusDayWritten(kit.DayOf(day, now), k, a.unit())).Children(func() {
			ui.Text(c, kit.DayOf(day, now)).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Theme.TextMuted).Grow(1)
			ui.Text(c, got).FontSize(kit.FsNote).TextColor(a.Palette.Colors().Theme.TextMuted)
		})
	}
}

func (a *Page) suffix(book string) (string, string) {
	if a.RollDay(time.Now()) {
		a.persist()
	}
	today := ""
	k := a.Today(time.Now())
	tip := goaltext.StatusCountTip(book, k, kit.Signed(k), a.unit())
	if a.Counter.Goal > 0 {
		today = goaltext.StatusTodayGoal(k, a.Counter.Goal)
		tip = goaltext.StatusCountGoalTip(book, k, kit.Signed(k), a.Counter.Goal, a.unit())
	}
	return today, tip
}
func (a *Page) popover(c *ui.Context, counts ui.Element, clicked bool) {
	t := a.Palette.Colors().Theme
	if clicked {
		a.Open = !a.Open
	}
	// It opens upward: the foot is the last line of the window.
	kit.Popover(c, counts, &a.Open, 1, 1, func(panel ui.Element) {
		panel.AttachTo(counts, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(250).Radius(kit.RCard).Padding(8).Gap(2).
			Background(a.Palette.Colors().Raised).Border(1, t.Border)
		a.Palette.Colors().Pop.On(panel)
		a.lastWeek(c)
		ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.Palette.Colors().Line).Shrink(0)
		ui.Column(c).FillWidth().Gap(6).Padding(4, 6).Children(func() {
			ui.Text(c, goaltext.StatusDailyGoal()).FontSize(kit.FsNote).TextColor(t.TextMuted)
			in := ui.TextInput(c, &a.Text).Label(goaltext.StatusGoalLabel(a.unit())).Placeholder(goaltext.StatusGoalHint()).FillWidth().AutoFocus()
			if in.Changed() {
				a.SetGoal(a.Text)
			}
			if in.Submitted() {
				a.Open = false
			}
		})
		// What is not a count leaves the goal be, and says so.
		note, ink := goaltext.StatusGoalNote(a.unit()), t.TextMuted
		if _, ok := CountOrNone(a.Text); !ok {
			note, ink = goaltext.StatusGoalError(), t.Danger
		}
		ui.Text(c, note).FontSize(kit.FsNote).TextColor(ink).LineHeight(kit.LhBody).FillWidth().Padding(0, 6, 4, 6)
	})
}
