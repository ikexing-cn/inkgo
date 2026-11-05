package main

import (
	"slices"
	"testing"
	"writer/ext/export/exporttext"

	"writer/ext"
	"writer/ext/backup"
	"writer/ext/cats"
	"writer/ext/context"
	"writer/ext/export"
	"writer/ext/find"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/hooks"
	"writer/ext/lore"
	"writer/ext/marks"
	"writer/ext/names"
	"writer/ext/notes"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/snaps"
	"writer/ext/talk"
	"writer/ext/trash"
)

func testModules() []ext.Module {
	mod, _ := backup.New(false)
	return []ext.Module{focus.Module, goal.Module, names.Module, find.Module, notes.Module, lore.Module, outline.Module, hooks.Module, talk.Module, snaps.Module, trash.Module, cats.Module, mod, export.Module, phone.Module, context.Module, marks.Module}
}

func TestApplicationAssemblyIncludesExportFeature(t *testing.T) {
	for _, m := range modules {
		if m.ID == "export" {
			if m.Kind != ext.Feature || m.Name() != exporttext.ModExport() || m.Desc() != exporttext.ModExportAbout() {
				t.Fatal("incorrect export metadata")
			}
			return
		}
	}
	t.Fatal("export missing from application assembly")
}

func TestApplicationListsImplementedExtensionsInPrototypeGroupOrder(t *testing.T) {
	var ids []string
	for _, m := range assembledModules("", "") {
		ids = append(ids, m.ID)
	}
	want := []string{"focus", "goal", "names", "find", "notes", "lore", "outline", "hooks", "talk", "snaps", "trash", "cats", "backup", "export", "phone", "context", "marks", "ai"}
	if !slices.Equal(ids, want) {
		t.Fatal(ids)
	}
}
