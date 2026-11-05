package context_test

import (
	"testing"
	"writer/ext"
	"writer/ext/context"
	"writer/ext/context/contextapi"
	"writer/workspace"
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

func TestLibraryDistinguishesMissingEntryProviderFromPresentEmptyEntries(t *testing.T) {
	h, err := ext.New(ext.Options{}, context.Module, ext.Module{ID: "provider", Requires: []string{"context"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, contextapi.Entries, "empty", contextapi.EntryProvider(func(struct{}) []workspace.Entry { return nil }))
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service, _ := ext.Lookup(h, contextapi.Service)
	if entries, present := service.Entries(); entries == nil || len(entries) != 0 || !present {
		t.Fatal(entries, present)
	}
	if err := h.Set("provider", false); err != nil {
		t.Fatal(err)
	}
	if entries, present := service.Entries(); entries != nil || present {
		t.Fatal(entries, present)
	}
}

func TestHasOutlinesAsksOnlyForProviderPresence(t *testing.T) {
	calls := 0
	h, err := ext.New(ext.Options{}, context.Module, ext.Module{ID: "provider", Requires: []string{"context"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, contextapi.Outlines, "outline", contextapi.OutlineProvider(func(string) string { calls++; return "" }))
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service, _ := ext.Lookup(h, contextapi.Service)
	if !service.HasOutlines() || calls != 0 {
		t.Fatal(calls)
	}
	if err := h.Set("provider", false); err != nil {
		t.Fatal(err)
	}
	if service.HasOutlines() || calls != 0 {
		t.Fatal(calls)
	}
}

func TestHooksDistinguishesAbsentProviderFromPresentEmpty(t *testing.T) {
	h, err := ext.New(ext.Options{}, context.Module, ext.Module{ID: "provider", Requires: []string{"context"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, contextapi.Hooks, "empty-hooks", contextapi.HookProvider(func(struct{}) []workspace.Foreshadow { return nil }))
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service, _ := ext.Lookup(h, contextapi.Service)
	if hooks, present := service.Hooks(); hooks == nil || !present {
		t.Fatal(hooks, present)
	}
	if err = h.Set("provider", false); err != nil {
		t.Fatal(err)
	}
	if hooks, present := service.Hooks(); hooks != nil || present {
		t.Fatal(hooks, present)
	}
}
