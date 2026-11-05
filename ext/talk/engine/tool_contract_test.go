package discussion

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"writer/ext/ai/aiapi"
	"writer/workspace"
)

func TestDiscussionProductionFilesContainNoContributedToolNames(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range []string{toolLookUp, toolReadOutline, toolNote} {
			if strings.Contains(string(source), name) {
				t.Errorf("%s contains contributed tool name %q", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestToolListUsesDeclaredOrderAndKeepsRegistrationOrderForTies(t *testing.T) {
	extra := []aiapi.Tool{
		{Name: "zeta", Order: 15, Prompt: aiapi.ToolPrompt{Order: 9, Lookup: "second lookup"}, Schema: fixedSchema(`{"type":"object"}`)},
		{Name: "alpha", Order: 15, Prompt: aiapi.ToolPrompt{Order: 8, Lookup: "first lookup"}, Schema: fixedSchema(`{"type":"object"}`)},
	}
	table := combined(extra)
	var names []string
	for _, tool := range toolsOf(workspace.Work{}, table) {
		names = append(names, tool.Name)
	}
	want := []string{toolSearchBook, "zeta", "alpha", toolReadProse, toolSearchTalks}
	if !slices.Equal(names, want) {
		t.Fatalf("tool order %v, want %v", names, want)
	}
	rules := rulesWith(aiapi.Material{}, table)
	if !strings.Contains(rules, "first lookup，second lookup，search_book 搜正文") {
		t.Fatal("prompt order followed list order", rules)
	}
	if extra[0].Name != "zeta" || extra[1].Name != "alpha" {
		t.Fatal("sorting mutated registration order")
	}
}

func TestSettingIndexGuidanceComesOnlyFromOfferedTools(t *testing.T) {
	empty := talkIndex(workspace.Work{}, workspace.Setting{}, view{}, nil)
	if strings.Contains(empty, toolLookUp) {
		t.Fatal("index names unoffered tool", empty)
	}
	all := talkIndex(workspace.Work{}, workspace.Setting{}, view{}, allTools())
	if !strings.HasPrefix(all, indexNote+"\n\n") {
		t.Fatal("setting index bytes changed", all)
	}
	custom := talkIndex(workspace.Work{}, workspace.Setting{}, view{}, []aiapi.Tool{{Prompt: aiapi.ToolPrompt{SettingIndex: "Custom index guidance."}}})
	if !strings.Contains(custom, "Custom index guidance.") {
		t.Fatal("ignored contributed index guidance", custom)
	}
}
