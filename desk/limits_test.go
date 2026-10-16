package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
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
	// save presses 保存提供商; when want is not "", the form says it before,
	// and pressing does nothing.
	save := func(want string) {
		t.Helper()
		a.Error, a.notice = "", ""
		if want != "" && !tt.HasText(want) {
			t.Fatalf("the form does not say %q: %q", want, tt.Texts())
		}
		if err := tt.Click("保存提供商"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if saved := a.notice == "提供商已保存"; a.Error != "" || saved != (want == "") {
			t.Fatalf("saved %v, said %q", saved, a.Error)
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

// A new service left without an ID takes one from its name, shown in the
// empty field; one another service has would take its place, and is said
// in red with the form waiting, as an address or a variable that cannot
// be used is, by what to change. Its form comes into view as it is
// added, and 取消 puts it away.
func TestANewServiceSaysWhatIsWrongWhereItIs(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	a.bk.edit, a.providerID, a.limits = "+", "", nil
	taken, was := a.ai.Providers()[0], len(a.ai.Providers())
	a.provider = aiport.Provider{Name: taken.Name, BaseURL: "https://api.example.com/v1", Protocol: "chat"}
	tt.Frame()
	tt.Scroll(633, 400, 0, 1600) // down to the form
	tt.Frame()
	for _, c := range []struct {
		say  string
		form func(p *aiport.Provider)
	}{
		{"该 ID 已被「" + taken.Name + "」使用，请换一个", func(p *aiport.Provider) {}},
		{"API 地址须以 http:// 或 https:// 开头", func(p *aiport.Provider) { p.Name, p.BaseURL = "Open Router", "api.example.com" }},
		{"API 地址里没有域名", func(p *aiport.Provider) { p.BaseURL = "https://" }},
		{"API 地址不能带账号、查询参数或锚点", func(p *aiport.Provider) { p.BaseURL = "https://openrouter.ai/api/v1?key=1" }},
		{"环境变量名只能用字母、数字和 _，不能以数字开头", func(p *aiport.Provider) { p.BaseURL, p.Env = "https://openrouter.ai/api/v1", "1 KEY" }},
	} {
		c.form(&a.provider)
		tt.Frame()
		if !tt.HasText(c.say) {
			t.Fatalf("the form does not say %q: %q", c.say, tt.Texts())
		}
		if err := tt.Click("保存提供商"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.Error != "" || a.notice != "" || len(a.ai.Providers()) != was {
			t.Fatalf("pressed with %q: %q %q", c.say, a.Error, a.notice)
		}
	}
	a.provider.Env = "OPENROUTER_API_KEY"
	tt.Frame()
	if err := tt.Click("保存提供商"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ps := a.ai.Providers(); len(ps) != was+1 || ps[was].ID != "open-router" || ps[0].Name != taken.Name || len(ps[0].Models) != len(taken.Models) {
		t.Fatalf("saved %+v, said %q", ps, a.Error)
	}
	// Another is added from the foot of the list, out of sight: its form
	// comes into view, and 取消 puts it away unsaved.
	a.bk.edit = ""
	tt.Frame()
	tt.Scroll(633, 400, 0, 1600) // to the foot, 添加服务 at the bottom
	tt.Frame()
	if err := tt.Click("+ 添加服务"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	// As much of it as fits, from its top: not its first row at the bottom.
	if r, ok := tt.Find("显示名称"); !ok || r.Y < 0 || r.Y > 390 {
		t.Fatal("a new service opens out of sight", r, ok)
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.bk.edit != "" || tt.HasText("显示名称") || len(a.ai.Providers()) != was+1 {
		t.Fatal("取消 left the new service's form", a.bk.edit)
	}
	for name, id := range map[string]string{"DeepSeek": "deepseek", "月之暗面 Kimi 2": "kimi-2", "my_api-": "my_api", "通义": ""} {
		if got := idFrom(name); got != id {
			t.Errorf("%s: %q", name, got)
		}
	}
}
