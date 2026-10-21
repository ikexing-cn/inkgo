package workspace

import "testing"

func TestNextTitle(t *testing.T) {
	for before, want := range map[string]string{
		"第四章 名单":    "第五章",
		"第九章":       "第十章",
		"第十九章 雨":    "第二十章",
		"第九十九回":     "第一百回",
		"第一百零九章":    "第一百一十章",
		"第一千九百九十九章": "第二千章",
		"第二卷":       "第三卷",
		"第12章 夜":    "第13章",
		"楔子":        "",
		"第一二章":      "",
		"":          "",
	} {
		if got := nextTitle(before); got != want {
			t.Errorf("after %q: %q, want %q", before, got, want)
		}
	}
}

func TestTitleAfter(t *testing.T) {
	for _, tc := range []struct {
		name              string
		before, me, after *Node
		want              string
	}{
		{"chapter", &Node{ID: "before", Title: "第四章 名单"}, &Node{ID: "new"}, nil, "第五章"},
		{"between chapters", &Node{ID: "before", Title: "第四章"}, &Node{ID: "new"}, &Node{ID: "after", Title: "第五章 雨"}, ""},
		{"first chapter", nil, &Node{ID: "new"}, &Node{ID: "after", Title: "第一章"}, ""},
		{"unnumbered", &Node{ID: "before", Title: "楔子"}, &Node{ID: "new"}, nil, ""},
		{"skip folder", &Node{ID: "before", Title: "第一卷", Folder: true}, &Node{ID: "new"}, nil, ""},
		{"folder", &Node{ID: "before", Title: "第二卷", Folder: true}, &Node{ID: "new", Folder: true}, nil, "第三卷"},
		{"nested previous", &Node{ID: "volume", Title: "第一卷", Folder: true, Children: []*Node{{ID: "before", Title: "第九章"}}}, &Node{ID: "new"}, nil, "第十章"},
		{"nested next", &Node{ID: "before", Title: "第九章"}, &Node{ID: "new"}, &Node{ID: "volume", Folder: true, Children: []*Node{{ID: "after", Title: "第十章"}}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := Tree{Root: &Node{}, byID: map[string]*Node{}}
			for _, n := range []*Node{tc.before, tc.me, tc.after} {
				if n != nil {
					tree.Root.Children = append(tree.Root.Children, n)
				}
			}
			tree.Walk(func(n *Node, _ int) { tree.byID[n.ID] = n })
			if got := tree.TitleAfter("new"); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if got := tree.TitleAfter("missing"); got != "" {
				t.Fatalf("missing node: %q", got)
			}
		})
	}
	if got := (Tree{}).TitleAfter("new"); got != "" {
		t.Fatal(got)
	}
}

func TestHanNumbers(t *testing.T) {
	for text, want := range map[string]int{"十": 10, "十二": 12, "两百零三": 203, "一千零一": 1001, "九千九百九十九": 9999} {
		if got, ok := fromHan(text); !ok || got != want {
			t.Fatalf("fromHan(%q) = %d, %v", text, got, ok)
		}
	}
	for _, text := range []string{"", "零", "一二", "百"} {
		if _, ok := fromHan(text); ok {
			t.Fatalf("accepted %q", text)
		}
	}
	for n := 1; n < 10000; n++ {
		text := toHan(n)
		if got, ok := fromHan(text); !ok || got != n {
			t.Fatalf("round trip %d through %q: %d, %v", n, text, got, ok)
		}
	}
	for n, want := range map[int]string{0: "0", -1: "-1", 10000: "10000"} {
		if got := toHan(n); got != want {
			t.Fatalf("toHan(%d) = %q", n, got)
		}
	}
}

func TestTitleNumber(t *testing.T) {
	for title, want := range map[string]string{
		"第五章 名单": "第五章",
		"第12卷 夜": "第12卷",
		"楔子":     "",
		"":       "",
	} {
		if got := TitleNumber(title); got != want {
			t.Errorf("TitleNumber(%q) = %q, want %q", title, got, want)
		}
	}
}
