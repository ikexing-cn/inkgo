# Agent Note: 宿主给扩展的 API

Status: proposed

## Problem

[扩展模块](2026-11-02-extension-modules.md)定了模型和行为：核心只有编辑器、目录、存储、扩展宿主；两种扩展；必需依赖加连带开关；不连带的可选依赖；全局和按书的开关；替换和四种注入；详情页全部来自宿主的账。作者 2026-11-03 要求「API 设计要做好」。本篇定宿主交给扩展的接口：扩展怎么写、宿主怎么记账、哪些规矩由类型和测试来守。

写这篇时的接口是每个子包一个窄 `Host`：`desk/phone/host.go` 有 7 个方法，`desk/exportdialog/host.go` 有 12 个，都由 [desk/host.go](../../../desk/host.go) 的 `appHost` 实现（这两个包现在已搬成扩展，见下）。它们有三个缺口：

- 只能取，不能加：命令、菜单、右栏标签都写死在 `desk`。
- 宿主不知道谁用了什么，详情页没有账可读。
- 没有关掉、拆干净的路。

## Proposal

包 `writer/ext` 是宿主对扩展的全部 API：登记、开关、Session、注入、出错。宿主（`desk`）开的点和给的服务放在 `writer/ext/hostapi`，只有类型和键。扩展（`ext/` 下除这两个以外的包）的非测试文件只能 import 这几样：

- `ext`、`ext/hostapi`、`desk/kit`、`editor`、`locales`、mygo；
- 书的数据：`workspace`、`desk/book`（界面线程看到的内容）、`internal/atomicfile`；
- 自己名下的子包（`ext/export/format` 这类）；
- 自己在 Requires 或 Optional 里声明了的扩展的公开包（`ext/context/contextapi` 这类）。别的扩展的其余子包不行。

`desk` 实现宿主，非测试文件不 import 任何具体扩展，扩展也不 import `desk`、`shelf`。具体装哪些扩展由 `cmd/inkgo/modules.go` 的清单定。`ext/boundary_test.go` 读源码查这几条，Requires 从 `Module` 字面量里读。

### 扩展

```go
type Module struct {
	ID       string        // "phone"：[a-z][a-z0-9]*，也是它所有 id 的前缀、翻译表的命名空间
	Kind     Kind          // ext.Feature 或 ext.Library（前置）
	Name     func() string // 显示时才取，取自翻译表
	Desc     func() string
	Requires []string      // 必需依赖：连带开关
	Optional []string      // 可选依赖：不连带，对方没开时用到它的部分不生效
	Setup    func(r *Registrar)
}
```

### 公开包只放类型和键

扩展对别人公开的东西放在 `<id>api` 子包里：数据类型、接口，以及带类型的键。键只是「名字加类型」，不带状态：

```go
package contextapi

type Context interface{ Entries(kind Kind) []Entry }

var Service ext.ServiceKey[Context] = "context"
var Kinds ext.PointKey[Kind] = "context.kinds"
```

import 一个公开包只是编译期用到它的类型，不等于运行时依赖它。所以边界测试查的是：import 了谁的公开包，谁就必须出现在 Requires 或 Optional 里；取服务也只许取这两处声明了的。

### 生命周期：Setup 一次，Start、Stop 多次

- **Setup** 每个进程跑一次，不管扩展开没开：
  - 只许登记，不许读写文件、起后台任务、取服务的实现；
  - 宿主把它登记的一切记进账里，所以扩展关着也列得出加了什么（VS Code 的 Feature Contributions）；
  - Setup 里调 IO 或 `Ref.Get` 直接 panic，测试能抓到。
- **Start** 在扩展生效时跑：
  - 跑的时机：开着且依赖都已 Start、打开一本书、换书；
  - 用 `r.OnStart(func(s *ext.Session) error)` 登记，读写文件、建状态都在这里做。
- **Session** 的寿命是「这个扩展开着」乘「这本书开着」：
  - 换书时所有开着的扩展都先 Stop 再 Start，所以两者都要便宜，有测试计时；
  - 按书开关变了的，只 Stop 或只 Start。
- **Stop** 按 Start 的相反顺序跑：
  - 宿主先拆掉这次 Session 里起的后台任务；
  - 再跑 `r.OnStop` 登记的清理。
- **贡献的回调**（命令的 Run、标签的 Draw……）只在扩展生效时被调，读 Start 时建好的状态。宿主在扩展没生效时不调它们。
- **状态**有四种：关着、启动中、开着、出错。出错时带错误原文和出错的位置：哪个回调，在谁的注入链里。

### 登记表

所有 id 由宿主加前缀：在 `phone` 里登记的 `open` 就是 `phone.open`。同名在 Setup 时就报错，不覆盖。

- **扩展点** `PointKey[T]`：
  - 宿主开的有命令、菜单项、右栏标签、整页、底栏项、正文工具条项、补全来源；
  - 别的点由扩展自己开，比如 AI 开「AI 工具」（`ai.tools`），导出开「导出格式」（`export.formats`）；
  - 扩展用 `ext.Declare(r, contextapi.Kinds)` 开自己的点，开的必须是自己名下的键；
  - 往点里放用 `ext.Contribute(r, point, "name", value)`。
- **服务** `ServiceKey[T]`：
  - 提供：`ext.Provide(r, key, func(*ext.Session) T)`。实现在 Start 时才造出来，所以能读这本书的数据。
  - 取用必需依赖的：`ref := ext.Use(r, key)`，返回 `*ext.Ref[T]`，用的时候 `ref.Get()`。这样关掉再开、换书之后，拿到的总是当前那份；Setup 时提供方可能还关着，也没有东西可返回。
  - 取用可选依赖的：`ref := ext.UseOptional(r, key)`，`ref.Get()` 返回 `(T, bool)`，对方没开时是 false。类型上就逼着调用方处理「没有」的情况。
  - 往可选依赖的点里放东西、注入它，写法和必需依赖一样；对方没开时宿主不生效这些登记，详情页写「X 没开，以下不生效」。对方中途开、关，宿主照账接上或拆下，依赖方不用重启。
- **宿主的能力也是服务**，键在 `hostapi` 包里：`BookService`（当前的书、选中的章节和编辑器、先存下显示着的那一章）、`PaletteService`（颜色）、`PaneService`（打开右栏的某个标签）、`NoticesService`（提示条）、`JobsService`（后台任务）、`DialogsService`（选文件夹、存文件、在文件夹里显示）。原来每个子包一个的窄 `Host` 已经换成这些。宿主的服务由 `ext.DeclareHost` 在装配时提供，一直在，不用写进 Requires。每个方法的 doc comment 写明在哪个线程调。
- **设置**：
  - `ext.Setting(r, "name", ext.Schema[T]{…})`，返回 `*ext.Pref[T]`，带 `Get()`、`Set()` 和 `OnChange`；值按 JSON 存；
  - Schema 有标题、说明、默认值、可选项（每项带说明）、分组、顺序、作用范围（`Local` 本机、`BookScope` 按书、`Both` 两者都有）；
  - 存放：本机的在 `works.json` 旁边的 `ext.json`（键 `off` 本机关掉的扩展、`settings`、`injections` 作者调过的顺序和选择），按书的在作品的 `.writer/ext.json`（键 `switches`、`settings`）。都用 `internal/atomicfile` 写。文件读不出来时这一次按默认处理、提示条说一句（`locales.ExtFileUnreadable`），在真正存一次以前不覆盖它；
  - 宿主照声明用 `kit` 画在设置里这个扩展的那一节：**还没做**，现在三个扩展的设置都是自己画的；要和别的扩展放在同一页，就写宿主开的具名分组（`ext.SettingsGroupChecks` 这类），宿主把放进同一组的项画在一页，每项记着是谁加的；
  - 要自己画就往 `hostapi.SettingsPages` 里放一页（备份就是这样），详情页标明「自定义设置页」。
- **数据目录**：`Session.BookDir()` 是 `.writer/ext/<id>/`，`Session.LocalDir()` 是 shelf 旁边的 `ext/<id>/`。宿主只在 Start 之后给。

### 注入

每个扩展点在声明时说明它的「调用」是哪个函数：命令是 Run，标签是 Draw，导出格式是 Write。点里每一项贡献的调用都是一个 `Hook[In, Out]`，`ext.HookOf(point, "phone.open")` 取到它。核心另外声明一些点，比如 `ext.EditorKey`（编辑器处理按键）、`ext.PaintProse`（画正文）。

```go
ext.Before(r, hook, func(in *In))                              // 改参数
ext.After(r, hook, func(in In, out *Out))                      // 改结果
ext.Around(r, hook, func(in In, next func(In) Out) Out)        // 决定调不调原来的
ext.Overwrite(r, hook, func(in In) Out)                        // 整个换掉
ext.Replace(r, point, "export.docx", value)                    // 换掉一整项贡献
```

- Replace 就是对那项贡献的 Overwrite，换的是整个值（标题、快捷键一起换）。
- 注入的目标必须属于 Requires 或 Optional 里的扩展，或者是核心的点。
- 目标没生效时，注入什么也不做；详情页写「X 没开，以下不生效」。
- before、after、around 可以叠好几层：
  - 默认按装配清单的顺序排；
  - 作者在被注入那一处调过的顺序存进设置。
- overwrite（包括 Replace）同一处只能有一个：
  - 两个扩展都要时，两个都不生效，原来的照跑，那一处标冲突；
  - 作者选了哪个，存进设置。
- 宿主内部（登记表、开关、账、详情页）没有键，注入不到。
- 链在登记变化时（开关、换书、作者调顺序）建好存起来，不在每次调用时现拼闭包。画和按键这类每帧都跑的链，要过现有的帧耗时检查。
- `Host.Changes()` 是一个计数：生效的贡献、开关、链有任何变化时加一，读它不分配。desk 由扩展派生的表（右栏标签、命令、菜单行、对话框、底栏项……）只在它变了时重建，平时用存着的。

### 出错

宿主在每一环调扩展代码的地方 recover：

- 只把出错的那个扩展标成出错，Stop 它，依赖它的跟着关。注入方出错时，只拆掉它的注入，被注入的照常。
- 提示条和详情页写明是哪个扩展、哪个回调、在谁的注入链里。
- 画界面的回调半路 panic 时，desk 把每个扩展画的东西包在一个 `ui.Box` 里，出错就把这个盒子藏起来：这一帧画了一半的不露出来，下一帧起不再调它。
- 宿主调扩展有两种办法，都 recover、都记账、都只停出错的和必需依赖它的：
  - `ext.Call(h, hook, in, original)`：要让别的扩展能改的调用，经注入链。命令的 Run、各种 Draw、每帧的 Tick、离开作品前的回调都走它。
  - `ext.Guard(h, owner, target, fn)`：只是问一下、不该被注入的，不声明调用处、不进链。对话框开没开（`Overlay.Open`）、待处理的缓存键和那一组（`PendingSource`）、点了待处理的一项都走它。
  - 扩展没生效、或调用出错，都返回能用 `errors.Is(err, ext.ErrCallFailed)` 判断的错误。宿主自己（`ext.HostOwner`）的 panic 不吞。
- 提示条只写扩展名和错误原文：「{name}出错，已停下：{error}」（`locales.ExtFailed`）。出错的阶段、调用处、在谁的注入里记在账的 `Errors` 里，留给详情页。

### 账和详情页

宿主给每个扩展记一本账，详情页、「扩展」页、被注入那一处的链都只读这本账：

- 必需依赖、可选依赖（各自开没开），以及反过来谁依赖它、谁注入它；
- 开的点、放进去的贡献、提供和取用的服务；
- 注入：哪一处、哪一种、排第几、生效还是冲突、目标开没开；
- 设置、自定义设置页；
- 数据目录和大小；
- 状态、出错记录。

扩展经 Registrar 和 Session 做的每件事都记在它名下（cordis 的 `Fiber.effect` 和 `getEffects()`）。关掉时宿主照账拆，拆完这个扩展在生效表里一条不剩，有测试。

### 两层

- **Go 层**：仓库里的扩展用，就是上面这些。可以在帧里拿 `*ui.Context` 用 `kit`、`ui` 画，守 `mygo vet`。
- **数据层**：以后给外部插件用。规矩同一套，参数换成 id 和普通数据；界面交出描述，由宿主画；不交出 Go 指针或帧内对象。外部插件的清单另写宿主 API 版本（VS Code 的 `engines.vscode`）和权限。数据层等 mygo 的脚本方案出来再定细节。

### 例子

写例子是为了找出接口缺的东西。名字按现在的代码起，落地时再定。

**手机预览**：一个标签、一个命令、一个本机设置。

```go
var Module = ext.Module{ID: "phone", Kind: ext.Feature, Name: locales.PhoneName, Desc: locales.PhoneDesc, Setup: setup}

func setup(r *ext.Registrar) {
	p := &preview{}
	p.dark = ext.Setting(r, "dark", ext.Schema[bool]{Title: locales.PhoneDark, Scope: ext.Local})
	p.book = ext.Use(r, ext.Book)
	ext.Contribute(r, ext.PaneTabs, "preview", ext.PaneTab{Title: locales.PhoneTab, Draw: p.draw})
	ext.Contribute(r, ext.Commands, "open", ext.Command{
		Label: locales.PhoneOpen, Menu: ext.MenuView, Run: p.open,
	})
	r.OnStop(p.reset)
}

func (p *preview) draw(c *ui.Context) ui.Element {
	id, ed, ok := p.book.Get().Chapter()
	…
}
```

**导出 docx 被别的扩展换掉**：导出开了「导出格式」这个点，docx 是它自己放进去的一项；另一个扩展换掉它。

```go
// ext/export
ext.Declare(r, exportapi.Formats)
ext.Contribute(r, exportapi.Formats, "docx", exportapi.Format{Name: locales.ExportDocx, Ext: ".docx", Write: writeDocx})

// ext/fancydocx，Optional: []string{"export"}
ext.Replace(r, exportapi.Formats, "export.docx", exportapi.Format{Name: locales.FancyDocx, Ext: ".docx", Write: fancy})
```

关掉导出时 fancydocx 不跟着关，只是详情页写「导出没开，以下不生效」。再有一个扩展也 Replace `export.docx` 时，两个都不生效，导出对话框里 docx 那一项照原来的导，标出冲突。

**名字补全**：从 context 取设定的名字，放进核心的补全来源。现在 [desk/names.go](../../../desk/names.go) 的 `entities()` 直接读 `a.book.Setting()`。

```go
// ext/names，Requires: []string{"context"}
func setup(r *ext.Registrar) {
	lore := ext.Use(r, contextapi.Service)
	ext.Contribute(r, ext.Completions, "setting", ext.Completion{Names: func() []editor.Entity {
		return entities(lore.Get().Entries(contextapi.Names))
	}})
}
```

**注入核心的点**：一个把半角标点换成全角的扩展，在编辑器处理按键之前改输入。

```go
ext.Before(r, ext.EditorKey, func(in *ext.KeyIn) { in.Text = fullWidth(in.Text) })
```

写例子时发现的缺口，落地时一并定：

- 菜单位置要有名字：`ext.MenuView` 这类常量，加上「排在谁后面」。现在的标题栏菜单和 macOS 的系统菜单都从 [desk/commands.go](../../../desk/commands.go) 的表生成，命令的贡献要能进同一张表。
- 命令的快捷键要分系统写，和现在的 `macKey`、`macMods` 一样，冲突照快捷键表的规矩标出来。
- `ext.Book` 交出 `*editor.Editor` 只在 Go 层可以；数据层要换成章节 id 加读写正文的服务。
- `ext.EditorKey` 每按一下都跑整条链，链要预先建好，并有帧耗时的测试。
- 补全来源要说明多久取一次。现在 `namesKey` 按设定是否变了来缓存，点里要留一个「什么变了才重取」的键。

## Alternatives considered

- **每个子包一个窄 `Host` 接口（现在的做法）** — 最强的理由是每个包要什么一眼看得出，测试好替身。没选：只能取，不能往宿主里加东西；宿主不知道谁用了什么，账没法记。按需取服务以后，每个扩展用了什么照样列得出来，而且是宿主记的。
- **`ext.Use` 在 Setup 时直接返回实现** — 最强的理由是用起来少一层 `Get()`。没选：Setup 时提供方可能关着，关掉再开、换书以后拿到的会是旧的那份。
- **只有 Setup，开的时候跑、关的时候拆** — 最强的理由是只有一个入口，更简单。没选：Setup 读文件的话，扩展关着就列不出它加了什么；IO 只能放在 Start 里。
- **只有必需依赖，注入也得写进 Requires** — 最强的理由是只有一种关系。没选：作者 2026-11-03 要可选依赖；只有必需依赖的话，关掉目标会连带关掉注入方，也没法往可有可无的东西上注入（Fabric 的可选 mixin 就是为这个）。
- **可选依赖的服务也用 `Use`，`Get()` 没有时返回零值** — 最强的理由是少一个函数。没选：零值的接口一调就 panic，调用方容易忘了判断；返回 `(T, bool)` 让「没有」在类型上看得见。
- **改写机器码（gomonkey 这类）注入任意 Go 函数** — 最强的理由是什么都能改，不用宿主预先开口子。没选：内联后的函数改不到，macOS 有代码页写保护，`CGO_ENABLED=0` 的发行版上也不可靠；改了什么宿主也记不了账。
- **反射式依赖注入（uber/fx、dig）** — 最强的理由是不用手写装配。没选：依赖图在运行时才解，出错难读，也不好显示给作者。
- **字符串键、参数是 `any` 的事件总线** — 最强的理由是灵活，扩展之间不用共享类型。没选：编译期查不出类型错。带类型的键同样只是一个名字，而且查得出。
- **服务一出现就自动启用依赖方（cordis 的 `inject`）** — 最强的理由是不用作者动手。没选：作者要开关都看得见，见扩展模块篇。

## Acceptance criteria

每条规矩一个用假扩展的测试：

- 打开一个，它的依赖一起打开；关掉一个，依赖它的一起关；再开时各自回到原来的开关。
- 扩展关着时，账里也列得出它的贡献、注入、设置。
- Setup 里读文件或调 `Ref.Get` 会失败。
- 同一个扩展内 id 重名时，Setup 报错。
- 取用没声明依赖的服务、注入没声明的目标时，Setup 报错。
- 两个 overwrite 抢同一处：两个都不生效，原来的照跑，账里标冲突；作者选一个以后，被选的那个生效。
- 注入的目标关着时，注入方照常开着，注入不生效，账里写「X 没开，以下不生效」。
- 一个扩展的回调 panic：只有它和依赖它的变成出错或关着，别的照常。
- 关掉以后，这个扩展在生效表里一条不剩：命令、菜单、标签、注入、服务、后台任务都没了。
- 换书时开着的扩展 Stop、Start 一轮，总耗时有上限。
- 依赖边界测试：import 了谁的公开包，谁就在 Requires 或 Optional 里。
- 可选依赖：打开依赖方不会打开对方；关掉对方，依赖方照常开着，`UseOptional` 的 `Get()` 返回 false，往对方放的东西和注入都不生效；再打开对方，这些不重启依赖方就回来。

## Risks

- 例子只写了四个，伏笔、检查这类要碰正文标记的功能可能还要新的点。这等 marks 时再设计，先迁手机预览、导出、备份，验证这套接口。
- 「凡经宿主走的调用都能注入」会让每个贡献多一层间接调用。不经宿主的内部调用不受影响，每帧都跑的链要量。
- 画界面的回调出错后，这一帧能不能保持完整，取决于 mygo（待核实）。
- 换书时整轮 Stop、Start，扩展一多会变慢；到时可以改成只重启用了书的数据的那些扩展。
- 数据层没有实现，Go 层的接口可能长成数据层表达不了的样子。所以 Go 层每加一种点，都要说明它在数据层怎么表达；表达不了的，标成只限 Go 层。

## 相关笔记当场审计

- [扩展模块](2026-11-02-extension-modules.md)：本篇的上篇，互链。那篇管模型和行为，本篇管接口；两篇说法不一样时以那篇为准，并当场改齐。
- [扩展的安全模式](../feature/2026-11-03-extension-safe-mode.md)：部分重叠。安全模式读的就是本篇的账和状态。
- [desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)、[理一遍代码](../../implemented/architecture/2026-10-21-code-reorganization.md)：部分重叠。它们定的窄 `Host` 会被服务取代，落地时改那两篇。
- [多语言方案](2026-11-02-i18n-on-mygo.md)：部分重叠。`Name`、`Label` 存函数，显示时才取，和那篇的规矩一致。
