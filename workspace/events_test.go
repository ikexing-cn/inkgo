package workspace

import (
	"reflect"
	"slices"
	"testing"
)

func TestLooks(t *testing.T) {
	events := []Event{
		{Role: RoleCall, ReplyTo: "one", Calls: []Call{{ID: "a", Label: "查正文"}, {ID: "p", Name: ProposeTool, Label: "提议"}, {ID: "b", Label: "记设定"}}},
		{Role: RoleResult, ReplyTo: "one", Call: "b", Text: "记好了", Revision: "revision"},
		{Role: RoleCall, ReplyTo: "two", Calls: []Call{{ID: "other", Label: "另一次查阅"}}},
		{Role: RoleResult, ReplyTo: "two", Call: "a", Text: "其他轮的同名调用"},
		{Role: RoleResult, ReplyTo: "one", Call: "p", Proposal: &Proposal{Text: "新正文"}},
		{Role: RoleResult, ReplyTo: "one", Call: "a", Text: "查到了", Failed: true},
		{Role: RoleCall, ReplyTo: "one", Calls: []Call{{ID: "running", Label: "正在查"}}},
	}
	got := Looks(events, "one")
	if len(got) != 3 || got[0].Label != "查正文" || got[0].Result.Text != "查到了" || !got[0].Result.Failed || got[1].Label != "记设定" || got[1].Result.Revision != "revision" || got[2].Label != "正在查" || !reflect.DeepEqual(got[2].Result, Event{}) {
		t.Fatal(got)
	}
	all := Looks(events, "")
	if len(all) != 4 || all[2].Label != "另一次查阅" || all[3].Label != "正在查" {
		t.Fatal(all)
	}
	if got := Looks(events, "missing"); got != nil {
		t.Fatal(got)
	}
	if got := Looks(nil, ""); got != nil {
		t.Fatal(got)
	}
}

func TestSpent(t *testing.T) {
	reply := Event{ID: "reply", ReplyTo: "one", Usage: &Usage{Input: 10, CacheRead: 2, CacheWrite: 1, Output: 3, Reasoning: 1}}
	events := []Event{
		{Role: RoleCall, ReplyTo: "one", Usage: &Usage{Input: 20, CacheRead: 4, CacheWrite: 2, Output: 6, Reasoning: 2}},
		{Role: RoleCall, ReplyTo: "two", Usage: &Usage{Input: 999}},
		{Role: RoleResult, ReplyTo: "one", Usage: &Usage{Input: 999}},
		{Role: RoleCall, ReplyTo: "one"},
		{Role: RoleCall, ReplyTo: "one", Usage: &Usage{Input: 30, CacheRead: 6, CacheWrite: 3, Output: 9, Reasoning: 3}},
		reply,
	}
	if got, ok := Spent(events, reply); !ok || got != (Usage{Input: 60, CacheRead: 12, CacheWrite: 6, Output: 18, Reasoning: 6}) {
		t.Fatal(got, ok)
	}
	if got, ok := Spent(nil, reply); !ok || got != *reply.Usage {
		t.Fatal(got, ok)
	}
	legacy := Event{ReplyTo: "one"}
	if got, ok := Spent(events, legacy); ok || got != (Usage{}) {
		t.Fatal(got, ok)
	}
	if reply.Usage.Input != 10 || events[0].Usage.Input != 20 {
		t.Fatal("aggregation mutated recorded usage")
	}
}

func TestRequests(t *testing.T) {
	reply := Event{ReplyTo: "one", Usage: &Usage{}}
	events := []Event{
		{Role: RoleCall, ReplyTo: "one", Usage: &Usage{}},
		{Role: RoleCall, ReplyTo: "one"},
		{Role: RoleCall, ReplyTo: "two", Usage: &Usage{}},
		{Role: RoleResult, ReplyTo: "one", Usage: &Usage{}},
		{Role: RoleCall, ReplyTo: "one", Usage: &Usage{}},
	}
	if got := Requests(events, reply); got != 3 {
		t.Fatal(got)
	}
	if got := Requests(nil, Event{}); got != 1 {
		t.Fatal(got)
	}
	if got := Requests([]Event{{Role: RoleCall, ReplyTo: "one"}}, reply); got != 1 {
		t.Fatal(got)
	}
}

func TestFateOf(t *testing.T) {
	events := []Event{
		{Role: RoleFate, Call: "call", Fate: FateReplaced},
		{Role: RoleResult, Call: "call", Fate: FateVoid},
		{Role: RoleFate, Call: "other", Fate: FateDropped},
		{Role: RoleFate, Call: "call", Fate: FateTaken},
		{Role: RoleFate, Call: "call", Fate: FateOff},
	}
	for _, tc := range []struct {
		from       int
		call, want string
	}{
		{0, "call", FateReplaced}, {1, "call", FateTaken}, {4, "call", FateOff},
		{len(events), "call", ""}, {0, "missing", ""}, {0, "other", FateDropped},
	} {
		if got := FateOf(events, tc.from, tc.call); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	if got := FateOf(nil, 0, "call"); got != "" {
		t.Fatal(got)
	}
}

func TestADigestSplitsAtItsHeadings(t *testing.T) {
	got := DigestSections("开头的话\n## 在聊什么\n第一行\n\n第二行\n##不是标题\n## 下一步\n")
	want := [][2]string{{"", "开头的话"}, {"在聊什么", "第一行\n\n第二行\n##不是标题"}, {"下一步", ""}}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
}
