package desk

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"
	"time"
	"writer/ai"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
	"writer/prefs"
	"writer/workspace"
)

func TestRecoverableExtensionStateBelongsToBook(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "disabled"}[off], func(t *testing.T) {
			a, _, ch := writingFixture(t, "原文。")
			talkOf(a).Input = "unsent A"
			talkOf(a).ContextReply = "old audit"
			proposeOf(a).Frame.Session = talkOf(a).Session
			proposeOf(a).TakeProposal("test", workspace.Proposal{Chapter: ch, At: workspace.ProposeEnd, Text: "A"}, false, "model")
			if off {
				a.setExtension("propose", false, false)
				a.setExtension("talk", false, false)
				a.setExtension("talk", true, false)
				a.setExtension("propose", true, false)
				if talkOf(a).Input != "unsent A" || len(proposeOf(a).Props) != 1 {
					t.Fatal("same-book drafts lost")
				}
				a.setExtension("propose", false, false)
				a.setExtension("talk", false, false)
			}
			if err := a.Open(extensionBook(t)); err != nil {
				t.Fatal(err)
			}
			a.setExtension("talk", true, false)
			a.setExtension("propose", true, false)
			if talkOf(a).Input != "" || talkOf(a).ContextReply != "" || len(proposeOf(a).Props) != 0 {
				t.Fatal("book A state survived")
			}
		})
	}
}

func TestQueuedActivationCallbacksAreDroppedOnRestart(t *testing.T) {
	var jobs hostapi.Jobs
	mod := ext.Module{ID: "worker", Setup: func(r *ext.Registrar) {
		ref := ext.Use(r, hostapi.JobsService)
		r.OnStart(func(*ext.Session) error { jobs = ref.Get(); return nil })
	}}
	a := newTestApp(ai.New(""), mod)
	defer a.Close()
	old := jobs
	called := false
	old.Post(ui.Services{}, func() { called = true })
	old.Request(ui.Services{}, "request", func(context.Context) (func(), error) { return func() { called = true }, errors.New("stale error") })
	awaitJob(t, a)
	requireExtension(t, a.extensions.Set("worker", false))
	requireExtension(t, a.extensions.Set("worker", true))
	a.poll()
	old.Post(ui.Services{}, func() { called = true })
	a.poll()
	if called || a.busy != "" || a.status.Err() != nil {
		t.Fatal("stale callback accepted", called, a.busy, a.status.Err())
	}
}

func TestOpaquePreferencesSurviveInactiveSaveAndReopen(t *testing.T) {
	var value string
	mod := ext.Module{ID: "opaque", Setup: func(r *ext.Registrar) {
		r.Persist(ext.BookState{Load: func(m map[string]json.RawMessage) { value = ""; _ = json.Unmarshal(m["unknown-key"], &value) }, Save: func() map[string]json.RawMessage {
			b, _ := json.Marshal(value)
			return map[string]json.RawMessage{"unknown-key": b}
		}})
	}}
	a := newTestApp(ai.New(""), mod)
	defer a.Close()
	root := extensionBook(t)
	requireExtension(t, a.Open(root))
	value = "kept"
	requireExtension(t, a.savePrefs())
	requireExtension(t, a.extensions.Set("opaque", false))
	requireExtension(t, a.savePrefs())
	requireExtension(t, a.Open(root))
	requireExtension(t, a.savePrefs())
	value = ""
	requireExtension(t, a.extensions.Set("opaque", true))
	if value != "kept" {
		t.Fatal(value)
	}
}

func TestCheckTimingPersistsAcrossActivationAndReopen(t *testing.T) {
	a, _, _ := writingFixture(t, "正文。")
	checkOf(a).State.When = "always"
	requireExtension(t, a.savePrefs())
	root := a.Store.Root()
	a.setExtension("check", false, false)
	requireExtension(t, a.Open(root))
	a.setExtension("check", true, false)
	if checkOf(a).State.When != "always" {
		t.Fatal(checkOf(a).State.When)
	}
}

func TestUnreadablePreferencesUseDefaultsAndCanSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.json")
	requireExtension(t, os.WriteFile(path, []byte("{"), 0600))
	p := prefs.Load(path)
	if !p.Unreadable() || !p.Writing {
		t.Fatal("expected defaults and notice")
	}
	requireExtension(t, p.Save(path))
	if prefs.Load(path).Unreadable() {
		t.Fatal("next save did not repair preferences")
	}
}

func TestToolProviderRestartDropsPausedProposal(t *testing.T) {
	a, _, _ := writingFixture(t, "原文。")
	entered, release := make(chan struct{}), make(chan struct{})
	var reader *ext.PointReader[aiapi.Tool]
	// Pause inside the tool's invocation boundary, before its original callback.
	mod := ext.Module{ID: "pause", Optional: []string{"ai", "propose"}, Setup: func(r *ext.Registrar) {
		reader = ext.ReadPoint(r, aiapi.Tools)
		point := ext.CallPoint[aiapi.Tool, aiapi.ToolCall, aiapi.ToolAnswer]{Point: aiapi.Tools}
		ext.Around(r, ext.HookOf(point, "propose.propose"), "pause", func(in aiapi.ToolCall, next func(aiapi.ToolCall) aiapi.ToolAnswer) aiapi.ToolAnswer {
			close(entered)
			<-release
			return next(in)
		})
	}}
	// Reassemble the real modules with the pause extension before opening the book.
	root := a.Store.Root()
	requireExtension(t, a.Close())
	a = newTestApp(ai.New(""), append(testModules(), mod)...)
	defer a.Close()
	requireExtension(t, a.Open(root))
	tools := aiapi.SnapshotReadTools(reader)
	var tool aiapi.Tool
	for _, candidate := range tools {
		if candidate.Name == workspace.ProposeTool {
			tool = candidate
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tool.Run(context.Background(), aiapi.ToolEnv{Store: a.Store, Shared: aiapi.NewToolShared(a.Store), Session: talkOf(a).Session, Call: "paused"}, json.RawMessage(`{"chapter":"第一章","at":"end","text":"stale"}`))
	}()
	<-entered
	requireExtension(t, a.extensions.Set("propose", false))
	requireExtension(t, a.extensions.Set("propose", true))
	close(release)
	<-done
	// The native controller was replaced during reassembly; use the contribution.
	for _, item := range ext.Items(a.extensions, talkapi.Proposals) {
		item.Value.Follow(talkapi.ProposalFrame{Session: talkOf(a).Session})
		ok, _ := item.Value.State(talkOf(a).Session, "paused")
		if ok {
			t.Fatal("old provider result accepted")
		}
	}
}

func TestUsageLogFailureNoticesAreScopedAndRearmed(t *testing.T) {
	a, tt, _ := writingFixture(t, "正文。")
	call := func(record workspace.Record) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for _, i := range ext.Items(a.extensions, aiapi.Calls) {
				i.Value(aiapi.Call{Store: a.Store, Record: record})
			}
		}()
		<-done
	}
	bad := workspace.Record{}
	call(bad)
	tt.Frame()
	first := a.status.Err()
	if first == nil {
		t.Fatal("missing usage notice")
	}
	a.status.SetError(nil)
	call(bad)
	tt.Frame()
	if a.status.Err() != nil {
		t.Fatal("continuous failure repeated")
	}
	call(workspace.Record{Model: "test/model", Feature: workspace.FeatureTalk, At: time.Now(), Outcome: workspace.OutcomeDone})
	call(bad)
	tt.Frame()
	if a.status.Err() == nil {
		t.Fatal("success did not rearm")
	}
	a.status.SetError(nil)
	old := ext.ActivationOf(a.extensions, "usage")
	call(bad)
	a.setExtension("usage", false, false)
	a.setExtension("usage", true, false)
	tt.Frame()
	if a.status.Err() != nil {
		t.Fatal("old activation notice survived")
	}
	for _, i := range ext.Items(a.extensions, aiapi.Calls) {
		i.Value(aiapi.Call{Store: a.Store, Record: bad, Activation: old})
	}
	tt.Frame()
	if a.status.Err() != nil {
		t.Fatal("old observer call reached restarted usage")
	}
	call(bad)
	requireExtension(t, a.Open(extensionBook(t)))
	a.status.SetError(nil)
	tt.Frame()
	if a.status.Err() != nil {
		t.Fatal("old book notice survived")
	}
}

func TestDeskSavesPreserveUnreadablePreferences(t *testing.T) {
	for _, extension := range []bool{false, true} {
		t.Run(map[bool]string{false: "desk", true: "extension"}[extension], func(t *testing.T) {
			a, _, _ := writingFixture(t, "正文。")
			path := a.Store.PrefsPath()
			original := []byte("{ corrupt\n")
			requireExtension(t, os.WriteFile(path, original, 0600))
			a.loadPrefs()
			if extension {
				viewStateStore{a}.SaveState(ext.Book{Dir: a.Store.Root()}, "test", map[string]json.RawMessage{"value": json.RawMessage(`true`)})
			} else {
				requireExtension(t, a.savePrefs())
			}
			b, err := os.ReadFile(path + ".unreadable")
			if err != nil || string(b) != string(original) {
				t.Fatal(string(b), err)
			}
			b, err = os.ReadFile(path)
			if err != nil || !json.Valid(b) {
				t.Fatal(string(b), err)
			}
		})
	}
}

func TestDeskRetainsTransientPreferencesReadFailureUntilSave(t *testing.T) {
	a, _, _ := writingFixture(t, "正文。")
	path := a.Store.PrefsPath()
	requireExtension(t, os.Remove(path))
	requireExtension(t, os.Mkdir(path, 0700))
	a.loadPrefs()
	requireExtension(t, os.Remove(path))
	original := []byte(`{"extensions":{"inactive":{"raw":123}}}`)
	requireExtension(t, os.WriteFile(path, original, 0600))
	requireExtension(t, a.savePrefs())
	b, err := os.ReadFile(path + ".unreadable")
	if err != nil || string(b) != string(original) {
		t.Fatal(string(b), err)
	}
	if string(prefs.Load(path).Extensions["inactive"]) != `{"raw":123}` {
		t.Fatal("inactive namespace lost")
	}
}
