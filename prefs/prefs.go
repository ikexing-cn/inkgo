package prefs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"writer/internal/atomicfile"
)

// Prefs are the desk's own per-work settings, kept in .writer, which is
// not part of the work.
type Prefs struct {
	SettingHidden bool   `json:"setting_hidden"`
	SideHidden    bool   `json:"side_hidden,omitempty"` // the sidebar is put away
	Chapter       string `json:"chapter,omitempty"`     // the node last open
	// Tabs are the nodes open in tabs, left to right; Chapter is the one shown.
	Tabs []string `json:"tabs,omitempty"`
	// Blank says the author closed every tab: none opens by itself.
	Blank bool `json:"blank,omitempty"`
	// Writing reopens the work on the writing page, where it was left.
	Writing bool `json:"writing,omitempty"`
	// AutoPredict continues the prose as typing pauses, not on demand.
	AutoPredict bool `json:"auto_predict,omitempty"`
	// The right column's width and selected tab.
	PanelWidth float32 `json:"panel_width,omitempty"`
	PaneTab    string  `json:"pane_tab,omitempty"`
	// Carets is where the caret was left in each chapter, by ID.
	Carets map[string]int `json:"carets,omitempty"`
	// TextSize is the size of the chapters' text, or 0 for the default.
	TextSize int `json:"text_size,omitempty"`
	// Ignored are the keys of the pending things the author ignored.
	Ignored []string `json:"ignored,omitempty"`
	// LateAfter is how many chapters may follow a foreshadowing before it
	// is listed as buried long, or 0 for the default.
	LateAfter int `json:"late_after,omitempty"`
	// Extensions preserves each module's opaque preferences, even while off.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	unreadable bool
	recovery   *bool
}

func Load(path string) Prefs {
	p := Prefs{Writing: true} // a work new to this machine opens on its prose
	if b, err := os.ReadFile(path); err == nil {
		if !json.Valid(b) {
			p = Prefs{Writing: true, unreadable: true, recovery: new(bool)}
			return p
		}
		p = Prefs{}
		if err := json.Unmarshal(b, &p); err != nil {
			return Prefs{Writing: true, unreadable: true, recovery: new(bool)}
		}
		// The old pair is read only for files without pane_tab. In those
		// files foreshadowings took precedence over the remembered talk.
		var old struct {
			PaneTab *string `json:"pane_tab"`
			TalkTab bool    `json:"talk_tab"`
			ForeTab bool    `json:"fore_tab"`
		}
		_ = json.Unmarshal(b, &old)
		if old.PaneTab == nil {
			if old.ForeTab {
				p.PaneTab = "fore"
			} else if old.TalkTab {
				p.PaneTab = "talk"
			}
		}
	} else if !os.IsNotExist(err) {
		p.unreadable = true
		p.recovery = new(bool)
	}
	return p
}

func (p Prefs) Save(path string) error {
	b, err := json.Marshal(p)
	if err == nil {
		// .writer/ is kept out of git: a work fetched has none till now.
		err = os.MkdirAll(filepath.Dir(path), 0755)
	}
	if err == nil {
		err = p.PreserveUnreadable(path)
	}
	if err == nil {
		err = atomicfile.Write(path, b, 0644)
	}
	return err
}

// Unreadable reports whether loading used defaults after a read or decode failure.
func (p Prefs) Unreadable() bool { return p.unreadable }

// PreserveUnreadable moves an unreadable original beside its replacement once.
// A failed move prevents saving, including after a transient read failure.
func (p Prefs) PreserveUnreadable(path string) error {
	if !p.unreadable || p.recovery != nil && *p.recovery {
		return nil
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("cannot preserve unreadable preferences: %s is not a regular file", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(path, path+".unreadable"); err != nil && !os.IsNotExist(err) {
		return err
	}
	if p.recovery != nil {
		*p.recovery = true
	}
	return nil
}
