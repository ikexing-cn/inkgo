package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func isolatedDirectory(t *testing.T) {
	t.Helper()
	old := cachedDirectory.Load()
	cachedDirectory.Store(nil)
	t.Cleanup(func() { cachedDirectory.Store(old) })
}

func entry(window int, price float64) modelEntry {
	var m modelEntry
	m.Limit = limits{Context: window, Output: 100}
	m.Cost.Input, m.Cost.Output = &price, &price
	return m
}

func TestFilterModelsJSON(t *testing.T) {
	raw := `{"openai":{"name":"ignored","models":{"ok":{"limit":{"context":1000,"output":10,"input":900,"unknown":1},"cost":{"input":2,"output":3,"unknown":1},"name":"ignored"},"bad":{"limit":"bad"},"null":null}},"other":{"models":{"ok":{"limit":{"context":1000,"output":10},"cost":{"input":0,"output":0}}}}}`
	for _, official := range []bool{true, false} {
		data, err := FilterModelsJSON([]byte(raw), official)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "unknown") || strings.Contains(string(data), "ignored") || strings.Contains(string(data), "bad") || strings.Contains(string(data), "null") {
			t.Fatal(string(data))
		}
		d := mustDirectory(data)
		if _, ok := d["other"]; ok == official {
			t.Fatal(string(data))
		}
		if *d["openai"].Models["ok"].Limit.Input != 900 {
			t.Fatal(string(data))
		}
		again, err := FilterModelsJSON(data, official)
		if err != nil || !bytes.Equal(data, again) {
			t.Fatal("unstable output", err)
		}
	}
	for _, bad := range []string{`{`, `null`, `[]`, `{}`, `{"x":{"models":{"bad":{}}}}`} {
		if _, err := FilterModelsJSON([]byte(bad), false); err == nil {
			t.Fatal(bad)
		}
	}
	// The embedded seed uses exactly the generated format.
	data, err := FilterModelsJSON(embeddedModels, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, embeddedModels) {
		t.Fatal("noncanonical seed")
	}
	again, err := FilterModelsJSON(data, true)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("unstable generated format", err)
	}
}

func TestDirectoryLookupOrder(t *testing.T) {
	isolatedDirectory(t)
	d := directory{
		"deepseek":   {map[string]modelEntry{"same": entry(1000, 1)}},
		"moonshotai": {map[string]modelEntry{"same": entry(2000, 2)}},
		"relay":      {map[string]modelEntry{"same": entry(3000, 3), "relay-only": entry(4000, 4)}},
	}
	cachedDirectory.Store(&d)
	for id, want := range map[string]int{"same": 1000, "relay-only": 4000, "deepseek-flash": 1048576, "unknown": 32000} {
		if got := Builtin(id); got.Window != want {
			t.Fatal(id, got)
		}
	}
	delete(d, "deepseek")
	if Builtin("same").Window != 2000 {
		t.Fatal("official order")
	}
	delete(d, "moonshotai")
	d["a-relay"] = providerEntry{map[string]modelEntry{"same": entry(5000, 5)}}
	if Builtin("same").Window != 5000 {
		t.Fatal("third-party order")
	}

}

func TestTiersAndCacheFallback(t *testing.T) {
	isolatedDirectory(t)
	data := []byte(`{"openai":{"models":{"tiered":{"limit":{"context":10000,"output":100},"cost":{"input":1,"output":2,"tiers":[{"tier":{"type":"context","size":200},"input":5,"output":6},{"tier":{"type":"context","size":100},"input":3,"output":4}]}}}}}`)
	d := mustDirectory(data)
	cachedDirectory.Store(&d)
	for _, tc := range []struct {
		input int
		rate  float64
	}{{100, 1}, {101, 3}, {200, 3}, {201, 5}} {
		cost, cur, ok := Cost("tiered", Usage{Input: tc.input, CacheRead: 20, CacheWrite: 10})
		if !ok || cur != "$" || math.Abs(cost-float64(tc.input)*tc.rate/1e6) > 1e-12 {
			t.Fatal(tc, cost, cur, ok)
		}
	}
	// models.dev explicitly lists this cache-write price (unlike the old table).
	cost, _, _ := Cost("gpt-6-astra", Usage{Input: 100, CacheWrite: 100})
	if cost != 100*12.5/1e6 {
		t.Fatal(cost)
	}
}

// Exercise HTTP through httptest without a listening socket. The transport
// honors cancellation so http.Client.Timeout is exercised in the timeout case.
type handlerTransport struct{ handler http.Handler }

func (tr handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	done := make(chan *http.Response, 1)
	go func() {
		rec := httptest.NewRecorder()
		tr.handler.ServeHTTP(rec, req)
		done <- rec.Result()
	}()
	select {
	case res := <-done:
		return res, nil
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
}

func TestRefreshModels(t *testing.T) {
	for _, mode := range []string{"success", "304", "bad", "timeout", "http-error"} {
		t.Run(mode, func(t *testing.T) {
			isolatedDirectory(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "providers.yaml")
			// Both limits are explicit and must survive directory updates.
			yaml := "schema: 1\nproviders:\n- id: kimi\n  name: Kimi\n  base_url: https://example.com\n  protocol: chat\n  models:\n  - id: kimi-k3\n    context: 2M\n    max_output: 500\n"
			if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "models.json"), embeddedModels, 0600); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			meta, _ := json.Marshal(refreshState{ETag: `"old"`, Checked: now.Add(-25 * time.Hour)})
			if err := os.WriteFile(filepath.Join(dir, "models-meta.json"), meta, 0600); err != nil {
				t.Fatal(err)
			}
			var calls int
			var mu sync.Mutex
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls++
				mu.Unlock()
				if r.Header.Get("If-None-Match") != `"old"` {
					t.Error("missing etag")
				}
				switch mode {
				case "success":
					w.Header().Set("ETag", `"new"`)
					io.WriteString(w, `{"moonshotai":{"models":{"kimi-k3":{"limit":{"context":3000000,"output":200},"cost":{"input":1,"output":2}}}},"relay":{"models":{"new":{"limit":{"context":9000,"output":100},"cost":{"input":1,"output":2}}}}}`)
				case "304":
					w.WriteHeader(http.StatusNotModified)
				case "bad":
					io.WriteString(w, `{`)
				case "timeout":
					<-r.Context().Done()
				default:
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			})
			client := &http.Client{Transport: handlerTransport{handler}, Timeout: time.Second}
			err = refreshModels(context.Background(), dir, "https://example.com", client, now)
			if (err == nil) != (mode == "success" || mode == "304") {
				t.Fatal(err)
			}
			if mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "models.json"))
			if err != nil {
				t.Fatal(err)
			}
			if mode != "success" && !bytes.Equal(data, embeddedModels) {
				t.Fatal("cache changed")
			}
			if mode == "success" {
				if m, ok := Lookup("new"); !ok || m.Window != 9000 {
					t.Fatal(m, ok)
				}
				// Already decoded configs and explicit YAML values both stay unchanged.
				after, err := Load(path)
				if err != nil || before.Model("kimi", "kimi-k3") != after.Model("kimi", "kimi-k3") {
					t.Fatal(before, after, err)
				}
				if Builtin("kimi-k3").Window != 3000000 {
					t.Fatal("memory not replaced")
				}
			}
			if err := refreshModels(context.Background(), dir, "https://example.com", client, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			count := calls
			mu.Unlock()
			if count != 1 {
				t.Fatal(count)
			}
			meta, err = os.ReadFile(filepath.Join(dir, "models-meta.json"))
			var state refreshState
			if err != nil || json.Unmarshal(meta, &state) != nil || !state.Checked.Equal(now) {
				t.Fatal(string(meta), err)
			}
			if mode == "success" && state.ETag != `"new"` {
				t.Fatal(state)
			}
		})
	}
}

func TestBadCacheFallsBack(t *testing.T) {
	isolatedDirectory(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.json"), []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if loadModelCache(dir) {
		t.Fatal("accepted bad cache")
	}
	if m := Builtin("deepseek-flash"); m.Window != 1048576 {
		t.Fatal(m)
	}
	if err := os.WriteFile(filepath.Join(dir, "models.json"), embeddedModels, 0600); err != nil {
		t.Fatal(err)
	}
	if !loadModelCache(dir) {
		t.Fatal("rejected cache")
	}
}

func TestYAMLDefaultsAndOverrides(t *testing.T) {
	isolatedDirectory(t)
	d := directory{
		"moonshotai": {map[string]modelEntry{"same": entry(2000, 2)}},
	}
	d["moonshotai"].Models["same"] = func() modelEntry { m := entry(2000, 2); m.Limit.Output = 300; return m }()
	cachedDirectory.Store(&d)
	path := filepath.Join(t.TempDir(), "providers.yaml")
	data := "schema: 1\nproviders:\n- id: kimi\n  name: Kimi\n  protocol: chat\n  base_url: https://example.com\n  models:\n  - id: same\n    context: 9K\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := before.Model("kimi", "same"); got.Window != 9000 || got.Output != 300 {
		t.Fatal(got)
	}
	newer := directory{"moonshotai": {map[string]modelEntry{"same": entry(5000, 3)}}}
	cachedDirectory.Store(&newer)
	after, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Model("kimi", "same"); got.Window != 9000 || got.Output != 100 {
		t.Fatal(got)
	}
	if got := before.Model("kimi", "same"); got.Window != 9000 || got.Output != 300 {
		t.Fatal("already loaded configuration changed", got)
	}
}

func TestBoundedModelsResponse(t *testing.T) {
	// The byte limit is enforced before parsing, for cache files and HTTP alike.
	_, err := readModels(io.LimitReader(zeroReader{}, modelsBodyLimit+1), false)
	if err == nil || !strings.Contains(err.Error(), "32 MB") {
		t.Fatal(err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestDirectoryOutputBudget(t *testing.T) {
	isolatedDirectory(t)
	m := Builtin("kimi-k3")
	if m.Window != 1048576 || m.Output != 262144 || m.Input() != 786432 {
		t.Fatal(m)
	}
	for _, tc := range []struct{ input, output, want int }{{800, 500, 200}, {800, 100, 100}, {0, 1000, 250}, {1000, 1000, 250}, {-1, 1000, 250}, {1001, 1000, 250}, {0, 500, 500}} {
		d := directory{"openai": {map[string]modelEntry{"budget": entry(1000, 1)}}}
		v := d["openai"].Models["budget"]
		v.Limit.Input = &tc.input
		v.Limit.Output = tc.output
		d["openai"].Models["budget"] = v
		cachedDirectory.Store(&d)
		if got := Builtin("budget"); got.Output != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestLimitsWithoutPricesAndBadTiers(t *testing.T) {
	isolatedDirectory(t)
	raw := `{"openai":{"models":{
 "none":{"limit":{"context":1000,"output":100,"input":"unknown"}},
 "partial":{"limit":{"context":1000,"output":100},"cost":{"input":1}},
 "bad":{"limit":{"context":1000,"output":100},"cost":{"input":"oops","output":1}},
 "tiers":{"limit":{"context":1000,"output":100},"cost":{"input":1,"output":2,"tiers":[{"tier":{"type":"context","size":100},"input":3,"output":4},{"tier":{"type":"context","size":200},"input":"bad","output":5},{"tier":{"type":"context","size":300},"input":9}]}}
 }}}`
	filtered, err := FilterModelsJSON([]byte(raw), false)
	if err != nil {
		t.Fatal(err)
	}
	d := mustDirectory(filtered)
	cachedDirectory.Store(&d)
	for _, id := range []string{"none", "partial", "bad"} {
		if m, ok := Lookup(id); !ok || m.Window != 1000 {
			t.Fatal(id, m, ok)
		}
		if _, _, ok := Cost(id, Usage{Input: 500}); ok {
			t.Fatal(id, "unexpected price")
		}
	}
	if len(d["openai"].Models["tiers"].Cost.Tiers) != 1 {
		t.Fatal("bad tier retained")
	}
	if c, _, ok := Cost("tiers", Usage{Input: 500}); !ok || c != 0.0015 {
		t.Fatal(c, ok)
	}
}

func TestConditionalHeaderRequiresETagAndCache(t *testing.T) {
	for _, tc := range []struct {
		etag, cache string
		want        bool
	}{{"", string(embeddedModels), false}, {"old", "{", false}, {"old", "", false}, {"old", string(embeddedModels), true}} {
		t.Run(tc.etag+tc.cache[:min(1, len(tc.cache))], func(t *testing.T) {
			isolatedDirectory(t)
			dir := t.TempDir()
			meta, _ := json.Marshal(refreshState{ETag: tc.etag})
			if err := os.WriteFile(filepath.Join(dir, "models-meta.json"), meta, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.cache != "" {
				if err := os.WriteFile(filepath.Join(dir, "models.json"), []byte(tc.cache), 0600); err != nil {
					t.Fatal(err)
				}
			}
			client := &http.Client{Transport: handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, exists := r.Header["If-None-Match"]
				if exists != tc.want {
					t.Error("header presence", exists)
				}
				w.Write(embeddedModels)
			})}}
			if err := refreshModels(context.Background(), dir, "https://example.com", client, time.Now()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkLoadModelCache(b *testing.B) {
	old := cachedDirectory.Load()
	defer cachedDirectory.Store(old)
	d := directory{}
	for p := 0; p < 235; p++ {
		models := map[string]modelEntry{}
		for m := 0; m < 15; m++ {
			models[fmt.Sprintf("model-%04d", m)] = entry(1048576, 1.5)
		}
		d[fmt.Sprintf("provider-%03d", p)] = providerEntry{models}
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		b.Fatal(err)
	}
	// Exactly 1.2 MB of valid JSON; padding represents insignificant whitespace.
	if len(data) > 1200000 {
		b.Fatal("fixture exceeds target", len(data))
	}
	data = append(data, bytes.Repeat([]byte(" "), 1200000-len(data))...)
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "models.json"), data, 0600); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !loadModelCache(dir) {
			b.Fatal("load failed")
		}
	}
}
