// Package findapi contains stateless contracts for optional find UI consumers.
package findapi

import "writer/ext"

// Replacement opens the find interface for author review on the frame thread.
// It never writes prose; the author must confirm replacement in the find UI.
type Replacement interface {
	// Review fills the whole-book find bar with the old and new text on the frame thread.
	Review(old, new string)
}

// ReplacementService resolves replacement review only while find is active.
const ReplacementService ext.ServiceKey[Replacement] = "find.replacement"
