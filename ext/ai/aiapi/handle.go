package aiapi

import (
	"context"
	"writer/model"
)

// Role identifies the selected model's purpose when constructing a request.
type Role string

const (
	// RoleTalk selects the book's discussion model.
	RoleTalk Role = "talk"
	// RoleWriting selects the writing model, falling back to discussion.
	RoleWriting Role = "writing"
)

// Handle contains selected model identity, effort, limits and a completer.
// Build it on the frame thread; Complete runs on a worker. The backend wraps
// every completion with the same usage, trace and cancellation accounting.
type Handle struct {
	Model
	Name      string
	Effort    model.Effort
	Completer Completer
}

// Ready reports whether a selected model has a completion implementation.
func (h Handle) Ready() bool { return h.Completer != nil }

// Complete sends a request through the backend's adapted completion boundary.
func (h Handle) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	if req.MaxTokens == 0 || req.MaxTokens >= h.Output {
		req.MaxTokens = h.Output
	}
	return h.Completer.Complete(ctx, req, on)
}

// Completer is the model request boundary required by extension callers.
type Completer interface {
	Complete(context.Context, model.Request, func(model.Delta)) (model.Reply, error)
}
