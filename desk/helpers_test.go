package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/internal/testshot"
)

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
