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
		return errors.New("provider ID must be a lowercase identifier")
	}
	if strings.ContainsAny(key, "\r\n") {
		return errors.New("API key must not contain line breaks")
	}
	if key == "" {
		err := remove(keyringService, id)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return errors.New("OS keychain could not remove the API key; no plaintext fallback is used")
		}
		return nil
	}
	if err := set(keyringService, id, key); err != nil {
		return errors.New("OS keychain could not store the API key; no plaintext fallback is used")
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
		return "", errors.New("OS keychain is unavailable; unlock it or set the provider's API key environment variable")
	}
	// A credential-free endpoint (for example a local Ollama server) is supported.
	return "", nil
}
