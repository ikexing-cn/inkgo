package kit

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSplitToast(t *testing.T) {
	for _, c := range []struct{ text, title, detail string }{
		{"网络失败：请稍后重试", "网络失败", "请稍后重试"},
		{"已保存。", "已保存", ""},
		{"需要处理；  再试一次  ", "需要处理", "再试一次"},
		{"提示，后面还有话", "提示", "后面还有话"},
		{"一句话", "一句话", ""},
		{"：开头不是标题", "：开头不是标题", ""},
		{"", "", ""},
	} {
		title, detail := SplitToast(c.text)
		if title != c.title || detail != c.detail {
			t.Errorf("SplitToast(%q) = %q, %q; want %q, %q", c.text, title, detail, c.title, c.detail)
		}
	}
}

func TestToastActions(t *testing.T) {
	p := LightPalette()
	var done []string
	tt := ui.NewTester(func(c *ui.Context) {
		p.ToastActions(c, []ToastAction{
			{Label: "重试", Fn: func() { done = append(done, "重试") }},
			{Label: "复制详情", Name: "复制错误详情", Fn: func() { done = append(done, "复制详情") }},
		})
	}, 400, 200)
	copyAt, ok := tt.Find("复制错误详情")
	if !ok {
		t.Fatal("the action's name was lost")
	}
	retryAt, ok := tt.Find("重试")
	if !ok || copyAt.X >= retryAt.X {
		t.Fatal("the likely action is not at the right", copyAt, retryAt)
	}
	if err := tt.Click("复制错误详情"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []string{"复制详情", "重试"}) {
		t.Fatal(done)
	}
}
