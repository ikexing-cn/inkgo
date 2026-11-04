package desk

import (
	"os/exec"
	"slices"
	"sort"
	"strings"
	"testing"
)

// What each package under desk may depend on, through direct imports. Public API contracts may expose model handle types.
// desk draws all of the AI and sends nothing itself: what sends is in package
// ai, which desk reaches through aiapi alone. book is the book's content, with
// no view and nothing of desk. kit is what the pages draw with, and knows
// nothing of desk. Any other package under desk is a page, which reaches desk
// only through kit and book; one added later is held to that without naming it
// here, as go list finds it.
func TestImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "writer/desk/...").CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	deps := map[string][]string{}
	var packages []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		deps[f[0]] = f[1:]
		packages = append(packages, f[0])
	}
	if !slices.Contains(deps["writer/desk"], "writer/ext/ai/aiapi") {
		t.Fatalf("not the dependencies of desk: %q", deps["writer/desk"])
	}
	sort.Strings(packages)
	within := func(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
	for _, p := range packages {
		wrong := slices.DeleteFunc(slices.Clone(deps[p]), func(d string) bool {
			underDesk := within(d, "writer/desk")
			switch {
			case slices.Contains([]string{"writer/ai", "writer/model", "writer/ext/talk/engine", "writer/writing", "writer/inference", "writer/prompt"}, d):
				return false
			case p == "writer/desk":
				return true
			case p == "writer/desk/book":
				return !underDesk && !within(d, "writer/editor") && !within(d, "github.com/egoist/mygo")
			case p == "writer/desk/kit":
				return !underDesk
			}
			return !underDesk || d == "writer/desk/kit" || d == "writer/desk/book"
		})
		if len(wrong) > 0 {
			t.Errorf("%s depends on %s", p, strings.Join(wrong, ", "))
		}
	}
}
