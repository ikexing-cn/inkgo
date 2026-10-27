package shelf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Old shelves use actual size; other values snap to the nearest step.
func TestShelfKeepsZoomOnAStep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "works.json")
	for _, tc := range []struct {
		value string
		want  float32
	}{
		{"", 1}, {`,"zoom":0`, 1}, {`,"zoom":1.25`, 1.25},
		{`,"zoom":0.66`, 0.67}, {`,"zoom":1.38`, 1.5},
		{`,"zoom":0.01`, 0.5}, {`,"zoom":-1`, 0.5}, {`,"zoom":4`, 2},
	} {
		t.Run(fmt.Sprint(tc.want, tc.value), func(t *testing.T) {
			if err := os.WriteFile(path, []byte(`{"schema":1,"books":[]`+tc.value+`}`), 0644); err != nil {
				t.Fatal(err)
			}
			s, fresh, err := Load(path)
			if err != nil || fresh || NearestZoom(s.Zoom) != tc.want || s.Zoom != 0 && s.Zoom != tc.want {
				t.Fatalf("read %+v, fresh %v, error %v", s, fresh, err)
			}
			if err := s.Save(path); err != nil {
				t.Fatal(err)
			}
			got, _, err := Load(path)
			if err != nil || got.Zoom != s.Zoom {
				t.Fatalf("round trip: %+v, %v", got, err)
			}
			if tc.want == 1 {
				b, err := os.ReadFile(path)
				if err != nil || strings.Contains(string(b), `"zoom"`) {
					t.Fatalf("default is not omitted: %s, %v", b, err)
				}
			}
		})
	}
}
