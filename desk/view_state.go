package desk

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"writer/ext"
	"writer/prefs"
)

type viewStateStore struct{ a *App }

func (s viewStateStore) LoadState(b ext.Book, id string) map[string]json.RawMessage {
	p := prefs.Load(filepath.Join(b.Dir, ".writer", "desk.json"))
	var state map[string]json.RawMessage
	_ = json.Unmarshal(p.Extensions[id], &state)
	return state
}
func (s viewStateStore) SaveState(b ext.Book, id string, state map[string]json.RawMessage) {
	path := filepath.Join(b.Dir, ".writer", "desk.json")
	p := prefs.Load(path)
	if p.Extensions == nil {
		p.Extensions = map[string]json.RawMessage{}
	}
	raw, _ := json.Marshal(state)
	if bytes.Equal(p.Extensions[id], raw) {
		return
	}
	p.Extensions[id] = raw
	s.a.warn(p.Save(path))
}
