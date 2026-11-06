package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/internal/testshot"
)

// Geometry and screenshots use the system's reduced-motion path.
func stillTester(view func(*ui.Context), width, height int) *ui.Tester {
	tt := ui.NewTester(view, width, height)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func click(t *testing.T, tt *ui.Tester, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if err := tt.Click(label); err != nil {
			t.Fatal(err, tt.Texts())
		}
		tt.Frame()
		tt.Frame()
	}
}

func find(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("nothing shows %q: %q", label, tt.Texts())
	}
	return r
}

func hasText(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if !tt.HasText(text) {
		t.Fatal("missing", text, tt.Texts())
	}
}

func shot(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	tt.Frame()
	tt.Frame()
	testshot.Save(t, name, tt.Image())
}

// scrollToLabel scrolls the settings dialog till the element labelled s is
// in view, for tests that would otherwise scroll by a guessed distance.
func scrollToLabel(t *testing.T, tt *ui.Tester, s string) {
	t.Helper()
	for range 40 {
		r, ok := tt.Find(s)
		if ok && r.Y >= 150 && r.Y+r.H <= 680 {
			return
		}
		dy := float32(150)
		if ok && r.Y < 150 {
			dy = -150
		}
		tt.Scroll(633, 400, 0, dy)
		tt.Frame()
	}
	t.Fatalf("%q never came into view", s)
}
