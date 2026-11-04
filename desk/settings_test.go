package desk

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Every settings close route cancels only the pending question. The tab,
// service draft and backup task belong to their own lifetimes.
func TestSettingsCloseKeepsTheWorkAndBackup(t *testing.T) {
	for _, how := range []string{"button", "escape", "backdrop", "relaunch"} {
		t.Run(how, func(t *testing.T) {
			a, tt, _ := fixture(t)
			a.openPrefs("appearance")
			aiViewOf(a).ServiceForm.New()
			aiViewOf(a).ServiceForm.Provider.Name = "unsaved service"
			backupBefore := backupOf(a).State
			tt.Frame()
			switch how {
			case "button":
				click(t, tt, "关闭")
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "backdrop":
				tt.ClickAt(5, 400)
			case "relaunch":
				a.settings.askAI = true
				a.Relaunch = func() {}
				a.relaunch()
			}
			tt.Frame()
			if a.settings.open || a.settings.askAI || a.settings.tab != "appearance" {
				t.Fatal("settings did not close cleanly", a.settings)
			}
			if aiViewOf(a).ServiceForm.Provider.Name != "unsaved service" || !reflect.DeepEqual(backupOf(a).State, backupBefore) {
				t.Fatal("closing settings changed a draft or backup state")
			}
			a.openPrefs(a.settings.tab)
			tt.Frame()
			if !a.settings.open || a.settings.tab != "appearance" || a.settings.askAI {
				t.Fatal("settings did not reopen at the last tab")
			}
		})
	}
}

// A part of the settings with nothing to say under its title says nothing
// there, not an empty line; one that does still says it.
func TestSettingsSayNothingNeedless(t *testing.T) {
	a, tt, _ := fixture(t)
	for tab, note := range map[string]string{"AI": "", "backup.settings": "", "appearance": "这台电脑上的所有作品", "reminders": "只管这部作品"} {
		a.openPrefs(tab)
		tt.Frame()
		texts := tt.Texts()
		at := -1 // the list beside names the parts too; the title comes after it
		for i, s := range texts {
			if s == a.prefTitle(tab) {
				at = i
			}
		}
		if at < 0 || at+1 >= len(texts) {
			t.Fatal(tab, texts)
		}
		if a.prefNote(tab) != note || note == "" && texts[at+1] == "" || note != "" && texts[at+1] != note {
			t.Fatalf("%s: note %q, then %q", tab, a.prefNote(tab), texts[at+1])
		}
	}
}

// While 关闭显卡加速 is not offered, the settings do not show it, and a
// machine that ticked it draws on the GPU again.
func TestNoGPUNotOffered(t *testing.T) {
	offerNoGPU = false
	t.Cleanup(func() { offerNoGPU = true })
	a, tt, _ := fixture(t)
	a.shelf.NoGPU = true
	a.openPrefs("appearance")
	tt.Frame()
	if tt.HasText("关闭显卡加速") || a.NoGPU() {
		t.Fatal("not offered, yet shown or drawn by", tt.Texts())
	}
}

// 关闭显卡加速 is kept for this machine and waits for the software to open
// again, which the settings say; the link closes them and relaunches,
// asking first about what the AI proposed, as closing the window does.
func TestNoGPUWaitsForTheNextStart(t *testing.T) {
	a, tt, _ := writingFixture(t, "她走到井边。")
	f := &script{}
	penWith(a, f)
	f.then(proposal("第一章", "end", "", "井里有光。"), []string{"接了一句。"})
	setChapterOutline(a, "井里有光。")
	a.openDraft()
	tt.Frame()
	click(t, tt, "开始起草")
	replied(t, a, tt)

	a.openPrefs("appearance")
	tt.Frame()
	click(t, tt, "关闭显卡加速")
	if !tt.HasText("重启软件后生效。") || tt.HasText("立即重启") {
		t.Fatal("ticked, it does not say it waits, or offers what cannot be done", tt.Texts())
	}
	b := newApp(a.ai.ConfigFile())
	if !b.NoGPU() || b.ranNoGPU != b.NoGPU() {
		t.Fatal("not kept for the next start")
	}
	b.Close()
	click(t, tt, "关闭显卡加速")
	if tt.HasText("重启软件后生效。") {
		t.Fatal("as the software opened, and still it waits")
	}

	relaunched := 0
	a.Relaunch = func() { relaunched++ }
	click(t, tt, "关闭显卡加速")
	click(t, tt, "立即重启")
	if relaunched != 0 || a.settings.open || !tt.HasText("还有 1 处改稿建议没有处理") {
		t.Fatal("relaunched without asking about the proposal", relaunched, tt.Texts())
	}
	click(t, tt, "丢弃并关闭")
	if relaunched != 1 || len(a.write.props) != 0 {
		t.Fatal("let go, it did not relaunch", relaunched)
	}
}
