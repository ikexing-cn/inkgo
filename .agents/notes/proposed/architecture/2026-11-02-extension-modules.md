# Agent Note: 核心以外的功能做成扩展模块，为外部插件留出接口

Status: proposed

## Problem

2026-11-02 作者提出：维护一个插件目录，除了编辑器、目录（章节树）和 AI 基建，其余功能都做成插件，插件能新增、修改界面和功能。往后想给外部开发者提供 MCP（或别的）服务，让他们用 AI 写「nice for self」的插件；这样也更容易守住架构和基建。约束是 mygo 作者说过以后会出一套基于脚本语言的动态插件方案，到时要能接进来。mygo 0.7.4 的源码、文档和 issue 里都还没有这套方案，作者的原话链接也还没拿到，所以下面「运行时」那一层只能先定接口规则，不定实现。

现状：

- 已经有插件的雏形。[desk/outline](../../../desk/outline/host.go)、[desk/phone](../../../desk/phone/host.go)、[desk/exportdialog](../../../desk/exportdialog/host.go) 各经自己的 `Host` 接口取宿主能力，由 [desk/host.go](../../../desk/host.go) 的 `appHost` 实现；命令只有 [desk/commands.go](../../../desk/commands.go) 一张表；AI 工具只有 [discussion/tools.go](../../../discussion/tools.go) 一张表。
- 这些 `Host` 直接交出 `*editor.Editor`、`*book.Book`、`*kit.Palette`、`*ui.Context`。仓库里的 Go 代码这么用没问题；脚本或 wasm 里的代码拿不住这些指针，`mygo vet` 也禁止把 `*ui.Context`、`ui.Element` 留到帧外。
- `desk` 约 2.9 万行，状态集中在 `App`（见[理一遍代码](../../implemented/architecture/2026-10-21-code-reorganization.md)和[desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)）。
- 插件的数据没地方放：`workspace` 读 jsonl 时遇到不认识的字段直接报错；[prefs](../../../prefs/prefs.go) 是固定字段，[shelf](../../../shelf/shelf.go) 是本机设置，都没给插件留位置。
- `mygo.Plugin`（mygo 的 `plugin.go`）是「Go 服务 + 网页里的 JS 包」，服务 webview 页面，Inkgo 的原生界面用不上它。

## Proposal

分三层。现在只做第一层，第二层边迁移边定，第三层等 mygo。

### 第一层：仓库里的扩展模块

- 模块放在 `ext/<id>/`，每个是普通 Go 包。Go 类型叫 `Module`（或 `Extension`），不叫 `Plugin`，免得和 `mygo.Plugin` 混。
- 在 `cmd/inkgo` 用一份显式清单装配：`ext.Use(phone.Module, export.Module, …)`。顺序就是清单的顺序，测试可以只装一部分。不用 `init()` 加空白 import 的隐式登记。
- 模块在 `Setup(reg)` 里往登记表登记，能登记的就是这几类扩展点：命令（并入 `commands.go` 的那张表）、菜单项、右栏标签页、整页、AI 工具（并入 `discussion/tools.go` 的那张表）、设置项。模块只往这些预留的位置里放东西，不在核心界面的任意位置画。
- 经登记表加进来的，停用模块时一并拆掉（Obsidian 的 `register*` 那样），以后能不重启地启用、停用。
- 宿主能力照现有做法给：每个模块一个窄的 `Host` 接口。
- 每本书的模块数据放 `.writer/ext/<id>/`，本机的放 shelf 旁边按模块分开；各带自己的 schema 版本。这是一次存储格式的增补，要和[文件存储的取舍](../../implemented/architecture/2026-10-08-m1-storage-without-index.md)对齐，并在 `workspace` 的读写里给出这块目录。
- 每个模块包和现有的包一样，配依赖边界测试（`imports_test.go`）：模块不 import `desk` 的内部，`desk` 不 import 具体模块。
- 每个模块带自己的翻译表，见[多语言方案](2026-11-02-i18n-on-mygo.md)。

第一批：手机预览、导出、备份。它们已经经 Host 取能力，迁移成本最低，用来检验登记表够不够用。伏笔、检查、批注直接在正文上做标记，要等 `editor` 开放一套标记接口才能拆。

#### 设置里的「功能」（作者 2026-11-02 让先在原型里做）

模块能停用，作者就要有一处看得到、开得了关得了。先在原型里做，作者看过再定：

- 设置多一类「功能」，排在「外观」后面，管本机的所有作品，书架上也有。每个模块一行：勾选框加模块名，下面一行灰字说它在哪儿出现（「右栏『预览』和『视图』菜单里的『手机预览』」）。顺序就是装配清单的顺序。
- 只列模块，不列核心（编辑器、目录、AI 基建）。AI 有自己的「关闭 AI」，这里不重复。第一批只有手机预览、导出、备份三行，以后迁过来的接着往下列。
- 关掉一个，照「AI 可以整个关掉」的做法：它的命令、菜单项、右栏标签、设置里它那一类整个不见，不是变灰；它的数据不删，再开照旧；马上生效，不重开。
- 不问一句，因为什么都不丢。备份例外地在那一行下面多一句灰字：「关着时不再自动备份，已有的备份留着。」
- 外部插件的安装、来源、权限这一页先不放，等第二层。
- 原型已做（2026-11-02，等作者看）：状态 `modsOff` 记关掉的模块，换书时带着；`MODS()`、`modOn()` 在 `base.js`。手机预览关掉时右栏停在「预览」的回到「设定」，再开回到「预览」，和关 AI 时的讨论一样。走查截 `66b-modules`、`66c-modules-off`。

### 第二层：公开接口

给将来的外部插件用的数据契约：只交出 ID、普通数据和对界面的描述，宿主用 `kit` 画；never 交出 Go 指针或帧内对象。这条规则不依赖 mygo 选哪种脚本语言：每帧调一次脚本太贵，脚本也拿不住帧内对象，剩下的路就是返回描述、由宿主来画。

以后新写的非核心功能尽量只经这一层写（VS Code 自带的功能也是用公开接口写的扩展），哪里不够用，在仓库里就会暴露出来。

### 第三层：运行时

一个适配层，把第二层接到 mygo 将来的脚本方案上。mygo 迟迟不出时的候选：wazero 或 Extism（纯 Go、有沙箱、多语言）、goja（JS）。外部插件从一开始就在清单里声明权限（读哪些数据、能否联网），由宿主强制执行。

### MCP

MCP 不承载界面，单独一条线，两个用途：

- 把书的内容开放给外部 AI。`discussion/tools.go` 已有带参数定义的工具，可以复用。
- 给写插件的 AI 提供第二层的文档、清单格式和校验工具。

## Alternatives considered

- **Go 标准库的 `plugin` 包** — 最强的理由是插件就是 Go，能直接拿宿主的类型。没选：只支持 Linux、FreeBSD、macOS，关掉 cgo 时直接用 stub 返回 not implemented（`$GOROOT/src/plugin/plugin_stubs.go` 的构建条件是 `!cgo`）；宿主和插件要用同一工具链、同一份依赖构建。Inkgo 是 `CGO_ENABLED=0`，还要上 Windows。
- **`init()` 加空白 import 的隐式登记（Caddy、Telegraf 的做法）** — 最强的理由是加模块只动模块自己。没选：顺序藏在 import 里，测试没法只装一部分；模块不多，一份显式清单更好读。
- **HashiCorp go-plugin（子进程加 gRPC）** — 最强的理由是进程隔离，插件用什么语言都行。没选做界面插件：每次调用都跨进程，画界面撑不住。以后做「后台跑、不画界面」的插件（导入、联网查证）可以再看。
- **Yaegi（Traefik 的 Go 解释器）** — 最强的理由是插件作者写的还是 Go。没选：只能写 Go，解释执行慢，解释的代码不经过 `mygo vet`。
- **现在就选定 wasm 或 goja，先做外部插件** — 最强的理由是早一点让外部开发者用上。没选：mygo 说过要出自己的方案，现在选定，以后多半要换；第一层、第二层不管选哪个都得先做。
- **拿 MCP 当插件机制** — 最强的理由是 AI 工具已经都会说 MCP。没选：MCP 管的是 AI 能调用什么（工具、资源、提示词），不管界面；Zed 用它接 context server，也正打算弃用这条扩展路线。
- **让插件随意画界面** — 最强的理由是插件作者自由度最大。没选：[DESIGN.md](../../../DESIGN.md) 要求的一致性守不住；VS Code 也不给扩展碰 DOM，完全自定义的界面只能进 Webview。

## Acceptance criteria

- 手机预览、导出、备份迁到 `ext/` 后行为、截图不变，`npm run verify` 全绿。
- 三个模块全经登记表和各自的 `Host` 接入，`desk` 里不再有它们的具体名字（装配清单除外）。
- 停用一个模块，它的命令、菜单项、标签页都消失，再启用都回来，不用重启。
- 依赖边界测试覆盖 `ext/` 下每个包。

## Risks

- 登记表先按三个简单模块设计，到伏笔、检查这类要碰正文的功能时可能不够用。所以先迁三个，再定 `editor` 的标记接口。
- `App` 的状态没理清时拆模块，会把一大块状态整个搬进 `Host`，接口变宽。要和[项目架构审查](2026-10-27-maintainable-project-review.md)里「整理 App 的状态归属」一起排。
- mygo 的脚本方案若要求宿主接口长得不一样（比如是每帧回调），第二层要改。所以第二层先只在仓库里用，不对外承诺。
- 模块数据目录是存储格式的改动，作品被旧版本打开时要能忽略它。

## 相关笔记当场审计

- [项目架构审查](2026-10-27-maintainable-project-review.md)：部分重叠，互链。它管 `App` 的状态归属和 Book 的边界，本篇的扩展点要在那之上做。
- [多语言方案](2026-11-02-i18n-on-mygo.md)：部分重叠，互链。模块的翻译表按那篇的规矩来。
- [desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)、[理一遍代码](../../implemented/architecture/2026-10-21-code-reorganization.md)：无冲突，本篇沿用它们定下的 Host 做法。
