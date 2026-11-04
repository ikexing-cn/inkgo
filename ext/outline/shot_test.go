package outline

import (
	"testing"
	"time"
	"writer/internal/testshot"
)

func TestShotOutline(t *testing.T) {
	testshot.Dir(t)
	h, tt, _ := outlineFixture(t)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "outline-grid.png", tt.Image())
	h.plan.mode = 1
	tt.Frame()
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "outline-timeline.png", tt.Image())
	h.plan.newCard("", "", "", "")
	tt.Frame()
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "outline-card.png", tt.Image())
}
