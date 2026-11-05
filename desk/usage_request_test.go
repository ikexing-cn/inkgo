package desk

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

type usageTransport struct {
	entered, release chan struct{}
	requests         [][]byte
}

func (f *usageTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	f.requests = append(f.requests, b)
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"续写。\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))}, nil
}

func TestCachedContinuePenCapturesUsageAtEachRequest(t *testing.T) {
	for _, initiallyOn := range []bool{false, true} {
		t.Run(map[bool]string{false: "off-then-on", true: "on-off-on"}[initiallyOn], func(t *testing.T) {
			a, tt, chapter := writingFixture(t, "正文。")
			transport := &usageTransport{}
			old := http.DefaultTransport
			http.DefaultTransport = transport
			defer func() { http.DefaultTransport = old }()
			t.Setenv("INKGO_USAGE_TEST_KEY", "test")
			backend := aiOf(a)
			requireExtension(t, backend.SaveProvider(aiapi.Provider{ID: "usage-test", Name: "Test", BaseURL: "https://usage.example/v1", Protocol: "chat", Env: "INKGO_USAGE_TEST_KEY"}))
			w := a.Store.Work()
			w.Writing = workspace.Selection{Provider: "usage-test", Model: "deepseek-flash"}
			requireExtension(t, a.Store.SaveWork(w))
			a.setExtension("usage", initiallyOn, false)
			continueViews[a].Invalidate()
			tt.Frame()
			tt.Frame()
			cached := continueViews[a].Writer.Load()
			if cached == nil {
				t.Fatal(continueViews[a].Err)
			}
			predict := func() {
				_, err := continueViews[a].Predict(context.Background(), chapter, "正文。")
				requireExtension(t, err)
			}
			records := func(want int) {
				got, err := a.Store.Records()
				if err != nil || len(got) != want {
					t.Fatalf("records %d, want %d: %v", len(got), want, err)
				}
			}
			predict()
			n := 0
			if initiallyOn {
				n = 1
			}
			records(n)
			a.setExtension("usage", false, false)
			predict()
			records(n)
			a.setExtension("usage", true, false)
			tt.Frame()
			tt.Frame()
			if continueViews[a].Writer.Load() != cached {
				t.Fatal("pen rebuilt on usage toggle")
			}
			predict()
			n++
			records(n)
			// Keep a request in flight while the receiver is replaced.
			transport.entered, transport.release = make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() { _, err := continueViews[a].Predict(context.Background(), chapter, "正文。"); done <- err }()
			<-transport.entered
			a.setExtension("usage", false, false)
			a.setExtension("usage", true, false)
			close(transport.release)
			requireExtension(t, <-done)
			records(n)
			transport.entered = nil
			predict()
			records(n + 1)
			for _, request := range transport.requests[1:] {
				if !bytes.Equal(request, transport.requests[0]) {
					t.Fatal("toggle changed request bytes")
				}
			}
		})
	}
}
