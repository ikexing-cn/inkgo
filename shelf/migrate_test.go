package shelf

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestExtensionMigrationSeparatesSingleSettingsFromFolderTables(t *testing.T) {
	old := json.RawMessage("{}")
	s := Shelf{MovedExport: old, MovedPhone: old, Books: []Book{{Path: "work", MovedBackup: old}}}
	saved := map[string]bool{}
	merges := 0
	changed, err := s.MigrateExtensions(func(key string, data json.RawMessage) error {
		if key != "phone.typesetting" && key != "export.choices" {
			t.Fatalf("folder table passed as a single setting: %s", key)
		}
		if string(data) != string(old) {
			t.Fatal(data)
		}
		saved[key] = true
		return nil
	}, func(key string, folders map[string]json.RawMessage) error {
		if key != "backup.choices" || len(folders) != 1 || string(folders["work"]) != string(old) {
			t.Fatal(key, folders)
		}
		merges++
		return nil
	})
	if err != nil || !changed || len(saved) != 2 || merges != 1 {
		t.Fatal(changed, err, saved, merges)
	}
	if len(s.MovedPhone) != 0 || len(s.MovedExport) != 0 || len(s.Books[0].MovedBackup) != 0 {
		t.Fatal("successful migration retained source fields")
	}
}

func TestExtensionMigrationRetainsFolderSourceWhenMergeFails(t *testing.T) {
	old := json.RawMessage("{}")
	s := Shelf{Books: []Book{{Path: "work", MovedBackup: old}}}
	cause := errors.New("cannot persist folders")
	changed, err := s.MigrateExtensions(func(string, json.RawMessage) error { t.Fatal("unexpected single setting"); return nil }, func(string, map[string]json.RawMessage) error { return cause })
	if changed || !errors.Is(err, cause) || string(s.Books[0].MovedBackup) != string(old) {
		t.Fatal(changed, err, s.Books)
	}
}
