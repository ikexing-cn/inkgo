package desk

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/editor"
	"writer/workspace"
)

// The settled names of the categories of names complete in the prose,
// another name showing whose it is; open ones and other categories do
// not.
func TestNamesComplete(t *testing.T) {
	a, tt, _ := writingFixture(t, "")
	if _, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚修路灯", Names: []string{"林晚", "晚晚"}},
		{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "林舟是她哥哥", Names: []string{"林舟"}},
		{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: workspace.Other, Text: "林间小屋", Names: []string{"林间"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.write.ed.Focus()
	tt.Frame()
	tt.Type("林")
	if c := a.write.ed.Completions(); !slices.Equal(c, []editor.Entity{{Name: "林晚", Kind: "人物"}}) {
		t.Fatal("completions", c)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Type("和晚")
	if c := a.write.ed.Completions(); len(c) != 1 || c[0].Of != "林晚" {
		t.Fatal("another name", c)
	}
}

// A name selected in the prose joins the setting from the context menu.
func TestAddANameFromTheProse(t *testing.T) {
	a, tt, _ := writingFixture(t, "沈砚推开门。")
	a.write.ed.Select(0, 2)
	tt.Frame()
	tt.RightClickAt(550, 300) // on the prose
	if err := tt.ChooseMenuItem("加入名字表", "人物"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Frame()
	e := a.set.Entries[len(a.set.Entries)-1]
	if e.Category != "人物" || e.Place != workspace.PlaceBody || !slices.Equal(e.Names, []string{"沈砚"}) {
		t.Fatalf("entry %+v", e)
	}
	tt.RightClickAt(550, 300)
	if !slices.Contains(tt.Menu(), "已在名字表中") {
		t.Fatal("offered twice", tt.Menu())
	}
}

// The author names an entry in the panel; renaming it offers to replace
// the old name through the book.
func TestRenameOffersToReplace(t *testing.T) {
	a, tt, id := writingFixture(t, "林晚推开门。林晚笑了。")
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角修路灯"})
	e := a.set.Entries[len(a.set.Entries)-1]
	a.doc.editing, a.doc.text, a.doc.category, a.doc.names = e.ID, e.Text, e.Category, " 林晚、晚晚，林晚"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if e, _ = a.set.Entry(e.ID); !slices.Equal(e.Names, []string{"林晚", "晚晚"}) || a.renamed.count != 0 {
		t.Fatal("names", e.Names, a.renamed)
	}
	a.doc.editing, a.doc.text, a.doc.category, a.doc.names = e.ID, e.Text, e.Category, "苏晴、晚晚"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("正文中仍有 2 处「林晚」") {
		t.Fatal("no notice", a.renamed, tt.Texts())
	}
	if err := tt.Click("将旧名替换为新名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	w := &a.write
	if !w.finding || !w.wide || w.query != "林晚" || w.with != "苏晴" || w.node != id {
		t.Fatalf("find bar %v %v %q %q", w.finding, w.wide, w.query, w.with)
	}
	if !tt.HasText("全书 1 章、共 2 处") || strings.Contains(w.ed.Doc.String(), "苏晴") {
		t.Fatal("replaced before the confirmation", tt.Texts())
	}
}
