package main

import (
	"testing"

	"writer/ext"
	"writer/ext/backup"
	"writer/ext/export"
	"writer/ext/phone"
	"writer/locales"
)

func testModules() []ext.Module {
	mod, _ := backup.New(false)
	return []ext.Module{mod, export.Module, phone.Module}
}

func TestApplicationAssemblyIncludesExportFeature(t *testing.T) {
	for _, m := range modules {
		if m.ID == "export" {
			if m.Kind != ext.Feature || m.Name() != locales.ModExport() || m.Desc() != locales.ModExportAbout() {
				t.Fatal("incorrect export metadata")
			}
			return
		}
	}
	t.Fatal("export missing from application assembly")
}
