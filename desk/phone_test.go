package desk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"writer/shelf"
	"writer/workspace"
)

const phoneLongText = "雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。"

func TestPhonePreviewOpensAndKeepsItsTabs(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	a.doc.hidden = true
	tt.Frame()
	if err := clickMenu(tt, "视图", "菜单-手机预览"); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		tt.Frame()
	}
	if a.doc.hidden || shownTab(a) != panePhone || !tt.HasText("第 1 屏，共 1 屏") {
		t.Fatal("preview did not open", tt.Texts())
	}
	a.shelf.NoAI = true
	tt.Frame()
	if _, ok := tt.Find("右栏-预览"); !ok {
		t.Fatal("AI off hid preview")
	}
	if _, ok := tt.Find("右栏-讨论"); ok {
		t.Fatal("AI off retained talk tab")
	}
	click(t, tt, "右栏-设定")
	if shownTab(a) != paneSetting || !tt.HasText("主角怕黑") {
		t.Fatal("cannot leave preview")
	}
	click(t, tt, "右栏-预览")
}

func TestPhonePreferencesAreLocal(t *testing.T) {
	a, tt, _ := writingFixture(t, "灯灭了。")
	if a.shelf.Phone != nil {
		t.Fatal("untouched phone preferences should be absent")
	}
	a.openPhone()
	tt.Frame()
	click(t, tt, "滚动")
	click(t, tt, "Aa")
	for _, label := range []string{"宽屏", "A+", "宽松", "护眼", "显示手机外框", "标出超过 5 行的段落"} {
		click(t, tt, label)
	}
	want := a.phone.Preferences()
	if want.Mode != "scroll" || want.Screen != "wide" || want.Size != 21 || want.Spacing != "loose" || want.Theme != "eye" || !want.NoFrame || !want.NoLong {
		t.Fatal(want)
	}
	config := a.ai.ConfigFile()
	data, err := os.ReadFile(shelf.File(config))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Phone *shelf.Phone `json:"phone"`
	}
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Phone == nil || *saved.Phone != want {
		t.Fatalf("works.json phone: %s", data)
	}
	for _, dir := range []string{filepath.Dir(config), filepath.Join(a.Store.Root(), ".writer")} {
		if _, err = os.Stat(filepath.Join(dir, "phone.json")); !os.IsNotExist(err) {
			t.Fatal("separate phone.json", dir, err)
		}
	}
	b := newApp(config)
	defer b.Close()
	if b.phone.Preferences() != want {
		t.Fatalf("reopened %+v want %+v", b.phone.Preferences(), want)
	}
	root := filepath.Join(t.TempDir(), "other")
	store, err := workspace.Create(root, "另一本", "网文")
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err = b.Open(root); err != nil {
		t.Fatal(err)
	}
	if b.phone.Preferences() != want {
		t.Fatal("work changed phone preferences")
	}
	if _, err = os.Stat(filepath.Join(root, ".writer", "phone.json")); !os.IsNotExist(err) {
		t.Fatal("phone preferences were written in work")
	}
}

func TestPhonePreferencesReadUnknownChoices(t *testing.T) {
	a, _, _ := writingFixture(t, "灯灭了。")
	config := a.ai.ConfigFile()
	for _, tc := range []struct {
		name  string
		phone *shelf.Phone
	}{
		{"untouched", nil},
		{"missing choices", &shelf.Phone{}},
		{"unknown choices", &shelf.Phone{Mode: "future", Screen: "future", Size: 25, Spacing: "future", Theme: "future"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.shelf.Phone = tc.phone
			a.saveShelf()
			b := newApp(config)
			defer b.Close()
			if b.errorText() != "" {
				t.Fatalf("read defaults: %+v, error %q", b.phone.Preferences(), b.errorText())
			}
			if tc.phone == nil && b.shelf.Phone != nil {
				t.Fatal("reading untouched preferences materialized defaults")
			}
		})
	}
}
