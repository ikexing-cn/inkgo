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

// Provider describes an API endpoint. Credentials live only in the OS keychain or environment.
type Provider struct {
	ID       string  `yaml:"id"`
	Name     string  `yaml:"name"`
	BaseURL  string  `yaml:"base_url"`
	Protocol string  `yaml:"protocol"`
	Env      string  `yaml:"env"`
	Models   []Model `yaml:"models"`
}

type Config struct {
	Schema    int        `yaml:"schema"`
	Providers []Provider `yaml:"providers"`
}

func DefaultConfig() Config {
	return Config{Schema: 1, Providers: []Provider{
		{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Protocol: "chat", Env: "DEEPSEEK_API_KEY", Models: builtins("deepseek-flash", "deepseek-v4-pro")},
		{ID: "kimi", Name: "Kimi", BaseURL: "https://api.moonshot.cn/v1", Protocol: "chat", Env: "MOONSHOT_API_KEY", Models: builtins("kimi-k3")},
		{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Protocol: "responses", Env: "OPENAI_API_KEY", Models: []Model{}},
		{ID: "anthropic", Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1", Protocol: "anthropic", Env: "ANTHROPIC_API_KEY", Models: []Model{}},
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
		return "", fmt.Errorf("找不到用户配置目录：%w", err)
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
		return Config{}, fmt.Errorf("无法打开模型配置：%w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, configLimit+1))
	if err != nil {
		return Config{}, fmt.Errorf("无法读取模型配置：%w", err)
	}
	if len(data) > configLimit {
		return Config{}, errors.New("模型配置超过 1 MiB")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		var limits modelError
		if errors.As(err, &limits) {
			return Config{}, limits
		}
		return Config{}, errors.New("模型配置不是合法的 YAML：请检查语法、字段名和字段类型")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Config{}, errors.New("模型配置只能包含一个 YAML 文档")
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
		return errors.New("无法编码模型配置")
	}
	if len(data) > configLimit {
		return errors.New("模型配置超过 1 MiB")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("无法创建配置目录：%w", err)
	}
	file, err := os.CreateTemp(dir, ".providers-*.yaml")
	if err != nil {
		return fmt.Errorf("无法创建临时配置文件：%w", err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("无法写入模型配置：%w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("无法写入模型配置：%w", closeErr)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("无法替换模型配置：%w", err)
	}
	return nil
}

var providerID = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

var errIDFormat = errors.New("提供商 ID 只能用小写字母、数字、- 和 _，并以字母开头")
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateConfig(cfg Config) error {
	if cfg.Schema != 1 {
		return errors.New("不支持的模型配置版本（应为 1）")
	}
	ids := map[string]bool{}
	for i, p := range cfg.Providers {
		if err := validateProvider(p); err != nil {
			return fmt.Errorf("第 %d 个提供商：%w", i+1, err)
		}
		if ids[p.ID] {
			return errors.New("提供商 ID 重复")
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
		out.Name = errors.New("请填写显示名称")
	}
	switch p.Protocol {
	case "chat", "responses", "anthropic":
	default:
		out.Protocol = errors.New("协议必须是 chat、responses 或 anthropic")
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		out.BaseURL = errors.New("API 地址必须是 http(s) 根地址，不能带账号、查询参数或锚点")
	}
	if p.Env != "" && !envName.MatchString(p.Env) {
		out.Env = errors.New("环境变量名称无效")
	}
	out.Models = validateModels(p.Models)
	return out
}

func validateProvider(p Provider) error { return Check(p).First() }
