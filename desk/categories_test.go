package desk

import "testing"

// Opened on a category to change, the form can be put away, leaving the
// category as it was; a new name has nothing to put away.
func TestACategoryOpenedCanBePutAway(t *testing.T) {
	a, tt, _ := fixture(t)
	if err := tt.Click("作品分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.categoryName = "线索"
	tt.Frame()
	if _, ok := tt.Find("取消修改分类"); ok {
		t.Fatal("a new name can be put away")
	}
	if err := tt.Click("编辑-人物"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	was := a.Store.Work().Categories
	a.categoryTarget = "大纲"
	tt.Frame()
	if err := tt.Click("取消修改分类"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.categoryName != "" || a.categoryTarget != "" {
		t.Fatalf("the form keeps %q, %q", a.categoryName, a.categoryTarget)
	}
	for i, cat := range a.Store.Work().Categories {
		if cat != was[i] {
			t.Fatalf("putting the form away changed %+v", cat)
		}
	}
	if _, ok := tt.Find("取消修改分类"); ok {
		t.Fatal("取消 stays with nothing to put away")
	}
}
