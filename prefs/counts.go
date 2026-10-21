package prefs

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Counter keeps the daily baseline, goal and history in per-work preferences.
type Counter struct {
	Day      string         `json:"day,omitempty"`       // the local date the count below was taken
	DayStart int            `json:"day_start,omitempty"` // the book's count as that day began
	Goal     int            `json:"day_goal,omitempty"`  // a day's goal, or 0
	Days     map[string]int `json:"days,omitempty"`      // what was written each day before, by local date
}

// keepDays is how many days' counts are kept.
const keepDays = 60

// lastDays is what was written on each of the n days up to now, the last
// being today; a day the work was not open is 0.
func (c *Counter) LastDays(total int, now time.Time, n int, save func()) []int {
	out := make([]int, n)
	k := c.Today(total, now, save) // a new day keeps the last first
	for i := range n {
		day := now.AddDate(0, 0, i-n+1).Format(time.DateOnly)
		out[i] = c.Days[day]
	}
	out[n-1] = k
	return out
}

// today is how much the book grew today, which the first look on a new
// day starts from. Taking chapters out counts against it.
func (c *Counter) Today(total int, now time.Time, save func()) int {
	if day := now.Format(time.DateOnly); day != c.Day {
		if c.Day != "" {
			// What the book has now is where the last day the work was
			// open ended.
			if c.Days == nil {
				c.Days = map[string]int{}
			}
			c.Days[c.Day] = total - c.DayStart
			for len(c.Days) > keepDays {
				delete(c.Days, slices.Min(slices.Collect(maps.Keys(c.Days))))
			}
		}
		c.Day, c.DayStart = day, total
		save()
	}
	return total - c.DayStart
}

// setGoal takes the day's goal as typed: a whole number of characters, or
// nothing for none. Anything else leaves the goal as it was.
func (c *Counter) SetGoal(text string, save func()) {
	if n, ok := CountOrNone(text); ok {
		c.Goal = n
		save()
	}
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
