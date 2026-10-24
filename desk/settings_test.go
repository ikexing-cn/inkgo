package desk

import "testing"

// A part of the settings with nothing to say under its title says nothing
// there, not an empty line; one that does still says it.
func TestSettingsSayNothingNeedless(t *testing.T) {
	a, tt, _ := fixture(t)
	for tab, note := range map[string]string{"AI": "", "备份": "", "外观": "仅在本机生效，适用于所有作品。", "提醒与检查": "仅适用于本作品。"} {
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

// 禁用 GPU 加速 is kept for this machine and waits for the software to open
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

	a.openPrefs("外观")
	tt.Frame()
	click(t, tt, "禁用 GPU 加速")
	if !tt.HasText("重新打开软件后生效。") || tt.HasText("立即重新打开") {
		t.Fatal("ticked, it does not say it waits, or offers what cannot be done", tt.Texts())
	}
	b := newApp(a.ai.ConfigFile())
	if !b.NoGPU() || b.ranNoGPU != b.NoGPU() {
		t.Fatal("not kept for the next start")
	}
	b.Close()
	click(t, tt, "禁用 GPU 加速")
	if tt.HasText("重新打开软件后生效。") {
		t.Fatal("as the software opened, and still it waits")
	}

	relaunched := 0
	a.Relaunch = func() { relaunched++ }
	click(t, tt, "禁用 GPU 加速")
	click(t, tt, "立即重新打开")
	if relaunched != 0 || a.bk.prefs || !tt.HasText("还有 1 处 AI 提议没有处理") {
		t.Fatal("relaunched without asking about the proposal", relaunched, tt.Texts())
	}
	click(t, tt, "丢弃并关闭")
	if relaunched != 1 || len(a.write.props) != 0 {
		t.Fatal("let go, it did not relaunch", relaunched)
	}
}
