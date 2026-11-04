package talk

import (
	"strings"
	"testing"
	"writer/ext"
	"writer/ext/ai/aiapi"
)

type labelBackend struct{ aiapi.Backend }

func (labelBackend) EffortLabel(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func labelPanel(t *testing.T) *Panel {
	var ref *ext.Ref[aiapi.Backend]
	h, err := ext.New(ext.Options{}, ext.Module{ID: "ai", Kind: ext.Library, Setup: func(r *ext.Registrar) {
		ext.Provide(r, aiapi.Service, func(*ext.Session) aiapi.Backend { return labelBackend{} })
	}}, ext.Module{ID: "labeltest", Requires: []string{"ai"}, Setup: func(r *ext.Registrar) { ref = ext.Use(r, aiapi.Service) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return &Panel{aiRef: ref}
}

func TestReplyLabel(t *testing.T) {
	a := labelPanel(t)
	for _, tc := range []struct {
		name, effort, want string
	}{
		{"deepseek/deepseek-flash", "", "deepseek-flash"},
		{"relay/vendor/deepseek-flash", "high", "deepseek-flash · High"},
		{"bare", "low", "bare · Low"},
		{"", "high", ""}, {"provider/", "high", ""},
	} {
		if got := a.ReplyLabel(tc.name, tc.effort); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
