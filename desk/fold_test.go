package desk

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

func nearFold(t *testing.T, what string, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > 0.6 {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// Total contrast against the box's background is nearly conserved by
// fractional translation: antialiasing redistributes ink between pixels.
// The most common color is the flat background, so empty space contributes
// nothing. A column-wide opacity fade reduces this sum, even for thin fonts.
func inkAmount(img *image.RGBA, r ui.Rect) int64 {
	box := image.Rect(int(math.Ceil(float64(r.X))), int(r.Y)+5, int(r.X+r.W), int(r.Y+r.H)-5).Intersect(img.Bounds())
	colors := make(map[[3]uint8]int)
	var background [3]uint8
	most := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := img.RGBAAt(x, y)
			key := [3]uint8{c.R, c.G, c.B}
			colors[key]++
			if colors[key] > most {
				most, background = colors[key], key
			}
		}
	}
	var amount int64
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := img.RGBAAt(x, y)
			for i, channel := range [3]uint8{c.R, c.G, c.B} {
				amount += int64(max(int(channel)-int(background[i]), int(background[i])-int(channel)))
			}
		}
	}
	return amount
}

// Allow 5% for pixel rounding and the edge of the measured box. This is
// far below a half-opacity fade, which removes about half the contrast.
func lostInk(got, still int64) bool { return got*100 < still*95 }

func TestColumnsTravelTheirWholeWidthWithoutFading(t *testing.T) {
	for _, side := range []bool{false, true} {
		name, label, close, open := "pane", "右栏标签行", "收起右栏", "展开右栏"
		if side {
			name, label, close, open = "sidebar", "侧栏标题行", "收起侧栏", "展开侧栏"
		}
		t.Run(name, func(t *testing.T) {
			a, tt, _ := marginFixture(t, true)
			tt.SetPreferences(ui.Preferences{})
			tt.Move(0, 0)
			static := find(t, tt, label)
			still := tt.Image()
			for _, opening := range []bool{false, true} {
				button := close
				if opening {
					button = open
				}
				click(t, tt, button)
				elapsed := time.Duration(0)
				for _, at := range []time.Duration{40, 100, 160} {
					tt.Advance(at*time.Millisecond - elapsed)
					elapsed = at * time.Millisecond
					r := find(t, tt, label)
					// Tester labels are clipped at the window edge.
					if side {
						r.X += r.W - static.W
					}
					r.W = static.W
					way := kit.Out(a.fold.progress)
					if opening {
						way = 1 - way
					}
					dx := (static.W + 1) * way
					if side {
						dx = -dx
					}
					nearFold(t, "column position", r.X, static.X+dx)
					if opening && at == 160 || !opening && at == 40 {
						// Compare the same visible part of the settled column;
						// sliding out of the window legitimately clips some ink.
						ref := static
						left := max(r.X, 0)
						right := min(r.X+r.W, float32(still.Bounds().Max.X))
						ref.X += left - r.X
						ref.W = right - left
						ink := inkAmount(still, ref)
						if got := inkAmount(tt.Image(), r); lostInk(got, ink) {
							t.Fatalf("column faded: ink amount %v, still %v", got, ink)
						}
					}
				}
				tt.Advance(kit.MoveTime)
			}
		})
	}
}

func TestTheCountsAndScrollBarFollowThePaneEdge(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。\n", 200))
	tt.SetPreferences(ui.Preferences{})
	shownEditor(a).MoveTo(0, false)
	tt.Move(0, 0)
	tt.Frame()
	backup, counts := find(t, tt, "备份状态"), find(t, tt, "字数")
	gap := counts.X - backup.X - backup.W
	bar := find(t, tt, "正文滚动条")
	barInk := tt.Image().RGBAAt(int(bar.X+bar.W-6), int(bar.Y+12))
	for _, button := range []string{"收起右栏", "展开右栏", "收起侧栏", "展开侧栏"} {
		click(t, tt, button)
		elapsed := time.Duration(0)
		for _, at := range []time.Duration{40, 100, 160} {
			tt.Advance(at*time.Millisecond - elapsed)
			elapsed = at * time.Millisecond
			pane := find(t, tt, "右栏标签行")
			backup, counts = find(t, tt, "备份状态"), find(t, tt, "字数")
			bar = find(t, tt, "正文滚动条")
			nearFold(t, "counts at pane", counts.X+counts.W+24, pane.X-1)
			nearFold(t, "scroll bar at pane", bar.X+bar.W, pane.X-1)
			nearFold(t, "backup before counts", counts.X-backup.X-backup.W, gap)
			if got := tt.Image().RGBAAt(int(bar.X+bar.W-6), int(bar.Y+12)); got != barInk {
				t.Fatalf("%s at %v: scroll bar pixel %v, want %v", button, at, got, barInk)
			}
			if strings.Contains(button, "侧栏") && a.fold.edge != 0 {
				t.Fatal("the sidebar moved the paper's right edge")
			}
		}
		tt.Advance(kit.MoveTime)
	}
}

func TestUnfoldingKeepsTheTopRuleAcrossTheGap(t *testing.T) {
	for _, page := range []page{pageWrite, pagePlan, pageTalk} {
		for _, side := range []bool{false, true} {
			a, tt, _ := writingFixture(t, "雨还没有停。")
			a.page = page
			outlinePage(a).Pane = true
			tt.Frame()
			close, open := "收起右栏", "展开右栏"
			if side {
				close, open = "收起侧栏", "展开侧栏"
			}
			click(t, tt, close)
			tt.SetPreferences(ui.Preferences{})
			click(t, tt, open)
			elapsed := time.Duration(0)
			for _, at := range []time.Duration{40, 100, 160} {
				tt.Advance(at*time.Millisecond - elapsed)
				elapsed = at * time.Millisecond
				pane := find(t, tt, "右栏标签行")
				from, to := a.fold.last.width-a.fold.last.paneWidth-1, pane.X-1
				if side {
					head := find(t, tt, "侧栏标题行")
					from, to = head.X+head.W+1, a.fold.last.sideWidth+1
				}
				img, y := tt.Image(), int(pane.Y+pane.H-1)
				for x := int(math.Ceil(float64(from))); x < int(to); x++ {
					c, want := img.RGBAAt(x, y), a.pal.Line
					if c.R != want.R || c.G != want.G || c.B != want.B {
						t.Fatalf("page %v side %v at %v: rule at %d,%d = %v, want %v", page, side, at, x, y, c, want)
					}
				}
			}
		}
	}
}

func TestExplicitUnfoldingBringsTheNotesWithThePane(t *testing.T) {
	for _, menu := range []bool{false, true} {
		a, tt, _ := marginFixture(t, true)
		click(t, tt, "收起右栏")
		tt.SetPreferences(ui.Preferences{})
		if menu {
			if err := clickMenu(tt, "视图", "菜单-展开右栏"); err != nil {
				t.Fatal(err)
			}
		} else {
			click(t, tt, "展开右栏")
		}
		tt.Move(0, 0)
		tt.Advance(100 * time.Millisecond)
		pane := find(t, tt, "右栏标签行")
		part := tt.Image()
		rows := make(map[string]ui.Rect)
		for _, n := range a.marginNotes() {
			rows[n.id] = find(t, tt, "批注-"+n.id)
		}
		tt.Advance(kit.MoveTime)
		end, done := find(t, tt, "右栏标签行"), tt.Image()
		for _, n := range a.marginNotes() {
			r, settled := rows[n.id], find(t, tt, "批注-"+n.id)
			nearFold(t, "note on pane", r.X-settled.X, pane.X-end.X)
			nearFold(t, "note height", r.H, settled.H)
			// Read only the part still inside the window halfway in.
			r.W = min(r.W, float32(part.Bounds().Dx())-r.X)
			settled.W = r.W
			if r.W > 20 && lostInk(inkAmount(part, r), inkAmount(done, settled)) {
				t.Fatalf("%s faded separately from the pane", n.id)
			}
		}
		if tt.FramePending() {
			t.Fatal("notes kept asking for frames after the pane arrived")
		}
		click(t, tt, "右栏-设定")
		click(t, tt, "右栏-批注")
		if !tt.FramePending() {
			t.Fatal("switching tabs no longer brings notes in one after another")
		}
	}
}

func TestSwitchingNotesDuringAFoldStillBringsThemInTurn(t *testing.T) {
	a, tt, _ := marginFixture(t, true)
	click(t, tt, "收起右栏")
	tt.SetPreferences(ui.Preferences{})
	click(t, tt, "展开右栏")
	tt.Advance(40 * time.Millisecond)
	// Change tabs while their moving targets are partly off the window.
	a.doc.tab = paneSetting
	tt.Frame()
	a.doc.tab = paneMargin
	tt.Frame()
	for _, n := range a.marginNotes() {
		if a.write.margin.runs[n.id] == 0 {
			t.Fatal("a tab change was treated as the explicit opening")
		}
	}
	tt.Advance(170 * time.Millisecond)
	if !tt.FramePending() {
		t.Fatal("the notes stopped with the pane instead of arriving in turn")
	}
	tt.Advance(100 * time.Millisecond)
	if tt.FramePending() {
		t.Fatal("tab arrivals kept asking for frames after a quarter second")
	}
}

func TestColumnInkSurvivesFractionalTranslationButRejectsHalfOpacity(t *testing.T) {
	makeInk := func(split, faded bool) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 40, 30))
		for y := 0; y < 30; y++ {
			for x := 0; x < 40; x++ {
				v := uint8(255)
				if y >= 8 && y < 22 {
					if split && (x == 15 || x == 16) {
						v = 207
					}
					if !split && x == 15 {
						v = 159
					}
				}
				if faded {
					v = uint8((int(v) + 255) / 2)
				}
				img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
		return img
	}
	r := ui.Rect{W: 40, H: 30}
	still := inkAmount(makeInk(false, false), r)
	if still == 0 {
		t.Fatal("empty reference ink")
	}
	if got := inkAmount(makeInk(true, false), r); lostInk(got, still) {
		t.Fatalf("half-pixel translation lost ink: %d vs %d", got, still)
	}
	if got := inkAmount(makeInk(true, true), r); !lostInk(got, still) {
		t.Fatalf("half-opacity column escaped detection: %d vs %d", got, still)
	}
}
