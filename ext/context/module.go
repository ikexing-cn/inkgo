// Package context owns the reference library and its provider contracts.
package context

import (
	"writer/ext"
	"writer/ext/context/contextapi"
	"writer/ext/context/contexttext"
)

// Module registers the reference library without reading book data in Setup.
var Module = ext.Module{ID: "context", Kind: ext.Library, Name: contexttext.ModContext, Desc: contexttext.ModContextAbout, Setup: func(r *ext.Registrar) {
	ext.Declare(r, contextapi.Outlines)
	ext.DeclareCalls(r, contextapi.Outlines, func(p contextapi.OutlineProvider) func(string) string { return p })
	providers := ext.ReadPoint(r, contextapi.Outlines)
	ext.Provide(r, contextapi.Service, func(*ext.Session) contextapi.Context { return library{providers} })
}}

type library struct {
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
