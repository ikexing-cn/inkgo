package desk

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/locales"
	"writer/tokens"
	"writer/workspace"
)

var protocols = []string{"chat", "responses", "anthropic"}

func serviceProtocolNames() []string {
	return []string{locales.SettingsProtoChat(), "OpenAI Responses", "Anthropic Messages"}
}

func isLocal(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func (a *App) destination(sel workspace.Selection) string {
	for _, p := range a.ai.Providers() {
		if p.ID == sel.Provider {
			u, err := url.Parse(p.BaseURL)
			if err != nil {
				return locales.SettingsInvalidAddress()
			}
			kind := locales.SettingsRemote()
			if isLocal(p.BaseURL) {
				kind = locales.SettingsLocal()
			}
			return kind + " " + u.Host + " / " + sel.Model
		}
	}
	return locales.ChatNeedModel()
}

func (a *App) chooseProvider(id string) {
	for _, p := range a.ai.Providers() {
		if p.ID == id {
			a.serviceForm.load(p)
			return
		}
	}
}

func (a *App) providerModels(id string) []string {
	for _, p := range a.ai.Providers() {
		if p.ID == id {
			return modelIDs(p)
		}
	}
	return nil
}

func modelIDs(p aiport.Provider) []string {
	ids := make([]string, len(p.Models))
	for i, m := range p.Models {
		ids[i] = m.ID
	}
	return ids
}

// modelsView is the settings' AI with the AI on: which models the work
// uses, how the writing calls them, and the services kept on this
// computer.
func (a *App) modelsView(c *ui.Context) {
	if a.Store != nil {
		a.usage(c)
		a.spent(c)
		a.writingSwitches(c)
	}
	a.services(c)
}

// usage chooses which configured models this work talks to: one row each
// for the discussion, and for continuing and checking. A choice is kept as
// it is made, a model ID typed once the caret leaves it.
func (a *App) usage(c *ui.Context) {
	a.pal.Subhead(c, locales.SettingsBookModels(), "")
	var typing [2]bool
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		typing[0] = a.useRow(c, locales.SettingsJobChat(), locales.SettingsJobChatAbout(), &a.talkProvider, "", "讨论提供商", &a.talkModel, "讨论模型", &a.talkEffort, "讨论的思考程度")
		typing[1] = a.useRow(c, locales.SettingsJobCheck(), locales.SettingsJobCheckAbout(), &a.penProvider, locales.SettingsSameAsTalk(), "续写和检查提供商", &a.penModel, "续写和检查模型", &a.penEffort, "续写和检查的思考程度")
	})
	if typing == [2]bool{} {
		if err := a.saveUsage(); err != nil {
			a.SetError(err)
		}
	}
}

// useRow is one of usage's rows: who, the service, the model and the
// effort. With none, the row can go without a service of its own. A
// service chosen brings its first model with it. It reports whether the
// model ID is being typed.
func (a *App) useRow(c *ui.Context, who, what string, provider *string, none, providerLabel string, model *string, modelLabel string, effort *string, effortLabel string) (typing bool) {
	busy := a.held()
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
			typing = modelField(c, model, a.providerModels(*provider)).Label(modelLabel).Placeholder(locales.SettingsModelHint()).FillWidth().Disabled(busy).Focused()
		})
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			a.effortSelect(c, effort, effortLabel, busy)
		})
	})
	return typing
}

// writingSwitches are the writing page's calls to the models, kept as
// they are ticked.
func (a *App) writingSwitches(c *ui.Context) {
	busy := a.held()
	a.pal.Subhead(c, locales.SettingsWriting(), "")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.Checkbox(c, &a.write.autoPredict, locales.SettingsAutoContinue()).Disabled(busy).Changed() {
			for _, ed := range a.write.editors() {
				ed.SetPredictOnDemand(!a.write.autoPredict)
			}
			a.savePrefs() // kept as ticked, not with the models above
		}
		a.pal.TickNote(c, locales.SettingsAutoNote())
	})
}

// services lists the model services kept on this computer; 编辑 opens a
// service's form in place, and so does adding one.
func (a *App) services(c *ui.Context) {
	f := &a.serviceForm
	t := a.pal.Theme
	busy := a.held()
	a.pal.Subhead(c, locales.SettingsServices(), locales.SettingsServicesNote())
	// A form just opened comes into view with its card: one added opens at
	// the foot of the list, below what is shown.
	reveal := f.edit != f.shown
	f.shown = f.edit
	card := func(id string) ui.Element {
		box := ui.Column(c).FillWidth().Gap(12).Padding(12, 14).Radius(kit.RCard).Border(1, a.pal.Line)
		if reveal && f.edit == id {
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
						sub = locales.SettingsServiceMeta(sub, len(p.Models))
						if a.Store != nil && a.Store.Work().Discussion.Provider == p.ID {
							sub += " · " + locales.SettingsInUse()
						}
						ui.Text(c, sub).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).SingleLine().Ellipsis("…")
					})
					open := f.edit == p.ID
					text := locales.SettingsEdit()
					if open {
						text = locales.SettingsCollapse()
					}
					if a.pal.Secondary(c, text).Label("提供商-" + p.ID).Disabled(busy).Clicked() {
						f.close()
						if !open {
							a.chooseProvider(p.ID)
							f.edit = p.ID
						}
					}
				})
				if f.edit == p.ID {
					a.providerForm(c)
				}
			})
		}
		if f.edit == "+" {
			card("+").Children(func() { a.providerForm(c) })
			return
		}
		add := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).Gap(8).AlignItems(ui.Center).Padding(12, 14).Radius(kit.RCard).Border(1, a.pal.Line).Label("+ 添加服务").Disabled(busy)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			kit.Glyph(c, kit.IcPlus, kit.IconSm, t.TextMuted)
			ui.Text(c, locales.SettingsAddService()).FontSize(kit.FsBody).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			f.new()
		}
	})
	ui.Row(c).FillWidth().Gap(16).Justify(ui.End).Children(func() {
		if a.pal.Ghost(c, locales.SettingsOpenConfig(), ui.Color{}).Disabled(a.ai.ConfigFile() == "").Clicked() {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(a.ai.ConfigFile())}
			if len(u.Path) > 1 && u.Path[1] == ':' {
				u.Path = "/" + u.Path
			}
			c.OpenURL(u.String())
		}
		if a.pal.Ghost(c, locales.SettingsReload(), ui.Color{}).Disabled(busy || a.ai.ConfigFile() == "").Clicked() {
			err := a.ai.Reload()
			if err == nil {
				a.modelsChanged()
				f.close()
				if ps := a.ai.Providers(); len(ps) > 0 {
					a.chooseProvider(ps[0].ID)
				}
				a.status.Did(locales.SettingsReloaded())
			}
			a.SetError(err)
		}
	})
}

// providerSelect picks a configured service, shown by its name, into *id;
// none, when given, is the choice of no service of its own. It reports
// whether the choice changed.
func (a *App) providerSelect(c *ui.Context, id *string, none, label string, disabled bool) bool {
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

func providerLabel(p aiport.Provider) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}

// effortSelect picks an effort, shown by its label, into *value.
func (a *App) effortSelect(c *ui.Context, value *string, label string, disabled bool) {
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

// saveUsage keeps the models usage shows when they differ from the work's
// and each names a model; a service chosen without one waits for it.
func (a *App) saveUsage() error {
	w := a.Store.Work()
	was := w
	w.Discussion = workspace.Selection{Provider: kit.Trim(a.talkProvider), Model: kit.Trim(a.talkModel), Effort: a.talkEffort}
	w.Writing = workspace.Selection{Provider: kit.Trim(a.penProvider), Model: kit.Trim(a.penModel), Effort: a.penEffort}
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
	return a.Store.SaveWork(w)
}

// providerForm edits the service open in the list, or a new one, under
// its row. What keeps it from being saved is said in red under the field
// it is in, and the buttons that save it wait; a field left empty only
// makes them wait.
func (a *App) providerForm(c *ui.Context) {
	f := &a.serviceForm
	t := a.pal.Theme
	busy := a.held()
	p, bad := f.value(a.ai)
	red := func(why string) {
		ui.Text(c, why).FontSize(kit.FsNote).TextColor(t.Danger).LineHeight(kit.LhBody)
	}
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			kit.Labeled(c, locales.SettingsIDLabel(), func() {
				ui.TextInput(c, &f.provider.ID).Label(locales.SettingsProviderID()).Placeholder(p.ID).FillWidth().Disabled(busy || f.providerID != "")
			}).Width(200)
			kit.Labeled(c, locales.SettingsDisplayName(), func() {
				ui.TextInput(c, &f.provider.Name).Label(locales.SettingsDisplayName()).FillWidth().Disabled(busy)
			}).Grow(1)
		})
		if bad.ID != "" && p.ID != "" {
			red(bad.ID)
		}
		kit.Labeled(c, locales.SettingsAPIURL(), func() {
			ui.TextInput(c, &f.provider.BaseURL).Label(locales.SettingsAPIURL()).Placeholder("https://api.example.com/v1").FillWidth().Disabled(busy)
			if kit.Trim(f.provider.BaseURL) != "" {
				if bad.BaseURL != "" {
					red(bad.BaseURL)
				} else if isLocal(f.provider.BaseURL) {
					ui.Text(c, locales.SettingsLocalNote()).FontSize(kit.FsNote).TextColor(t.TextMuted)
				} else {
					ui.Text(c, locales.SettingsRemoteNote()).FontSize(kit.FsNote).TextColor(t.Warning)
				}
			}
		})
		kit.Labeled(c, locales.SettingsProtocol(), func() {
			a.pal.SelectNamed(c, &f.provider.Protocol, protocols, protocolName).Label(locales.SettingsProtocolAPI()).FillWidth().Disabled(busy)
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			kit.Labeled(c, "API key", func() {
				hint := locales.SettingsKeyEmptySaved()
				if f.providerID == "" {
					hint = locales.SettingsKeyEmptyNew() // a new service has none kept
				}
				ui.TextInput(c, &f.key).Label("API key").Password().Placeholder(hint).FillWidth().Disabled(busy)
			}).Grow(1).Basis(0)
			kit.Labeled(c, locales.SettingsEnvOr(), func() {
				ui.TextInput(c, &f.provider.Env).Label(locales.SettingsEnvName()).Placeholder(locales.SettingsEnvExample()).FillWidth().Disabled(busy)
				if bad.Env != "" {
					red(bad.Env)
				}
			}).Grow(1).Basis(0)
		})
		hint := locales.SettingsKeyNote()
		if env := kit.Trim(f.provider.Env); env != "" && os.Getenv(env) != "" {
			hint += locales.SettingsEnvDetected(env)
		}
		ui.Text(c, hint).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).Margin(-6, 0, 0, 0)
		kit.Labeled(c, locales.SettingsModels(), func() {
			a.limitTable(c)
			if bad.Models != "" {
				red(bad.Models)
			}
		})
		ui.Text(c, locales.SettingsCompressNote()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if f.providerID != "" && a.pal.Ghost(c, locales.SettingsClearKey(), t.Danger).Disabled(busy).Clicked() {
				err := a.ai.SetKey(f.providerID, "")
				a.modelsChanged()
				if err == nil {
					a.status.Did(locales.SettingsKeyCleared())
				}
				a.SetError(err)
				f.key = ""
			}
			// A new service has no 收起 above it: it is put away from here.
			if f.providerID == "" && a.pal.Secondary(c, locales.CommonCancel()).Disabled(busy).Clicked() {
				f.close()
			}
			if a.pal.Secondary(c, locales.SettingsAddModel()).Disabled(busy).Clicked() {
				m, _ := a.ai.Limits("")
				f.limits = append(f.limits, f.limitRow(m))
			}
			if a.pal.Secondary(c, locales.SettingsFetchModels()).Disabled(a.busy != "" || bad.First() != "").Clicked() { // it waits for a turn in flight: one request at a time
				a.fetchModels(c)
			}
			if a.pal.Primary(c, locales.SettingsSaveProvider()).Disabled(busy || bad.First() != "").Clicked() {
				err := a.saveProvider()
				if err == nil {
					a.status.Did(locales.SettingsSaved())
					f.edit = f.providerID
				}
				a.SetError(err)
			}
		})
	})
}

// limitTable edits the models of the service in the form, one row each:
// the ID and three numbers. An ID typed that the table built in knows
// brings its numbers.
func (a *App) limitTable(c *ui.Context) {
	f := &a.serviceForm
	t := a.pal.Theme
	busy := a.held()
	if len(f.limits) == 0 {
		ui.Text(c, locales.SettingsNoModels()).FontSize(kit.FsNote).TextColor(t.TextMuted)
		return
	}
	gone := -1
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			head := func(s string) ui.Element { return ui.Text(c, s).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0) }
			head(locales.SettingsModelID()).Grow(1).Shrink(1).MinWidth(0)
			head(locales.SettingsContextLimit()).Width(96)
			head(locales.SettingsMaxOutput()).Width(96)
			head(locales.SettingsCompressPct()).Width(72)
			ui.Column(c).Width(kit.HControlSm).Shrink(0)
		})
		for i := range f.limits {
			r := &f.limits[i]
			n := "-" + strconv.Itoa(i+1) // the row's fields are told apart by its place
			ui.Row(c.Key("limit-" + strconv.Itoa(r.key))).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.TextInput(c, &r.id).Label("模型 ID" + n).Placeholder(locales.SettingsModelID()).Grow(1).Shrink(1).MinWidth(0).Disabled(busy).Changed() {
					if m, ok := a.ai.Limits(kit.Trim(r.id)); ok {
						r.window, r.output, r.at = tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)
					}
				}
				ui.TextInput(c, &r.window).Label("上下文上限" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.output).Label("最大输出" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.at).Label("压缩阈值" + n).Width(72).Shrink(0).Disabled(busy)
				if kit.IconButton(c, kit.IcX, locales.SettingsRemoveModel()+n, ui.Color{}).Tooltip(locales.SettingsRemoveModel()).Disabled(busy).Clicked() {
					gone = i
				}
			})
		}
	})
	if gone >= 0 {
		f.limits = append(f.limits[:gone], f.limits[gone+1:]...)
	}
}

func (a *App) fetchModels(c *ui.Context) {
	if err := a.saveProvider(); err != nil {
		a.SetError(err)
		return
	}
	catalog, err := a.ai.Catalog(a.serviceForm.provider.ID)
	if err != nil {
		a.SetError(err)
		return
	}
	generation := a.serviceForm.generation
	a.run(c, locales.SettingsFetching(), func(ctx context.Context) (func(), error) {
		models, err := catalog.ListModels(ctx)
		return func() {
			if a.serviceForm.generation != generation {
				return // the form was closed, reloaded or replaced while waiting
			}
			if err == nil {
				a.serviceForm.limits = nil
				for _, id := range models {
					m, _ := a.ai.Limits(id)
					a.serviceForm.limits = append(a.serviceForm.limits, a.serviceForm.limitRow(m))
				}
				a.status.Did(locales.SettingsModelsFetched())
			}
		}, err
	})
}

func (a *App) saveProvider() error {
	f := &a.serviceForm
	p, bad := f.value(a.ai)
	if why := bad.First(); why != "" {
		return &aiport.SetupError{Err: errors.New(why)}
	}
	if err := a.ai.SaveProvider(p); err != nil {
		return err
	}
	f.provider = p
	f.setModels(p.Models)
	f.providerID = p.ID
	defer a.modelsChanged()
	if f.key != "" {
		if err := a.ai.SetKey(p.ID, f.key); err != nil {
			return err
		}
		f.key = ""
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
