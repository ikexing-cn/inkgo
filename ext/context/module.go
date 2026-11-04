// Package context owns the reference library and its provider contracts.
package context

import (
	"writer/ext"
	"writer/ext/context/contextapi"
	"writer/ext/context/contexttext"
	"writer/workspace"
)

// Module registers the reference library without reading book data in Setup.
var Module = ext.Module{ID: "context", Kind: ext.Library, Name: contexttext.ModContext, Desc: contexttext.ModContextAbout, Setup: func(r *ext.Registrar) {
	ext.Declare(r, contextapi.Entries)
	ext.DeclareCalls(r, contextapi.Entries, func(p contextapi.EntryProvider) func(struct{}) []workspace.Entry { return p })
	entries := ext.ReadPoint(r, contextapi.Entries)
	ext.Declare(r, contextapi.Outlines)
	ext.DeclareCalls(r, contextapi.Outlines, func(p contextapi.OutlineProvider) func(string) string { return p })
	providers := ext.ReadPoint(r, contextapi.Outlines)
	ext.Provide(r, contextapi.Service, func(*ext.Session) contextapi.Context { return library{providers: providers, entries: entries} })
}}

type library struct {
	entries   *ext.PointReader[contextapi.EntryProvider]
	providers *ext.PointReader[contextapi.OutlineProvider]
}

func (l library) Outline(id string) (string, bool) {
	for _, item := range l.providers.Items() {
		text, err := ext.CallReadPoint(l.providers, item.ID, id, item.Value)
		if err == nil {
			return text, true
		}
	}
	return "", false
}

func (l library) Entries() ([]workspace.Entry, bool) {
	for _, item := range l.entries.Items() {
		value, err := ext.CallReadPoint(l.entries, item.ID, struct{}{}, item.Value)
		if err == nil {
			if value == nil {
				value = []workspace.Entry{}
			}
			return value, true
		}
	}
	return nil, false
}
