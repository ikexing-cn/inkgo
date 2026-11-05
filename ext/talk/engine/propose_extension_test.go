package discussion

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/model"
	"writer/workspace"
)

func TestProposeOffRequestBytesEqualTodaysRequestMinusToolAndItsPromptLines(t *testing.T) {
	e, _ := setup(t)
	w := e.Store.Work()
	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for _, beside := range []*Writing{nil, {Title: "章"}} {
		on := allTools()
		off := slices.DeleteFunc(slices.Clone(on), func(t aiapi.Tool) bool { return t.Name == workspace.ProposeTool })
		build := func(tools []aiapi.Tool) []byte {
			got, err := Assemble(w, 100000, workspace.Setting{}, tree, nil, "你好", Along{Tools: tools, Material: e.Material, Beside: beside})
			if err != nil {
				t.Fatal(err)
			}
			r := got.Request
			if len(tools) == len(on) {
				r.Tools = slices.DeleteFunc(r.Tools, func(t model.Tool) bool { return t.Name == workspace.ProposeTool })
				r.System = strings.ReplaceAll(r.System, proposeRules, "")
				r.System = strings.ReplaceAll(r.System, "\n"+besideProposeRule, "")
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
		if a, b := build(on), build(off); !bytes.Equal(a, b) {
			t.Fatalf("request differs beyond proposal tool and rules:\n%s\n%s", a, b)
		}
	}
}
