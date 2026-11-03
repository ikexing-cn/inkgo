package kit

import (
	"time"

	"writer/locales"
)

// DayOf names the day of t from now: 今天, 昨天, another day this year by
// its month and day (10-06), an earlier year's in full (2025-10-06).
func DayOf(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	switch t.Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		return locales.TimeTodayDay()
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		return locales.TimeYesterdayDay()
	}
	if t.Year() == now.Year() {
		return t.Format("01-02")
	}
	return t.Format(time.DateOnly)
}

// When says when something happened: its day and time, 今天 10:32,
// 昨天 23:15, 10-06 21:40.
func When(t, now time.Time) string {
	clock := t.Local().Format("15:04")
	if SameDay(t, now) {
		return locales.TimeToday(clock)
	}
	if SameDay(t, now.AddDate(0, 0, -1)) {
		return locales.TimeYesterday(clock)
	}
	return DayOf(t, now) + " " + clock
}

// Opened says when a work was last opened, briefly: the time only today
// and yesterday.
func Opened(t, now time.Time) string {
	if t.IsZero() {
		return locales.ShelfNotOpened()
	}
	if !SameDay(t, now) && !SameDay(t, now.AddDate(0, 0, -1)) {
		return DayOf(t, now)
	}
	return When(t, now)
}

// SameDay says t falls on the day of now, by the local calendar: what is
// compared, where the day's name is only shown.
func SameDay(t, now time.Time) bool {
	return t.Local().Format(time.DateOnly) == now.Local().Format(time.DateOnly)
}
