package model

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
	"sync/atomic"
)

//go:embed models.json
var embeddedModels []byte

var officialProviders = []string{"deepseek", "moonshotai", "openai", "anthropic"}

type limits struct {
	Context int  `json:"context"`
	Output  int  `json:"output"`
	Input   *int `json:"input,omitempty"`
}
type modelEntry struct {
	Limit limits `json:"limit"`
}
type providerEntry struct {
	Models map[string]modelEntry `json:"models"`
}
type directory map[string]providerEntry

var embeddedDirectory = mustDirectory(embeddedModels)
var cachedDirectory atomic.Pointer[directory]

func mustDirectory(data []byte) directory {
	d, err := parseDirectory(data, false)
	if err != nil {
		panic(err)
	}
	return d
}

// parseDirectory isolates schema errors to individual providers/models.
func parseDirectory(data []byte, officialOnly bool) (directory, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := directory{}
	for id, b := range raw {
		if officialOnly {
			found := false
			for _, p := range officialProviders {
				found = found || id == p
			}
			if !found {
				continue
			}
		}
		var p struct {
			Models map[string]json.RawMessage `json:"models"`
		}
		if json.Unmarshal(b, &p) != nil {
			continue
		}
		models := map[string]modelEntry{}
		for id, b := range p.Models {
			var rawModel struct {
				Limit json.RawMessage `json:"limit"`
			}
			if json.Unmarshal(b, &rawModel) != nil {
				continue
			}
			var rawLimit struct {
				Context int             `json:"context"`
				Output  int             `json:"output"`
				Input   json.RawMessage `json:"input"`
			}
			if json.Unmarshal(rawModel.Limit, &rawLimit) != nil || rawLimit.Context <= 0 || rawLimit.Output <= 0 {
				continue
			}
			m := modelEntry{Limit: limits{Context: rawLimit.Context, Output: rawLimit.Output}}
			var input *int
			if json.Unmarshal(rawLimit.Input, &input) == nil {
				m.Limit.Input = input
			}

			models[id] = m
		}
		if len(models) > 0 {
			out[id] = providerEntry{models}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("models.dev: no usable models")
	}
	return out, nil
}

// FilterModelsJSON retains only models.dev limits. officialOnly
// selects the four providers shipped with the application; caches retain all.
// Output uses sorted keys and two-space indentation, including nested objects.
func FilterModelsJSON(data []byte, officialOnly bool) ([]byte, error) {
	d, err := parseDirectory(data, officialOnly)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var sorted any
	if err := json.Unmarshal(b, &sorted); err != nil {
		return nil, err
	}
	b, err = json.MarshalIndent(sorted, "", "  ")
	return append(b, '\n'), err
}

func (d directory) find(id string) (modelEntry, bool) {
	for _, p := range officialProviders {
		if m, ok := d[p].Models[id]; ok {
			return m, true
		}
	}
	// Stable tie breaking for IDs shared by third-party providers.
	keys := make([]string, 0, len(d))
	for p := range d {
		switch p {
		case "deepseek", "moonshotai", "openai", "anthropic":
			continue
		}
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		if m, ok := d[p].Models[id]; ok {
			return m, true
		}
	}
	return modelEntry{}, false
}

func findModel(id string) (modelEntry, bool) {
	if d := cachedDirectory.Load(); d != nil {
		if m, ok := d.find(id); ok {
			return m, true
		}
	}
	return embeddedDirectory.find(id)
}
