package ai

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/ai/aitext"
	"writer/tokens"
	"writer/workspace"
)

var protocols = []string{"chat", "responses", "anthropic"}

func serviceProtocolNames() []string {
	return []string{aitext.SettingsProtoChat(), "OpenAI Responses", "Anthropic Messages"}
}

func isLocal(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// ChooseProvider runs on the frame thread.
func (a *View) ChooseProvider(id string) {
	for _, p := range a.ai.Providers() {
		if p.ID == id {
			a.ServiceForm.load(p)
			return
		}
	}
}

func (a *View) providerModels(id string) []string {
	for _, p := range a.ai.Providers() {
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

// modelsView is the settings' AI with the AI on: which models the work
// uses, how the writing calls them, and the services kept on this
// computer.
func (a *View) modelsView(c *ui.Context) {
	if a.book.Get().Store() != nil {
		a.usage(c)
		for _, item := range a.usageItems.Items() {
			_, _ = ext.CallReadPoint(a.usageItems, item.ID, c, func(c *ui.Context) struct{} { item.Value(c); return struct{}{} })
		}
		a.host.Get().WritingSettings(c)
	}
	a.services(c)
}

// usage chooses which configured models this work talks to: one row each
// for the discussion, and for continuing and checking. A choice is kept as
// it is made, a model ID typed once the caret leaves it.
func (a *View) usage(c *ui.Context) {
	a.pal.Subhead(c, aitext.SettingsBookModels(), "")
	var typing [2]bool
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		typing[0] = a.useRow(c, locales.SettingsJobChat(), aitext.SettingsJobChatAbout(), &a.host.Get().Choices().TalkProvider, "", "讨论提供商", &a.host.Get().Choices().TalkModel, "讨论模型", &a.host.Get().Choices().TalkEffort, "讨论的思考程度")
		typing[1] = a.useRow(c, aitext.SettingsJobCheck(), aitext.SettingsJobCheckAbout(), &a.host.Get().Choices().PenProvider, aitext.SettingsSameAsTalk(), "续写和检查提供商", &a.host.Get().Choices().PenModel, "续写和检查模型", &a.host.Get().Choices().PenEffort, "续写和检查的思考程度")
	})
	if typing == [2]bool{} {
		if err := a.SaveUsage(); err != nil {
			a.status.SetError(err)
		}
	}
}

// useRow is one of usage's rows: who, the service, the model and the
// effort. With none, the row can go without a service of its own. A
// service chosen brings its first model with it. It reports whether the
// model ID is being typed.
func (a *View) useRow(c *ui.Context, who, what string, provider *string, none, providerLabel string, model *string, modelLabel string, effort *string, effortLabel string) (typing bool) {
	busy := a.host.Get().Held()
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(120).Shrink(0).Children(func() {
			ui.Text(c, who).FontSize(kit.FsBody).FontWeight(500)
			ui.Text(c, what).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).LineHeight(kit.LhBody)
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
			typing = modelField(c, model, a.providerModels(*provider)).Label(modelLabel).Placeholder(aitext.SettingsModelHint()).FillWidth().Disabled(busy).Focused()
		})
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			a.effortSelect(c, effort, effortLabel, busy)
		})
	})
	return typing
}

// services lists the model services kept on this computer; 编辑 opens a
// service's form in place, and so does adding one.
func (a *View) services(c *ui.Context) {
	f := &a.ServiceForm
	t := a.pal.Theme
	busy := a.host.Get().Held()
	a.pal.Subhead(c, aitext.SettingsServices(), aitext.SettingsServicesNote())
	// A form just opened comes into view with its card: one added opens at
	// the foot of the list, below what is shown.
	reveal := f.Edit != f.Shown
	f.Shown = f.Edit
	card := func(id string) ui.Element {
		box := ui.Column(c).FillWidth().Gap(12).Padding(12, 14).Radius(kit.RCard).Border(1, a.pal.Line)
		if reveal && f.Edit == id {
			box.ScrollIntoView()
		}
		return box
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		for _, p := range a.ai.Providers() {
			card(p.ID).Key("provider-" + p.ID).Children(func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Children(func() {
						ui.Text(c, providerLabel(p)).FontSize(kit.FsBody).FontWeight(500).SingleLine().Ellipsis("…")
						sub := p.BaseURL
						if u, err := url.Parse(p.BaseURL); err == nil && u.Host != "" {
							sub = u.Host
						}
						sub = aitext.SettingsServiceMeta(sub, len(p.Models))
						if a.book.Get().Store() != nil && a.book.Get().Store().Work().Discussion.Provider == p.ID {
							sub += " · " + aitext.SettingsInUse()
						}
						ui.Text(c, sub).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).SingleLine().Ellipsis("…")
					})
					open := f.Edit == p.ID
					text := locales.SettingsEdit()
					if open {
						text = aitext.SettingsCollapse()
					}
					if a.pal.Secondary(c, text).Label("提供商-" + p.ID).Disabled(busy).Clicked() {
						f.Close()
						if !open {
							a.ChooseProvider(p.ID)
							f.Edit = p.ID
						}
					}
				})
				if f.Edit == p.ID {
					a.providerForm(c)
				}
			})
		}
		if f.Edit == "+" {
			card("+").Children(func() { a.providerForm(c) })
			return
		}
		add := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).Gap(8).AlignItems(ui.Center).Padding(12, 14).Radius(kit.RCard).Border(1, a.pal.Line).Label("+ 添加服务").Disabled(busy)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			kit.Glyph(c, kit.IcPlus, kit.IconSm, t.TextMuted)
			ui.Text(c, aitext.SettingsAddService()).FontSize(kit.FsBody).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			f.New()
		}
	})
	ui.Row(c).FillWidth().Gap(16).Justify(ui.End).Children(func() {
		if a.pal.Ghost(c, aitext.SettingsOpenConfig(), ui.Color{}).Disabled(a.ai.ConfigFile() == "").Clicked() {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(a.ai.ConfigFile())}
			if len(u.Path) > 1 && u.Path[1] == ':' {
				u.Path = "/" + u.Path
			}
			c.OpenURL(u.String())
		}
		if a.pal.Ghost(c, aitext.SettingsReload(), ui.Color{}).Disabled(busy || a.ai.ConfigFile() == "").Clicked() {
			err := a.ai.Reload()
			if err == nil {
				a.host.Get().ModelsChanged()
				f.Close()
				if ps := a.ai.Providers(); len(ps) > 0 {
					a.ChooseProvider(ps[0].ID)
				}
				a.status.Did(aitext.SettingsReloaded())
			}
			a.status.SetError(err)
		}
	})
}

// providerSelect picks a configured service, shown by its name, into *id;
// none, when given, is the choice of no service of its own. It reports
// whether the choice changed.
func (a *View) providerSelect(c *ui.Context, id *string, none, label string, disabled bool) bool {
	var names []string
	if none != "" {
		names = append(names, none)
	}
	chosen := none
	for _, p := range a.ai.Providers() {
		names = append(names, providerLabel(p))
		if p.ID == *id {
			chosen = providerLabel(p)
		}
	}
	if *id != "" && chosen == none { // a service no longer configured
		chosen = *id
		names = append(names, *id)
	}
	if !a.pal.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		return false
	}
	*id = ""
	if chosen != none {
		*id = chosen
	}
	for _, p := range a.ai.Providers() {
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
func (a *View) effortSelect(c *ui.Context, value *string, label string, disabled bool) {
	efforts := a.ai.Efforts()
	names := make([]string, len(efforts))
	chosen := a.ai.EffortLabel(*value)
	for n, e := range efforts {
		names[n] = a.ai.EffortLabel(e)
	}
	if a.pal.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		for _, e := range efforts {
			if a.ai.EffortLabel(e) == chosen {
				*value = e
			}
		}
	}
}

// SaveUsage keeps the models usage shows when they differ from the work's
// and each names a model; a service chosen without one waits for it.
func (a *View) SaveUsage() error {
	w := a.book.Get().Store().Work()
	was := w
	w.Discussion = workspace.Selection{Provider: kit.Trim(a.host.Get().Choices().TalkProvider), Model: kit.Trim(a.host.Get().Choices().TalkModel), Effort: a.host.Get().Choices().TalkEffort}
	w.Writing = workspace.Selection{Provider: kit.Trim(a.host.Get().Choices().PenProvider), Model: kit.Trim(a.host.Get().Choices().PenModel), Effort: a.host.Get().Choices().PenEffort}
	if w.Writing.Provider == "" {
		w.Writing.Model = ""
	}
	if w.Discussion == was.Discussion && w.Writing == was.Writing {
		return nil
	}
	// The store takes a service only with a model.
	for _, s := range []workspace.Selection{w.Discussion, w.Writing} {
		if s.Provider != "" && s.Model == "" {
			return nil
		}
	}
	return a.book.Get().Store().SaveWork(w)
}

// providerForm edits the service open in the list, or a new one, under
// its row. What keeps it from being saved is said in red under the field
// it is in, and the buttons that save it wait; a field left empty only
// makes them wait.
func (a *View) providerForm(c *ui.Context) {
	f := &a.ServiceForm
	t := a.pal.Theme
	busy := a.host.Get().Held()
	p, bad := f.value(a.ai)
	red := func(why string) {
		ui.Text(c, why).FontSize(kit.FsNote).TextColor(t.Danger).LineHeight(kit.LhBody)
	}
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			kit.Labeled(c, aitext.SettingsIDLabel(), func() {
				ui.TextInput(c, &f.Provider.ID).Label(aitext.SettingsProviderID()).Placeholder(p.ID).FillWidth().Disabled(busy || f.ProviderID != "")
			}).Width(200)
			kit.Labeled(c, aitext.SettingsDisplayName(), func() {
				ui.TextInput(c, &f.Provider.Name).Label(aitext.SettingsDisplayName()).FillWidth().Disabled(busy)
			}).Grow(1)
		})
		if bad.ID != nil && p.ID != "" {
			red(a.host.Get().ErrorText(bad.ID))
		}
		kit.Labeled(c, aitext.SettingsAPIURL(), func() {
			ui.TextInput(c, &f.Provider.BaseURL).Label(aitext.SettingsAPIURL()).Placeholder("https://api.example.com/v1").FillWidth().Disabled(busy)
			if kit.Trim(f.Provider.BaseURL) != "" {
				if bad.BaseURL != nil {
					red(a.host.Get().ErrorText(bad.BaseURL))
				} else if isLocal(f.Provider.BaseURL) {
					ui.Text(c, aitext.SettingsLocalNote()).FontSize(kit.FsNote).TextColor(t.TextMuted)
				} else {
					ui.Text(c, aitext.SettingsRemoteNote()).FontSize(kit.FsNote).TextColor(t.Warning)
				}
			}
		})
		kit.Labeled(c, aitext.SettingsProtocol(), func() {
			a.pal.SelectNamed(c, &f.Provider.Protocol, protocols, protocolName).Label(aitext.SettingsProtocolAPI()).FillWidth().Disabled(busy)
			if bad.Protocol != nil {
				red(a.host.Get().ErrorText(bad.Protocol))
			}
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			kit.Labeled(c, "API key", func() {
				hint := aitext.SettingsKeyEmptySaved()
				if f.ProviderID == "" {
					hint = aitext.SettingsKeyEmptyNew() // a new service has none kept
				}
				ui.TextInput(c, &f.Key).Label("API key").Password().Placeholder(hint).FillWidth().Disabled(busy)
			}).Grow(1).Basis(0)
			kit.Labeled(c, aitext.SettingsEnvOr(), func() {
				ui.TextInput(c, &f.Provider.Env).Label(aitext.SettingsEnvName()).Placeholder(aitext.SettingsEnvExample()).FillWidth().Disabled(busy)
				if bad.Env != nil {
					red(a.host.Get().ErrorText(bad.Env))
				}
			}).Grow(1).Basis(0)
		})
		hint := aitext.SettingsKeyNote()
		if env := kit.Trim(f.Provider.Env); env != "" && os.Getenv(env) != "" {
			hint += aitext.SettingsEnvDetected(env)
		}
		ui.Text(c, hint).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).Margin(-6, 0, 0, 0)
		kit.Labeled(c, aitext.SettingsModels(), func() {
			a.limitTable(c)
			if bad.Models != nil {
				red(a.host.Get().ErrorText(bad.Models))
			}
		})
		ui.Text(c, aitext.SettingsCompressNote()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if f.ProviderID != "" && a.pal.Ghost(c, aitext.SettingsClearKey(), t.Danger).Disabled(busy).Clicked() {
				err := a.ai.SetKey(f.ProviderID, "")
				a.host.Get().ModelsChanged()
				if err == nil {
					a.status.Did(aitext.SettingsKeyCleared())
				}
				a.status.SetError(err)
				f.Key = ""
			}
			// A new service has no 收起 above it: it is put away from here.
			if f.ProviderID == "" && a.pal.Secondary(c, locales.CommonCancel()).Disabled(busy).Clicked() {
				f.Close()
			}
			if a.pal.Secondary(c, aitext.SettingsAddModel()).Disabled(busy).Clicked() {
				m, _ := a.ai.Limits("")
				f.Limits = append(f.Limits, f.LimitRow(m))
			}
			if a.pal.Secondary(c, aitext.SettingsFetchModels()).Disabled(a.host.Get().Busy() || bad.First() != nil).Clicked() { // it waits for a turn in flight: one request at a time
				a.FetchModels(c)
			}
			if a.pal.Primary(c, aitext.SettingsSaveProvider()).Disabled(busy || bad.First() != nil).Clicked() {
				err := a.SaveProvider()
				if err == nil {
					a.status.Did(aitext.SettingsSaved())
					f.Edit = f.ProviderID
				}
				a.status.SetError(err)
			}
		})
	})
}

// limitTable edits the models of the service in the form, one row each:
// the ID and three numbers. An ID typed that the table built in knows
// brings its numbers.
func (a *View) limitTable(c *ui.Context) {
	f := &a.ServiceForm
	t := a.pal.Theme
	busy := a.host.Get().Held()
	if len(f.Limits) == 0 {
		ui.Text(c, aitext.SettingsNoModels()).FontSize(kit.FsNote).TextColor(t.TextMuted)
		return
	}
	gone := -1
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			head := func(s string) ui.Element { return ui.Text(c, s).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0) }
			head(aitext.SettingsModelID()).Grow(1).Shrink(1).MinWidth(0)
			head(aitext.SettingsContextLimit()).Width(96)
			head(aitext.SettingsMaxOutput()).Width(96)
			head(aitext.SettingsCompressPct()).Width(72)
			ui.Column(c).Width(kit.HControlSm).Shrink(0)
		})
		for i := range f.Limits {
			r := &f.Limits[i]
			n := "-" + strconv.Itoa(i+1) // the row's fields are told apart by its place
			ui.Row(c.Key("limit-" + strconv.Itoa(r.Key))).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.TextInput(c, &r.ID).Label("模型 ID" + n).Placeholder(aitext.SettingsModelID()).Grow(1).Shrink(1).MinWidth(0).Disabled(busy).Changed() {
					if m, ok := a.ai.Limits(kit.Trim(r.ID)); ok {
						r.Window, r.Output, r.At = tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)
					}
				}
				ui.TextInput(c, &r.Window).Label("上下文上限" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.Output).Label("最大输出" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.At).Label("压缩阈值" + n).Width(72).Shrink(0).Disabled(busy)
				if kit.IconButton(c, kit.IcX, aitext.SettingsRemoveModel()+n, ui.Color{}).Tooltip(aitext.SettingsRemoveModel()).Disabled(busy).Clicked() {
					gone = i
				}
			})
		}
	})
	if gone >= 0 {
		f.Limits = append(f.Limits[:gone], f.Limits[gone+1:]...)
	}
}

// FetchModels runs on the frame thread.
func (a *View) FetchModels(c *ui.Context) {
	if err := a.SaveProvider(); err != nil {
		a.status.SetError(err)
		return
	}
	catalog, err := a.ai.Catalog(a.ServiceForm.Provider.ID)
	if err != nil {
		a.status.SetError(err)
		return
	}
	generation := a.ServiceForm.Generation
	a.host.Get().Run(c, aitext.SettingsFetching(), func(ctx context.Context) (func(), error) {
		models, err := catalog.ListModels(ctx)
		return func() {
			if a.ServiceForm.Generation != generation {
				return // the form was closed, reloaded or replaced while waiting
			}
			if err == nil {
				a.ServiceForm.Limits = nil
				for _, id := range models {
					m, _ := a.ai.Limits(id)
					a.ServiceForm.Limits = append(a.ServiceForm.Limits, a.ServiceForm.LimitRow(m))
				}
				a.status.Did(aitext.SettingsModelsFetched())
			}
		}, err
	})
}

// SaveProvider runs on the frame thread.
func (a *View) SaveProvider() error {
	f := &a.ServiceForm
	p, bad := f.value(a.ai)
	if why := bad.First(); why != nil {
		return &aiapi.SetupError{Err: why}
	}
	if err := a.ai.SaveProvider(p); err != nil {
		return err
	}
	f.Provider = p
	f.setModels(p.Models)
	f.ProviderID = p.ID
	defer a.host.Get().ModelsChanged()
	if f.Key != "" {
		if err := a.ai.SetKey(p.ID, f.Key); err != nil {
			return err
		}
		f.Key = ""
	}
	return nil
}

// modelField edits a model ID. A click or Down lists all of the service's
// models, typing narrows them to those containing what was typed, and an ID
// none of them is can be typed as well.
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

// protocolName is how a protocol ID reads.
func protocolName(id string) string {
	names := serviceProtocolNames()
	for i, p := range protocols {
		if p == id {
			return names[i]
		}
	}
	return names[0]
}
