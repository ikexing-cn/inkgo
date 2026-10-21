package prefs

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCounterReadDoesNotRoll(t *testing.T) {
	c := Counter{Day: "2026-10-09", DayStart: 3, Goal: 2000, Days: map[string]int{"2026-10-08": 2}}
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.FixedZone("local", 8*60*60))
	if got := c.Today(10, now); got != 0 {
		t.Fatal(got)
	}
	if got := c.LastDays(10, now, 3); !slices.Equal(got, []int{2, 0, 0}) {
		t.Fatal(got)
	}
	if c.Day != "2026-10-09" || c.DayStart != 3 || len(c.Days) != 1 {
		t.Fatal("read changed counter", c)
	}
	if !c.Roll(10, now) || c.Days["2026-10-09"] != 7 || c.Today(12, now) != 2 {
		t.Fatal(c)
	}
	if c.Roll(12, now) {
		t.Fatal("rolled twice")
	}
	if c.Today(9, now) != -1 {
		t.Fatal("deleted chapters do not count against today")
	}
}

func TestCounterRetainsSixtyDays(t *testing.T) {
	now := time.Date(2026, 1, 1, 23, 59, 0, 0, time.Local)
	var c Counter
	for i := 0; i < 65; i++ {
		c.Roll(i, now.AddDate(0, 0, i))
	}
	if len(c.Days) != 60 || c.Days["2026-01-01"] != 0 || c.Days["2026-01-05"] != 1 {
		t.Fatal(c.Days)
	}
}

func TestCounterGoal(t *testing.T) {
	c := Counter{Goal: 2000}
	for _, text := range []string{"0", "-1", "两千", "1.5"} {
		if c.SetGoal(text) || c.Goal != 2000 {
			t.Fatal(text, c.Goal)
		}
	}
	for text, want := range map[string]int{" 3000 ": 3000, "": 0, "  ": 0} {
		if !c.SetGoal(text) || c.Goal != want {
			t.Fatal(fmt.Sprint(text), c.Goal)
		}
	}
}

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
	c.Roll(total, day)
	save()
	if k := c.Today(total, day); k != 0 {
		t.Fatalf("today starts at %d", k)
	}
	total = 10
	if k := c.Today(total, day.Add(time.Hour)); k != 7 {
		t.Fatalf("today is %d, want 7", k)
	}
	c.Day, c.DayStart = "", 0
	p := Load(path)
	c = p.Counter
	if k := c.Today(total, day.Add(2*time.Hour)); k != 7 {
		t.Fatalf("after reopening today is %d, want 7", k)
	}
	c.Roll(total, day.Add(24*time.Hour))
	save()
	if k := c.Today(total, day.Add(24*time.Hour)); k != 0 {
		t.Fatalf("the next day starts at %d", k)
	}
	// The day gone is kept, also with the work, and shows among the last
	// days; a day the work was not open is nothing.
	total = 15
	c.Roll(total, day.Add(72*time.Hour))
	save()
	week := c.LastDays(total, day.Add(72*time.Hour), 4)
	if want := []int{7, 5, 0, 0}; !slices.Equal(week, want) {
		t.Fatalf("the last days are %v, want %v", week, want)
	}
	if p := Load(path); p.Days["2026-10-09"] != 7 || p.Days["2026-10-10"] != 5 {
		t.Fatalf("days kept %v", p.Days)
	}
}
