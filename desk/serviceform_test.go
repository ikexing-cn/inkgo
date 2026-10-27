package desk

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/workspace"
)

type waitingCatalog struct {
	aiport.Service
	ready chan struct{}
}

func (s waitingCatalog) Catalog(string) (aiport.Catalog, error) { return s, nil }

func (s waitingCatalog) ListModels(ctx context.Context) ([]string, error) {
	select {
	case <-s.ready:
		return []string{"catalog-result"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// A catalog may finish after the author has left its form. Even reopening
// the same provider is a new draft; the old result must not replace it.
func TestCatalogBelongsToItsServiceForm(t *testing.T) {
	for _, action := range []string{"stay", "close", "reopen", "switch work"} {
		t.Run(action, func(t *testing.T) {
			a, _, _ := fixture(t)
			ready := make(chan struct{}, 1)
			t.Cleanup(func() { close(ready) })
			a.ai = waitingCatalog{a.ai, ready}
			a.chooseProvider("deepseek")
			a.serviceForm.edit = "deepseek"
			start := true
			tt := stillTester(func(c *ui.Context) {
				if start {
					start = false
					a.fetchModels(c)
				}
			}, 64, 64)
			tt.Frame()
			if a.busy == "" {
				t.Fatal("catalog did not start", a.errorText())
			}
			switch action {
			case "close":
				a.serviceForm.close()
			case "reopen":
				a.serviceForm.close()
				a.chooseProvider("deepseek")
				a.serviceForm.edit = "deepseek"
				a.serviceForm.provider.Name = "unsaved edit"
			case "switch work":
				a.serviceForm.key = "unsaved-test-key"
				next, err := workspace.Create(filepath.Join(t.TempDir(), "next"), "另一本", "网文")
				if err != nil {
					t.Fatal(err)
				}
				if err := a.attach(next, next.Root(), false); err != nil {
					t.Fatal(err)
				}
				if a.serviceForm.key != "" || a.serviceForm.edit != "" {
					t.Fatal("switching work kept the old service draft")
				}
			}
			before := a.serviceForm
			ready <- struct{}{}
			for deadline := time.Now().Add(3 * time.Second); !a.jobs.pending(); {
				if time.Now().After(deadline) {
					t.Fatal("catalog did not hand back its result")
				}
				time.Sleep(time.Millisecond)
			}
			a.poll()
			if a.busy != "" || a.errorText() != "" {
				t.Fatal("catalog did not finish", a.errorText())
			}
			if action == "stay" {
				if len(a.serviceForm.limits) != 1 || a.serviceForm.limits[0].id != "catalog-result" {
					t.Fatal("current form did not receive its catalog")
				}
			} else if !reflect.DeepEqual(before, a.serviceForm) {
				t.Fatal("old catalog changed a later draft")
			}
		})
	}
}
