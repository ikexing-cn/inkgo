package main

import (
	"testing"
	"writer/ext/export/exporttext"

	"writer/ext"
	"writer/ext/backup"
	"writer/ext/context"
	"writer/ext/export"
	"writer/ext/outline"
	"writer/ext/phone"
)

func testModules() []ext.Module {
	mod, _ := backup.New(false)
	return []ext.Module{context.Module, outline.Module, mod, export.Module, phone.Module}
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
