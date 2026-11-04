package writing

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// What the pen shares with the talk is in inference: writing does not
// depend on discussion, through any package between.
func TestImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "writer/writing").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if slices.Contains(strings.Fields(string(out)), "writer/ext/talk/engine") {
		t.Error("writing depends on writer/ext/talk/engine")
	}
}
