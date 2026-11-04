package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
)

// Each model of a service has a row of its own: the numbers start from the
// table built in, may be written as 128K or 1M, and are written back so; a
// row that breaks a rule is named under the table, in red, and the form
// cannot be saved till it is mended.
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
	if len(a.serviceForm.limits) != 2 || a.serviceForm.limits[0].id != "deepseek-flash" || a.serviceForm.limits[0].window != "1M" || a.serviceForm.limits[0].output != "384K" || a.serviceForm.limits[0].at != "80" {
		t.Fatalf("rows %+v", a.serviceForm.limits)
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
	// save presses 保存; when want is not "", the form says it before,
	// and pressing does nothing.
	save := func(want string) {
		t.Helper()
		a.SetError(nil)
		setNotice(a, "")
		if want != "" && !tt.HasText(want) {
			t.Fatalf("the form does not say %q: %q", want, tt.Texts())
		}
		if err := tt.Click("保存"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if saved := noticeOf(a) == "已保存"; a.errorText() != "" || saved != (want == "") {
			t.Fatalf("saved %v, said %q", saved, a.errorText())
		}
	}
	retype("最大输出-1", "2M")
	save("deepseek-flash 的最大输出须小于上下文上限")
	retype("最大输出-1", "8 k")
	retype("压缩阈值-1", "99")
	save("deepseek-flash 的压缩阈值须在 50% 到 95% 之间")
	retype("压缩阈值-1", "七十")
	save("deepseek-flash 的压缩阈值须为整数")
	retype("上下文上限-1", "一百万")
	save("deepseek-flash 的上下文上限和最大输出须为正整数，可写成 128K、1M")
	if got := a.ai.Providers()[0].Models[0]; got.Window != 1_000_000 {
		t.Fatalf("a form that broke a rule was saved: %+v", got)
	}
	retype("上下文上限-1", "131072")
	retype("压缩阈值-1", "70")
	save("")
	if got := a.ai.Providers()[0].Models[0]; got != (aiport.Model{ID: "deepseek-flash", Window: 131072, Output: 8000, CompactAt: 70}) {
		t.Fatalf("saved %+v", got)
	}
	if r := a.serviceForm.limits[0]; r.window != "131072" || r.output != "8K" {
		t.Fatalf("written back as %+v", r)
	}

	// Rows go, and come: a new one starts from careful numbers, and an ID
	// the table knows brings its own.
	for range 2 {
		if err := tt.Click("移除-1"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if len(a.serviceForm.limits) != 0 || !tt.HasText("尚无模型，可获取可用模型或手动添加。") {
		t.Fatalf("rows %+v", a.serviceForm.limits)
	}
	if err := tt.Click("添加模型"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.serviceForm.limits) != 1 || a.serviceForm.limits[0].window != "32K" || a.serviceForm.limits[0].output != "4096" || a.serviceForm.limits[0].at != "80" {
		t.Fatalf("a new row %+v", a.serviceForm.limits)
	}
	retype("模型 ID-1", "kimi-k3")
	if r := a.serviceForm.limits[0]; r.window != "1048576" || r.output != "128K" {
		t.Fatalf("kimi-k3 brought %+v", r)
	}
	save("")
	if got := a.ai.Providers()[0].Models; len(got) != 1 || got[0].ID != "kimi-k3" {
		t.Fatalf("saved %+v", got)
	}
}

// A new service left without an ID takes one from its name, shown in the
// empty field; one another service has would take its place, and is said
// in red with the form waiting, as an address or a variable that cannot
// be used is, by what to change. Its form comes into view as it is
// added, and 取消 puts it away.
func TestANewServiceSaysWhatIsWrongWhereItIs(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	a.serviceForm.edit, a.serviceForm.providerID, a.serviceForm.limits = "+", "", nil
	taken, was := a.ai.Providers()[0], len(a.ai.Providers())
	a.serviceForm.provider = aiport.Provider{Name: taken.Name, BaseURL: "https://api.example.com/v1", Protocol: "chat"}
	tt.Frame()
	tt.Scroll(633, 400, 0, 1600) // down to the form
	tt.Frame()
	for _, c := range []struct {
		say  string
		form func(p *aiport.Provider)
	}{
		{"和「" + taken.Name + "」重名了，换一个", func(p *aiport.Provider) {}},
		{"地址要以 http:// 或 https:// 开头", func(p *aiport.Provider) { p.Name, p.BaseURL = "Open Router", "api.example.com" }},
		{"地址不完整", func(p *aiport.Provider) { p.BaseURL = "https://" }},
		{"地址里不要带账号，也不要带 ? 或 # 后面的部分", func(p *aiport.Provider) { p.BaseURL = "https://openrouter.ai/api/v1?key=1" }},
		{"环境变量名只能用字母、数字和 _，不能以数字开头", func(p *aiport.Provider) { p.BaseURL, p.Env = "https://openrouter.ai/api/v1", "1 KEY" }},
	} {
		c.form(&a.serviceForm.provider)
		tt.Frame()
		if !tt.HasText(c.say) {
			t.Fatalf("the form does not say %q: %q", c.say, tt.Texts())
		}
		if err := tt.Click("保存"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.errorText() != "" || noticeOf(a) != "" || len(a.ai.Providers()) != was {
			t.Fatalf("pressed with %q: %q %q", c.say, a.errorText(), noticeOf(a))
		}
	}
	a.serviceForm.provider.Env = "OPENROUTER_API_KEY"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ps := a.ai.Providers(); len(ps) != was+1 || ps[was].ID != "open-router" || ps[0].Name != taken.Name || len(ps[0].Models) != len(taken.Models) {
		t.Fatalf("saved %+v, said %q", ps, a.errorText())
	}
	// Another is added from the foot of the list, out of sight: its form
	// comes into view, and 取消 puts it away unsaved.
	a.serviceForm.edit = ""
	tt.Frame()
	tt.Scroll(633, 400, 0, 1600) // to the foot, 添加服务 at the bottom
	tt.Frame()
	if err := tt.Click("+ 添加服务"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	// As much of it as fits, from its top: not its first row at the bottom.
	if r, ok := tt.Find("名称"); !ok || r.Y < 0 || r.Y > 390 {
		t.Fatal("a new service opens out of sight", r, ok)
	}
	a.serviceForm.key = "unsaved-test-key"
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.serviceForm.edit != "" || tt.HasText("名称") || len(a.ai.Providers()) != was+1 {
		t.Fatal("取消 left the new service's form", a.serviceForm.edit)
	}
	if a.serviceForm.key != "" || a.serviceForm.provider.ID != "" || len(a.serviceForm.limits) != 0 {
		t.Fatal("cancel kept the discarded service draft")
	}
	for name, id := range map[string]string{"DeepSeek": "deepseek", "月之暗面 Kimi 2": "kimi-2", "my_api-": "my_api", "通义": ""} {
		if got := idFrom(name); got != id {
			t.Errorf("%s: %q", name, got)
		}
	}
}
