package model

import (
	"errors"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const keyringService = "writer"

// SetKey stores a provider credential in the OS keychain. An empty key removes
// the stored credential so the provider's environment variable can take over.
// Keychain errors never cause plaintext storage or include the supplied key.
func SetKey(providerIDValue, key string) error {
	return setKey(providerIDValue, key, keyring.Set, keyring.Delete)
}

func setKey(id, key string, set func(string, string, string) error, remove func(string, string) error) error {
	if !providerID.MatchString(id) {
		return errIDFormat
	}
	if strings.ContainsAny(key, "\r\n") {
		return errors.New("API key 不能包含换行")
	}
	if key == "" {
		err := remove(keyringService, id)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return errors.New("系统钥匙串无法删除 API key")
		}
		return nil
	}
	if err := set(keyringService, id, key); err != nil {
		return errors.New("系统钥匙串无法保存 API key；不会改用明文保存")
	}
	return nil
}

func loadKey(p Provider, get func(string, string) (string, error)) (string, error) {
	key, err := get(keyringService, p.ID)
	if err == nil && key != "" {
		return key, nil
	}
	if p.Env != "" {
		if fallback := os.Getenv(p.Env); fallback != "" {
			return fallback, nil
		}
	}
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return "", errors.New("系统钥匙串不可用：请解锁钥匙串，或设置这个提供商的 API key 环境变量")
	}
	// A credential-free endpoint (for example a local Ollama server) is supported.
	return "", nil
}
