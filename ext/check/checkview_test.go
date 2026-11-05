package check

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
	"writer/desk/kit"
)

type testPalette struct{}

func (testPalette) Colors() *kit.Palette { return &kit.Palette{Theme: &ui.Theme{}} }
func (testPalette) ScreenColors() (*kit.Palette, *kit.Palette) {
	return kit.LightPalette(), kit.DarkPalette()
}
func TestAlikeHoldToOneSentence(t *testing.T) {
	const text = "主角，二十六岁，旧城电力局的路灯维修工。怕黑，天黑以后不进没有灯的巷子。左撇子。"
	for _, c := range []struct {
		x, y string
		want bool
	}{
		{"怕黑", "天黑以后不进没有灯的巷子", true},
		{"怕黑", "怕黑，天黑以后", true},
		{"怕黑", "左撇子", false},
		{"路灯维修工", "左撇子", false},
		{"左撇子", "", true},
		{"左撇子", "右撇子", true},
	} {
		if got := overlap(text, c.x, c.y); got != c.want {
			t.Errorf("%s / %s: %v", c.x, c.y, got)
		}
	}
}

func TestQuotedMeasure(t *testing.T) {
	a := &Panel{palette: testPalette{}}
	spans := a.quoted(strings.Repeat("甲", 121)+"关键"+strings.Repeat("a", 121), "关键")
	if len(spans) != 3 || spans[0].Text != "…"+strings.Repeat("甲", 120) || spans[1].Text != "关键" || spans[2].Text != strings.Repeat("a", 120)+"…" {
		t.Fatalf("quoted = %+v", spans)
	}
}
