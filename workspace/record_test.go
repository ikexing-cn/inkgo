package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRecordCursorExternalChanges(t *testing.T) {
	for _, mode := range []string{"append", "truncate", "replace same size", "incomplete", "corrupt complete", "edit then grow", "no newline"} {
		t.Run(mode, func(t *testing.T) {
			s := testStore(t)
			r := Record{At: time.Now().UTC(), Feature: FeatureTalk, Model: "test/model", Outcome: OutcomeDone}
			for range 2 {
				if err := s.Log(r); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(s.Root(), logFile)
			before := readBytes(t, path)
			line := before[:bytes.IndexByte(before, '\n')+1]
			want, badLine := 3, 0
			switch mode {
			case "append":
				appendOutside(t, path, line)
				want = 4
			case "truncate":
				if err := os.Truncate(path, int64(len(line))); err != nil {
					t.Fatal(err)
				}
				want = 2
			case "replace same size":
				st, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := bytes.Replace(before, []byte(`"schema":2`), []byte(`"schema":3`), 1)
				tmp := path + ".replacement"
				putBytes(t, tmp, changed)
				if err := os.Chtimes(tmp, st.ModTime(), st.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(tmp, path); err != nil {
					t.Fatal(err)
				}
				badLine = 1
			case "incomplete":
				appendOutside(t, path, []byte(`{"schema":2,"at":`))
			case "corrupt complete":
				appendOutside(t, path, []byte("{broken}\n"))
				badLine = 3
			case "edit then grow":
				changed := bytes.Replace(before, []byte(`"schema":2`), []byte(`"schema":3`), 1)
				putBytes(t, path, append(changed, line...))
				badLine = 1
			case "no newline":
				putBytes(t, path, bytes.TrimSuffix(before, []byte("\n")))
			}
			unchanged := readBytes(t, path)
			err := s.Log(r)
			if badLine > 0 {
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%s:%d:", path, badLine)) {
					t.Fatalf("error = %v, want line %d", err, badLine)
				}
				if !bytes.Equal(unchanged, readBytes(t, path)) {
					t.Fatal("failed append changed file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			all, err := s.Records()
			if err != nil || len(all) != want {
				t.Fatalf("records = %d, want %d, %v", len(all), want, err)
			}
			c := s.cursors[path]
			if c.lines != want {
				t.Fatalf("cursor lines %d, want %d", c.lines, want)
			}
		})
	}
}

func TestRecordCursorRevalidatesAfterFailedTail(t *testing.T) {
	s := testStore(t)
	r := Record{Schema: Schema, At: time.Now().UTC(), Feature: FeatureTalk, Model: "test/model", Outcome: OutcomeDone}
	for range 2 {
		if err := s.Log(r); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(s.Root(), logFile)
	good := readBytes(t, path)
	line, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	appendOutside(t, path, append(append(line, '\n'), []byte("{}\n")...))
	if err := s.Log(r); err == nil {
		t.Fatal("corrupt tail accepted")
	}
	putBytes(t, path, good)
	if err := s.Log(r); err != nil {
		t.Fatal(err)
	}
	if records, err := s.Records(); err != nil || len(records) != 3 {
		t.Fatalf("repaired = %v, %v", records, err)
	}
}

// Each request is a line of the AI record, in the work's folder, the
// oldest first; a line written in half is dropped.
func TestTheRecordKeepsEachRequest(t *testing.T) {
	s := testStore(t)
	if got, err := s.Records(); err != nil || len(got) != 0 {
		t.Fatalf("a new work has %+v %v", got, err)
	}
	at := time.Date(2026, 10, 16, 10, 40, 0, 0, time.UTC)
	talk := Record{At: at, Took: 4200, Feature: FeatureTalk, Talk: "s1", Model: "deepseek/deepseek-flash", Effort: "high",
		Parts: []Part{{PartRules, 1100}, {PartTalk, 9600}}, Usage: Usage{Input: 12000, CacheRead: 9000, Output: 300},
		Cost: 0.004, Currency: "$", Outcome: OutcomeDone}
	check := Record{At: at.Add(time.Minute), Feature: FeatureCheck, Chapter: "c1", Model: "kimi/kimi-k3", Outcome: OutcomeFailed, Error: "无法连接模型服务"}
	for _, r := range []Record{talk, check} {
		if err := s.Log(r); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(s.Root(), "ai-log.jsonl")
	b, err := os.ReadFile(p)
	if err != nil || strings.Count(string(b), "\n") != 2 {
		t.Fatalf("the file holds %q, %v", b, err)
	}
	if err = os.WriteFile(p, append(b, []byte(`{"schema":2,"at":"2026-`)...), 0600); err != nil {
		t.Fatal(err)
	}
	talk.Schema, check.Schema = Schema, Schema
	got, err := s.Records()
	if err != nil || !reflect.DeepEqual(got, []Record{talk, check}) {
		t.Fatalf("read %+v, %v", got, err)
	}
	cancelled := Record{At: at.Add(2 * time.Minute), Feature: FeatureContinue, Chapter: "c1", Model: "kimi/kimi-k3", Outcome: OutcomeCancelled}
	if err = s.Log(cancelled); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Records(); err != nil || len(got) != 3 || got[2].Outcome != OutcomeCancelled {
		t.Fatalf("after the half line: %+v, %v", got, err)
	}
	for _, bad := range []Record{
		{At: at, Feature: FeatureTalk, Outcome: OutcomeDone},
		{At: at, Feature: "summary", Model: "a/b", Outcome: OutcomeDone},
		{Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeFailed},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone, Cost: 1},
		{At: at, Feature: FeatureTalk, Model: "a/b", Outcome: OutcomeDone, Usage: Usage{Input: -1}},
	} {
		if err := s.Log(bad); err == nil {
			t.Errorf("logged %+v", bad)
		}
	}
}

// A line gone wrong by hand is reported with the file and the line.
func TestTheRecordIsChecked(t *testing.T) {
	s := testStore(t)
	line := `{"schema":2,"at":"2026-10-16T10:40:00Z","feature":"talk","model":"a/b","usage":{"input":1,"output":1},"outcome":"done","words":"灯"}` + "\n"
	if err := os.WriteFile(filepath.Join(s.Root(), "ai-log.jsonl"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Records(); err == nil || !strings.Contains(err.Error(), "ai-log.jsonl:1") || !strings.Contains(err.Error(), `unknown field "words"`) {
		t.Fatal(err)
	}
}

// A request's purpose goes with its context.
func TestThePurposeGoesWithTheContext(t *testing.T) {
	if p := PurposeOf(context.Background()); !reflect.DeepEqual(p, Purpose{}) {
		t.Fatal(p)
	}
	p := Purpose{Feature: FeatureCheck, Chapter: "c1", Parts: []Part{{PartProse, 900}}}
	if got := PurposeOf(WithPurpose(context.Background(), p)); !reflect.DeepEqual(got, p) {
		t.Fatal(got)
	}
}
