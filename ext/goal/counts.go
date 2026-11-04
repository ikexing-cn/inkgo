package goal

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Counter keeps the daily baseline, goal and history in machine-local per-book extension storage.
type Counter struct {
	Day      string         `json:"day,omitempty"`       // the local date the count below was taken
	DayStart int            `json:"day_start,omitempty"` // the book's count as that day began
	Goal     int            `json:"day_goal,omitempty"`  // a day's goal, or 0
	Days     map[string]int `json:"days,omitempty"`      // what was written each day before, by local date
}

// keepDays is how many days' counts are kept.
const keepDays = 60

// LastDays is what was written on each of the n days up to now, the last
// being today; a day the work was not open is 0.
func (c Counter) LastDays(total int, now time.Time, n int) []int {
	out := make([]int, n)
	k := c.Today(total, now)
	for i := range n {
		day := now.AddDate(0, 0, i-n+1).Format(time.DateOnly)
		out[i] = c.Days[day]
	}
	out[n-1] = k
	return out
}

// Today is how much the book grew today. Before Roll begins a new day,
// its count is zero. Taking chapters out counts against it.
func (c Counter) Today(total int, now time.Time) int {
	if now.Format(time.DateOnly) != c.Day {
		return 0
	}
	return total - c.DayStart
}

// Roll starts a new local day from the current total. The caller supplies
// a known book count and decides when to persist it.
func (c *Counter) Roll(total int, now time.Time) bool {
	day := now.Format(time.DateOnly)
	if day == c.Day {
		return false
	}
	if c.Day != "" {
		if c.Days == nil {
			c.Days = map[string]int{}
		}
		c.Days[c.Day] = total - c.DayStart
		for len(c.Days) > keepDays {
			delete(c.Days, slices.Min(slices.Collect(maps.Keys(c.Days))))
		}
	}
	c.Day, c.DayStart = day, total
	return true
}

// SetGoal takes the day's goal as typed: a whole number of characters, or
// nothing for none. Anything else leaves the goal as it was.
func (c *Counter) SetGoal(text string) bool {
	if n, ok := CountOrNone(text); ok {
		c.Goal = n
		return true
	}
	return false
}

// CountOrNone reads a count the author may leave empty, as a goal of words
// is: none when empty, or a whole number above nought; ok is false for
// anything else.
func CountOrNone(text string) (n int, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, true
	}
	n, err := strconv.Atoi(text)
	return n, err == nil && n > 0
}
