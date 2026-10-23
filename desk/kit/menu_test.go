package kit

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// A menu's rows are listed with nothing drawn and nothing done: what a row
// does is kept with it, for the one picked.
func TestMenuListsItsRows(t *testing.T) {
	var done []string
	drawn := 0
	m := &Menu{}
	m.Head("章节")
	for _, label := range []string{"重命名", "上移", "下移"} {
		m.Item(label).Disabled(label == "上移").Do(func() { done = append(done, label) })
	}
	m.Separator()
	m.Draw(func(*ui.Context) { drawn++ })
	m.Submenu("移到", func(m *Menu) {})
	m.Item("重命名").Hint("F2")
	if len(m.Rows) != 8 || len(done) != 0 || drawn != 0 {
		t.Fatalf("%d rows, done %v, drawn %d", len(m.Rows), done, drawn)
	}
	if it := m.Rows[2]; it.Kind != RowItem || it.Label != "上移" || !it.off {
		t.Fatalf("the third row is %+v", it)
	}
	m.Rows[3].do()
	if !slices.Equal(done, []string{"下移"}) {
		t.Fatalf("done %v", done)
	}
	// A row may have nothing to do: picked, it only closes the menu.
	if it := m.Rows[7]; it.Kind != RowItem || it.hint != "F2" || it.do != nil {
		t.Fatalf("the last row is %+v", it)
	}
}

func TestMenuClosesBeforeItsAction(t *testing.T) {
	p := LightPalette()
	m := MenuState{Owner: "试", X: 100, Y: 100}
	done, builds := 0, 0
	openAtAction := true
	tt := ui.NewTester(func(c *ui.Context) {
		m.MenuAt(c, p, "试", func(rows *Menu) {
			builds++
			rows.Item("做一件事").Do(func() { done++; openAtAction = m.Owner != "" })
			rows.Item("不能点").Disabled(true).Do(func() { done += 10 })
			rows.Item("什么也不做")
		})
	}, 720, 480)
	tt.Frame()
	if err := tt.Click("菜单-不能点"); err != nil {
		t.Fatal(err)
	}
	if done != 0 || m.Owner == "" {
		t.Fatal("a disabled row was picked", done, m.Owner)
	}
	if err := tt.Click("菜单-做一件事"); err != nil {
		t.Fatal(err)
	}
	if done != 1 || openAtAction || m.Owner != "" {
		t.Fatal("picked", done, openAtAction, m.Owner)
	}
	before := builds
	tt.Frame()
	if builds != before || done != 1 {
		t.Fatal("a closed menu was rebuilt", builds, before, done)
	}
	m = MenuState{Owner: "试", X: 100, Y: 100}
	tt.Frame()
	if err := tt.Click("菜单-什么也不做"); err != nil {
		t.Fatal(err)
	}
	if done != 1 || m.Owner != "" {
		t.Fatal("an empty action did not close the menu", done, m.Owner)
	}
}
