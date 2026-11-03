package export

import (
	"testing"
	"time"
	"writer/internal/testshot"
)

func TestShotExport(t *testing.T) {
	testshot.Dir(t)
	_, d, tt, _ := dialogFixture(t, "Book text.")
	d.Show()
	tt.Frame()
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "export-dialog.png", tt.Image())
}
