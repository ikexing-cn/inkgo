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
	"writer/desk/aiport"
	"writer/tokens"
	"writer/workspace"
)

var protocols = []string{"chat", "responses", "anthropic"}
var protocolNames = []string{"OpenAI Chat Completions（多数兼容服务）", "OpenAI Responses", "Anthropic Messages"}

const sameAsTalk = "沿用讨论模型"

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
				return "地址无效"
			}
			kind := "远程"
			if isLocal(p.BaseURL) {
				kind = "本机"
			}
			return kind + " " + u.Host + " / " + sel.Model
		}
	}
	return "请先配置讨论模型"
}

func (a *App) chooseProvider(id string) {
	for _, p := range a.ai.Providers() {
		if p.ID == id {
			a.providerID = id
			a.provider = p
			a.limits = nil
			for _, m := range p.Models {
				a.limits = append(a.limits, a.limitRow(m))
			}
			a.key = ""
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

// labeled stacks a muted caption over a control.
func labeled(c *ui.Context, caption string, fn func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, caption).FontSize(fsNote).TextColor(c.Theme().TextMuted)
		fn()
	})
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

// subhead titles a block of the settings.
func (a *App) subhead(c *ui.Context, title, caption string) {
	ui.Column(c).FillWidth().Gap(2).Padding(12, 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(fsGroup).FontWeight(600)
		if caption != "" {
			ui.Text(c, caption).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody)
		}
	})
}

// usage chooses which configured models this work talks to: one row each
// for the discussion, and for continuing and checking. A choice is kept as
// it is made, a model ID typed once the caret leaves it.
func (a *App) usage(c *ui.Context) {
	a.subhead(c, "本作品的模型", "思考程度选 Default 时由服务商决定。修改自下次发送起生效。")
	var typing [2]bool
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		typing[0] = a.useRow(c, "讨论", "与你讨论故事", &a.talkProvider, "", "讨论提供商", &a.talkModel, "讨论模型", &a.talkEffort, "讨论的思考程度")
		typing[1] = a.useRow(c, "续写和检查", "灰字续写，检查正文", &a.penProvider, sameAsTalk, "续写和检查提供商", &a.penModel, "续写和检查模型", &a.penEffort, "续写和检查的思考程度")
	})
	if typing == [2]bool{} {
		if err := a.saveUsage(); err != nil {
			a.setError(err)
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
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			ui.Text(c, who).FontSize(fsBody).FontWeight(500)
			ui.Text(c, what).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody).SingleLine().Ellipsis("…")
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
				ui.Text(c, "与讨论使用同一模型").FontSize(fsControl).TextColor(a.pal.faint).Padding(0, 10)
				*model = ""
				return
			}
			typing = modelField(c, model, a.providerModels(*provider)).Label(modelLabel).Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy).Focused()
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
	a.subhead(c, "写作时", "")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.Checkbox(c, &a.write.autoPredict, "停笔后自动续写").Disabled(busy).Changed() {
			a.savePrefs() // kept as ticked, not with the models above
		}
		a.tickNote(c, "开启后每次停笔都会调用一次 AI。")
	})
}

// tickNote explains the tick box over it, in line with the box's words.
func (a *App) tickNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody).Margin(-4, 0, 4, tickIndent)
}

// services lists the model services kept on this computer; 编辑 opens a
// service's form in place, and so does adding one.
func (a *App) services(c *ui.Context) {
	t := a.pal.theme
	busy := a.held()
	a.subhead(c, "模型服务", "保存在本机，所有作品共用")
	// A form just opened comes into view with its card: one added opens at
	// the foot of the list, below what is shown.
	reveal := a.bk.edit != a.bk.shown
	a.bk.shown = a.bk.edit
	card := func(id string) ui.Element {
		box := ui.Column(c).FillWidth().Gap(12).Padding(12, 14).Radius(rCard).Border(1, a.pal.line)
		if reveal && a.bk.edit == id {
			box.ScrollIntoView()
		}
		return box
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		for _, p := range a.ai.Providers() {
			card(p.ID).Key("provider-" + p.ID).Children(func() {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Children(func() {
						ui.Text(c, providerLabel(p)).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…")
						sub := p.BaseURL
						if u, err := url.Parse(p.BaseURL); err == nil && u.Host != "" {
							sub = u.Host
						}
						sub += " · " + strconv.Itoa(len(p.Models)) + " 个模型"
						if a.Store != nil && a.Store.Work().Discussion.Provider == p.ID {
							sub += " · 本作品使用中"
						}
						ui.Text(c, sub).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).SingleLine().Ellipsis("…")
					})
					open := a.bk.edit == p.ID
					text := "编辑"
					if open {
						text = "收起"
					}
					if a.pal.secondary(c, text).Label("提供商-" + p.ID).Disabled(busy).Clicked() {
						a.bk.edit = ""
						if !open {
							a.chooseProvider(p.ID)
							a.bk.edit = p.ID
						}
					}
				})
				if a.bk.edit == p.ID {
					a.providerForm(c)
				}
			})
		}
		if a.bk.edit == "+" {
			card("+").Children(func() { a.providerForm(c) })
			return
		}
		add := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).Gap(8).AlignItems(ui.Center).Padding(12, 14).Radius(rCard).Border(1, a.pal.line).Label("+ 添加服务").Disabled(busy)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			glyph(c, icPlus, iconSm, t.TextMuted)
			ui.Text(c, "添加服务").FontSize(fsBody).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			a.providerID = ""
			a.provider = aiport.Provider{Protocol: "chat"}
			a.key, a.limits = "", nil
			a.bk.edit = "+"
		}
	})
	ui.Row(c).FillWidth().Gap(16).Justify(ui.End).Children(func() {
		if a.pal.ghost(c, "打开配置文件", ui.Color{}).Disabled(a.ai.ConfigFile() == "").Clicked() {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(a.ai.ConfigFile())}
			if len(u.Path) > 1 && u.Path[1] == ':' {
				u.Path = "/" + u.Path
			}
			c.OpenURL(u.String())
		}
		if a.pal.ghost(c, "重新加载配置", ui.Color{}).Disabled(busy || a.ai.ConfigFile() == "").Clicked() {
			err := a.ai.Reload()
			if err == nil {
				a.modelsChanged()
				a.bk.edit = ""
				if ps := a.ai.Providers(); len(ps) > 0 {
					a.chooseProvider(ps[0].ID)
				}
				a.notice = "配置已重新读取"
			}
			a.setError(err)
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
	if !ui.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
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
	if ui.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
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
	w.Discussion = workspace.Selection{Provider: trim(a.talkProvider), Model: trim(a.talkModel), Effort: a.talkEffort}
	w.Writing = workspace.Selection{Provider: trim(a.penProvider), Model: trim(a.penModel), Effort: a.penEffort}
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
	t := a.pal.theme
	busy := a.held()
	p, bad := a.serviceInForm()
	red := func(why string) {
		ui.Text(c, why).FontSize(fsNote).TextColor(t.Danger).LineHeight(lhBody)
	}
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "ID（小写字母、数字、- _）", func() {
				ui.TextInput(c, &a.provider.ID).Label("Provider ID（小写）").Placeholder(p.ID).FillWidth().Disabled(busy || a.providerID != "")
			}).Width(200)
			labeled(c, "显示名称", func() { ui.TextInput(c, &a.provider.Name).Label("显示名称").FillWidth().Disabled(busy) }).Grow(1)
		})
		if bad.ID != "" && p.ID != "" {
			red(bad.ID)
		}
		labeled(c, "API 地址", func() {
			ui.TextInput(c, &a.provider.BaseURL).Label("API 地址").Placeholder("https://api.example.com/v1").FillWidth().Disabled(busy)
			if trim(a.provider.BaseURL) != "" {
				if bad.BaseURL != "" {
					red(bad.BaseURL)
				} else if isLocal(a.provider.BaseURL) {
					ui.Text(c, "本机地址：请确认该服务不会转发到云端").FontSize(fsNote).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "远程服务：讨论内容将发送到该地址").FontSize(fsNote).TextColor(t.Warning)
				}
			}
		})
		proto := protocolNames[0]
		for n, p := range protocols {
			if p == a.provider.Protocol {
				proto = protocolNames[n]
			}
		}
		labeled(c, "接口协议", func() {
			if ui.Select(c, &proto, protocolNames).Label("API 协议").FillWidth().Disabled(busy).Changed() {
				for n, name := range protocolNames {
					if name == proto {
						a.provider.Protocol = protocols[n]
					}
				}
			}
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			labeled(c, "API key", func() {
				hint := "留空则保留已存的 key"
				if a.providerID == "" {
					hint = "留空则不用 key" // a new service has none kept
				}
				ui.TextInput(c, &a.key).Label("API key").Password().Placeholder(hint).FillWidth().Disabled(busy)
			}).Grow(1).Basis(0)
			labeled(c, "或读取环境变量", func() {
				ui.TextInput(c, &a.provider.Env).Label("环境变量名称").Placeholder("例如 DEEPSEEK_API_KEY").FillWidth().Disabled(busy)
				if bad.Env != "" {
					red(bad.Env)
				}
			}).Grow(1).Basis(0)
		})
		hint := "key 保存在系统钥匙串中，不会写入配置文件或作品。"
		if env := trim(a.provider.Env); env != "" && os.Getenv(env) != "" {
			hint += "已检测到环境变量 " + env + "。"
		}
		ui.Text(c, hint).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).Margin(-6, 0, 0, 0)
		labeled(c, "可用模型", func() {
			a.limitTable(c)
			if bad.Models != "" {
				red(bad.Models)
			}
		})
		ui.Text(c, "数值默认取自内置模型表，表中没有的模型按保守值填写；上下文上限和最大输出可写成 128K、1M。讨论超过压缩阈值时，较早的对话会整理成摘要。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if a.providerID != "" && a.pal.ghost(c, "清除已保存 key", t.Danger).Disabled(busy).Clicked() {
				err := a.ai.SetKey(a.providerID, "")
				a.modelsChanged()
				if err == nil {
					a.notice = "已清除保存的 key"
				}
				a.setError(err)
				a.key = ""
			}
			// A new service has no 收起 above it: it is put away from here.
			if a.providerID == "" && a.pal.secondary(c, "取消").Disabled(busy).Clicked() {
				a.bk.edit = ""
			}
			if a.pal.secondary(c, "添加模型").Disabled(busy).Clicked() {
				m, _ := a.ai.Limits("")
				a.limits = append(a.limits, a.limitRow(m))
			}
			if a.pal.secondary(c, "获取可用模型").Disabled(a.busy != "" || bad.First() != "").Clicked() { // it waits for a turn in flight: one request at a time
				a.fetchModels(c)
			}
			if a.pal.primary(c, "保存提供商").Disabled(busy || bad.First() != "").Clicked() {
				err := a.saveProvider()
				if err == nil {
					a.notice = "提供商已保存"
					a.bk.edit = a.providerID
				}
				a.setError(err)
			}
		})
	})
}

// limitRow is a model of the service in the form, as typed; key keeps
// its fields its own as rows above it go.
type limitRow struct {
	key                    int
	id, window, output, at string
}

func (a *App) limitRow(m aiport.Model) limitRow {
	a.limitKeys++
	return limitRow{a.limitKeys, m.ID, tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)}
}

// limitTable edits the models of the service in the form, one row each:
// the ID and three numbers. An ID typed that the table built in knows
// brings its numbers.
func (a *App) limitTable(c *ui.Context) {
	t := a.pal.theme
	busy := a.held()
	if len(a.limits) == 0 {
		ui.Text(c, "尚无模型，可获取可用模型或手动添加。").FontSize(fsNote).TextColor(t.TextMuted)
		return
	}
	gone := -1
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			head := func(s string) ui.Element { return ui.Text(c, s).FontSize(fsNote).TextColor(t.TextMuted).Shrink(0) }
			head("模型 ID").Grow(1).Shrink(1).MinWidth(0)
			head("上下文上限").Width(96)
			head("最大输出").Width(96)
			head("压缩阈值 %").Width(72)
			ui.Column(c).Width(hControlSm).Shrink(0)
		})
		for i := range a.limits {
			r := &a.limits[i]
			n := "-" + strconv.Itoa(i+1) // the row's fields are told apart by its place
			ui.Row(c.Key("limit-" + strconv.Itoa(r.key))).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.TextInput(c, &r.id).Label("模型 ID" + n).Placeholder("模型 ID").Grow(1).Shrink(1).MinWidth(0).Disabled(busy).Changed() {
					if m, ok := a.ai.Limits(trim(r.id)); ok {
						r.window, r.output, r.at = tokens.Format(m.Window), tokens.Format(m.Output), strconv.Itoa(m.CompactAt)
					}
				}
				ui.TextInput(c, &r.window).Label("上下文上限" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.output).Label("最大输出" + n).Width(96).Shrink(0).Disabled(busy)
				ui.TextInput(c, &r.at).Label("压缩阈值" + n).Width(72).Shrink(0).Disabled(busy)
				if iconButton(c, icX, "移除模型"+n, ui.Color{}).Tooltip("移除模型").Disabled(busy).Clicked() {
					gone = i
				}
			})
		}
	})
	if gone >= 0 {
		a.limits = append(a.limits[:gone], a.limits[gone+1:]...)
	}
}

// models reads the rows of the form that have an ID. It names the first
// model that breaks a rule, the rules taken in turn.
func (a *App) models() ([]aiport.Model, error) {
	var out []aiport.Model
	for _, r := range a.limits {
		id := trim(r.id)
		if id == "" {
			continue
		}
		window, _ := tokens.Parse(r.window)
		output, _ := tokens.Parse(r.output)
		at, err := strconv.Atoi(trim(r.at))
		if err != nil {
			at = -1
		}
		out = append(out, aiport.Model{ID: id, Window: window, Output: output, CompactAt: at})
	}
	for _, rule := range []struct {
		bad func(m aiport.Model) bool
		why string
	}{
		{func(m aiport.Model) bool { return m.Window == 0 || m.Output == 0 }, "的上下文上限和最大输出须为正整数，可写成 128K、1M"},
		{func(m aiport.Model) bool { return m.CompactAt < 0 }, "的压缩阈值须为整数"},
		{func(m aiport.Model) bool { return m.Output >= m.Window }, "的最大输出须小于上下文上限"},
		{func(m aiport.Model) bool { return m.CompactAt < 50 || m.CompactAt > 95 }, "的压缩阈值须在 50% 到 95% 之间"},
	} {
		for _, m := range out {
			if rule.bad(m) {
				return nil, errors.New(m.ID + " " + rule.why)
			}
		}
	}
	return out, nil
}

func (a *App) fetchModels(c *ui.Context) {
	if err := a.saveProvider(); err != nil {
		a.setError(err)
		return
	}
	catalog, err := a.ai.Catalog(a.provider.ID)
	if err != nil {
		a.setError(err)
		return
	}
	a.run(c, "正在读取可用模型…", func(ctx context.Context) (func(), error) {
		models, err := catalog.ListModels(ctx)
		return func() {
			if err == nil {
				a.limits = nil
				for _, id := range models {
					m, _ := a.ai.Limits(id)
					a.limits = append(a.limits, a.limitRow(m))
				}
				a.notice = "已获取模型目录，保存提供商后生效"
			}
		}, err
	})
}

// serviceInForm is the service the form would save, and what keeps it
// from being saved, by field. A new service left without an ID takes one
// from its name; it may not take the ID of another, whose place it would
// take.
func (a *App) serviceInForm() (aiport.Provider, aiport.Problems) {
	p := a.provider
	p.ID = trim(p.ID)
	if a.providerID == "" && p.ID == "" {
		p.ID = idFrom(p.Name)
	}
	models, err := a.models()
	p.Models = models
	bad := a.ai.Check(p)
	if err != nil {
		bad.Models = err.Error()
	}
	if a.providerID == "" {
		for _, q := range a.ai.Providers() {
			if q.ID == p.ID {
				bad.ID = "该 ID 已被「" + providerLabel(q) + "」使用，请换一个"
			}
		}
	}
	return p, bad
}

// idFrom is an ID made from a name: its Latin letters in lower case, and
// the digits, - and _ after the first of them, a space as -.
func idFrom(name string) string {
	var id []rune
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', len(id) > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_'):
			id = append(id, r)
		case len(id) > 0 && r == ' ' && id[len(id)-1] != '-':
			id = append(id, '-')
		}
	}
	return strings.TrimRight(string(id), "-_")
}

func (a *App) saveProvider() error {
	p, bad := a.serviceInForm()
	if why := bad.First(); why != "" {
		return errors.New(why)
	}
	if err := a.ai.SaveProvider(p); err != nil {
		return err
	}
	a.provider = p
	a.limits = nil
	for _, m := range p.Models {
		a.limits = append(a.limits, a.limitRow(m))
	}
	a.providerID = p.ID
	defer a.modelsChanged()
	if a.key != "" {
		if err := a.ai.SetKey(p.ID, a.key); err != nil {
			return err
		}
		a.key = ""
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
	in.Padding(t.Space(1.5), t.Space(2.5)).Radius(rControl).Background(t.Surface).Border(1, t.Border)
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
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(rCard).Background(t.Background).Border(1, t.Border)
		shadowOf(c, "pop").on(panel)
		ui.Scroll(c).MaxHeight(t.Space(70)).FillWidth().Children(func() {
			for _, m := range shown {
				item := cb.Item(m).Padding(t.Space(1.5), t.Space(2.5)).Radius(rControl)
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
