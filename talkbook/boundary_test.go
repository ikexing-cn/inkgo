package talkbook

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNeutralBookAndToolOwnersDoNotImportDiscussionOrExtensions(t *testing.T) {
	for _, pkg := range []string{"writer/talkbook", "writer/ext/lore", "writer/ext/outline"} {
		out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, pkg).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		for _, imp := range strings.Fields(string(out)) {
			if imp == "writer/ext/talk/engine" || pkg == "writer/talkbook" && strings.HasPrefix(imp, "writer/ext") {
				t.Fatalf("%s imports %s", pkg, imp)
			}
		}
	}
}
