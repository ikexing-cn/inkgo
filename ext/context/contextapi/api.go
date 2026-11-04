// Package contextapi contains stateless reference-material contracts.
package contextapi

import "writer/ext"

// OutlineProvider returns a node's live chapter or volume outline on the frame
// thread. The context service invokes it through ext.Call, including injections.
type OutlineProvider func(id string) string

// Context reads book reference material on the frame thread. Outline reports
// false when no provider is active; an empty outline from a provider is present.
// Service consumers must call through ext.Guard at their host boundary.
type Context interface {
	// Outline returns live text and whether a provider is present; frame thread only.
	Outline(id string) (string, bool)
}

// Outlines accepts live chapter and volume outline providers. Calls run through
// ext.Call on the frame thread; the first successful provider supplies the value.
const Outlines ext.PointKey[OutlineProvider] = "context.outlines"

// Service resolves the current reference library; frame thread only.
const Service ext.ServiceKey[Context] = "context.service"
