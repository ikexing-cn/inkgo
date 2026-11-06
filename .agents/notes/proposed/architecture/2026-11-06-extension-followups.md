# Agent Note: 扩展契约的后续范围

Status: proposed

## Problem

可信 Go 模块的失败路径已修复，但尚不能据此承诺稳定第三方插件接口。

## Proposal

保留待讨论范围：外部运行时、安全模式、通用身份协议、异步隔离、深拷贝、多语言扩展命名空间及运行时缺译回退。它们不属于已落地规则。安全模式沿用[既有提案](../feature/2026-11-03-extension-safe-mode.md)。已落地部分见[契约修复](../../implemented/architecture/2026-11-06-extension-contract-review.md)。

### 数据接口、运行时和 MCP 的待实施设计

外部层只传 ID、普通 DTO 和 UI 描述，由宿主用 kit 绘制，不交出 Go 指针或帧对象；每帧跨脚本调用有成本。Go 层新增点须注明数据表达，不能表达的标只限 Go。未来运行时为适配层，候选 wazero/Extism（纯 Go、沙箱、多语言）或 goja（JS），等待 mygo 方案后再选；插件清单声明数据/网络权限及宿主 API 版本，由宿主执行权限控制。

MCP 单独提供书的 context/工具给外部 AI，并为写插件的 AI 提供接口文档、清单格式和校验工具，不承载界面。作者直接写批注也仍是待设计功能，当前 notes 只排已有批注；先在原型确认后实施。

### 数据适配盘点（尚未实现）

下面保留旧 API 篇的数据表达候选，而非已发布的 wire 契约；具体键及 Go 签名以当前源码为准，实施时逐项核对。

| 包和键 | 现在的约定 | 数据层怎么表达 |
|---|---|---|
| ext/ai/aiapi: `ai.calls` (Calls) | `PointKey[func(Call)]` | 只限 Go 层：捕获的 Store 和 Session 身份。适配：书 ID 和这次开启 ID，加上不可变的用量 Record DTO，以及有类型的完成通知。 |
| ext/ai/aiapi: `ai.model-changes` (ModelChanges) | `PointKey[func()]` | 提供方/配置修订事件，没有原生负载；调用订阅者的动作 ID。 |
| ext/ai/aiapi: `ai.service` (Service) | `ServiceKey[Backend]` | 只限 Go 层：原生后端里的 Store 和补全函数；也给选好的模型句柄配写作的笔（`Assistant.Writer`，检查用）。远端使用固定的书 ID、模型/角色/上限 DTO、请求 ID、流式增量和取消。 |
| ext/ai/aiapi: `ai.tools` (Tools) | `PointKey[Tool]` | 只限 Go 层：ToolEnv.Store、context 和回调函数。适配：工具 ID/schema JSON，书 ID、这次开启 ID、会话 ID、轮次 ID、调用 ID，参数 JSON 和 ToolResult DTO。 |
| ext/ai/aiapi: `ai.usage` (Usage) | `PointKey[func(*ui.Context)]` | 只限 Go 层：绘制回调拿着帧内的 `*ui.Context`。适配：贡献 ID，宿主每帧要一份 UI 描述 DTO，交互回成动作 ID。 |
| ext/context/contextapi: `context.entries` (Entries) | `PointKey[EntryProvider]` | 提供方 ID + 书 ID → 条目 DTO 列表；空列表与不存在要区分开。 |
| ext/context/contextapi: `context.hooks` (Hooks) | `PointKey[HookProvider]` | 提供方 ID + 书 ID → 伏笔 DTO 列表；空列表与不存在要区分开。 |
| ext/context/contextapi: `context.outlines` (Outlines) | `PointKey[OutlineProvider]` | 章/卷 ID → 章纲文本和提供方是否存在的标记。 |
| ext/context/contextapi: `context.service` (Service) | `ServiceKey[Context]` | 书 ID、条目 ID、章 ID → Entries/Hooks/Outline DTO，以及明确的提供方是否存在标记。 |
| ext/find/findapi: `find.replacement` (ReplacementService) | `ServiceKey[Replacement]` | 章 ID、替换前后的字符串，以及替换/编辑操作 DTO；由宿主协调编辑器改动。 |
| ext/hostapi: `host.activities` (Activities) | `PointKey[Activity]` | 贡献 ID → 「在跑、不挡作者操作」「进度正显示着」两个标记，帧线程上查。 |
| ext/hostapi: `host.before-chapter-change` (BeforeChapterChange) | `PointKey[func()]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.before-editor` (BeforeEditor) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.before-leave` (BeforeLeave) | `PointKey[func()]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.before-prose` (BeforeProse) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.beside` (BesideItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.book` (BookService) | `ServiceKey[Book]` | 只限 Go 层：借用的 *book.Book 和受信任的 *workspace.Store；远端适配必须使用固定书 ID 和读取 DTO。 |
| ext/hostapi: `host.chapter-edits` (ChapterEditStates) | `PointKey[ChapterEdits]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.chapter-head` (ChapterHeadItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.chapter-open` (ChapterOpens) | `PointKey[func(ChapterOpened)]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.commands` (Commands) | `PointKey[Command]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.dialogs` (DialogsService) | `ServiceKey[Dialogs]` | 只限 Go 层：帧 Context 和原生回调。远端使用对话框请求 ID 和所选路径/结果数据。 |
| ext/hostapi: `host.editing` (EditingService) | `ServiceKey[Editing]` | 只限 Go 层：实时编辑器和 Snapshot/编辑协调。远端：书 ID、章 ID、编辑操作、快照 ID、撤销动作 ID 和状态 DTO。 |
| ext/hostapi: `host.error-texts` (ErrorTexts) | `PointKey[ErrorTranslator]` | 只限 Go 层：Go 错误身份和翻译回调。适配：结构化错误码/参数，以及按本地化时间生成的消息。 |
| ext/hostapi: `host.extensions` (ExtensionsService) | `ServiceKey[Extensions]` | 扩展 ID → 是否启用的布尔值；本地化名字 + 是否装着。 |
| ext/hostapi: `host.footer` (FooterItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.jobs` (JobsService) | `ServiceKey[Jobs]` | 只限 Go 层：工作函数、Context 和 Services。远端使用这次开启 ID/任务 ID、取消请求和结果 DTO。 |
| ext/hostapi: `host.leave-questions` (LeaveQuestions) | `PointKey[func(func()) bool]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.machine-paths` (MachinePathsService) | `ServiceKey[MachinePaths]` | 目录字符串和路径比较请求；在宿主权限下解析机器路径。 |
| ext/hostapi: `host.name-highlights` (NameHighlights) | `PointKey[NameProvider]` | 只限 Go 层：编辑器装饰数据和回调导航。适配：章 ID、范围/名称/样式 DTO 和动作 ID。 |
| ext/hostapi: `host.native-ui` (NativeUIService) | `ServiceKey[NativeUI]` | 只限 Go 层：借用的 Context/Element、菜单状态、动画和窗格绘制。宿主渲染 UI 描述；历史标签委托给 aiapi。 |
| ext/hostapi: `host.navigation` (NavigationService) | `ServiceKey[Navigation]` | 页面/设置/章/条目/扩展 ID、条目 DTO、焦点/打开/丢失/暂停标记；导航请求和状态回复。 |
| ext/hostapi: `host.notices` (NoticesService) | `ServiceKey[Notices]` | 只限 Go 层：*kit.Status 和原生撤销闭包；远端使用通知 DTO 和动作 ID。 |
| ext/hostapi: `host.overlays` (Overlays) | `PointKey[Overlay]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.pages` (Pages) | `PointKey[Page]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.palette` (PaletteService) | `ServiceKey[Palette]` | 只限 Go 层：原生调色板指针；远端适配会发布命名的颜色令牌数据。 |
| ext/hostapi: `host.pane` (PaneService) | `ServiceKey[Pane]` | pane/tab/page ID 与打开/选中标记，show/reveal/toggle 请求。 |
| ext/hostapi: `host.panes` (PaneTabs) | `PointKey[PaneTab]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.pending` (PendingGroups) | `PointKey[PendingSource]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.pending-state` (PendingService) | `ServiceKey[Pending]` | 待处理项目键 → 清除忽略请求。 |
| ext/hostapi: `host.predictions` (Predictions) | `PointKey[Prediction]` | 只限 Go 层：交出 `editor.Predictor`。适配：章 ID、光标前的正文 → 续写文字的请求和流式回复，接受/放弃回成动作 ID。 |
| ext/hostapi: `host.prose-marks` (ProseMarks) | `PointKey[ProseMarkProvider]` | 只限 Go 层：编辑器装饰数据和回调导航。适配：章 ID、范围/名称/样式 DTO 和动作 ID。 |
| ext/hostapi: `host.prose-overlays` (ProseOverlays) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.prose-ui` (ProseUIService) | `ServiceKey[ProseUI]` | 只限 Go 层：帧几何、编辑器和绘制/菜单回调。远端使用章 ID/文本、几何和 UI 描述消息。 |
| ext/hostapi: `host.save-edits` (SaveEdits) | `PointKey[func() error]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.search` (SearchService) | `ServiceKey[Search]` | 章 ID + 查询/引文 → 跳转或计数。 |
| ext/hostapi: `host.selection-buttons` (SelectionButtons) | `PointKey[SelectionButton]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.selection-menu` (SelectionMenus) | `PointKey[SelectionMenu]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.settings` (SettingsPages) | `PointKey[SettingsPage]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.settings-details` (SettingsDetails) | `PointKey[SettingsDetail]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.settings-sections` (SettingsSections) | `PointKey[SettingsSection]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.state` (State) | `PointKey[ViewState]` | 只限 Go 层：刷新/窗格回调；远端使用书 ID、模块 ID、窗格 ID 和修订通知。 |
| ext/hostapi: `host.ticks` (FrameTicks) | `PointKey[FrameTick]` | 只限 Go 层：每帧的回调拿着 `*ui.Context`。适配：宿主按帧或按修订号发事件（普通数据），扩展回动作 ID；不交出帧内对象。 |
| ext/hostapi: `host.title` (TitleItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.tool-hints` (ToolHints) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.tools` (Tools) | `PointKey[Tool]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.window-activity` (WindowActivities) | `PointKey[func(WindowActivity)]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.word-popovers` (WordPopovers) | `PointKey[WordPopover]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.word-suffix` (WordSuffix) | `PointKey[CountSuffix]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.writing-keys` (WritingKeys) | `PointKey[WritingKey]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/lore/loreapi: `lore.active` (Active) | `ServiceKey[struct{}]` | 提供方存在标记，没有负载。 |
| ext/lore/loreapi: `lore.entry-navigation` (EntryNavigationService) | `ServiceKey[EntryNavigator]` | 条目 ID 和条目 DTO，用于打开/定位/跳转/编辑/回答；待处理列表的动作 ID。 |
| ext/lore/loreapi: `lore.revisions` (RevisionsService) | `ServiceKey[Revisions]` | 书 ID + 作者的 Revision/Op DTO → 修订后的记录或错误；撤销动作 ID；刷新请求。 |
| ext/lore/loreapi: `lore.selection` (SelectionService) | `ServiceKey[Selection]` | 当前章 ID/选择 ID + 引文/文本/类别草稿 DTO；打开/关闭表单的动作 ID 和打开状态。 |
| ext/marks/marksapi: `marks.margin-sources` (MarginSources) | `PointKey[MarginSource]` | 只限 Go 层：可比较的原生缓存键和回调动作。适配：提供方修订号 + 章 ID/文本 → 范围/批注 DTO 和动作 ID。 |
| ext/marks/marksapi: `marks.service` (Service) | `ServiceKey[Reader]` | 只限 Go 层：MarginItem 回调和编辑器 MarkKind。适配：章 ID/文本、批注/范围/样式 DTO、动作 ID 和修订令牌。 |
| ext/outline/outlineapi: `outline.active` (Active) | `ServiceKey[struct{}]` | 提供方存在标记，没有负载。 |
| ext/talk/talkapi: `talk.proposals` (Proposals) | `PointKey[Suggestions]` | 只限 Go 层的 ProposalFrame.Stop 回调。适配：讨论 ID、调用 ID、章 ID，不可变的建议更新/失败，停止动作的动作 ID，以及每次操作的请求/回复。 |
| ext/talk/talkapi: `talk.service` (Service) | `ServiceKey[ServiceAPI]` | 讨论 ID → 打开请求，或本地化标题 + 是否找到标记；开讨论的来源 Event DTO → 已开始标记，起草上下文、在跑、进度的 DTO。只限 Go 层的是开讨论时带的 `ui.Services`，适配成发起请求的 ID。 |
| ext/talk/talkapi: `talk.starters` (TalkStarters) | `PointKey[TalkStarter]` | 讨论自己开的点，在 `talk.service` 背后，不给别的扩展直接读。只限 Go 层的 Start.Services。适配：讨论来源 Event DTO，返回已开始标记；起草上下文/在跑/进度 DTO。 |

## Alternatives considered

立即引入脚本隔离可以统一权限，但扩大迁移且不能自动解决现有书级状态语义；先修可信模块的可达故障，余项保留提案。

## Acceptance criteria

实施前分别明确运行时边界、权限与数据所有权；命名空间隔离须用不同扩展的同名消息验证，运行时回退与内置翻译齐全检查分别验证。

## Risks

外部运行时未选定；不能把 Go 类型接口直接当作跨进程协议，也不能把发布检查当作缺译回退。
