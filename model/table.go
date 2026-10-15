package model

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"writer/tokens"
)

// Model is a model of a provider and the room it works in.
type Model struct {
	ID string
	// Window is the most tokens one request may take in and give out
	// together; Output is how many of them the reply may use.
	Window, Output int
	// CompactAt is the percent of Window at which a talk is compacted.
	CompactAt int
}

// Input is what a request may send: the window less the reply's share.
func (m Model) Input() int { return m.Window - m.Output }

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

// IDs are the model IDs of models.
func IDs(models []Model) []string {
	out := make([]string, len(models))
	for i, m := range models {
		out[i] = m.ID
	}
	return out
}

// Model is the model id of the provider providerID as configured, or as
// Builtin has it when the provider does not list it.
func (c Config) Model(providerID, id string) Model {
	for _, p := range c.Providers {
		if p.ID != providerID {
			continue
		}
		for _, m := range p.Models {
			if m.ID == id {
				return m
			}
		}
	}
	return Builtin(id)
}

// modelYAML is a model as providers.yaml writes it: the two limits as
// tokens.Format writes them, digits as numbers.
type modelYAML struct {
	ID        string `yaml:"id"`
	Window    any    `yaml:"context"`
	Output    any    `yaml:"max_output"`
	CompactAt int    `yaml:"compact_at"`
}

func yamlCount(n int) any {
	s := tokens.Format(n)
	if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "M") {
		return s
	}
	return n
}

func (m Model) MarshalYAML() (any, error) {
	return modelYAML{m.ID, yamlCount(m.Window), yamlCount(m.Output), m.CompactAt}, nil
}

// modelError is a model written in a way it cannot be read, which Load
// reports as it is rather than as YAML that does not parse.
type modelError string

func (e modelError) Error() string { return string(e) }

const errModelFields modelError = "模型只能写 id、context、max_output、compact_at 几项"

// UnmarshalYAML reads a model written as MarshalYAML writes it. What it
// leaves out is as Builtin has it, so a model may be written as its ID
// alone, in a mapping or bare.
func (m *Model) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*m = Builtin(strings.TrimSpace(n.Value))
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return errModelFields
	}
	var raw struct {
		ID        string  `yaml:"id"`
		Window    *string `yaml:"context"`
		Output    *string `yaml:"max_output"`
		CompactAt *int    `yaml:"compact_at"`
	}
	for i := 0; i < len(n.Content); i += 2 {
		switch n.Content[i].Value {
		case "id", "context", "max_output", "compact_at":
		default:
			return errModelFields
		}
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	*m = Builtin(strings.TrimSpace(raw.ID))
	for _, f := range []struct {
		raw *string
		to  *int
	}{{raw.Window, &m.Window}, {raw.Output, &m.Output}} {
		if f.raw == nil {
			continue
		}
		v, ok := tokens.Parse(*f.raw)
		if !ok {
			return modelError(m.ID + " 的上下文上限和最大输出须为正整数，可写成 128K、1M")
		}
		*f.to = v
	}
	if raw.CompactAt != nil {
		m.CompactAt = *raw.CompactAt
	}
	return nil
}

// validateModels checks the models of a provider: each has an ID of its
// own, a reply shorter than the window, and a threshold between half and
// nineteen twentieths of it.
func validateModels(models []Model) error {
	seen := map[string]bool{}
	for _, m := range models {
		switch {
		case strings.TrimSpace(m.ID) == "" || m.ID != strings.TrimSpace(m.ID):
			return errors.New("模型 ID 不能为空，前后不能有空格")
		case seen[m.ID]:
			return fmt.Errorf("模型 %s 重复", m.ID)
		case m.Window < 1 || m.Output < 1:
			return fmt.Errorf("%s 的上下文上限和最大输出须为正整数，可写成 128K、1M", m.ID)
		case m.Output >= m.Window:
			return fmt.Errorf("%s 的最大输出须小于上下文上限", m.ID)
		case m.CompactAt < 50 || m.CompactAt > 95:
			return fmt.Errorf("%s 的压缩阈值须在 50%% 到 95%% 之间", m.ID)
		}
		seen[m.ID] = true
	}
	return nil
}
