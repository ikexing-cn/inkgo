package backup

import (
	"strings"

	"writer/workspace"
)

// Meta is what a backup of f says of the work.
func (f Files) Meta(workID string) Meta {
	m := Meta{Work: workID}
	for _, file := range f.List {
		if strings.HasPrefix(file.Rel, "chapters/") && strings.HasSuffix(file.Rel, ".txt") && !strings.HasSuffix(file.Rel, ".outline.txt") {
			m.Chapters++
			m.Words += workspace.Count(string(file.Data))
		}
	}
	return m
}
