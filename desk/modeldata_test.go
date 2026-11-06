package desk

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	aimod "writer/ext/ai"
)

type fakeModelUpdater struct {
	mu      sync.Mutex
	state   aimod.ModelDataState
	fail    bool
	enabled bool
}

func (f *fakeModelUpdater) Status() aimod.ModelDataState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}
func (f *fakeModelUpdater) Sync(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		f.state.Failure = "connect"
		return errors.New("offline")
	}
	f.state.Success = time.Now()
	f.state.Failure = ""
	return nil
}
func (f *fakeModelUpdater) SetEnabled(on bool) { f.enabled = on }
func (*fakeModelUpdater) OnChange(func())      {}
func (*fakeModelUpdater) Close()               {}
func TestAIModelData(t *testing.T) {
	a, tt, _ := fixture(t)
	f := &fakeModelUpdater{enabled: true}
	aiViewOf(a).SetModelUpdater(f)
	a.openPrefs("AI")
	tt.Frame()
	for _, s := range []string{"模型资料", "后台自动同步", "还没同步过，用的是软件自带的资料", "立即同步"} {
		if !tt.HasText(s) {
			t.Fatal(s, tt.Texts())
		}
	}
	scrollToLabel(t, tt, "模型资料-后台自动同步")
	if err := tt.Click("模型资料-后台自动同步"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()

	shot(t, tt, "model-data")
	for _, setting := range a.extensions.Ledger("ai").Settings {
		if setting.ID == "ai.models-auto" && setting.Read().(bool) {
			t.Fatal("switch not persisted")
		}
	}
	if f.enabled {
		t.Fatal("switch not applied")
	}
	for _, fail := range []bool{false, true} {
		f.fail = fail
		if err := tt.Click("立即同步"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Frame()
		deadline := time.Now().Add(time.Second)
		for tt.HasText("正在同步模型资料…") && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			tt.Frame()
		}
		if fail {
			if !tt.HasText("上次同步没成功：无法连接 models.dev") {
				t.Fatal(tt.Texts())
			}
		} else if noticeOf(a) != "模型资料已同步" || !slices.ContainsFunc(tt.Texts(), func(s string) bool { return strings.HasPrefix(s, "上次同步：今天 ") }) {
			t.Fatal(noticeOf(a), tt.Texts())
		}
	}
}
