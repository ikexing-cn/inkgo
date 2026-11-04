package prefs

import (
	"encoding/json"
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
	// The right column's width and selected tab, and whether a turn from
	// there leaves the chapter out.
	PanelWidth float32 `json:"panel_width,omitempty"`
	PaneTab    string  `json:"pane_tab,omitempty"`
	NoAttach   bool    `json:"no_attach,omitempty"`
	// Carets is where the caret was left in each chapter, by ID.
	Carets map[string]int `json:"carets,omitempty"`
	// TextSize is the size of the chapters' text, or 0 for the default.
	TextSize int `json:"text_size,omitempty"`
	// Shelved are the conversations put out of the list, still on disk.
	Shelved []string `json:"shelved_sessions,omitempty"`
	// Ignored are the keys of the pending things the author ignored.
	Ignored []string `json:"ignored,omitempty"`
	// LateAfter is how many chapters may follow a foreshadowing before it
	// is listed as buried long, or 0 for the default.
	LateAfter int `json:"late_after,omitempty"`
	// CheckWhen is when chapters are checked against the setting: as the
	// author leaves one changed when empty, "manual" or "always".
	CheckWhen string `json:"check_when,omitempty"`
}

func Load(path string) Prefs {
	p := Prefs{Writing: true} // a work new to this machine opens on its prose
	if b, err := os.ReadFile(path); err == nil {
		p = Prefs{}
		_ = json.Unmarshal(b, &p) // a damaged file is as good as none
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
		err = atomicfile.Write(path, b, 0644)
	}
	return err
}
