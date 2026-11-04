package model

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"go.yaml.in/yaml/v3"
)

func TestConfigRoundTripAndDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "providers.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	cfg.Providers = append(cfg.Providers, Provider{ID: "local", Name: "Local", BaseURL: "http://localhost:11434/v1", Protocol: "chat", Models: []Model{Builtin("manual/model")}})
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, loaded) {
		t.Fatalf("round trip: %#v", loaded)
	}
	cfg.Providers[0].Models[0].ID = "changed"
	if DefaultConfig().Providers[0].Models[0].ID == "changed" {
		t.Fatal("defaults share mutable state")
	}
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ConfigPath()
	if err != nil || got != filepath.Join(root, "writer", "providers.yaml") {
		t.Fatalf("ConfigPath = %q, %v", got, err)
	}
}

func TestConfigStrictness(t *testing.T) {
	valid := "schema: 1\nproviders:\n  - id: local\n    name: Local\n    base_url: http://localhost:11434/v1\n    protocol: chat\n    env: LOCAL_KEY\n    models: [manual]\n"
	cases := map[string]string{
		"unknown root":       valid + "api_key: secret-do-not-print\n",
		"unknown provider":   strings.Replace(valid, "    name:", "    key: secret-do-not-print\n    name:", 1),
		"duplicate field":    valid + "schema: 1\n",
		"multiple documents": valid + "---\nschema: 1\nproviders: []\n",
		"bad schema":         strings.Replace(valid, "schema: 1", "schema: 2", 1),
		"bad protocol":       strings.Replace(valid, "protocol: chat", "protocol: unknown", 1),
		"bad ID":             strings.Replace(valid, "id: local", "id: ../secret-do-not-print", 1),
		"URL key":            strings.Replace(valid, "http://localhost:11434/v1", "https://host/v1?key=secret-do-not-print", 1),
		"URL credentials":    strings.Replace(valid, "http://localhost:11434/v1", "https://secret-do-not-print@host/v1", 1),
		"URL fragment":       strings.Replace(valid, "http://localhost:11434/v1", "https://host/v1#secret-do-not-print", 1),
		"type":               strings.Replace(valid, "schema: 1", "schema: secret-do-not-print", 1),
		"empty":              "",
		"oversize":           strings.Repeat(" ", configLimit+1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.yaml")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil {
				t.Fatal("accepted invalid configuration")
			}
			if strings.Contains(err.Error(), "secret-do-not-print") {
				t.Fatalf("error leaks secret: %v", err)
			}
		})
	}
}

func TestSaveValidationPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.yaml")
	if err := Save(path, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := DefaultConfig()
	invalid.Providers = append(invalid.Providers, invalid.Providers[0])
	if err := Save(path, invalid); err == nil {
		t.Fatal("accepted duplicate provider")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("failed save changed existing config")
	}
}

func TestCredentialsNeverSerialize(t *testing.T) {
	const secret = "secret-do-not-print"
	cfg := DefaultConfig()
	client, err := NewClientWithKey(cfg.Providers[0], "manual", secret, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{cfg, client} {
		for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
			data, err := marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), secret) {
				t.Fatal("serialized a credential")
			}
		}
	}
}

func TestKeychainAndEnvironment(t *testing.T) {
	p := DefaultConfig().Providers[0]
	p.Env = "WRITER_TEST_MODEL_KEY"
	t.Setenv(p.Env, "environment-key")
	for _, tc := range []struct {
		name, key string
		err       error
		want      string
	}{
		{"stored wins", "stored-key", nil, "stored-key"},
		{"missing falls back", "", keyring.ErrNotFound, "environment-key"},
		{"unavailable falls back", "", errors.New("secret"), "environment-key"},
		{"empty falls back", "", nil, "environment-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadKey(p, func(service, id string) (string, error) {
				if service != "writer" || id != p.ID {
					t.Fatal("unexpected keychain account")
				}
				return tc.key, tc.err
			})
			if err != nil || got != tc.want {
				t.Fatalf("loadKey = %q, %v", got, err)
			}
		})
	}
	t.Setenv(p.Env, "")
	if _, err := loadKey(p, func(string, string) (string, error) { return "", errors.New("secret") }); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe keychain error: %v", err)
	}
	got, err := loadKey(p, func(string, string) (string, error) { return "", keyring.ErrNotFound })
	if err != nil || got != "" {
		t.Fatalf("keyless local endpoint: %q %v", got, err)
	}
}

func TestSetKeyNoPlaintextFallback(t *testing.T) {
	const secret = "never-print-this-key"
	called := false
	err := setKey("local", secret, func(service, id, key string) error {
		called = true
		if service != "writer" || id != "local" || key != secret {
			t.Fatal("incorrect keychain values")
		}
		return errors.New(secret)
	}, func(string, string) error { t.Fatal("unexpected delete"); return nil })
	if !called || err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe keychain failure: %v", err)
	}
	deleted := false
	err = setKey("local", "", nil, func(service, id string) error { deleted = true; return keyring.ErrNotFound })
	if err != nil || !deleted {
		t.Fatalf("clear key: %v", err)
	}
	if err := setKey("../bad", secret, nil, nil); err == nil {
		t.Fatal("accepted invalid keychain account")
	}
}

func TestUnknownSelectedProvider(t *testing.T) {
	_, err := NewClient(DefaultConfig(), "missing", "manual")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unknown provider: %v", err)
	}
}

func TestSaveErrorsKeepMessages(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(blocked, "providers.yaml"), DefaultConfig()); err == nil || !strings.HasPrefix(err.Error(), "model: mkdir configuration:") {
		t.Fatalf("directory error = %v", err)
	}
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(target, DefaultConfig()); err == nil || !strings.HasPrefix(err.Error(), "model: replace configuration:") {
		t.Fatalf("replace error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary file left: %v, %v", entries, err)
	}
}
