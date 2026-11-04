package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"writer/ai"
	"writer/writing"
)

// TestCopySaysWhereTheModelsAre holds the copy to DESIGN.md: what sends the
// author to the models names the place as the settings show it, 设置中「AI」
// 配置, never a 模型设置 there is none of; the line the prototype shows
// when no model writes is the same here.
func TestCopySaysWhereTheModelsAre(t *testing.T) {
	var files []string
	for _, dir := range []string{".", "../ai", "../model", "../writing", "../discussion"} {
		got, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, got...)
	}
	if len(files) < 40 {
		t.Fatalf("%d files read: the check no longer finds them", len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"模型设置", "设置的「AI」", "在设置中提高", "作品预算", "提高预算"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s says %s: write 设置中「AI」配置", f, bad)
			}
		}
	}
	if got := testApp(ai.New("")).errText(writing.ErrNoModel); got != "请在设置中「AI」配置里选择续写和检查模型" {
		t.Errorf("ErrNoModel says %q, not the prototype's line", got)
	}
}
