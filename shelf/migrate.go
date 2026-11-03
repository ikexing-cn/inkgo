package shelf

import (
	"encoding/json"
	"errors"
)

// MigrateExtensions moves legacy local settings once; remove after 2026-12.
// Successfully persisted values are cleared; failures retain their source.
func (s *Shelf) MigrateExtensions(save func(key string, data json.RawMessage) error, merge func(key string, folders map[string]json.RawMessage) error) (bool, error) {
	changed := false
	var failures []error
	for _, old := range []struct {
		key  string
		data *json.RawMessage
	}{{"phone.typesetting", &s.MovedPhone}, {"export.choices", &s.MovedExport}} {
		if len(*old.data) == 0 {
			continue
		}
		if err := save(old.key, *old.data); err != nil {
			failures = append(failures, err)
		} else {
			*old.data = nil
			changed = true
		}
	}
	folders := map[string]json.RawMessage{}
	for _, b := range s.Books {
		if len(b.MovedBackup) > 0 {
			folders[b.Path] = b.MovedBackup
		}
	}
	if len(folders) > 0 {
		err := merge("backup.choices", folders)
		if err != nil {
			failures = append(failures, err)
		} else {
			for i := range s.Books {
				s.Books[i].MovedBackup = nil
			}
			changed = true
		}
	}
	return changed, errors.Join(failures...)
}
