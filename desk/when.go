package desk

import "time"

// dayOf names the day of t from now: 今天, 昨天, another day this year by
// its month and day (10-06), an earlier year's in full (2025-10-06).
func dayOf(t, now time.Time) string {
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

// when says when something happened: its day and time, 今天 10:32,
// 昨天 23:15, 10-06 21:40.
func when(t, now time.Time) string {
	return dayOf(t, now) + " " + t.Local().Format("15:04")
}

// opened says when a work was last opened, briefly: the time only today
// and yesterday.
func opened(t, now time.Time) string {
	if t.IsZero() {
		return "尚未打开"
	}
	if day := dayOf(t, now); day != "今天" && day != "昨天" {
		return day
	}
	return when(t, now)
}
