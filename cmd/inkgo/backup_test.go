package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/workspace"
)

func TestApplicationBackupAssemblyAsksButTestAssemblyDoesNot(t *testing.T) {
	for _, tc := range []struct {
		name string
		mods []ext.Module
		ask  bool
	}{{"production", modules, true}, {"tests", testModules(), false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "work")
			st, err := workspace.Create(dir, "测试作品", "网文")
			if err != nil {
				t.Fatal(err)
			}
			st.Close()
			a := newModuleApp(filepath.Join(root, "providers.yaml"), tc.mods...)
			defer a.Close()
			if err := a.Open(dir); err != nil {
				t.Fatal(err)
			}
			tt := ui.NewTester(a.View, 1266, 780)
			tt.Frame()
			if tt.HasText("作品目前仅保存在本机") != tc.ask {
				t.Fatal("incorrect first-open question", tt.Texts())
			}
			found := false
			for _, m := range tc.mods {
				if m.ID == "backup" {
					found = true
				}
			}
			if !found {
				t.Fatal("backup missing")
			}
		})
	}
}
