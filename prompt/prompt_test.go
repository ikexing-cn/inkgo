package prompt

import (
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"writer/workspace"
)

func TestEstimateCountsCharacters(t *testing.T) {
	if got := Estimate(strings.Repeat("雨", 1000), workspace.Chinese); got != 1516 {
		t.Fatalf("1000 汉字 = %d", got)
	}
	if got := Estimate(strings.Repeat("rain ", 300), workspace.Chinese); got != 516 {
		t.Fatalf("1500 ASCII bytes = %d", got)
	}
}

// A budget records what each part took, in the order a request has them,
// whatever order they were charged in, with the rules by a digest and the
// setting by its revision.
func TestBudgetRecordsItsParts(t *testing.T) {
	b := NewBudget(slack+100, workspace.Chinese)
	b.Must(workspace.PartInput, "问")
	if !b.Take(workspace.PartRules, "规则") || b.Take(workspace.PartTalk, strings.Repeat("长", 100)) {
		t.Fatal("took what does not fit, or left what does")
	}
	b.Must(workspace.PartIndex, "目录")
	b.Return(workspace.PartIndex)
	if b.Left() != 100-Estimate("问", workspace.Chinese)-Estimate("规则", workspace.Chinese) {
		t.Fatalf("left %d", b.Left())
	}
	got := b.Sent("规则", workspace.Setting{Revisions: make([]workspace.Revision, 3)})
	if len(got.Rules) != 8 || got.Setting != 3 || len(got.Parts) != 2 ||
		got.Parts[0] != (workspace.Part{Name: workspace.PartRules, Tokens: Estimate("规则", workspace.Chinese)}) || got.Part(workspace.PartInput) != Estimate("问", workspace.Chinese) {
		t.Fatalf("%+v", got)
	}
	if Size(got) != slack+Estimate("规则", workspace.Chinese)+Estimate("问", workspace.Chinese) {
		t.Fatalf("size %d", Size(got))
	}
	if other := b.Sent("别的规则", workspace.Setting{}); other.Rules == got.Rules {
		t.Fatal("two rules, one digest")
	}
	if b.Must(workspace.PartProse, strings.Repeat("长", 100)) || b.Left() >= 0 {
		t.Fatal("a request too big still fits")
	}
}

// An answer of the author's to the AI's question goes with the question,
// as the pane shows it: alone, 缺了两根 says nothing of whose hand.
func TestAnAnswerKeepsItsQuestion(t *testing.T) {
	e := workspace.Entry{Category: "人物", Text: "缺了两根，小时候被机器轧的。", Asked: "周祥的左手是不是缺了两根手指？"}
	if got, want := Whole(e), "缺了两根，小时候被机器轧的。（回答：周祥的左手是不是缺了两根手指？）"; got != want {
		t.Fatal(got)
	}
	e.Names = []string{"周祥"}
	if got, want := Whole(e), "周祥：缺了两根，小时候被机器轧的。（回答：周祥的左手是不是缺了两根手指？）"; got != want {
		t.Fatal(got)
	}
	// The index has the first sentence of the answer; look_up_setting has the rest.
	if got, want := Short(e), "人物 · 周祥：缺了两根，小时候被机器轧的"; got != want {
		t.Fatal("the index line", got)
	}
}

// The setting reads by category in the work's order, then as entries
// were added, whatever changed last; whole, an entry has its names before
// its text; the index has each entry's category, first name and first
// sentence; a budget too small keeps the newest.
func TestTheSettingReadsInAStableOrder(t *testing.T) {
	w := workspace.Work{Language: workspace.Chinese, Categories: workspace.Categories("网文", workspace.Chinese)}
	set := workspace.Setting{Entries: []workspace.Entry{
		{ID: "a", Place: workspace.PlaceBody, Category: "物品", Text: "铜钥匙挂在她脖子上。不知道开哪扇门。", Names: []string{"铜钥匙"}, Rev: 3},
		{ID: "b", Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑。\n她是左撇子。", Names: []string{"林晚", "晚晚"}, Rev: 1},
		{ID: "c", Place: workspace.PlaceDropped, Category: "人物", Text: "林晚会飞", Rev: 2},
		{ID: "d", Place: workspace.PlaceBody, Category: "人物", Text: "周祥是她的徒弟？还是同事。", Rev: 4},
	}}
	places := []Place{{Place: workspace.PlaceBody, Heading: "正文："}, {Place: workspace.PlaceDropped, Label: "已放弃", Heading: "已放弃："}}
	if got, want := Render(w, set, places, nil, Whole), "正文：\n【人物】\n- 林晚、晚晚：林晚怕黑。 她是左撇子。\n- 周祥是她的徒弟？还是同事。\n【物品】\n- 铜钥匙：铜钥匙挂在她脖子上。不知道开哪扇门。\n\n已放弃：\n- 林晚会飞"; got != want {
		t.Fatalf("whole:\n%s", got)
	}
	if got, want := Render(w, set, places, nil, Short), "正文：\n【人物】\n- 人物 · 林晚：林晚怕黑\n- 人物：周祥是她的徒弟？\n【物品】\n- 物品 · 铜钥匙：铜钥匙挂在她脖子上\n\n已放弃：\n- 人物：林晚会飞"; got != want {
		t.Fatalf("index:\n%s", got)
	}
	if got := places[1].Item(set.Entries[2], Whole); !reflect.DeepEqual(got, workspace.ContextItem{ID: "c", Text: "已放弃：林晚会飞"}) {
		t.Fatalf("item %+v", got)
	}
	if got := Render(w, workspace.Setting{}, places, nil, Whole); got != "设定文档还是空的。" {
		t.Fatal(got)
	}
	b := NewBudget(slack+Estimate("- "+Whole(set.Entries[3]), workspace.Chinese)+Estimate("- "+Whole(set.Entries[0]), workspace.Chinese), workspace.Chinese)
	shown, left := b.Fit(set, []string{workspace.PlaceBody, workspace.PlaceDropped}, Whole)
	if len(shown) != 2 || !shown["d"] || !shown["a"] || len(left) != 2 || left[0].ID != "b" || left[1].ID != "c" {
		t.Fatalf("shown %v left %v", shown, left)
	}
	if got := Render(w, set, places, shown, Whole); strings.Contains(got, "林晚") || !strings.Contains(got, "周祥") {
		t.Fatalf("rendered what was not shown:\n%s", got)
	}
}

func binaryClip(s string, tokens int) string {
	if Estimate(s, workspace.Chinese) <= tokens {
		return s
	}
	r := []rune(s)
	n := sort.Search(len(r)+1, func(n int) bool { return Estimate(string(r[:n])+CutMark, workspace.Chinese) > tokens }) - 1
	return string(r[:max(n, 0)]) + CutMark
}

func TestClipTokensMatchesBinarySearch(t *testing.T) {
	check := func(s string) {
		t.Helper()
		for budget := -2; budget <= Estimate(s, workspace.Chinese)+2; budget++ {
			if got, want := ClipTokens(s, budget, workspace.Chinese), binaryClip(s, budget); got != want {
				t.Fatalf("ClipTokens(%q, %d) = %q, want %q", s, budget, got, want)
			}
		}
	}
	for _, mix := range []string{"a", "甲", "甲a", "ab甲cd乙", "\n\t甲 a😀", string([]byte{0xff, 'a', 0xfe})} {
		for n := 0; n <= 100; n++ {
			check(strings.Repeat(mix, n))
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	alphabet := []rune("abc甲乙丙😀 \n\t")
	for range 100 {
		r := make([]rune, rng.IntN(200))
		for i := range r {
			r[i] = alphabet[rng.IntN(len(alphabet))]
		}
		check(string(r))
	}
}

func TestFirstSentenceMeasure(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{strings.Repeat("甲", 61), strings.Repeat("甲", 60) + "…"},
		{strings.Repeat("a", 61), strings.Repeat("a", 60) + "…"},
		{"甲a\n乙bcdef", "甲a"},
		{strings.Repeat("甲a", 30), strings.Repeat("甲a", 30)},
	} {
		if got := firstSentence(c.text); got != c.want {
			t.Errorf("firstSentence(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
