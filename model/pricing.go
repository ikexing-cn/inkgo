package model

import "strings"

// Price is what a million tokens cost, in Currency. Cache writes the
// provider does not charge for are billed as input.
type Price struct {
	Input, CacheRead, CacheWrite, Output float64
	Currency                             string
}

// Careful limits for a model the directory does not know, and the compaction
// threshold every model starts at.
const (
	guessWindow      = 32_000
	guessOutput      = 4096
	defaultCompactAt = 80
)

// Builtin is the model id as the directory has it, or with careful limits
// when the directory does not know it.
func Builtin(id string) Model {
	m, _ := Lookup(id)
	return m
}

// Lookup is Builtin, and whether the directory knows id.
func Lookup(id string) (Model, bool) {
	m := Model{ID: id, Window: guessWindow, Output: guessOutput, CompactAt: defaultCompactAt}
	b, ok := findModel(id)
	if ok {
		m.Window, m.Output = b.Limit.Context, b.Limit.Output
		if input := b.Limit.Input; input != nil && *input > 0 && *input < m.Window {
			m.Output = min(m.Output, m.Window-*input)
		} else if m.Output >= m.Window {
			m.Output = m.Window / 4
		}
	}
	return m, ok
}

// Cost is what a request to the model id that took and gave u costs by
// the directory's prices, and in what currency; ok is false when the directory has
// no price for id.
func Cost(id string, u Usage) (cost float64, currency string, ok bool) {
	b, ok := findModel(id)
	if !ok || !validRates(b.Cost.rates) {
		return 0, "", false
	}
	r := b.Cost.rates
	threshold := -1
	for _, tier := range b.Cost.Tiers {
		if tier.Tier.Type == "context" && u.Input > tier.Tier.Size && tier.Tier.Size > threshold {
			r, threshold = tier.rates, tier.Tier.Size
		}
	}
	p := Price{Input: *r.Input, Output: *r.Output, CacheRead: *r.Input, CacheWrite: *r.Input, Currency: "$"}
	if r.CacheRead != nil {
		p.CacheRead = *r.CacheRead
	}
	if r.CacheWrite != nil {
		p.CacheWrite = *r.CacheWrite
	}
	fresh := max(u.Input-u.CacheRead-u.CacheWrite, 0)
	cost = float64(fresh)*p.Input + float64(u.CacheRead)*p.CacheRead + float64(u.CacheWrite)*p.CacheWrite + float64(u.Output)*p.Output
	return cost / 1e6, p.Currency, true
}

// PriceID is the final part of a routed model ID, as the price directory
// names it. Callers that price the exact configured ID use Cost directly.
func PriceID(id string) string { return id[strings.LastIndex(id, "/")+1:] }
