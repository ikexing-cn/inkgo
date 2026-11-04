package archive

import "writer/workspace"

// Meta counts the live prose held by the files, identifying the work by its ID.
func (f Files) Meta(workID string, language workspace.Language) Meta {
	m := Meta{Work: workID}
	for _, file := range f.List {
		if workspace.IsProsePath(file.Rel) {
			m.Chapters++
			m.Words += language.Count(string(file.Data))
		}
	}
	return m
}
