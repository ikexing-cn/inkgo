package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"writer/internal/atomicfile"
)

const ModelsURL = "https://models.dev/api.json"
const modelsBodyLimit = 32 << 20

func modelsClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}
}

// DownloadModelsJSON fetches and filters a bounded models.dev response.
func DownloadModelsJSON(ctx context.Context, officialOnly bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsURL, nil)
	if err != nil {
		return nil, err
	}
	client := modelsClient()
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev: HTTP %d", res.StatusCode)
	}
	return readModels(res.Body, officialOnly)
}

func readModels(r io.Reader, officialOnly bool) ([]byte, error) {
	data, err := readModelsBody(r)
	if err != nil {
		return nil, err
	}
	return FilterModelsJSON(data, officialOnly)
}

func readModelsBody(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, modelsBodyLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > modelsBodyLimit {
		return nil, fmt.Errorf("models.dev: response exceeds 32 MB")
	}
	return data, nil
}

type refreshState struct {
	ETag    string    `json:"etag"`
	Checked time.Time `json:"checked"`
}

var refreshMu sync.Mutex

// StartModelUpdates loads the local cache before configuration is decoded,
// then refreshes it in the background at most once per day. No UI callbacks.
func StartModelUpdates(configPath string) {
	if configPath == "" {
		return
	}
	dir := filepath.Dir(configPath)
	loadModelCache(dir)
	go func() {
		client := modelsClient()
		defer client.CloseIdleConnections()
		for {
			if err := refreshModels(context.Background(), dir, ModelsURL, client, time.Now()); err != nil {
				log.Printf("models.dev: %v", err)
			}
			timer := time.NewTimer(24 * time.Hour)
			<-timer.C
		}
	}()
}

func loadModelCache(dir string) bool {
	file, err := os.Open(filepath.Join(dir, "models.json"))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		log.Printf("models.dev cache: %v", err)
		return false
	}
	defer file.Close()
	data, err := readModelsBody(file)
	if err != nil {
		log.Printf("models.dev cache: %v", err)
		return false
	}
	d, err := parseDirectory(data, false)
	if err != nil {
		log.Printf("models.dev cache: %v", err)
		return false
	}
	cachedDirectory.Store(&d)
	return true
}

func refreshModels(ctx context.Context, dir, url string, client *http.Client, now time.Time) error {
	refreshMu.Lock()
	defer refreshMu.Unlock()
	statePath := filepath.Join(dir, "models-meta.json")
	var state refreshState
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	if !state.Checked.IsZero() && now.Sub(state.Checked) < 24*time.Hour {
		return nil
	}
	// Persist attempts too: failures must not produce repeated startup requests.
	previousETag := state.ETag
	state.Checked = now
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	writeState := func() error {
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return atomicfile.Write(statePath, append(data, '\n'), 0600)
	}
	if err := writeState(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// Do not allow 304 to strand a missing/corrupt cache forever.
	if data, err := os.ReadFile(filepath.Join(dir, "models.json")); previousETag != "" && err == nil {
		if _, err := parseDirectory(data, false); err == nil {
			req.Header.Set("If-None-Match", previousETag)
		}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified {
		if req.Header.Get("If-None-Match") == "" {
			return fmt.Errorf("models.dev: unexpected 304")
		}
		return nil
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("models.dev: HTTP %d", res.StatusCode)
	}
	data, err := readModels(res.Body, false)
	if err != nil {
		return err
	}
	if err := atomicfile.Write(filepath.Join(dir, "models.json"), data, 0600); err != nil {
		return err
	}
	d := mustDirectory(data)
	cachedDirectory.Store(&d)
	state.ETag = res.Header.Get("ETag")
	return writeState()
}
