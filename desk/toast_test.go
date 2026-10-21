package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/aiport"
	"writer/internal/testshot"
)

// An error over the settings shows once, above the dialog, with what can be
// done about it; closing it clears it.
func TestToastOverTheSettings(t *testing.T) {
	a, tt, _ := fixture(t)
	a.openPrefs("AI")
	a.SetError(&aiport.SetupError{Err: errors.New("连不上 AI：检查网络和服务地址")})
	tt.Frame()
	tt.Frame()
	if n := strings.Count(strings.Join(tt.Texts(), "\n"), "连不上 AI"); n != 1 {
		t.Fatalf("shown %d times: %q", n, tt.Texts())
	}
	for _, s := range []string{"检查网络和服务地址", "打开设置", "复制详情"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	// What it offers is at the right, under the words and not from their
	// left edge; the one that matters more last.
	words, _ := tt.Find("检查网络和服务地址")
	copyIt, _ := tt.Find("复制详情")
	prefs, ok := tt.Find("打开设置")
	if !ok || copyIt.X <= words.X || prefs.X <= copyIt.X+copyIt.W {
		t.Fatalf("what the toast offers is not at the right: %v %v under %v", copyIt, prefs, words)
	}
	if err := tt.Click("复制详情"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != a.errorText() {
		t.Fatalf("copied %q", tt.Clipboard())
	}
	if err := tt.Click("关闭提示"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.errorText() != "" || tt.HasText("连不上 AI") {
		t.Fatal(a.errorText(), tt.Texts())
	}
}

// Going to the settings it points at is what the toast asked for: the
// settings open and the toast goes, as in the prototype.
func TestOpeningTheSettingsFromAToastClearsIt(t *testing.T) {
	a, tt, _ := fixture(t)
	a.SetError(&aiport.SetupError{Err: errors.New("尚未选择讨论模型：请在设置中「AI」配置里选择")})
	tt.Frame()
	if err := tt.Click("打开设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.errorText() != "" || tt.HasText("尚未选择讨论模型") || !tt.HasText("关闭 AI") {
		t.Fatal(a.errorText(), tt.Texts())
	}
}

func TestToastSettingsFollowTheErrorType(t *testing.T) {
	a, tt, _ := fixture(t)
	cause := errors.New("服务需要重新选择")
	wrapped := fmt.Errorf("请求失败：%w", &aiport.SetupError{Err: cause})
	a.SetError(wrapped)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("打开设置") || !errors.Is(a.err, cause) {
		t.Fatal("wrapped setup error lost its settings action or cause", tt.Texts())
	}
	for _, err := range []error{errors.New("模型回复被截断"), errors.New("模型请求超时：太慢了"), errors.New("AI 回复不是合法 JSON")} {
		a.SetError(err)
		tt.Frame()
		tt.Frame()
		if tt.HasText("打开设置") || !tt.HasText("复制详情") {
			t.Fatal(err, tt.Texts())
		}
	}
}

// A request that fails the same way again counts up in the one toast, which
// can run it again.
func TestRepeatedFailureCounts(t *testing.T) {
	a, tt, _ := fixture(t)
	var vc *ui.Context
	tt = ui.NewTester(func(c *ui.Context) { vc = c; a.View(c) }, 1266, 780)
	tt.Frame()
	tries := 0
	fail := func(context.Context) (func(), error) {
		tries++
		return nil, errors.New("模型请求超时：太慢了")
	}
	a.run(vc, "测试中的请求", fail)
	settle(t, a, tt)
	tt.Frame()
	if !tt.HasText("重试") || tt.HasText("×2") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	settle(t, a, tt)
	tt.Frame()
	if tries != 2 || !tt.HasText("×2") {
		t.Fatal(tries, tt.Texts())
	}
	if n := strings.Count(strings.Join(tt.Texts(), "\n"), "模型请求超时"); n != 1 {
		t.Fatalf("shown %d times", n)
	}
}

// More than two toasts fold the oldest into a line that opens them.
func TestToastsFold(t *testing.T) {
	a, tt, _ := fixture(t)
	a.SetError(errors.New("出错了"))
	tt.Frame()
	a.busy = "正在整理"
	tt.Frame()
	a.notice = "已保存"
	tt.Frame()
	tt.Frame()
	if !tt.HasText("还有 1 条") || tt.HasText("出错了") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("还有 1 条"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("出错了") || !tt.HasText("正在整理") || !tt.HasText("已保存") {
		t.Fatal(tt.Texts())
	}
}

// A word on what to do first has no mark: it is neither done nor wrong.
func TestAHintHasNoMark(t *testing.T) {
	a, tt, _ := fixture(t)
	a.notice, a.hint = "已经存好", "请先选中"
	tt.Frame()
	tt.Frame()
	done, ok := tt.Find("已经存好")
	hint, ok2 := tt.Find("请先选中")
	if !ok || !ok2 || hint.X > done.X-iconMd {
		t.Fatal(done, hint, tt.Texts())
	}
}

// TestShotToasts writes screenshots of the toasts when SHOT is a directory:
// SHOT=dir go test ./desk -run ShotToasts
func TestShotToasts(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)

	tt.SetDark(true)
	a.openPrefs("AI")
	a.SetError(&aiport.SetupError{Err: errors.New("连不上 AI：检查网络和服务地址")})
	shot(t, tt, "toast-settings-dark.png")
	tt.SetDark(false)
	a.bk.prefs = false
	a.SetError(nil)
	tt.Frame()
	a.page = pageWrite
	a.notice = "已保存。停笔片刻也会自动保存"
	shot(t, tt, "toast-saved.png")
	a.SetError(&aiport.SetupError{Err: errors.New("连不上 AI：检查网络和服务地址")})
	tt.Frame()
	a.busy = "正在读取可用模型…"
	shot(t, tt, "toast-stack.png")
}
