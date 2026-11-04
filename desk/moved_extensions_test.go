package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"writer/ext/cats"
	"writer/ext/goal"
)

var goalControllers = map[*App]*goal.Page{}
var catsControllers = map[*App]*cats.Page{}

func goalOf(a *App) *goal.Page { return goalControllers[a] }
func catsOf(a *App) *cats.Page { return catsControllers[a] }

func focusCommand(t *testing.T) *command {
	t.Helper()
	a, _, _ := writingFixture(t, "正文")
	for _, row := range a.extensionCommands() {
		if row.id == "focus.focus" {
			return row.cmd
		}
	}
	t.Fatal("focus command missing")
	return nil
}

func savedGoal(t *testing.T, a *App) goal.Counter {
	t.Helper()
	dir, err := a.Store.LocalExtDir("goal")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "counter.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c goal.Counter
	if err = json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}
