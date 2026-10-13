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
	"writer/model"
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
	for _, p := range a.Config.Providers {
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
	for _, p := range a.Config.Providers {
		if p.ID == id {
			a.providerID = id
			a.provider = p
			a.models = strings.Join(p.Models, "\n")
			a.key = ""
			return
		}
	}
}

func (a *App) providerModels(id string) []string {
	for _, p := range a.Config.Providers {
		if p.ID == id {
			return p.Models
		}
	}
	return nil
}

// labeled stacks a muted caption over a control.
func labeled(c *ui.Context, caption string, fn func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, caption).FontSize(fsNote).TextColor(c.Theme().TextMuted)
		fn()
	})
}

// modelsView is the settings' 模型: which models the work uses, how the
// writing calls them, and the services kept on this computer.
func (a *App) modelsView(c *ui.Context) {
	if a.Store != nil {
		a.usage(c)
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
// for the discussion, the bookkeeping and the writing.
func (a *App) usage(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	a.subhead(c, "本作品使用", "思考程度选 Default 时由服务商决定。")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		{
			a.useRow(c, "讨论", "与你讨论故事", &a.talkProvider, "", "讨论提供商", &a.talkModel, "讨论模型", &a.talkEffort, "讨论推理等级")
			a.useRow(c, "整理", "将讨论整理为设定", &a.bookProvider, sameAsTalk, "记账提供商", &a.bookModel, "记账模型", &a.bookEffort, "整理推理等级")
			a.useRow(c, "写作", "续写、改写、起草", &a.penProvider, sameAsTalk, "写作提供商", &a.penModel, "写作模型", &a.penEffort, "写作推理等级")
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "上下文预算").FontSize(fsBody).FontWeight(500).Width(110).Shrink(0)
			ui.TextInput(c, &a.budget).Label("上下文预算").Width(150).Disabled(busy)
			ui.Text(c, "token，含回答预留").FontSize(fsNote).TextColor(t.TextMuted).Grow(1).Shrink(1)
			if a.pal.primary(c, "保存").Label("保存作品模型选择").Disabled(busy).Clicked() {
				a.setError(a.saveUsage())
			}
		})
	})
	ui.Text(c, "请勿超过模型的上下文窗口；未能放入的设定会列在回复下方的“AI 参考内容”中。修改自下次发送起生效。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
}

// useRow is one of usage's rows: who, the service, the model and the
// effort. With none, the row can go without a service of its own.
func (a *App) useRow(c *ui.Context, who, what string, provider *string, none, providerLabel string, model *string, modelLabel string, effort *string, effortLabel string) {
	busy := a.busy != ""
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			ui.Text(c, who).FontSize(fsBody).FontWeight(500)
			ui.Text(c, what).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody).SingleLine().Ellipsis("…")
		})
		ui.Column(c).Width(150).Shrink(0).Children(func() {
			a.providerSelect(c, provider, none, providerLabel, busy)
		})
		ui.Column(c).Grow(1).Shrink(1).Children(func() {
			if none != "" && *provider == "" {
				ui.Text(c, "与讨论使用同一模型").FontSize(fsControl).TextColor(a.pal.faint).Padding(0, 10)
				*model = ""
				return
			}
			modelField(c, model, a.providerModels(*provider)).Label(modelLabel).Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
		})
		ui.Column(c).Width(110).Shrink(0).Children(func() {
			effortSelect(c, effort, effortLabel, busy)
		})
	})
}

// writingSwitches are the writing page's calls to the models, kept as
// they are ticked.
func (a *App) writingSwitches(c *ui.Context) {
	busy := a.busy != ""
	a.subhead(c, "写作时", "")
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.Checkbox(c, &a.write.autoPredict, "停笔后自动续写").Disabled(busy).Changed() {
			a.savePrefs() // kept as ticked, not with the models above
		}
		a.tickNote(c, "开启后每次停笔都会调用一次 AI。")
		if ui.Checkbox(c, &a.write.autoSummary, "章节修改后更新摘要").Disabled(busy).Changed() {
			a.savePrefs() // kept as ticked, not with the models above
		}
		a.tickNote(c, "手动填写的摘要不会被覆盖。")
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
	busy := a.busy != ""
	a.subhead(c, "模型服务", "保存在本机，所有作品共用")
	card := func() ui.Element {
		return ui.Column(c).FillWidth().Gap(12).Padding(12, 14).Radius(rCard).Border(1, a.pal.line)
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		for _, p := range a.Config.Providers {
			card().Key("provider-" + p.ID).Children(func() {
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
			card().Children(func() { a.providerForm(c) })
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
			a.provider = model.Provider{Protocol: "chat"}
			a.key, a.models = "", ""
			a.bk.edit = "+"
		}
	})
	ui.Row(c).FillWidth().Gap(16).Justify(ui.End).Children(func() {
		if a.pal.ghost(c, "打开配置文件", ui.Color{}).Disabled(a.ConfigFile == "").Clicked() {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(a.ConfigFile)}
			if len(u.Path) > 1 && u.Path[1] == ':' {
				u.Path = "/" + u.Path
			}
			c.OpenURL(u.String())
		}
		if a.pal.ghost(c, "重新加载配置", ui.Color{}).Disabled(busy || a.ConfigFile == "").Clicked() {
			cfg, err := model.Load(a.ConfigFile)
			if err == nil {
				a.Config = cfg
				a.bk.edit = ""
				if len(cfg.Providers) > 0 {
					a.chooseProvider(cfg.Providers[0].ID)
				}
				a.notice = "配置已重新读取"
			}
			a.setError(err)
		}
	})
}

// providerSelect picks a configured service, shown by its name, into *id;
// none, when given, is the choice of no service of its own.
func (a *App) providerSelect(c *ui.Context, id *string, none, label string, disabled bool) {
	var names []string
	if none != "" {
		names = append(names, none)
	}
	chosen := none
	for _, p := range a.Config.Providers {
		names = append(names, providerLabel(p))
		if p.ID == *id {
			chosen = providerLabel(p)
		}
	}
	if *id != "" && chosen == none { // a service no longer configured
		chosen = *id
		names = append(names, *id)
	}
	if ui.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		*id = ""
		if chosen != none {
			*id = chosen
		}
		for _, p := range a.Config.Providers {
			if providerLabel(p) == chosen {
				*id = p.ID
			}
		}
	}
}

func providerLabel(p model.Provider) string {
	if p.Name != "" {
		return p.Name
	}
	return p.ID
}

// effortSelect picks an effort, shown by its label, into *value.
func effortSelect(c *ui.Context, value *string, label string, disabled bool) {
	names := make([]string, len(model.Efforts))
	chosen := model.Effort(*value).Label()
	for n, e := range model.Efforts {
		names[n] = e.Label()
	}
	if ui.Select(c, &chosen, names).Label(label).FillWidth().MinWidth(0).Disabled(disabled).Changed() {
		for _, e := range model.Efforts {
			if e.Label() == chosen {
				*value = string(e)
			}
		}
	}
}

func (a *App) saveUsage() error {
	budget, err := strconv.Atoi(trim(a.budget))
	if err != nil {
		return errors.New("上下文预算须为整数")
	}
	if budget < 8192 {
		return errors.New("上下文预算至少为 8192，请确保不超过模型支持的窗口")
	}
	w := a.Store.Work()
	w.Discussion = workspace.Selection{Provider: trim(a.talkProvider), Model: trim(a.talkModel), Effort: a.talkEffort}
	w.Bookkeeping = workspace.Selection{Provider: trim(a.bookProvider), Model: trim(a.bookModel), Effort: a.bookEffort}
	if w.Bookkeeping.Provider == "" {
		w.Bookkeeping.Model = ""
	}
	w.Writing = workspace.Selection{Provider: trim(a.penProvider), Model: trim(a.penModel), Effort: a.penEffort}
	if w.Writing.Provider == "" {
		w.Writing.Model = ""
	}
	if w.Writing.Provider != "" && w.Writing.Model == "" {
		return errors.New("写作模型已选择单独的服务，还需填写模型 ID；或改回“沿用讨论模型”")
	}
	// The store requires a provider and a model together; say which is missing.
	if (w.Discussion.Provider == "") != (w.Discussion.Model == "") {
		return errors.New("讨论模型需要同时选择服务和填写模型 ID")
	}
	if w.Bookkeeping.Provider != "" && w.Bookkeeping.Model == "" {
		return errors.New("整理模型已选择单独的服务，还需填写模型 ID；或改回“沿用讨论模型”")
	}
	w.TokenBudget = budget
	if err = a.Store.SaveWork(w); err == nil {
		a.savePrefs()
		if a.write.ed != nil {
			a.write.ed.SetPredictOnDemand(!a.write.autoPredict)
		}
		a.notice = "设置已保存，下一条消息起生效"
		clear(a.sums.failed) // what failed for want of a model may work now
		a.kick()             // a model or budget that now works takes in what waited
	}
	return err
}

// providerForm edits the service open in the list, or a new one, under
// its row.
func (a *App) providerForm(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "ID（小写字母、数字、- _）", func() {
				ui.TextInput(c, &a.provider.ID).Label("Provider ID（小写）").FillWidth().Disabled(busy || a.providerID != "")
			}).Width(200)
			labeled(c, "显示名称", func() { ui.TextInput(c, &a.provider.Name).Label("显示名称").FillWidth().Disabled(busy) }).Grow(1)
		})
		labeled(c, "API 地址", func() {
			ui.TextInput(c, &a.provider.BaseURL).Label("API 地址").Placeholder("https://api.example.com/v1").FillWidth().Disabled(busy)
			if trim(a.provider.BaseURL) != "" {
				if isLocal(a.provider.BaseURL) {
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
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "API key", func() {
				ui.TextInput(c, &a.key).Label("API key").Password().Placeholder("留空则保留已存的 key").FillWidth().Disabled(busy)
			}).Grow(1).Basis(0)
			labeled(c, "或读取环境变量", func() {
				ui.TextInput(c, &a.provider.Env).Label("环境变量名称").Placeholder("例如 DEEPSEEK_API_KEY").FillWidth().Disabled(busy)
			}).Grow(1).Basis(0)
		})
		hint := "key 保存在系统钥匙串中，不会写入配置文件或作品。"
		if env := trim(a.provider.Env); env != "" && os.Getenv(env) != "" {
			hint += "已检测到环境变量 " + env + "。"
		}
		ui.Text(c, hint).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).Margin(-6, 0, 0, 0)
		labeled(c, "可用模型（每行一个；也可在上方直接输入目录外的模型）", func() {
			ui.TextArea(c, &a.models).Label("模型目录").Height(96).FillWidth().Disabled(busy)
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if a.providerID != "" && a.pal.ghost(c, "清除已保存 key", t.Danger).Disabled(busy).Clicked() {
				err := model.SetKey(a.providerID, "")
				if err == nil {
					a.notice = "已清除保存的 key"
				}
				a.setError(err)
				a.key = ""
			}
			if a.pal.secondary(c, "获取可用模型").Disabled(busy).Clicked() {
				a.fetchModels(c)
			}
			if a.pal.primary(c, "保存提供商").Disabled(busy).Clicked() {
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
func (a *App) fetchModels(c *ui.Context) {
	if err := a.saveProvider(); err != nil {
		a.setError(err)
		return
	}
	client, err := model.NewClient(a.Config, a.provider.ID, "")
	if err != nil {
		a.setError(err)
		return
	}
	a.run(c, "正在读取可用模型…", func(ctx context.Context) (func(), error) {
		models, err := client.ListModels(ctx)
		return func() {
			if err == nil {
				a.models = strings.Join(models, "\n")
				a.notice = "已获取模型目录，保存提供商后生效"
			}
		}, err
	})
}

func (a *App) saveProvider() error {
	p := a.provider
	p.ID = trim(p.ID)
	p.Models = nil
	for _, line := range strings.Split(a.models, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			p.Models = append(p.Models, s)
		}
	}
	cfg := a.Config
	cfg.Providers = append([]model.Provider(nil), cfg.Providers...)
	replaced := false
	for i, old := range cfg.Providers {
		if old.ID == p.ID {
			cfg.Providers[i] = p
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Providers = append(cfg.Providers, p)
	}
	if err := model.Save(a.ConfigFile, cfg); err != nil {
		return err
	}
	a.Config = cfg
	a.provider = p
	a.providerID = p.ID
	if a.key != "" {
		if err := model.SetKey(p.ID, a.key); err != nil {
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
