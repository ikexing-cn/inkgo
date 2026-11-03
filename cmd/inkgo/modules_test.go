package main

import (
	"testing"

	"writer/ext"
	"writer/locales"
)

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
