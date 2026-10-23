package kit

import (
	"time"
)

// DayOf names the day of t from now: 今天, 昨天, another day this year by
// its month and day (10-06), an earlier year's in full (2025-10-06).
func DayOf(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	switch t.Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		return "今天"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		return "昨天"
	}
	if t.Year() == now.Year() {
		return t.Format("01-02")
	}
	return t.Format(time.DateOnly)
}

// When says when something happened: its day and time, 今天 10:32,
// 昨天 23:15, 10-06 21:40.
func When(t, now time.Time) string {
	return DayOf(t, now) + " " + t.Local().Format("15:04")
}

// Opened says when a work was last opened, briefly: the time only today
// and yesterday.
func Opened(t, now time.Time) string {
	if t.IsZero() {
		return "尚未打开"
	}
	if day := DayOf(t, now); day != "今天" && day != "昨天" {
		return day
	}
	return When(t, now)
}
