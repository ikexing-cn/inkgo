package desk

import (
	"cmp"
	"github.com/egoist/mygo/ui"
	"slices"
	"writer/desk/kit"
	"writer/ext"
)

func (a *App) simpleSettings(id string) []ext.SettingInfo {
	var out []ext.SettingInfo
	for _, s := range a.extensions.Ledger(id).Settings {
		if s.Scope == ext.BookScope && a.Store == nil {
			continue
		}
		switch v := s.Schema.(type) {
		case ext.Schema[bool]:
			out = append(out, s)
		case ext.Schema[string]:
			if len(v.Choices) > 0 {
				out = append(out, s)
			}
		}
	}
	slices.SortStableFunc(out, func(x, y ext.SettingInfo) int {
		if n := cmp.Compare(x.Group, y.Group); n != 0 {
			return n
		}
		return cmp.Compare(x.Order, y.Order)
	})
	return out
}

func (a *App) drawSimpleSettings(c *ui.Context, id string) {
	for _, s := range a.simpleSettings(id) {
		_ = ext.Guard(a.extensions, id, s.ID, func() {
			title := s.ID
			if s.Title != nil {
				title = s.Title()
			}
			switch schema := s.Schema.(type) {
			case ext.Schema[bool]:
				value := s.Read().(bool)
				if ui.Checkbox(c, &value, title).Changed() {
					s.Write(value)
				}
			case ext.Schema[string]:
				value := s.Read().(string)
				var values, labels []string
				for _, choice := range schema.Choices {
					values = append(values, choice.Value)
					label := choice.Value
					if choice.Desc != nil {
						label = choice.Desc()
					}
					labels = append(labels, label)
				}
				a.prefsRow(c, title, func() {
					if kit.Dots(c, &value, title, values, labels) {
						s.Write(value)
					}
				})
			}
			if s.Desc != nil {
				ui.Text(c, s.Desc()).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).LineHeight(kit.LhBody)
			}
		})
	}
}
