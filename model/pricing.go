package model

// Price is what a million tokens cost, in Currency. Cache writes the
// provider does not charge for are billed as input.
type Price struct {
	Input, CacheRead, CacheWrite, Output float64
	Currency                             string
}

// builtin is a model as its provider documents it.
type builtin struct {
	window, output int
	price          Price
	// Requests whose input is over above tokens cost long instead.
	above int
	long  Price
}

// table is the models of the default providers, checked against each
// provider's documents on 2026-10-15; it changes with the releases of the
// writing software, not online. Where prices differ by the hour, it has
// the dearer, so that an estimate errs high.
var table = map[string]builtin{
	"deepseek-flash":    {window: 1_000_000, output: 384_000, price: Price{0.3, 0.006, 0.3, 1.2, "$"}},
	"deepseek-v4-pro":   {window: 1_000_000, output: 384_000, price: Price{1.32, 0.044, 1.32, 3.96, "$"}},
	"kimi-k3":           {window: 1_048_576, output: 128_000, price: Price{3, 0.3, 3, 15, "$"}},
	"claude-fable-5-1":  {window: 1_000_000, output: 128_000, price: Price{10, 0.25, 12.5, 50, "$"}},
	"claude-opus-5-5":   {window: 1_000_000, output: 128_000, price: Price{4, 0.2, 5, 20, "$"}},
	"claude-sonnet-5-5": {window: 1_000_000, output: 128_000, price: Price{2, 0.1, 2.5, 10, "$"}},
	"claude-haiku-5-5": {window: 1_000_000, output: 128_000, price: Price{0.1, 0.01, 0.125, 0.5, "$"},
		above: 100_000, long: Price{0.5, 0.05, 0.625, 2.5, "$"}},
	"gpt-6-astra": {window: 1_050_000, output: 128_000, price: Price{10, 1, 10, 50, "$"},
		above: 272_000, long: Price{20, 2, 20, 75, "$"}},
	"gpt-6.1-sol": {window: 1_050_000, output: 128_000, price: Price{2, 0.1, 2, 10, "$"},
		above: 272_000, long: Price{4, 0.2, 4, 15, "$"}},
	"gpt-6-luna": {window: 1_050_000, output: 128_000, price: Price{0.1, 0.01, 0.1, 0.5, "$"},
		above: 272_000, long: Price{0.2, 0.02, 0.2, 0.75, "$"}},
}

// Careful limits for a model the table does not know, and the compaction
// threshold every model starts at.
const (
	guessWindow      = 32_000
	guessOutput      = 4096
	defaultCompactAt = 80
)

// Builtin is the model id as the table has it, or with careful limits
// when the table does not know it.
func Builtin(id string) Model {
	m, _ := Lookup(id)
	return m
}

// Lookup is Builtin, and whether the table knows id.
func Lookup(id string) (Model, bool) {
	m := Model{ID: id, Window: guessWindow, Output: guessOutput, CompactAt: defaultCompactAt}
	b, ok := table[id]
	if ok {
		m.Window, m.Output = b.window, b.output
	}
	return m, ok
}

// Cost is what a request to the model id that took and gave u costs by
// the table's prices, and in what currency; ok is false when the table has
// no price for id.
func Cost(id string, u Usage) (cost float64, currency string, ok bool) {
	b, ok := table[id]
	if !ok {
		return 0, "", false
	}
	p := b.price
	if b.above > 0 && u.Input > b.above {
		p = b.long
	}
	fresh := max(u.Input-u.CacheRead-u.CacheWrite, 0)
	cost = float64(fresh)*p.Input + float64(u.CacheRead)*p.CacheRead + float64(u.CacheWrite)*p.CacheWrite + float64(u.Output)*p.Output
	return cost / 1e6, p.Currency, true
}
