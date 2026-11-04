// Package outlineapi contains the outline extension's public contracts.
package outlineapi

import "writer/ext"

// Active is provided while the outline extension runs in the current book.
// It carries no value: an optional consumer such as export resolves it with
// ext.UseOptional to learn whether outlines are part of the book right now.
const Active ext.ServiceKey[struct{}] = "outline.active"
