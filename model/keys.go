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
		return ErrKeyNewline
	}
	if key == "" {
		err := remove(keyringService, id)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return ErrKeyDelete
		}
		return nil
	}
	if err := set(keyringService, id, key); err != nil {
		return ErrKeySave
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
		return "", ErrKeychain
	}
	// A credential-free endpoint (for example a local Ollama server) is supported.
	return "", nil
}
