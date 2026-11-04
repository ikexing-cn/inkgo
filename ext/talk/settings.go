package talk

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"writer/desk/kit"
	"writer/ext/ai/aiapi"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// Choices returns the editable model draft on the frame thread.
func (a *service) Choices() *talkapi.ModelChoices { return &a.choices }

// SaveChoices persists the model draft on the frame thread.
func (a *service) SaveChoices() error {
	st := a.book.Get().Store()
	if st == nil {
		return nil
	}
	w := st.Work()
	selection := workspace.Selection{Provider: kit.Trim(a.choices.TalkProvider), Model: kit.Trim(a.choices.TalkModel), Effort: a.choices.TalkEffort}
	if selection.Provider != "" && selection.Model == "" || w.Discussion == selection {
		return nil
	}
	w.Discussion = selection
	return st.SaveWork(w)
}
func (a *service) modelSettings(c *ui.Context) {
	if a.book.Get().Store() == nil {
		return
	}
	if !a.useRow(c, locales.SettingsJobChat(), talktext.SettingsJobChatAbout(), &a.choices.TalkProvider, "", "讨论提供商", &a.choices.TalkModel, "讨论模型", &a.choices.TalkEffort, "讨论的思考程度") {
		if err := a.SaveChoices(); err != nil {
			a.notices.Get().Notices().SetError(err)
		}
	}
}

func (a *service) useRow(c *ui.Context, who, what string, provider *string, none, providerLabel string, model *string, modelLabel string, effort *string, effortLabel string) (typing bool) {
	busy := a.book.Get().Held()
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(120).Shrink(0).Children(func() {
			ui.Text(c, who).FontSize(kit.FsBody).FontWeight(500)
			ui.Text(c, what).FontSize(kit.FsNote).TextColor(a.markdown.Palette().Theme.TextMuted).LineHeight(kit.LhBody)
		})
		ui.Column(c).Width(150).Shrink(0).Children(func() {
			if a.providerSelect(c, provider, none, providerLabel, busy) {
				*model = ""
				if ids := a.providerModels(*provider); len(ids) > 0 {
					*model = ids[0]
				}
			}
		})
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			if none != "" && *provider == "" {
				*model = ""
				return
			}
			typing = modelField(c, model, a.providerModels(*provider)).Label(modelLabel).Placeholder(talktext.TalkModelHint()).FillWidth().Disabled(busy).Focused()
		})
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			a.effortSelect(c, effort, effortLabel, busy)
		})
	})
	return typing
}

func (a *service) providerSelect(c *ui.Context, id *string, none, label string, disabled bool) bool {
	var names []string
	if none != "" {
		names = append(names, none)
	}
	chosen := none
	for _, p := range a.ai.Get().Providers() {
		names = append(names, providerLabel(p))
		if p.ID == *id {
			chosen = providerLabel(p)
		}
	}
	if *id != "" && chosen == none { // a service no longer configured
		chosen = *id
		names = append(names, *id)
	}
	if !a.markdown.Palette().Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		return false
	}
	*id = ""
	if chosen != none {
		*id = chosen
	}
	for _, p := range a.ai.Get().Providers() {
		if providerLabel(p) == chosen {
			*id = p.ID
		}
	}
	return true
}

func providerLabel(p aiapi.Provider) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}

// effortSelect picks an effort, shown by its label, into *value.
func (a *service) effortSelect(c *ui.Context, value *string, label string, disabled bool) {
	efforts := a.ai.Get().Efforts()
	names := make([]string, len(efforts))
	chosen := a.ai.Get().EffortLabel(*value)
	for n, e := range efforts {
		names[n] = a.ai.Get().EffortLabel(e)
	}
	if a.markdown.Palette().Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		for _, e := range efforts {
			if a.ai.Get().EffortLabel(e) == chosen {
				*value = e
			}
		}
	}
}

func modelField(c *ui.Context, value *string, models []string) ui.Element {
	t := c.Theme()
	cb := ui.ComboboxBase(c, value)
	in := cb.Input
	in.Padding(t.Space(1.5), t.Space(2.5)).Radius(kit.RControl).Background(t.Surface).Border(1, t.Border)
	if in.Focused() {
		in.Border(1, t.Accent)
	} else if in.Hovered() {
		in.Border(1, t.Border.Mix(t.Text, 0.25))
	}
	shown := models
	if q := strings.ToLower(strings.TrimSpace(*value)); cb.Filtering() && q != "" {
		var starts, contains []string
		for _, m := range models {
			switch l := strings.ToLower(m); {
			case l == q:
			case strings.HasPrefix(l, q):
				starts = append(starts, m)
			case strings.Contains(l, q):
				contains = append(contains, m)
			}
		}
		shown = append(starts, contains...)
	}
	cb.Popup(func(panel ui.Element) {
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(kit.RCard).Background(t.Background).Border(1, t.Border)
		kit.ShadowOf(c, "pop").On(panel)
		ui.Scroll(c).MaxHeight(t.Space(70)).FillWidth().Children(func() {
			for _, m := range shown {
				item := cb.Item(m).Padding(t.Space(1.5), t.Space(2.5)).Radius(kit.RControl)
				switch {
				case item.Highlighted():
					item.Background(t.Accent).TextColor(t.AccentText)
					item.ScrollIntoView()
				case m == *value:
					item.Background(t.Surface)
				}
				item.Children(func() { ui.Text(c, m).SingleLine() })
			}
		})
	})
	if m, ok := cb.Chosen(); ok {
		*value = m
	}
	return in
}

func (a *service) providerModels(id string) []string {
	for _, p := range a.ai.Get().Providers() {
		if p.ID == id {
			return modelIDs(p)
		}
	}
	return nil
}

func modelIDs(p aiapi.Provider) []string {
	ids := make([]string, len(p.Models))
	for i, m := range p.Models {
		ids[i] = m.ID
	}
	return ids
}
