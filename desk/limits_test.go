package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
)

// Each model of a service has a row of its own: the numbers start from the
// table built in, may be written as 128K or 1M, and are written back so; a
// row that breaks a rule is named and nothing is saved.
func TestModelLimitsInTheServiceForm(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	tt.Frame()
	tt.Scroll(633, 400, 0, 400) // the services, under what the work uses and spent
	tt.Frame()
	if err := tt.Click("提供商-deepseek"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Scroll(633, 400, 0, 1200) // down to the form
	tt.Frame()
	for _, s := range []string{"上下文上限", "最大输出", "压缩阈值 %", "添加模型"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	if len(a.limits) != 2 || a.limits[0].id != "deepseek-flash" || a.limits[0].window != "1M" || a.limits[0].output != "384K" || a.limits[0].at != "80" {
		t.Fatalf("rows %+v", a.limits)
	}
	retype := func(label, text string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(text)
		tt.Frame()
	}
	save := func(want string) {
		t.Helper()
		a.Error = ""
		if err := tt.Click("保存提供商"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.Error != want {
			t.Fatalf("saving said %q, not %q", a.Error, want)
		}
	}
	retype("最大输出-1", "2M")
	save("deepseek-flash的最大输出须小于上下文上限")
	retype("最大输出-1", "8 k")
	retype("压缩阈值-1", "99")
	save("deepseek-flash的压缩阈值须在 50% 到 95% 之间")
	retype("压缩阈值-1", "七十")
	save("deepseek-flash的压缩阈值须为整数")
	retype("上下文上限-1", "一百万")
	save("deepseek-flash的上下文上限和最大输出须为正整数，可写成 128K、1M")
	if got := a.ai.Providers()[0].Models[0]; got.Window != 1_000_000 {
		t.Fatalf("a form that broke a rule was saved: %+v", got)
	}
	retype("上下文上限-1", "131072")
	retype("压缩阈值-1", "70")
	save("")
	if got := a.ai.Providers()[0].Models[0]; got != (aiport.Model{ID: "deepseek-flash", Window: 131072, Output: 8000, CompactAt: 70}) {
		t.Fatalf("saved %+v", got)
	}
	if r := a.limits[0]; r.window != "131072" || r.output != "8K" {
		t.Fatalf("written back as %+v", r)
	}

	// Rows go, and come: a new one starts from careful numbers, and an ID
	// the table knows brings its own.
	for range 2 {
		if err := tt.Click("移除模型-1"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if len(a.limits) != 0 || !tt.HasText("尚无模型，可获取可用模型或手动添加。") {
		t.Fatalf("rows %+v", a.limits)
	}
	if err := tt.Click("添加模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.limits) != 1 || a.limits[0].window != "32K" || a.limits[0].output != "4096" || a.limits[0].at != "80" {
		t.Fatalf("a new row %+v", a.limits)
	}
	retype("模型 ID-1", "kimi-k3")
	if r := a.limits[0]; r.window != "1048576" || r.output != "128K" {
		t.Fatalf("kimi-k3 brought %+v", r)
	}
	save("")
	if got := a.ai.Providers()[0].Models; len(got) != 1 || got[0].ID != "kimi-k3" {
		t.Fatalf("saved %+v", got)
	}
}
