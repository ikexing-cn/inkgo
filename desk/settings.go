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

func (a *App) settings(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(232).FillHeight().Padding(20, 10).Gap(2).Background(a.pal.side).Shrink(0).Children(func() {
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
					color := a.pal.brand
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
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(36, 40).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(680).Gap(32).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						ui.Text(c, "模型设置").FontSize(24).FontWeight(600)
						ui.Text(c, "选择这部作品和谁讨论，以及管理本机保存的模型服务。").FontSize(13.5).TextColor(t.TextMuted)
					})
					if a.Store != nil {
						a.usage(c)
					}
					a.providerForm(c)
				})
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
	var ids []string
	for _, p := range a.Config.Providers {
		ids = append(ids, p.ID)
	}
	a.section(c, "这部作品用哪个模型", "讨论模型负责回应你；整理模型把对话拆成思路卡片，可以用更便宜的模型或更低的推理等级。推理等级 Default 交给服务商决定。改动从下一次发送开始生效。", func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "讨论 · 服务", func() {
				ui.Select(c, &a.talkProvider, ids).Label("讨论提供商").FillWidth().Disabled(busy)
			}).Width(200)
			labeled(c, "讨论 · 模型", func() {
				ui.Autocomplete(c, &a.talkModel, a.providerModels(a.talkProvider)).Label("讨论模型").Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
			}).Grow(1)
			labeled(c, "推理等级", func() { effortSelect(c, &a.talkEffort, "讨论推理等级", busy) }).Width(136)
		})
		book := a.bookProvider
		if book == "" {
			book = sameAsTalk
		}
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			labeled(c, "整理 · 服务", func() {
				if ui.Select(c, &book, append([]string{sameAsTalk}, ids...)).Label("记账提供商").FillWidth().Disabled(busy).Changed() {
					a.bookProvider = ""
					if book != sameAsTalk {
						a.bookProvider = book
					}
				}
			}).Width(200)
			labeled(c, "整理 · 模型", func() {
				if a.bookProvider == "" {
					ui.Text(c, "和讨论用同一个模型").TextColor(t.TextMuted).Padding(7, 0)
					a.bookModel = ""
				} else {
					ui.Autocomplete(c, &a.bookModel, a.providerModels(a.bookProvider)).Label("记账模型").Placeholder("输入或选择模型 ID").FillWidth().Disabled(busy)
				}
			}).Grow(1)
			labeled(c, "推理等级", func() { effortSelect(c, &a.bookEffort, "整理推理等级", busy) }).Width(136)
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
			labeled(c, "上下文预算（token，含回答预留）", func() {
				ui.TextInput(c, &a.budget).Label("上下文预算").FillWidth().Disabled(busy)
			}).Width(200)
			ui.Text(c, "不要超过模型的上下文窗口。放不下的思路会在回复下的“AI 看到了什么”里列出。").FontSize(12).TextColor(a.pal.faint).LineHeight(1.5).Grow(1).Padding(0, 0, 8, 0)
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
	// The store requires a provider and a model together; say which is missing.
	if (w.Discussion.Provider == "") != (w.Discussion.Model == "") {
		return errors.New("讨论模型需要同时选择服务和填写模型 ID")
	}
	if w.Bookkeeping.Provider != "" && w.Bookkeeping.Model == "" {
		return errors.New("整理模型选了单独的服务，还需要填写模型 ID；或改回“沿用讨论模型”")
	}
	w.TokenBudget = budget
	if err = a.Store.SaveWork(w); err == nil {
		a.notice = "已保存，下一次发送开始使用"
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
