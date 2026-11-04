package desk

import "writer/workspace"

import (
	"testing"
	"time"
	"writer/ext/backup"

	"writer/ext/backup/archive"
)

// The talks and the backups say their day as kit.DayOf does.
func TestTalksAndBackupsSayTheirDay(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		at   time.Time
		talk string
	}{
		{now.Add(-time.Hour), "今天"},
		{now.Add(-20 * time.Hour), "昨天"},
		{time.Date(2026, 10, 6, 21, 40, 0, 0, time.Local), "10-06"},
		{time.Date(2026, 1, 6, 8, 5, 0, 0, time.Local), "01-06"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "2025-12-31"},
	} {
		if got := talkDay(c.at.Format("20060102-1504")+"-ab", now); got != c.talk {
			t.Errorf("talkDay(%v) = %q, want %q", c.at, got, c.talk)
		}
	}
	// The first of the backups kept, by its day: one from last year has it.
	for at, say := range map[time.Time]string{now.Add(-time.Hour): "今天", time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local): "09-28", time.Date(2025, 10, 3, 9, 0, 0, 0, time.Local): "2025-10-03"} {
		list := []archive.Info{{At: now, Words: 1200, Chapters: 3}, {At: at}}
		if got := backup.Kept(list, now, workspace.Chinese); got != "1,200 字 · 3 章 · 已存 2 份，最早 "+say {
			t.Errorf("kept, the first at %v: %q", at, got)
		}
	}
}
