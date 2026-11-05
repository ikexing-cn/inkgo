package propose

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// A proposal finds its place: the chapter named, and in it the words it
// replaces or follows, there once.
func TestAProposalFindsItsPlace(t *testing.T) {
	st, err := workspace.Create(t.TempDir(), "测试", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	volume, err := st.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := st.NewChapter(volume, "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.WriteChapter(two, "她推开门。\n灯灭了。\n灯灭了。她没动。", nil); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		in   proposeArgs
		want string // in the refusal
		got  workspace.Proposal
	}{
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "“她推开门”", Text: "她撞开门。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "replace", Quote: "她推开门", Text: "她撞开门。"}},
		{in: proposeArgs{Chapter: "第二章", At: "after", Quote: "她没动。", Text: "\n她等着。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "after", Quote: "她没动。", Text: "\n她等着。"}},
		{in: proposeArgs{Chapter: "第二章", At: "all", Text: "全新的一章。"}, got: workspace.Proposal{Chapter: two, Title: "第二章", At: "all", Text: "全新的一章。"}},
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "灯灭了。", Text: "x"}, want: "出现了 2 次"},
		{in: proposeArgs{Chapter: "第二章", At: "replace", Quote: "她跑了", Text: "x"}, want: "找不到 quote"},
		{in: proposeArgs{Chapter: "第二章", At: "after", Text: "x"}, want: "要在 quote 里逐字复制"},
		{in: proposeArgs{Chapter: "第一卷", At: "end", Text: "x"}, want: "是一卷"},
		{in: proposeArgs{Chapter: "第九章", At: "end", Text: "x"}, want: "没有叫「第九章」的章"},
		{in: proposeArgs{Chapter: "第二章", At: "middle", Text: "x"}, want: "at 只能是"},
		{in: proposeArgs{Chapter: "第二章", At: "end", Text: " \n "}, want: "text 要写新的正文"},
	} {
		raw, _ := json.Marshal(c.in)
		out := runProposal(context.Background(), aiapi.ToolEnv{Store: st, Shared: aiapi.NewToolShared(st)}, raw)
		switch {
		case c.want != "" && (!out.Failed || !strings.Contains(out.Text, c.want) || out.Proposal != nil):
			t.Errorf("%+v: %s", c.in, out.Text)
		case c.want == "" && (out.Failed || out.Proposal == nil || *out.Proposal != c.got):
			t.Errorf("%+v: %s %+v", c.in, out.Text, out.Proposal)
		}
	}
}
