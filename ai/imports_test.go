package ai

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// ai draws nothing: no package of desk, nor of mygo, is among what it
// depends on, through any package between.
func TestImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "writer/ai").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	within := func(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
	wrong := slices.DeleteFunc(strings.Fields(string(out)), func(d string) bool {
		return !within(d, "writer/desk") && !within(d, "github.com/egoist/mygo")
	})
	if len(wrong) > 0 {
		t.Errorf("ai depends on %s", strings.Join(wrong, ", "))
	}
}
