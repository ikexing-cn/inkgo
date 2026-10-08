// Package model provides global provider configuration and bounded model API clients.
package model

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const configLimit = 1 << 20

// Selection is safe to store in a work's configuration; it contains no credentials.
type Selection struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// Provider describes an API endpoint. Credentials live only in the OS keychain or environment.
type Provider struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	BaseURL  string   `yaml:"base_url"`
	Protocol string   `yaml:"protocol"`
	Env      string   `yaml:"env"`
	Models   []string `yaml:"models"`
}

type Config struct {
	Schema    int        `yaml:"schema"`
	Providers []Provider `yaml:"providers"`
}

func DefaultConfig() Config {
	return Config{Schema: 1, Providers: []Provider{
		{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Protocol: "chat", Env: "DEEPSEEK_API_KEY", Models: []string{"deepseek-chat", "deepseek-reasoner"}},
		{ID: "kimi", Name: "Kimi", BaseURL: "https://api.moonshot.cn/v1", Protocol: "chat", Env: "MOONSHOT_API_KEY", Models: []string{"kimi-k2.5"}},
		{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Protocol: "responses", Env: "OPENAI_API_KEY", Models: []string{}},
		{ID: "anthropic", Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1", Protocol: "anthropic", Env: "ANTHROPIC_API_KEY", Models: []string{}},
	}}
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration directory: %w", err)
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
		return Config{}, fmt.Errorf("open provider configuration: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, configLimit+1))
	if err != nil {
		return Config{}, fmt.Errorf("read provider configuration: %w", err)
	}
	if len(data) > configLimit {
		return Config{}, errors.New("provider configuration exceeds 1 MiB")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.New("invalid provider YAML: check syntax, field names, and field types")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Config{}, errors.New("provider configuration must contain exactly one YAML document")
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
		return errors.New("encode provider configuration")
	}
	if len(data) > configLimit {
		return errors.New("provider configuration exceeds 1 MiB")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".providers-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write provider configuration: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close provider configuration: %w", closeErr)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace provider configuration: %w", err)
	}
	return nil
}

var providerID = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateConfig(cfg Config) error {
	if cfg.Schema != 1 {
		return errors.New("unsupported provider configuration schema (expected 1)")
	}
	ids := map[string]bool{}
	for i, p := range cfg.Providers {
		if err := validateProvider(p); err != nil {
			return fmt.Errorf("provider %d: %w", i+1, err)
		}
		if ids[p.ID] {
			return errors.New("duplicate provider ID")
		}
		ids[p.ID] = true
	}
	return nil
}

func validateProvider(p Provider) error {
	if !providerID.MatchString(p.ID) {
		return errors.New("provider ID must be a lowercase identifier")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("provider name is required")
	}
	switch p.Protocol {
	case "chat", "responses", "anthropic":
	default:
		return errors.New("protocol must be chat, responses, or anthropic")
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return errors.New("base URL must be an HTTP(S) API root without credentials, query, or fragment")
	}
	if p.Env != "" && !envName.MatchString(p.Env) {
		return errors.New("invalid API key environment variable name")
	}
	for _, id := range p.Models {
		if strings.TrimSpace(id) == "" {
			return errors.New("model IDs must not be empty")
		}
	}
	return nil
}
