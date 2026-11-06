package backup

import (
	"context"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"testing"
	"time"
	"writer/ext"
	"writer/ext/backup/archive"
	"writer/ext/hostapi"
	"writer/workspace"
)

type panicContext struct{ context.Context }

func (panicContext) Err() error { panic("backup worker") }

type panicJobs struct {
	hostapi.Jobs
	s *ext.Session
}

func (j panicJobs) StartJob(_ ui.Services, fn func(context.Context) func()) {
	j.s.Go(func(ctx context.Context) { fn(panicContext{ctx}) })
}

type testBook struct {
	hostapi.Book
	st *workspace.Store
}

func (b testBook) Store() *workspace.Store { return b.st }

type testEditing struct{ hostapi.Editing }

func (testEditing) Flush() {}

type testPaths struct {
	hostapi.MachinePaths
	dir string
}

func (p testPaths) Dir() string               { return p.dir }
func (testPaths) SameFolder(a, b string) bool { return a == b }

func TestWorkerPanicClosesDoneAndAllowsLeave(t *testing.T) {
	for _, leave := range []string{"backupNow", "close"} {
		t.Run(leave, func(t *testing.T) {
			st, err := workspace.Create(filepath.Join(t.TempDir(), "book"), "Book", "", "en")
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			a := &View{}
			extJobs := &panicJobs{}
			mod := ext.Module{ID: "backup", Setup: func(r *ext.Registrar) {
				a.book = ext.Use(r, hostapi.BookService)
				a.editing = ext.Use(r, hostapi.EditingService)
				a.paths = ext.Use(r, hostapi.MachinePathsService)
				a.setting = ext.Setting(r, "choices", ext.Schema[Choices]{Scope: ext.Local})
				r.OnStart(func(s *ext.Session) error {
					// A bound fake Jobs service injects a panic inside Run's worker, after its defer.
					extJobs.s = s
					return nil
				})
				a.jobs = ext.Use(r, hostapi.JobsService)
				r.OnStop(func(*ext.Session) {
					if a.State.Running {
						<-a.State.Done
					}
				})
			}}
			h, err := ext.New(ext.Options{DeclareHost: func(r *ext.Registrar) {
				ext.Provide(r, hostapi.BookService, func(*ext.Session) hostapi.Book { return testBook{st: st} })
				ext.Provide(r, hostapi.EditingService, func(*ext.Session) hostapi.Editing { return testEditing{} })
				ext.Provide(r, hostapi.MachinePathsService, func(*ext.Session) hostapi.MachinePaths { return testPaths{dir: st.Root()} })
				ext.Provide(r, hostapi.JobsService, func(*ext.Session) hostapi.Jobs { return extJobs })
			}}, mod)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			a.Configure(archive.Settings{Dir: t.TempDir()})
			a.Run(ui.Services{})
			select {
			case <-a.State.Done:
			case <-time.After(time.Second):
				t.Fatal("worker did not close done")
			}
			finished := make(chan struct{})
			go func() {
				if leave == "backupNow" {
					a.backupNow()
				} else {
					h.Close()
				}
				close(finished)
			}()
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("leave stuck after panic")
			}
			h.Settle()
			ledger := h.Ledger("backup")
			if ledger.State != ext.Failed || len(ledger.Errors) == 0 {
				t.Fatal("panic not attributed to backup", ledger)
			}
			tt := ui.NewTester(func(c *ui.Context) { h.Settle(); ui.Text(c, "next frame") }, 160, 64)
			tt.Frame()
			tt.Frame()
			if !tt.HasText("next frame") {
				t.Fatal("next frame did not draw")
			}
		})
	}
}
