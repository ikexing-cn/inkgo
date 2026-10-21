package prefs

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// What was written today starts from the book as the day began, survives
// reopening the work, and starts over the next day.
func TestTodayCountsFromTheDaysStart(t *testing.T) {
	var c Counter
	total := 3
	path := filepath.Join(t.TempDir(), "desk.json")
	save := func() {
		if err := (Prefs{Counter: c}).Save(path); err != nil {
			t.Fatal(err)
		}
	}
	day := time.Date(2026, 10, 9, 9, 0, 0, 0, time.Local)
	c.Day = ""
	if k := c.Today(total, day, save); k != 0 {
		t.Fatalf("today starts at %d", k)
	}
	total = 10
	if k := c.Today(total, day.Add(time.Hour), save); k != 7 {
		t.Fatalf("today is %d, want 7", k)
	}
	c.Day, c.DayStart = "", 0
	p := Load(path)
	c = p.Counter
	if k := c.Today(total, day.Add(2*time.Hour), save); k != 7 {
		t.Fatalf("after reopening today is %d, want 7", k)
	}
	if k := c.Today(total, day.Add(24*time.Hour), save); k != 0 {
		t.Fatalf("the next day starts at %d", k)
	}
	// The day gone is kept, also with the work, and shows among the last
	// days; a day the work was not open is nothing.
	total = 15
	week := c.LastDays(total, day.Add(72*time.Hour), 4, save)
	if want := []int{7, 5, 0, 0}; !slices.Equal(week, want) {
		t.Fatalf("the last days are %v, want %v", week, want)
	}
	if p := Load(path); p.Days["2026-10-09"] != 7 || p.Days["2026-10-10"] != 5 {
		t.Fatalf("days kept %v", p.Days)
	}
}
