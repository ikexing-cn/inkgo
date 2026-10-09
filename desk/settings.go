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
func labeled(c *ui.Context, caption string, fn func()) *ui.Element {
	return ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, caption).FontSize(12.5).FontWeight(500).TextColor(c.Theme().TextMuted)
		fn()
	})
}

// modelNav lists the model services, in the settings page's left column.
func (a *App) modelNav(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	ui.Text(c, "模型服务").FontSize(11.5).FontWeight(600).TextColor(a.pal.faint).Padding(0, 10, 8, 10)
	for _, p := range a.Config.Providers {
		name := p.Name
		if name == "" {
			name = p.ID
		}
		item := ui.ButtonBase(c).Key("provider-"+p.ID).FillWidth().Padding(7, 10).Radius(7).Gap(9).Row().AlignItems(ui.Center).Label("提供商-" + p.ID).Disabled(busy)
		on := p.ID == a.providerID
		if on {
			item.Background(t.SurfacePressed)
		} else if item.Hovered() {
			item.Background(t.SurfaceHover)
		}
		item.Children(func() {
			color := a.pal.ink
			if isLocal(p.BaseURL) {
				color = t.Success
			}
			dot(c, color, 7)
			fg := t.TextMuted
			if on {
				fg = t.Text
			}
			ui.Text(c, name).FontSize(13.5).TextColor(fg).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.Store != nil && a.Store.Work().Discussion.Provider == p.ID {
				ui.Text(c, "在用").FontSize(11).TextColor(a.pal.faint)
			}
		})
		if item.Clicked() {
			a.chooseProvider(p.ID)
		}
	}
	ui.Box(c).Height(6)
	add := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).Gap(9).AlignItems(ui.Center).Padding(7, 10).Radius(7).Label("+ 添加自定义 API").Disabled(busy)
	if add.Hovered() {
		add.Background(t.SurfaceHover)
	}
	add.Children(func() {
		glyph(c, icPlus, 14, t.TextMuted)
		ui.Text(c, "添加自定义 API").FontSize(13.5).TextColor(t.TextMuted)
	})
	if add.Clicked() {
		a.providerID = ""
		a.provider = model.Provider{Protocol: "chat"}
		a.key, a.models = "", ""
	}
	ui.Spacer(c)
	ui.Column(c).FillWidth().AlignItems(ui.Start).Gap(2).Children(func() {
		if ghost(c, "打开配置文件", ui.Color{}).Disabled(a.ConfigFile == "").Clicked() {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(a.ConfigFile)}
			if len(u.Path) > 1 && u.Path[1] == ':' {
				u.Path = "/" + u.Path
			}
			c.OpenURL(u.String())
		}
		if ghost(c, "重新加载配置", ui.Color{}).Disabled(busy || a.ConfigFile == "").Clicked() {
			cfg, err := model.Load(a.ConfigFile)
			if err == nil {
				a.Config = cfg
				if len(cfg.Providers) > 0 {
					a.chooseProvider(cfg.Providers[0].ID)
				}
				a.notice = "已重新加载配置"
			}
			a.setError(err)
		}
	})
}

// modelPane is the settings page's 模型: which models the work uses, and
// the service chosen in the left column.
func (a *App) modelPane(c *ui.Context) {
	t := a.pal.theme
	ui.Scroll(c).Key("models").Grow(1).FillHeight().Children(func() {
		ui.Column(c).FillWidth().Padding(titleHeight+20, 56, 40, 56).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(760).Gap(32).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, "模型").FontSize(26).FontWeight(700)
					ui.Text(c, "选择这部作品和谁讨论，以及管理本机保存的模型服务。").FontSize(13.5).TextColor(t.TextMuted)
				})
				if a.Store != nil {
					a.usage(c)
				}
				a.providerForm(c)
			})
		})
	})
}

// section is a settings block: a title and caption over a bordered group.
func (a *App) section(c *ui.Context, title, caption string, fn func()) {
	t := a.pal.theme
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(15).FontWeight(600)
			if caption != "" {
				ui.Text(c, caption).FontSize(12.5).TextColor(t.TextMuted).LineHeight(1.55)
			}
		})
		a.pal.group(c).Padding(18, 20).Gap(16).Children(fn)
	})
}

// usage chooses which configured models this work talks to.
func (a *App) usage(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	a.section(c, "这部作品用哪个模型", "讨论模型负责回应你；整理模型在后台把讨论整理进设定文档，可以用更便宜的模型或更低的推理等级；写作模型负责正文页的续写、改写和起草。推理等级 Default 交给服务商决定。改动从下一次发送开始生效。", func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "讨论 · 服务", func() {
				a.providerSelect(c, &a.talkProvider, "", "讨论提供商", busy)
			}).Width(200)
			labeled(c, "讨论 · 模型", func() {
				modelField(c, &a.talkModel, a.providerModels(a.talkProvider)).Label("讨论模型").Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
			}).Grow(1)
			labeled(c, "推理等级", func() { effortSelect(c, &a.talkEffort, "讨论推理等级", busy) }).Width(136)
		})
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "整理 · 服务", func() {
				a.providerSelect(c, &a.bookProvider, sameAsTalk, "记账提供商", busy)
			}).Width(200)
			labeled(c, "整理 · 模型", func() {
				if a.bookProvider == "" {
					ui.Text(c, "和讨论用同一个模型").TextColor(t.TextMuted).Padding(7, 0)
					a.bookModel = ""
				} else {
					modelField(c, &a.bookModel, a.providerModels(a.bookProvider)).Label("记账模型").Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
				}
			}).Grow(1)
			labeled(c, "推理等级", func() { effortSelect(c, &a.bookEffort, "整理推理等级", busy) }).Width(136)
		})
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "写作 · 服务", func() {
				a.providerSelect(c, &a.penProvider, sameAsTalk, "写作提供商", busy)
			}).Width(200)
			labeled(c, "写作 · 模型", func() {
				if a.penProvider == "" {
					ui.Text(c, "和讨论用同一个模型").TextColor(t.TextMuted).Padding(7, 0)
					a.penModel = ""
				} else {
					modelField(c, &a.penModel, a.providerModels(a.penProvider)).Label("写作模型").Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
				}
			}).Grow(1)
			labeled(c, "推理等级", func() { effortSelect(c, &a.penEffort, "写作推理等级", busy) }).Width(136)
		})
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			if ui.Checkbox(c, &a.write.autoPredict, "停止输入后自动续写").Disabled(busy).Changed() {
				a.savePrefs() // kept as ticked, not with the models below
			}
			ui.Text(c, "续写总是不推理、只写一句。默认按 "+modKey+"J 才续写；打开上面这项后，每次停下来都会调用一次模型。").FontSize(12).TextColor(a.pal.faint).LineHeight(1.5)
		})
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			if ui.Checkbox(c, &a.write.autoSummary, "离开改过的章节时更新摘要").Disabled(busy).Changed() {
				a.savePrefs() // kept as ticked, not with the models below
			}
			ui.Text(c, "起草时 AI 会看到前面各章的摘要。每更新一章调用一次写作模型（不推理）；你自己写的摘要不会被覆盖。").FontSize(12).TextColor(a.pal.faint).LineHeight(1.5)
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
			labeled(c, "上下文预算（token，含回答预留）", func() {
				ui.TextInput(c, &a.budget).Label("上下文预算").FillWidth().Disabled(busy)
			}).Width(200)
			ui.Text(c, "不要超过模型的上下文窗口。放不下的设定会在回复下的“AI 看到了什么”里列出。").FontSize(12).TextColor(a.pal.faint).LineHeight(1.5).Grow(1).Padding(0, 0, 8, 0)
		})
		a.pal.hairline(c)
		ui.Row(c).FillWidth().Children(func() {
			ui.Spacer(c)
			if a.pal.primary(c, "保存作品模型选择").Disabled(busy).Clicked() {
				a.setError(a.saveUsage())
			}
		})
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
	if ui.Select(c, &chosen, names).Label(label).FillWidth().Disabled(disabled).Changed() {
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
	if ui.Select(c, &chosen, names).Label(label).FillWidth().Disabled(disabled).Changed() {
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
		return errors.New("上下文预算需要是整数")
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
		return errors.New("写作模型选了单独的服务，还需要填写模型 ID；或改回“沿用讨论模型”")
	}
	// The store requires a provider and a model together; say which is missing.
	if (w.Discussion.Provider == "") != (w.Discussion.Model == "") {
		return errors.New("讨论模型需要同时选择服务和填写模型 ID")
	}
	if w.Bookkeeping.Provider != "" && w.Bookkeeping.Model == "" {
		return errors.New("整理模型选了单独的服务，还需要填写模型 ID；或改回“沿用讨论模型”")
	}
	w.TokenBudget = budget
	if err = a.Store.SaveWork(w); err == nil {
		a.savePrefs()
		if a.write.ed != nil {
			a.write.ed.SetPredictOnDemand(!a.write.autoPredict)
		}
		a.notice = "模型设置已保存，从下一条发言起生效"
		clear(a.sums.failed) // what failed for want of a model may work now
		a.kick()             // a model or budget that now works takes in what waited
	}
	return err
}

func (a *App) providerForm(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	title := "添加自定义 API"
	if a.providerID != "" {
		title = a.provider.Name
		if title == "" {
			title = a.provider.ID
		}
	}
	a.section(c, title, "服务配置保存在本机，所有作品共用。", func() {
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
					ui.Text(c, "本机地址：请确认这个服务不会再转发到云端").FontSize(12).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "远程服务：你的讨论内容会发送到这个地址").FontSize(12).TextColor(t.Warning)
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
			}).Grow(1)
			labeled(c, "或读取环境变量", func() {
				ui.TextInput(c, &a.provider.Env).Label("环境变量名称").Placeholder("例如 DEEPSEEK_API_KEY").FillWidth().Disabled(busy)
			}).Width(240)
		})
		hint := "key 存入系统钥匙串，不会写进配置文件或作品。"
		if env := trim(a.provider.Env); env != "" && os.Getenv(env) != "" {
			hint += "已检测到环境变量 " + env + "。"
		}
		ui.Text(c, hint).FontSize(12).TextColor(a.pal.faint).Margin(-8, 0, 0, 0)
		labeled(c, "可用模型（每行一个；也可以在上方直接输入目录外的模型）", func() {
			ui.TextArea(c, &a.models).Label("模型目录").Height(96).FillWidth().Disabled(busy)
		})
		a.pal.hairline(c)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if a.pal.secondary(c, "获取可用模型").Disabled(busy).Clicked() {
				a.fetchModels(c)
			}
			if a.providerID != "" && ghost(c, "清除已保存 key", t.Danger).Disabled(busy).Clicked() {
				err := model.SetKey(a.providerID, "")
				if err == nil {
					a.notice = "已清除保存的 key"
				}
				a.setError(err)
				a.key = ""
			}
			ui.Spacer(c)
			if a.pal.primary(c, "保存提供商").Disabled(busy).Clicked() {
				err := a.saveProvider()
				if err == nil {
					a.notice = "提供商已保存"
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
	a.run(c, "正在获取模型目录…", func(ctx context.Context) (func(), error) {
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
func modelField(c *ui.Context, value *string, models []string) *ui.Element {
	t := c.Theme()
	cb := ui.ComboboxBase(c, value)
	in := cb.Input
	in.Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border)
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
	cb.Popup(func(panel *ui.Element) {
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(t.Radius+2).Background(t.Background).Border(1, t.Border)
		panel.Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.18))
		ui.Scroll(c).MaxHeight(t.Space(70)).FillWidth().Children(func() {
			for _, m := range shown {
				item := cb.Item(m).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius)
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
