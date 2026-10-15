package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Snapshots of a document are history/<ID>/<time>.txt, with <time>.json
// beside each saying why it was taken and the marks then. They live with
// the work, outside .writer/, so a backup of the work takes them along.
const historyDir = "history"

// Why a snapshot was taken.
const (
	SnapOpen     = "open"     // the document was opened
	SnapInterval = "interval" // ten minutes of writing went by
	SnapRestore  = "restore"  // a snapshot was about to replace the text
	SnapExternal = "external" // the text changed outside, and one side gave way
	SnapAI       = "ai"       // an AI was about to change the text
	SnapReplace  = "replace"  // a replace through the book was about to change it
)

// keepAll is how long every snapshot is kept; older ones keep the last of
// each day.
const keepAll = 7 * 24 * time.Hour

const snapLayout = "20060102-150405.000"

var snapName = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}\.[0-9]{3}$`)

// Snapshot is a version of a document's text kept aside.
type Snapshot struct {
	Name   string        `json:"-"`
	At     time.Time     `json:"at"`
	Reason string        `json:"reason"`
	Title  string        `json:"title"`
	Count  int           `json:"count"`
	Marks  []ChapterMark `json:"marks,omitempty"`
	// AI is what an AI was about to do, for a snapshot taken before it.
	AI *AIRecord `json:"ai,omitempty"`
}

// What an AI did to a document.
const (
	AIDraft   = "draft"   // wrote it from the outline
	AIRewrite = "rewrite" // rewrote the words selected
)

// AIRecord is one change an AI made: what it was asked, by which model,
// and what it was shown.
type AIRecord struct {
	Op          string `json:"op"`
	Instruction string `json:"instruction,omitempty"`
	Model       string `json:"model"`
	Effort      string `json:"effort,omitempty"`
	// Shown lists what the request held besides the setting, in words;
	// Setting and Omitted the entries of the setting it held and left out.
	Shown   []string      `json:"shown,omitempty"`
	Setting []ContextItem `json:"setting,omitempty"`
	Omitted []ContextItem `json:"omitted,omitempty"`
	Sent    *Sent         `json:"sent,omitempty"`
}

func (s *Store) historyPath(id string, name ...string) (string, error) {
	if err := validID(id); err != nil {
		return "", err
	}
	return s.path(append([]string{historyDir, id}, name...)...)
}

// TakeSnapshot keeps text and marks of document id aside for reason. It
// takes none when text is what the latest snapshot holds, and reports
// whether it took one.
func (s *Store) TakeSnapshot(id, reason, text string, marks []ChapterMark) (bool, error) {
	unlock, err := s.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	return s.takeSnapshot(id, reason, text, marks, nil, time.Now())
}

// SnapshotForAI keeps the text of document id aside before an AI changes
// it, with what the AI is to do; the record is kept even when the text is
// the latest snapshot's.
func (s *Store) SnapshotForAI(id, text string, marks []ChapterMark, rec AIRecord) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	_, err = s.takeSnapshot(id, SnapAI, text, marks, &rec, time.Now())
	return err
}

func (s *Store) takeSnapshot(id, reason, text string, marks []ChapterMark, ai *AIRecord, now time.Time) (bool, error) {
	t, err := s.tree()
	if err != nil {
		return false, err
	}
	n, ok := t.byID[id]
	if !ok || n.Folder {
		return false, fmt.Errorf("chapters: no document %q", id)
	}
	snaps, err := s.snapshots(id)
	if err != nil {
		return false, err
	}
	if len(snaps) > 0 && ai == nil {
		last, err := s.snapshotText(id, snaps[0].Name)
		if err != nil {
			return false, err
		}
		if last == text {
			return false, nil
		}
	}
	dir, err := s.historyPath(id)
	if err != nil {
		return false, err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return false, err
	}
	at := now
	name := at.Format(snapLayout)
	for len(snaps) > 0 && name <= snaps[0].Name {
		at = at.Add(time.Millisecond) // two in a millisecond, or a clock gone back
		name = at.Format(snapLayout)
	}
	runes := []rune(text)
	kept := make([]ChapterMark, 0, len(marks))
	for _, m := range marks {
		m.Start, m.End = clampIndex(m.Start, len(runes)), clampIndex(m.End, len(runes))
		m.End = max(m.End, m.Start)
		m.Quote = string(runes[m.Start:m.End])
		kept = append(kept, m)
	}
	snap := Snapshot{At: at.UTC(), Reason: reason, Title: n.Title, Count: Count(text), Marks: kept, AI: ai}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return false, err
	}
	if err = atomicWrite(filepath.Join(dir, name+".txt"), []byte(text), true); err != nil {
		return false, err
	}
	if err = atomicWrite(filepath.Join(dir, name+".json"), append(b, '\n'), false); err != nil {
		return false, err
	}
	return true, s.prune(id, now)
}

// prune keeps every snapshot of the last week, and the last of each day
// before.
func (s *Store) prune(id string, now time.Time) error {
	snaps, err := s.snapshots(id)
	if err != nil {
		return err
	}
	dir, err := s.historyPath(id)
	if err != nil {
		return err
	}
	days := map[string]bool{}
	for _, sn := range snaps { // the latest first
		if now.Sub(sn.At) < keepAll {
			continue
		}
		day := sn.At.Local().Format("20060102")
		if !days[day] {
			days[day] = true
			continue
		}
		for _, ext := range []string{".txt", ".json"} {
			if err = os.Remove(filepath.Join(dir, sn.Name+ext)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (s *Store) snapshots(id string) ([]Snapshot, error) {
	dir, err := s.historyPath(id)
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snaps []Snapshot
	for _, f := range files {
		name, ok := strings.CutSuffix(f.Name(), ".txt")
		if !ok || !snapName.MatchString(name) {
			continue
		}
		at, err := time.ParseInLocation(snapLayout, name, time.Local)
		if err != nil {
			continue
		}
		sn := Snapshot{At: at}
		if b, err := os.ReadFile(filepath.Join(dir, name+".json")); err == nil {
			_ = json.Unmarshal(b, &sn) // a snapshot without its record still restores
		}
		sn.Name = name
		snaps = append(snaps, sn)
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Name > snaps[j].Name })
	return snaps, nil
}

// Snapshots returns the snapshots of document id, the latest first.
func (s *Store) Snapshots(id string) ([]Snapshot, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.snapshots(id)
}

func (s *Store) snapshotText(id, name string) (string, error) {
	if !snapName.MatchString(name) {
		return "", fmt.Errorf("history: invalid snapshot %q", name)
	}
	p, err := s.historyPath(id, name+".txt")
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	return string(b), err
}

// SnapshotText returns the text of snapshot name of document id.
func (s *Store) SnapshotText(id, name string) (string, error) {
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	return s.snapshotText(id, name)
}
