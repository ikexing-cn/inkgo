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
