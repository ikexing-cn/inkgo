package kit

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestNamedSelectFitsEnglishKeep(t *testing.T) {
	p := LightPalette()
	selected := "30"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(p.Theme)
		ui.Column(c).AlignItems(ui.Start).Children(func() {
			p.SelectNamedSize(c, &selected, []string{"30"}, func(string) string { return "Last 30 copies" }, FsBody).Width(140)
			ui.Text(c, "Last 30 copies").FontSize(FsBody).Label("reference")
		})
	}, 400, 200)
	tt.Frame()
	text, ok := tt.Find("Last 30 copies")
	if !ok {
		t.Fatal(tt.Texts())
	}
	reference, ok := tt.Find("reference")
	if !ok {
		t.Fatal(tt.Texts())
	}
	t.Log(text, reference)
	if text.W < reference.W {
		t.Fatalf("label shrank: %v < %v", text, reference)
	}
}
