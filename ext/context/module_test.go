package context_test

import (
	"testing"
	"writer/ext"
	"writer/ext/context"
	"writer/ext/context/contextapi"
)

func TestLibraryDistinguishesMissingProviderFromPresentEmptyOutline(t *testing.T) {
	h, err := ext.New(ext.Options{}, context.Module, ext.Module{ID: "provider", Requires: []string{"context"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, contextapi.Outlines, "empty", contextapi.OutlineProvider(func(string) string { return "" }))
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service, ok := ext.Lookup(h, contextapi.Service)
	if !ok {
		t.Fatal("library missing")
	}
	if value, present := service.Outline("chapter"); value != "" || !present {
		t.Fatal(value, present)
	}
	if err := h.Set("provider", false); err != nil {
		t.Fatal(err)
	}
	if value, present := service.Outline("chapter"); value != "" || present {
		t.Fatal(value, present)
	}
	if err := h.Set("context", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := ext.Lookup(h, contextapi.Service); ok {
		t.Fatal("inactive library exposed")
	}
}
