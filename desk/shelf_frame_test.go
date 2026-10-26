package desk

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/shelf"
)

// Fresh keys deny every build its previous bounds, as on opening the shelf.
func TestShelfCardsFillTheFirstFrame(t *testing.T) {
	a, _, _ := fixture(t)
	store := a.Store
	defer func() { a.Store = store }()
	for _, name := range []string{"雨城", "井边", "灯塔", "旧信"} {
		a.shelf.Put(shelf.Book{Path: filepath.Join(t.TempDir(), name), Name: name, Genre: "网文", Opened: time.Now()})
	}
	a.shelf.Cards = true
	books := a.shelf.Recent()
	a.Store = nil // the shelf shows with no work open
	for _, tc := range []struct{ width, columns int }{{1266, 4}, {1000, 4}, {780, 3}} {
		build := 0
		tt := stillTester(func(c *ui.Context) {
			c.SetTheme(a.pal.Theme)
			ui.Column(c.Key(build)).Fill().Children(func() { a.welcome(c) })
			build++
		}, tc.width, 780)
		first := find(t, tt, "作品-"+books[0].Name)
		last := find(t, tt, "作品-"+books[tc.columns-1].Name)
		column := find(t, tt, "书架内容")
		if first.Y != last.Y || math.Abs(float64(last.X+last.W-column.X-column.W)) > 1 || math.Abs(float64(first.X-column.X)) > 1 {
			t.Fatalf("%d wide: first frame does not fill the column: %v %v column %v", tc.width, first, last, column)
		}
		next := find(t, tt, "作品-"+books[tc.columns].Name)
		if next.Y <= first.Y || math.Abs(float64(next.X-first.X)) > 1 {
			t.Fatal("the next row does not start at the column edge", next, first)
		}
	}
}
