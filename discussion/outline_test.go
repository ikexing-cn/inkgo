package discussion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"writer/workspace"
)

func writePlan(t *testing.T, st *workspace.Store, p workspace.Plan) []byte {
	t.Helper()
	p.Schema = workspace.Schema
	used := map[string]bool{}
	for _, l := range p.Lines {
		used[l.ID] = true
	}
	for _, c := range p.Cards {
		if c.ID != "" {
			used[c.ID] = true
		}
	}
	for i := range p.Cards {
		if p.Cards[i].ID == "" {
			id := workspace.NewID()
			for used[id] {
				id = workspace.NewID()
			}
			p.Cards[i].ID = id
			used[id] = true
		}
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Root(), "outline.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	return b
}

// Outline reads retain story positions, use live chapter names, and leave
// the author's file byte for byte as it was.
func TestReadOutline(t *testing.T) {
	e, session := setup(t)
	vol, one, two, _ := shelf(t, e)
	trash, err := e.Store.NewChapter(vol, "已删的章", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Store.TrashChapter(trash); err != nil {
		t.Fatal(err)
	}
	l1, l2, l3, missing := workspace.NewID(), workspace.NewID(), workspace.NewID(), workspace.NewID()
	p := workspace.Plan{
		Lines: []workspace.PlotLine{{ID: l1, Name: "小祥"}, {ID: l2, Name: "归来"}, {ID: l3, Name: "空线"}},
		Cards: []workspace.Card{
			{Title: "下井", When: "去年", About: "找到灯。\n  留下钥匙。", Chapter: two, Line: l1},
			{Title: "收徒", Chapter: one, Line: l1},
			{Title: "重逢", When: "明年", Line: l2},
			{Title: "告别", Chapter: trash},
			{Title: "远行", Chapter: missing},
			{Title: "归家", Chapter: vol},
		},
	}
	before := writePlan(t, e.Store, p)
	box := &toolbox{store: e.Store, session: session}
	first := "#1｜时间：去年｜发生：下井｜「第二章」｜情节线：小祥｜说明：找到灯。 留下钥匙。"
	second := "#2｜发生：收徒｜「第一章」｜情节线：小祥"
	for _, tc := range []struct {
		name   string
		in     map[string]any
		want   []string
		absent []string
		failed bool
	}{
		{"all", nil, []string{"大纲共 6 张卡片", "情节线「小祥」：2 张。\n情节线「归来」：1 张。\n情节线「空线」：0 张。", "没有情节线：3 张。未放进章节：4 张。", first + "\n" + second,
			"#3｜时间：明年｜发生：重逢｜未放进章节｜情节线：归来", "#4｜发生：告别｜未放进章节", "#5｜发生：远行｜未放进章节", "#6｜发生：归家｜未放进章节"}, nil, false},
		{"line", map[string]any{"line": " 小祥 ", "chapter": "第三章", "query": "雪"}, []string{first + "\n" + second}, []string{"#3", "章纲"}, false},
		{"chapter", map[string]any{"chapter": "《第二章》", "query": "雪"}, []string{"「第二章」章纲：\n停电。\n" + first}, []string{second}, false},
		{"volume", map[string]any{"chapter": "第一卷"}, []string{"「第一卷」卷纲：\n秋天，旧城。\n「第一章」章纲：\n她回到旧城。\n" + second + "\n「第二章」章纲：\n停电。\n" + first,
			"「第三章」章纲：\n（没有章纲）\n这一章没有卡片。"}, []string{"#3"}, false},
		{"title", map[string]any{"query": " 下井 "}, []string{first}, []string{second}, false},
		{"about", map[string]any{"query": "钥匙"}, []string{first}, []string{second}, false},
		{"time", map[string]any{"query": "去年"}, []string{first}, []string{second}, false},
		{"empty line", map[string]any{"line": "空线"}, []string{"这条线没有卡片。"}, nil, false},
		{"empty chapter", map[string]any{"chapter": "第三章"}, []string{"（没有章纲）", "这一章没有卡片。"}, nil, false},
		{"empty query", map[string]any{"query": "雪"}, []string{"没有含「雪」这个词的卡片。"}, nil, false},
		{"wrong line", map[string]any{"line": "小张"}, []string{"没有叫「小张」的情节线；现有的情节线：「小祥」「归来」「空线」。"}, nil, true},
		{"wrong chapter", map[string]any{"chapter": "第九章"}, []string{"没有叫「第九章」的章"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := box.run(call("1", toolReadOutline, tc.in))
			if out.failed != tc.failed {
				t.Fatalf("failed=%v: %s", out.failed, out.text)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.text, want) {
					t.Errorf("lacks %q:\n%s", want, out.text)
				}
			}
			for _, unwanted := range append(tc.absent, vol, one, two, trash, missing, l1, l2, l3) {
				if strings.Contains(out.text, unwanted) {
					t.Errorf("exposes %q:\n%s", unwanted, out.text)
				}
			}
		})
	}
	for _, tc := range []struct {
		in   map[string]any
		want string
	}{
		{nil, "查了大纲"},
		{map[string]any{"line": " 小祥 ", "chapter": "第三章", "query": "雪"}, "查了大纲「小祥」这条线"},
		{map[string]any{"chapter": "《第二章》", "query": "雪"}, "查了大纲「第二章」的卡片"},
		{map[string]any{"query": " 钥匙 "}, "查了大纲「钥匙」"},
	} {
		if got := box.label(call("1", toolReadOutline, tc.in)); got != tc.want {
			t.Errorf("label %q, want %q", got, tc.want)
		}
	}
	after, err := os.ReadFile(filepath.Join(e.Store.Root(), "outline.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("outline changed: %v", err)
	}
	for _, c := range p.Cards {
		if c.ID != "" && strings.Contains(box.readOutline(input{}).text, c.ID) {
			t.Fatal("exposed a card ID")
		}
	}
}

func TestReadOutlinePagesAndFailures(t *testing.T) {
	e, _ := setup(t)
	_, one, two, _ := shelf(t, e)
	box := &toolbox{store: e.Store}
	if out := box.readOutline(input{}); out.failed || !strings.Contains(out.text, "大纲是空的。") {
		t.Fatal(out)
	}
	if out := box.readOutline(input{Line: "小祥"}); !out.failed || !strings.Contains(out.text, "大纲还没有情节线") {
		t.Fatal(out)
	}
	empty := box.readOutline(input{Chapter: "第一卷"})
	if empty.failed || !strings.HasPrefix(empty.text, "「第一卷」卷纲：\n秋天，旧城。\n这一卷没有卡片。\n") ||
		!strings.Contains(empty.text, "「第一章」章纲：\n她回到旧城。\n这一章没有卡片。") {
		t.Fatalf("a volume with empty chapters lacks its empty notice: %+v", empty)
	}
	if out := box.readOutline(input{Chapter: "第一卷", From: 1}); out.failed || out.text != "「第一卷」卷纲：\n秋天，旧城。\n这一卷没有卡片。" {
		t.Fatalf("empty chapters repeated past the first page: %+v", out)
	}
	p := workspace.Plan{}
	for i := range maxCards + 3 {
		chapter := one
		if i%2 == 0 {
			chapter = two
		}
		p.Cards = append(p.Cards, workspace.Card{Title: fmt.Sprintf("事件%d", i+1), Chapter: chapter})
	}
	p.Cards[0].About = strings.Repeat("长", 201) + "\n结尾"
	before := writePlan(t, e.Store, p)
	for _, chapter := range []string{"", "第一卷", "第二章"} {
		out := box.readOutline(input{Chapter: chapter})
		count := 0
		for _, line := range strings.Split(out.text, "\n") {
			if strings.HasPrefix(line, "#") {
				count++
			}
		}
		want := maxCards
		if chapter == "第二章" {
			want = (maxCards + 4) / 2
		}
		if out.failed || count != want {
			t.Fatalf("%s: %d cards: %s", chapter, count, out.text)
		}
		if chapter != "第二章" && !strings.Contains(out.text, "要看后面的，from 填 200 再查一次。") {
			t.Fatal(out.text)
		}
	}
	all := box.readOutline(input{}).text
	firstVolumePage := box.readOutline(input{Chapter: "第一卷"})
	if firstVolumePage.failed || !strings.Contains(firstVolumePage.text, "「第三章」章纲：\n（没有章纲）\n这一章没有卡片。") ||
		strings.Contains(firstVolumePage.text, "这一卷没有卡片。") {
		t.Fatalf("the first volume page lost its empty chapter: %+v", firstVolumePage)
	}
	nextVolumePage := box.readOutline(input{Chapter: "第一卷", From: maxCards})
	if nextVolumePage.failed || strings.Contains(nextVolumePage.text, "「第三章」") || strings.Contains(nextVolumePage.text, "这一章没有卡片。") ||
		!strings.Contains(nextVolumePage.text, "#203｜") {
		t.Fatalf("empty chapters repeated on the next volume page: %+v", nextVolumePage)
	}
	if !strings.Contains(all, "说明："+strings.Repeat("长", 200)+"…") || strings.Contains(all, "结尾") {
		t.Fatal("about was not shortened")
	}
	page := box.readOutline(input{From: maxCards}).text
	if !strings.Contains(page, "#201｜") || !strings.Contains(page, "#203｜") || strings.Contains(page, "#200｜") || strings.Contains(page, "要看后面的") {
		t.Fatal(page)
	}
	if got := box.readOutline(input{From: -5}).text; got != all {
		t.Fatal("negative from did not start at zero")
	}
	if got := box.readOutline(input{From: 999}).text; !strings.Contains(got, "后面没有卡片了。") || strings.Contains(got, "第 204–203") {
		t.Fatal(got)
	}
	path := filepath.Join(e.Store.Root(), "outline.json")
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("paging wrote the outline: %v", err)
	}
	broken := []byte(`{"schema":1,"cards":[`)
	if err := os.WriteFile(path, broken, 0644); err != nil {
		t.Fatal(err)
	}
	out := box.run(call("1", toolReadOutline, nil))
	if !out.failed || !strings.Contains(out.text, "大纲文件读不出来") {
		t.Fatal(out)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, broken) {
		t.Fatalf("failure wrote the outline: %v", err)
	}
}

func TestFixedCardsAndPreloaded(t *testing.T) {
	e, _ := setup(t)
	_, one, two, _ := shelf(t, e)
	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		t.Run(origin, func(t *testing.T) {
			writePlan(t, e.Store, workspace.Plan{})
			baseline, err := Preloaded(e.Store, origin, two)
			if err != nil {
				t.Fatal(err)
			}
			p := workspace.Plan{Cards: []workspace.Card{{Title: "别章的事", Chapter: one}, {Title: "灯灭了", Chapter: two}, {Title: "她推开门", Chapter: two}}}
			writePlan(t, e.Store, p)
			f, err := fixedOf(e.Store, tree, origin, two)
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(f.Pieces, func(p Piece) bool { return p.Part == workspace.PartCards })
			if i != 2 || f.Pieces[i].Item != "本章的卡片 2 张" || f.Pieces[i].Text != "【本章的卡片】\n#2｜发生：灯灭了｜「第二章」\n#3｜发生：她推开门｜「第二章」" {
				t.Fatalf("pieces %+v", f.Pieces)
			}
			ctx, _, err := lay(e.Store.Work(), 1<<30, workspace.Setting{}, tree, talk{fixed: f})
			if err != nil || ctx.Sent.Part(workspace.PartCards) == 0 || !slices.ContainsFunc(ctx.Included, func(it workspace.ContextItem) bool { return it.Text == "本章的卡片 2 张" }) {
				t.Fatalf("context %+v, %v", ctx, err)
			}
			n, err := Preloaded(e.Store, origin, two)
			if err != nil || n.Tokens != baseline.Tokens+ctx.Sent.Part(workspace.PartCards) {
				t.Fatalf("preloaded %d, baseline %d, cards %d: %v", n.Tokens, baseline.Tokens, ctx.Sent.Part(workspace.PartCards), err)
			}
			for _, data := range [][]byte{writePlan(t, e.Store, workspace.Plan{Cards: p.Cards[:1]}), []byte(`{broken`)} {
				if err := os.WriteFile(filepath.Join(e.Store.Root(), "outline.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
				f, err := fixedOf(e.Store, tree, origin, two)
				if err != nil || slices.ContainsFunc(f.Pieces, func(p Piece) bool { return p.Part == workspace.PartCards }) {
					t.Fatalf("unexpected cards: %+v, %v", f, err)
				}
				if n, err := Preloaded(e.Store, origin, two); err != nil || n.Tokens != baseline.Tokens {
					t.Fatalf("fallback preloaded %d, want %d: %v", n.Tokens, baseline.Tokens, err)
				}
			}
		})
	}
	for _, want := range []string{"read_outline", "故事里发生的先后", "可能和章节的先后不同", "还没写的打算", "以正文为准，并告诉作者哪里不一致"} {
		if !strings.Contains(talkRules, want) {
			t.Errorf("talk rules lack %q", want)
		}
	}
	for _, origin := range []string{workspace.AIDraft, workspace.AIRewrite} {
		if !strings.Contains(originRules[origin], "本章的卡片") {
			t.Errorf("%s rules omit cards", origin)
		}
	}
	if !strings.Contains(originRules[workspace.AIDraft], "照章纲和本章的卡片写") {
		t.Fatal("draft rules do not follow cards")
	}
}

func TestReadOutlineIgnoresDamagedMarks(t *testing.T) {
	e, _ := setup(t)
	shelf(t, e)
	path := filepath.Join(e.Store.Root(), "chapters", "第一卷", "第一章.marks.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	box := toolbox{store: e.Store}
	for _, chapter := range []string{"第一卷", "第一章"} {
		out := box.readOutline(input{Chapter: chapter})
		if out.failed || !strings.Contains(out.text, "她回到旧城。") {
			t.Fatalf("outline-only read = %+v", out)
		}
	}
}

func TestReadOutlineBatchFailureNamesRequestedNode(t *testing.T) {
	e, _ := setup(t)
	shelf(t, e)
	path := filepath.Join(e.Store.Root(), "chapters", "第一卷", "第一章.outline.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	box := toolbox{store: e.Store}
	out := box.readOutline(input{Chapter: "第一卷"})
	if !out.failed || out.text != "读不出「第一卷」的纲。" {
		t.Fatalf("outline batch failure = %+v", out)
	}
}
