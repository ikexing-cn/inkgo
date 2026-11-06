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
type rates struct {
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}
type priceTier struct {
	rates
	Tier struct {
		Type string `json:"type"`
		Size int    `json:"size"`
	} `json:"tier"`
}
type modelEntry struct {
	Limit limits `json:"limit"`
	Cost  struct {
		rates
		Tiers []priceTier `json:"tiers,omitempty"`
	} `json:"cost"`
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

func validRates(r rates) bool {
	return r.Input != nil && r.Output != nil && *r.Input >= 0 && *r.Output >= 0 && (r.CacheRead == nil || *r.CacheRead >= 0) && (r.CacheWrite == nil || *r.CacheWrite >= 0)
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
				Cost  json.RawMessage `json:"cost"`
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

			var rawCost struct {
				Input      json.RawMessage   `json:"input"`
				Output     json.RawMessage   `json:"output"`
				CacheRead  json.RawMessage   `json:"cache_read"`
				CacheWrite json.RawMessage   `json:"cache_write"`
				Tiers      []json.RawMessage `json:"tiers"`
			}
			if json.Unmarshal(rawModel.Cost, &rawCost) == nil {
				// A malformed price must never discard usable limits.
				m.Cost.Input = parseRate(rawCost.Input)
				m.Cost.Output = parseRate(rawCost.Output)
				m.Cost.CacheRead = parseRate(rawCost.CacheRead)
				m.Cost.CacheWrite = parseRate(rawCost.CacheWrite)
				for _, b := range rawCost.Tiers {
					var tier priceTier
					if json.Unmarshal(b, &tier) == nil && validRates(tier.rates) && tier.Tier.Type == "context" && tier.Tier.Size > 0 {
						m.Cost.Tiers = append(m.Cost.Tiers, tier)
					}
				}
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

// FilterModelsJSON retains only models.dev limits and costs. officialOnly
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

func parseRate(b json.RawMessage) *float64 {
	var v *float64
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	return v
}
