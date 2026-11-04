package aiapi

import "writer/inference"

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
type Handle = inference.Model
