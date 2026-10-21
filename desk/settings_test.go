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
