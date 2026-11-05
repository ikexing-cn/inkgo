package desk

import (
	"slices"
	"writer/ext"
	"writer/ext/hostapi"
)

// nameFeed retains the point's borrowed results and aggregates only new revisions.
type nameFeed struct {
	changes uint64
	inputs  []hostapi.Names
	value   hostapi.Names
	builds  int
}

func (a *App) nameValues() hostapi.Names {
	a.syncExtensions()
	c := &a.write.names
	dirty := c.builds == 0 || c.changes != a.extViews.changes
	if dirty {
		c.inputs = make([]hostapi.Names, len(a.extViews.names))
		c.changes = a.extViews.changes
	}
	for i, item := range a.extViews.names {
		v, _ := ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.NameProvider, struct{}, hostapi.Names]{Point: hostapi.NameHighlights}, item.ID), struct{}{}, item.Value)
		dirty = dirty || c.inputs[i].Revision != v.Revision
		c.inputs[i] = v
	}
	if !dirty {
		return c.value
	}
	var out hostapi.Names
	for i, item := range a.extViews.names {
		v := c.inputs[i]
		out.Entities = append(out.Entities, v.Entities...)
		out.Links = append(out.Links, v.Links...)
		if v.Open != nil {
			owner, id, open := item.Owner, item.ID, v.Open
			out.Open = func(entry string) { _ = ext.Guard(a.extensions, owner, id, func() { open(entry) }) }
		}
	}
	c.builds++
	out.Revision = uint64(c.builds)
	c.value = out
	return c.value
}

func (a *App) feedNames() {
	if ed := a.write.cur().ed; ed != nil {
		w := &a.write
		names := a.nameValues()
		if w.named == ed && w.namedRevs == names.Revision {
			return
		}
		if w.named != ed || !slices.Equal(w.namedValues.Entities, names.Entities) {
			ed.SetEntities(names.Entities)
		}
		if w.named != ed || !slices.Equal(w.namedValues.Links, names.Links) {
			ed.SetNameLinks(names.Links, func(entry string) {
				if open := a.write.names.value.Open; open != nil {
					open(entry)
				}
			})
		}
		w.named, w.namedRevs = ed, names.Revision
		w.namedValues = names
	}
}
