package kit

import (
	"testing"
	"time"
)

// Times are written by how far back their day is: today and yesterday by
// name, this year without the year.
func TestWhenSaid(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		at                time.Time
		when, opened, day string
	}{
		{now.Add(-time.Hour), "今天 14:00", "今天 14:00", "今天"},
		{now.Add(-20 * time.Hour), "昨天 19:00", "昨天 19:00", "昨天"},
		{time.Date(2026, 10, 6, 21, 40, 0, 0, time.Local), "10-06 21:40", "10-06", "10-06"},
		{time.Date(2026, 1, 6, 8, 5, 0, 0, time.Local), "01-06 08:05", "01-06", "01-06"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "2025-12-31 09:05", "2025-12-31", "2025-12-31"},
	} {
		if got := When(c.at, now); got != c.when {
			t.Errorf("when(%v) = %q, want %q", c.at, got, c.when)
		}
		if got := Opened(c.at, now); got != c.opened {
			t.Errorf("opened(%v) = %q, want %q", c.at, got, c.opened)
		}
		if got := DayOf(c.at, now); got != c.day {
			t.Errorf("dayOf(%v) = %q, want %q", c.at, got, c.day)
		}
	}
	if got := Opened(time.Time{}, now); got != "尚未打开" {
		t.Errorf("opened(zero) = %q", got)
	}
}
