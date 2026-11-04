// Package model provides global provider configuration and bounded model API clients.
package model

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"writer/internal/atomicfile"
)

const configLimit = 1 << 20

type Protocol string

const (
	ProtocolChat      Protocol = "chat"
	ProtocolResponses Protocol = "responses"
	ProtocolAnthropic Protocol = "anthropic"
)

// Provider describes an API endpoint. Credentials live only in the OS keychain or environment.
type Provider struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	BaseURL  string   `yaml:"base_url"`
	Protocol Protocol `yaml:"protocol"`
	Env      string   `yaml:"env"`
	Models   []Model  `yaml:"models"`
}

type Config struct {
	Schema    int        `yaml:"schema"`
	Providers []Provider `yaml:"providers"`
}

func DefaultConfig() Config {
	return Config{Schema: 1, Providers: []Provider{
		{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Protocol: ProtocolChat, Env: "DEEPSEEK_API_KEY", Models: builtins("deepseek-flash", "deepseek-v4-pro")},
		{ID: "kimi", Name: "Kimi", BaseURL: "https://api.moonshot.cn/v1", Protocol: ProtocolChat, Env: "MOONSHOT_API_KEY", Models: builtins("kimi-k3")},
		{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Protocol: ProtocolResponses, Env: "OPENAI_API_KEY", Models: []Model{}},
		{ID: "anthropic", Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1", Protocol: ProtocolAnthropic, Env: "ANTHROPIC_API_KEY", Models: []Model{}},
	}}
}

func builtins(ids ...string) []Model {
	out := make([]Model, len(ids))
	for i, id := range ids {
		out[i] = Builtin(id)
	}
	return out
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", &ConfigError{Op: "locate", Err: err}
	}
	return filepath.Join(dir, "writer", "providers.yaml"), nil
}

// Load creates and persists the defaults when path does not exist. Unknown YAML
// fields, duplicate fields, multiple documents, and unsupported schemas fail closed.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig()
		return cfg, Save(path, cfg)
	}
	if err != nil {
		return Config{}, &ConfigError{Op: "open", Err: err}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, configLimit+1))
	if err != nil {
		return Config{}, &ConfigError{Op: "read", Err: err}
	}
	if len(data) > configLimit {
		return Config{}, ErrConfigTooLarge
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		var limits *LimitsError
		if errors.Is(err, ErrModelFields) {
			return Config{}, ErrModelFields
		}
		if errors.As(err, &limits) {
			return Config{}, limits
		}
		return Config{}, ErrConfigYAML
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Config{}, ErrConfigDocuments
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save atomically replaces a configuration. There is intentionally no key field.
func Save(path string, cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return ErrConfigEncode
	}
	if len(data) > configLimit {
		return ErrConfigTooLarge
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return &ConfigError{Op: "mkdir", Err: err}
	}
	if err := atomicfile.Write(path, data, 0600); err != nil {
		var step *atomicfile.Error
		if errors.As(err, &step) {
			switch step.Op {
			case "create":
				return &ConfigError{Op: "create", Err: err}
			case "rename":
				return &ConfigError{Op: "replace", Err: err}
			}
		}
		return &ConfigError{Op: "write", Err: err}
	}
	return nil
}

var providerID = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

var errIDFormat = ErrIDFormat
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateConfig(cfg Config) error {
	if cfg.Schema != 1 {
		return ErrConfigVersion
	}
	ids := map[string]bool{}
	for i, p := range cfg.Providers {
		if err := validateProvider(p); err != nil {
			return &ProviderError{Count: i + 1, Err: err}
		}
		if ids[p.ID] {
			return ErrProviderDuplicate
		}
		ids[p.ID] = true
	}
	return nil
}

// Problems are what keep a provider from being saved, by the field each
// is in; a field with none has nil.
type Problems struct {
	ID, Name, Protocol, BaseURL, Env, Models error
}

// First is the first problem in the order of the fields, or nil.
func (p Problems) First() error {
	for _, err := range []error{p.ID, p.Name, p.Protocol, p.BaseURL, p.Env, p.Models} {
		if err != nil {
			return err
		}
	}
	return nil
}

// Check says what is wrong with each field of p, so that a form can say it
// by the field before p is saved.
func Check(p Provider) Problems {
	var out Problems
	if !providerID.MatchString(p.ID) {
		out.ID = errIDFormat
	}
	if strings.TrimSpace(p.Name) == "" {
		out.Name = ErrNameRequired
	}
	switch p.Protocol {
	case ProtocolChat, ProtocolResponses, ProtocolAnthropic:
	default:
		out.Protocol = ErrProtocol
	}
	out.BaseURL = checkBaseURL(p.BaseURL)
	if p.Env != "" && !envName.MatchString(p.Env) {
		out.Env = ErrEnvFormat
	}
	out.Models = validateModels(p.Models)
	return out
}

func validateProvider(p Provider) error { return Check(p).First() }

// checkBaseURL says what keeps s from being a service's address, by what
// the author would change: its start, its host, or what follows the path.
func checkBaseURL(s string) error {
	u, err := url.Parse(s)
	switch {
	case err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Opaque != "":
		return ErrURLScheme
	case u.Host == "" || u.Hostname() == "":
		return ErrURLHost
	case u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "":
		return ErrURLExtra
	}
	return nil
}
