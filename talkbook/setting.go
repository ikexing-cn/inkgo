package talkbook

import (
	"writer/workspace"
)

// Changes formats entry changes by chapter.
func Changes(set workspace.Setting, tree workspace.Tree) map[string][]string {
	o, name := tree.Order(), Of(tree).Name
	out := map[string][]string{}
	for _, e := range set.Entries {
		for _, g := range e.Holding(o) {
			out[e.ID] = append(out[e.ID], "「"+name[g.From]+"」起："+oneLine(g.Text))
		}
		for _, g := range e.Changes {
			if _, ok := o[g.From]; ok && g.Place == workspace.PlaceOpen {
				out[e.ID] = append(out[e.ID], "「"+name[g.From]+"」起（悬而未决）："+oneLine(g.Text))
			}
		}
	}
	return out
}
