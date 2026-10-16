package desk

import (
	"testing"
	"time"

	"writer/backup"
)

// Times are written by how far back their day is: today and yesterday by
// name, this year without the year.
func TestWhenSaid(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		at                 time.Time
		when, opened, talk string
	}{
		{now.Add(-time.Hour), "今天 14:00", "今天 14:00", "今天"},
		{now.Add(-20 * time.Hour), "昨天 19:00", "昨天 19:00", "昨天"},
		{time.Date(2026, 10, 6, 21, 40, 0, 0, time.Local), "10-06 21:40", "10-06", "10-06"},
		{time.Date(2026, 1, 6, 8, 5, 0, 0, time.Local), "01-06 08:05", "01-06", "01-06"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "2025-12-31 09:05", "2025-12-31", "2025-12-31"},
	} {
		if got := when(c.at, now); got != c.when {
			t.Errorf("when(%v) = %q, want %q", c.at, got, c.when)
		}
		if got := opened(c.at, now); got != c.opened {
			t.Errorf("opened(%v) = %q, want %q", c.at, got, c.opened)
		}
		if got := talkDay(c.at.Format("20060102-1504")+"-ab", now); got != c.talk {
			t.Errorf("talkDay(%v) = %q, want %q", c.at, got, c.talk)
		}
	}
	if got := opened(time.Time{}, now); got != "尚未打开" {
		t.Errorf("opened(zero) = %q", got)
	}
	// The first of the backups kept, by its day: one from last year has it.
	for at, say := range map[time.Time]string{now.Add(-time.Hour): "今天", time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local): "09-28", time.Date(2025, 10, 3, 9, 0, 0, 0, time.Local): "2025-10-03"} {
		list := []backup.Info{{At: now, Words: 1200, Chapters: 3}, {At: at}}
		if got := kept(list, now); got != "1,200 字 · 3 章 · 已存 2 份，最早 "+say {
			t.Errorf("kept, the first at %v: %q", at, got)
		}
	}
}
