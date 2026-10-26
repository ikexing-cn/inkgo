package outline

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCardsFloatOnHoverAndWhenSeen(t *testing.T) {
	a, _, f := outlineFixture(t)
	card := f.cards[0]
	a.plan.seen = ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.pal.Theme)
		ui.Column(c).Fill().Padding(40).Background(a.pal.Canvas).Children(func() {
			a.plan.cardView(c, card, false, false)
		})
	}, 400, 300)
	tt.Move(0, 0)
	tt.Frame()
	idle := find(t, tt, "卡片-"+card.ID)
	paper := tt.Image().RGBAAt(1, 1)
	hasShadow := func(r ui.Rect) bool {
		img := tt.Image()
		return img.RGBAAt(int(r.X+r.W/2), int(r.Y+r.H+3)) != paper
	}
	if hasShadow(idle) {
		t.Fatal("idle card still has a shadow")
	}
	tt.Move(idle.X+idle.W/2, idle.Y+idle.H/2)
	tt.Frame()
	hover := find(t, tt, "卡片-"+card.ID)
	if hover.Y != idle.Y-2 || !hasShadow(hover) {
		t.Fatal("hover did not lift the card", idle, hover)
	}
	a.plan.seen = card.ID
	tt.Move(0, 0)
	tt.Frame()
	seen := find(t, tt, "卡片-"+card.ID)
	if seen.Y != idle.Y || !hasShadow(seen) {
		t.Fatal("the seen card has no shadow", seen)
	}
	a.plan.seen = ""
	tt.Frame()
	if hasShadow(find(t, tt, "卡片-"+card.ID)) {
		t.Fatal("inactive card kept its shadow")
	}
}
