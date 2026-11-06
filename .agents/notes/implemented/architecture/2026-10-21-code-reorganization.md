# Agent Note: 理一遍代码：分七批重构

Status: implemented

## Problem

2026-10-21 作者说：「目前的架构感觉安排的不是很合理，有些杂乱不方便人工review」。

当天做了四份只读审查：
1. 跨包的架构。
2. `desk` 的界面外壳。
3. `desk` 的功能页加上 `workspace`、`discussion`、`ai`、`writing`。
4. `editor`、`model` 和几个小包。

结论是下层各包之间没有环，边界大体清楚，乱集中在 `desk`，有两个根子。

**一个 `App` 管所有状态。**
- `App` 有 93 个字段，473 个方法分在 37 个文件里的 34 个；写作页的 `writeView` 有 95 个字段。
- 任何文件都能读写任何状态，审一处改动看不出会波及哪儿。
- 测试里直接读字段 `a.write.`、`a.doc.`、`a.plan.`、`a.ck.` 的有 469 处。

**书的数据没有主人。**
- 章节树挂在写作页上：`a.write.tree` 被 19 个文件读，`loadTree()` 有 13 处调用。
- `write.entered` 这个标志靠手设：4 处设，3 处读。
- 章节树、检查、标记、设定、大纲各有一份缓存，各自加载、各自失效。
- 打开的章节有两份：`writeView` 和标签页的 `sheet` 手抄同样 8 个字段，切标签时来回拷。

很多效率问题是这两个根子长出来的。待办分组、右栏角标、人名链接、侧栏文件夹字数、检查标注、分类计数，都是每帧把全书重算一遍。检查做完一章、改一个伏笔，又都要把全书重读一遍。

其余问题：
- **约 2200 行规则和存取代码写在 `desk` 里**：文字差异比较、章节编号、导出范围、备份策略、书架操作、`desk.json` 的读写和迁移、检查队列、伏笔模型、讨论里的统计。不建 `App` 就测不了，审的人也得读界面代码才看得到规则。
- **重复**：
  - 整份写文件 5 份 3 种写法，书架和 `desk.json` 两份没有 `Sync`。
  - 严格解码 JSON 4 份。
  - 截断加「…」约 7 份。
  - `previous`、`holds`、`talkTitle`、`freshID`、`pickInto`、起不重名的后缀，各有一两份重复。
  - 分段控件 3 份，表单的「标签 + 控件」3 份。
  - 关闭作品的收尾 3 份，其中 `Close` 那份少调了 `stopChecks`。
- **跨包的隐式约定**：
  - `discussion` 往文字里写 `[n]`，`desk` 用正则再解析出来。
  - 摘要的四个 `## ` 标题由 `desk` 去切。
  - 「服务/模型」一处用 `LastIndex` 拆，一处用 `Cut` 拆，模型 ID 里带 `/` 时两边结果不同。
  - 「打开设置」按钮靠错误文字里有没有「模型」「AI」来判断出不出现。
- **编辑器（实测）**：
  - 画字时每帧扫整段字形：一段 1.2 万字时一帧 2.2 毫秒，71% 花在这。
  - 移光标也要整篇重新排版：0.83 毫秒，什么都没变时是 45 纳秒。
  - 找行是从头扫。
  - 标记和正文共用一个版本号：改一个伏笔就整篇重排。
  - 编辑器和手机预览各有一份排版。
- **存取层（实测）**：
  - 讨论搜全书时逐章 `ReadChapter`，每章都重建一次章节树：100 章要 509 毫秒，一次 `Texts` 只要 48 毫秒。
  - AI 记录每追加一条都要解析全部历史：1 万条时 395 毫秒、26 MB，期间还持有 `Store` 的锁。
  - 流式提议每来一小段，都从头重新解析一遍。
  - 大纲页每打一个字就同步写一次盘。
- **名字对不上内容**：
  - 入口还叫 `cmd/poc`，里面混着一个 `-editor` 演示，窗口标题「写作 PoC」。
  - `setting.go` 放的是右栏状态、`desk.json` 和灵感排序；`settings.go` 是 AI 服务表单；`prefs.go` 才是设置对话框。
  - `workspace/chapters.go` 有 1277 行。
  - 只复述代码的注释很多：`desk` 外壳那 24 个文件里有 807 行注释。

## Decision

分七批做完，2026-10-21 到 2026-10-24。每批由 Codex 实施（它不通的那几天，有几段改由 dsh 跑），Claude 写任务书、审查、合入。第 1 批作者审过；第 2 批做到一半时，作者交代剩下的不必逐批等审，七批做完一起看。每批都做到了：

- `npm run verify` 全过。
- 只挪的改动和改写的改动分开提交，挪的那次 diff 只有搬家。
- 默认不改行为；改了的，逐条列在下面「会改行为的」。
- 动到界面的批次，前后各跑一遍布局测试截图，逐张对比过。
- 同一次提交更新了相关的 implemented 笔记，在本篇的「进度」里记了一行。

### 第 1 批：编辑器（只动 `editor/`）

- `shapeText` 时建好「每个字从第几个字形开始」，`drawRun` 直接切片，不再扫整段。
- 文字的版本号和标记的版本号分开：排版、人名、续写预测只看文字的。
- 排版的缓存键里去掉光标，只在有输入法组字或灰字时才带上。没有插入内容时，`display` 直接用原文，不再复制。
- `lineOf`、`lineAt`、`ranges` 改成二分查找。人名框每次排版只算一遍，画的时候和鼠标悬停共用。
- 编辑器排版和 `LayoutParagraph`（手机预览）共用一份按段缓存的「整形 + 断行」结果。`ScrollAnchor` 搬到 `view.go`。
- 整理：
  - 续写预测的 9 个字段收进一个子结构，放进 `predict.go`；
  - 拆开 `paintIn`、`key` 两个长函数；
  - 加标记只留 `Doc.Add` 一种；
  - 抽出夹住区间的 `Doc.span` 和记一步撤销的 `pushMarkStep`；
  - 没有人用的导出方法删掉或改成不导出；
  - 删掉只复述名字的注释。
- 加 `editor/bench_test.go`，放审查时用的那组基准测试，前后对比写进任务交付。

### 第 2 批：存取层的白做功和共用小工具

- 讨论里搜全书改用 `Texts`，读章纲改用 `Outlines`，读一处设定不再先把所有讨论解析一遍。工具标签只用已经读到的数据生成，不读盘。
- `workspace/jsonl.go` 记住已经校验过的位置，追加一条只读新增的那一截；文件被换掉或截短时，退回整份校验。
- 流式提议增量解析，`cleanProse` 只处理新完成的段。
- 抽出共用小工具，放在新包 `internal/` 下：
  - 整份写文件：`Write` 和流式的 `Create`，替换现有 5 份，都 `Sync`，临时文件名不重复。
  - 严格解码 JSON：替换现有 4 份。
  - 截断：`Clip` 按字素簇数（segmenter v0.3.5），`ClipTokens` 按 token 一次算出切点。现有约 7 处各自的口径（n 或 n−1、断不断在换行）原样保留，有测试守着。
- 重复的合成一份：
  - `previous`、`holds` 改用 `Node.Within`；
  - 修订改用 `freshID`；
  - 选备份文件夹改用 `pickInto`；
  - `export/docx.go` 重复的 XML 片段；
  - `slicesEqual` 改用 `slices.Equal`。
- 备份读全书快照放到后台做，界面先 flush；设置备份的对话框不再每帧找文件夹。
- `model`：`Protocol` 改成有类型的常量，`table.go` 按内容拆成 `model.go` 和 `pricing.go`。
- 测试：
  - 截图统一用一个 helper，`SHOT` 一律是目录；
  - `cmd/poc` 测试里的建书、建 `App` 合成一个 helper；
  - `desk` 的测试 helper 合并。

### 第 3 批：整理（挪代码、改名字）

- 页码改成有名字的类型常量，删掉 `View()` 里只有测试才走的第 1、4 页分支，测试改成直接打开设置。
- 三份关闭作品的收尾合成一个 `leaveWork()`，`Close` 也停掉正在跑的检查。
- 修好 `TestShotMenus`：它只在设了 `SHOT` 时才跑，第 1 批之前就在 `desk/menu_test.go:318` 找不到讨论的元素，最后一张菜单截图出不来。
- 文件按内容拆分、改名：
  - `setting.go` 拆成右栏状态、`desk.json`、灵感三处；
  - `settings.go` 改名为 AI 服务表单；
  - `prefs.go` 改名为设置对话框；
  - `write.go` 按区域拆开：侧栏、标签与章节、保存与看盘、字数与目标、写作页的帧；
  - `footBar` 搬出 `check.go`，大纲的常量搬出 `theme.go`；
  - `workspace/chapters.go` 拆成 8 个文件：树、路径、`.order`、增删改、回收站、标记、正文、汇总。
- 加 `desk/doc.go`：每个文件一行，写它管什么。
- 「打开设置」改成按错误的类型来判断（`aiport.SetupError`，用 `errors.As`），不再找错误文字里的子串。
- 删掉只复述代码的注释，改正两处已经过时的契约说明：`Work.Writing` 和 `Store.Open`。
- 删掉 `read_summary` 的兼容分支和它的 `Chapters` 参数。
- `cmd/poc` 改名为 `cmd/inkgo`：
  - 删掉 `-editor` 演示和只给它用的 `DemoPredictor`、`MarkKind.String`；
  - 窗口标题不再写 PoC；
  - `AGENTS.md`、`DESIGN.md`、`scripts/` 里的路径跟着改。

### 第 4 批：把规则搬出 `desk`

大约 10 次小提交，每次搬一块，测试跟着搬：

| 现在 | 搬到 |
| --- | --- |
| `diff.go` 的差异比较 | `internal/textdiff`，`desk` 调用 `Compare`、`Paragraphs` |
| `numbering.go` 的章节编号 | `Tree.TitleAfter`；编号前缀由 `workspace.TitleNumber` 读取，`nextTitle` 仅供包内使用 |
| 提议在正文里的定位（`prop.place`、`find`） | `workspace.Proposal.Place`（`workspace/placement.go`）；`editor` 不依赖作品存取层 |
| `desk/exportdialog/export.go` 的导出范围计算 | `export.Plan`（`export/plan.go`）；打开、内容格式或范围选择、当前章节变化及设定刷新时计算 |
| 备份的到期、重试、再问策略，`trouble`，`volumeGone`；`meta` 里写死的路径 | `backup.Policy`、`Policy.Trouble`、包内 `volumeGone`（`backup/policy.go`）、`Files.Meta`（`backup/meta.go`）；正文路径由 `workspace.IsProsePath` 判断 |
| `welcome.go` 的书架操作，`sameFolder` | `shelf.Describe`、`Shelf.Add`、`Shelf.RelocateWork`、`Shelf.Remove`、`SameFolder`；作品判断由 `workspace.IsWork` 统一 |
| `prefs.go` 的 `desk.json` 结构、读写和旧格式迁移 | `prefs.Prefs`、`Load`、`Save`；路径由 `workspace.Store.PrefsPath` 提供 |
| `ideas()` 的排序，`reading`/`readingBefore` | `workspace/ideas.go` 的 `Setting.Ideas`；`workspace/reading.go` 的 `Entry.Reading`、`ReadingBefore` |
| 伏笔模型和候选 | `workspace/foreshadow.go` 的伏笔和回收模型、状态、候选及逾期判断；界面传入即时标记和偏好阈值，保管文案 |
| 检查的队列、`keepCheck`、`pruneCheck` | `workspace/checkqueue.go` 的 `CheckQueue`；`workspace/check.go` 的 `Check.Prune`、`KeepMeant`、`Stale`；`desk/checkrunner.go` 的 `checkRunner` 执行并交回结果 |
| `chat.go` 的 `mentions`、`sections`、`looks`、`spent`、`requests`、`fateOf` | `workspace/events.go` 的 `Looks`、`Spent`、`Requests`、`FateOf`；`workspace/mentions.go` 的 `Setting.Mentions`。`sections` 第 5 批搬到 `workspace/events.go`，是 `DigestSections` |
| `sessionTitle`/`talkTitle` 两份 | `workspace.TalkTitle`，从事件和章节标题读取 |
| 每日字数和目标（`today()` 在取值时改并存偏好） | `prefs.Prefs` 嵌入 `Counter`；读取纯计算，`desk` 在计数读入后显式换日 |
| 手机预览的分页 | `editor.PaginatedText`（`editor/pagination.go`），`desk` 提供排版参数、保管面板设置并绘制 |

### 第 5 批：AI 层

- `desk/aiport` 挪到顶层 `aiport`，拆成「设置」（服务、key、目录）和「助手」（讨论、笔、检查）两个接口。
- `writing` 和 `discussion` 共用的绑好的模型、`Decode`、`Locate` 放进 `inference`，名字指模型推理的共同规则，两边都引用它，`writing` 不再引用 `discussion`。所在卷的判断放进 `workspace.Node.Volume`：排除树根是章节树的规则，不属于推理。遍历和前章读取共用 `Tree.Walk`、`Tree.Previous`；讨论的章名消歧、预加载范围和各处不同口径的裁剪各自保留。
- 跨包约定改成由拥有它的包导出解析函数：
  - 设定按序号取（`[n]`）：`workspace/mentions.go` 的 `Setting.EntryNumbers`、`Setting.EntryByNumber`、`EntryRef`，讨论写编号和摘要读编号共用；检查自己的答复列表由 `writing/check.go` 编号；
  - 摘要的几节：`workspace/events.go` 的 `DigestSections` 按 `## ` 分段，保留标题前的文字并修剪标题、正文两端；摘要是存下的 `RoleDigest` 事件，读取放在 `workspace`，让 `desk` 无须引用 `discussion`。四个标题由 `discussion` 的提示词要求，分段器不列标题；
  - 「服务/模型」的拆法：`workspace/types.go` 的 `ModelRef` 与 `ParseModelRef` 按第一个 `/` 划分服务和完整模型 ID。`model/pricing.go` 的 `PriceID` 为回复估价取模型 ID 的最后一段，`desk.replyLabel` 为显示取最后一段，存储保留完整名字。
- `Preloaded` 直接给出预加载了哪几项，`desk` 不再自己从章节树推。端口的 `Preload` 给 token 数、本章卡片数、前章是否整章带上，以及纲的来源（各卷或所在卷各章，空纲也保留来源，供对话框说「等」）；`discussion.fixedOf` 边加边记，`ai` 只把 `InVolume` 映射成端口种类。`draftCards` 在 `openDraft` 独立数卡片，预加载失败也能决定标题。`keepPen` 比较 `desk.penKey` 的指针，nil 表示没有可比的笔。
- 讨论工具共用的 16 个字段的参数结构，拆成每个工具一个，`search_book` 和 `search_talks` 共用 `searchArgs`；工具表每项记名字、说明、参数格式、处理函数和标签，`note_setting` 的参数格式由 `noteSchema(w)` 用作品的分类生成。
- AI 开关只存一份。

### 第 6 批：`desk` 的内部状态

这一批单独写了[一篇](../../implemented/architecture/2026-10-22-desk-state.md)，落地后的样子以那篇为准。

- **书模型**：章节树、字数、标记、设定、大纲、检查放在一起，带版本号。
  - 只有它读 `Store`，页面都从它取。
  - 待办分组、右栏角标、人名链接、文件夹字数、检查标注、分类计数，都按版本号缓存在它上面。
  - 检查完成、改一个伏笔，只更新那一章。
  - `write.entered` 不再需要。
- **「打开的章节」**：一个类型管编辑器、保存、看盘、冲突、标记的转换，标签页的 `sheet` 和 `park`/`unpark` 去掉。大纲页和写作页共用这份章纲的保存：停顿后保存，离开、关闭时立刻保存。
- **页面接口**：每个页面有 `enter`、`leave`、`view`、`count`，页码、右栏标签、设置的分页都用同一种注册表。
  - 待办项各自带一个「去那里」的函数，不再按来源分派。
  - 停得下来的 AI 部分向开关登记，`turnAI` 不必记着每一块。
- **一张命令表**：快捷键、菜单、快捷键帮助都从它出。
- **统一的写法**：一个 `status` 管错误、提示、通知，一个 `jobs` 管后台任务和送回界面。
- **右栏设定面板**：几个互相打架的标志改成一个 `settingRoute`。
- **AI 记录页**：改用 `ui.List`，只画看得见的行。
- 手机预览、导出对话框、大纲页各有自己的结构体，方法挂在它上面，经由一个小接口取 `App` 的东西：配色、书、`Store`、状态、存偏好、打开章节、换页。第 7 批把它们搬进各自的包，接口也分成每个包一个。

### 第 7 批：拆子包（只拆这几个）

- `desk/kit` 只画通用零件，不引用 `desk` 的其他包：
  - `theme.go` 和 `tokens.css`：调色板、尺寸、图标、按钮、对话框、空状态和表单小控件。CSS 由同目录的 `go:embed` 读取。
  - `pane.go`：通用栏顶、列表、详情、表单、引文和搜索命中着色。
  - `status.go`：`Status` 持有错误、连续失败、重试、完成通知和提示。
  - `when.go`：日期和时间的显示文字。
  - `text.go`：`Trim`、`Clip`、`Words`、`Grouped`、`Signed`、`Heading`、`FromLabel`、`Count`、`NearFind`。
  - `menu.go`：`MenuState` 持有菜单和子菜单状态，按传入的调色板绘制；`Menu` 列出行，`MenuItem` 保存行的内容和动作。
  - `toast.go`：提示卡片、动作行、展开更多的按钮和标题拆分。卡片接收状态、显示时长和绘制动作的回调。
  - `markdown.go`：`MdStyle`、`MarkdownState`、主题和代码块。`MarkdownOptions` 保持选项地址，只在调色板改变时重建主题；代码块读取这次取选项时的调色板。
  - `App` 持有调色板、状态、菜单状态和 Markdown 状态。业务菜单与命令、提示种类与动作、摘要内容与设定链接在 `desk`；`appHost.DropMenu` 把宿主的菜单状态和配色交给 `kit`。
  - 依赖是标准库、mygo 的 `ui` 和 `ui/markdown`、`workspace`、`internal/textclip`。文字裁剪复用已有实现。
- 手机预览、导出 2026-11-03 起是扩展，从 `desk/phone`、`desk/exportdialog`（和 `export/`）搬到 `ext/phone`、`ext/export`（格式库在 `ext/export/format`），各自的 `Host` 换成 `ext/hostapi` 的服务，见[扩展 API](2026-11-03-extension-api.md)。备份同样搬到 `ext/backup`。
- `desk/outline` 的 `Page` 持有大纲页：索引、布局缓存、章纲草稿、卡片和情节线的编辑、搜索、滚动和拖动；写作页章纲旁的「故事中」按钮也由它画（`ChapterStory`）。`Host` 在本包声明。
- 大纲包在 `desk` 下只引用 `kit` 和 `desk/book`，别的都在 `desk` 以外。`desk` 不再有自己的 `host` 接口，守着它的 `host_guard_test.go` 也去掉了：`desk` 下的包引用不了 `desk`，`App` 由编译器挡在外面。
- `desk/book/outline_draft.go` 的 `OutlineDraft` 是写作页和大纲页共用的草稿；`desk/book/timing.go` 的 `SaveAfter` 和 `CheckEvery` 是保存停顿和看盘间隔，均为变量。
- `desk/host.go` 的 `appHost` 显式持有 `*App`，用首字母大写的方法实现页面需要的接口；`App` 不增加导出方法。三个页面在 `New` 绑定适配器，大纲和导出在 `resetView` 重新绑定，手机缓存跨作品保留。
- `kit/theme_test.go` 的源码检查递归读取 `desk` 下的全部 Go 包；`scripts/verify-ui.ts` 递归发现这些包，与 `editor`、`cmd/inkgo` 一起运行工具包检查（2026-11-02 起不再列包，在模块根上跑一次）。
- `desk/imports_test.go` 的 `TestImports` 用一次 `go list` 读出 `desk` 下每个包经由任何包的全部依赖：都不许到 `ai`、`model`、`discussion`、`writing`；`desk/book` 不许到 `desk` 下的包、mygo 和 `editor`；`kit` 不许到 `desk` 下的包；其余的包算页面，在 `desk` 下只许到 `kit` 和 `desk/book`。以后在 `desk` 下加的包不用写进测试，照页面的规矩查。原来 `aiswitch_test.go` 的依赖检查和 `desk/book` 只看直接引用的 `TestImports` 并进了这一个。

写作页、讨论、右栏、伏笔、待办、提议这一团共用打开的章节和讨论状态，留在 `desk`；硬拆的话，它们要的那个接口会有四十多个方法，等于把 `App` 换了个名字。

### 会改行为的

| 批次 | 改动 | 作者看得到的 |
| --- | --- | --- |
| 2 | 备份时读全书改到后台做 | 界面不卡；按钮照旧显示「正在备份…」，关窗口时照旧等它做完 |
| 2 | 书架和 `desk.json` 写入加 `Sync`，临时文件不重名 | 看不出区别 |
| 2 | `works.json` 和检查结果的文件，JSON 后面多出内容时报错 | 手改坏的文件会提示读不了，不再悄悄忽略多出的部分 |
| 2 | 某章的标记或纲文件坏了，不再挡搜全书和只读纲 | 讨论照常搜到正文、读到别处的纲 |
| 3 | 删掉 `read_summary` 的兼容分支 | 旧讨论里那条记录照常显示（存着的标签）；模型再调它，和调别的没有的工具一样报错 |
| 3 | 关窗口时也停掉正在跑的检查 | 看不出区别：检查的结果只在界面收下时才存，窗口关了本来就存不下 |
| 3 | 「打开设置」按错误的类型判断 | 超时、限流、服务端出错、回复坏了，提示里不再带「打开设置」；服务、模型、key、上下文上限的问题照旧带 |
| 3 | 入口改名，去掉 `-editor` | 启动命令变成 `go run ./cmd/inkgo` |
| 4 | 新卡片和情节线的 ID 撞上大纲里已有的就重新取 | 生成示例书不再偶尔报「cards: invalid or duplicate id/title」 |
| 4 | `desk.json` 里 `days` 写在 `carets` 前面 | 看不出区别：键和值不变 |
| 4 | 新开的起草、改写讨论，标题里的章名和讨论列表一样取 | 章节已从磁盘上丢失时起草，新讨论一开始就叫「起草：已删除的章」，不再先显示原来的章名 |
| 4 | 导出范围不再提供时，马上回到「全书」 | 看不出区别：只勾设定时，范围那一行不显示 |
| 5 | 起草对话框的预加载提示，由讨论按实际带上的内容给出 | 章节刚在软件外面改过、界面还没刷新（两秒内），或上一章的文件读不出来时，提示说的是讨论实际带上的内容，不再说带上了上一章全文；别的情况措辞不变 |
| 5 | 讨论工具各用自己的参数类型 | 看不出区别：模型给一个工具带上别的工具的参数时，和带上不认识的参数一样忽略，不再报参数错误；调用不存在的工具时，总是报「没有叫……的工具」 |
| 6 | 大纲页的章纲停顿后保存 | 和写作页一样：停笔片刻就存，离开大纲页时立刻存 |
| 6 | 「停笔后自动续写」改到已经打开的这一章 | 勾上或取消，当前这章立即按新方式续写，不再等切回来 |
| 6 | 字号、续写方式、AI 开关设给每个打开的编辑器 | 看不出区别：以前是切回标签时补设 |
| 6 | 从人名链接或讨论跳到设定条目时，一并关掉变化的表单 | 正在给某条新增变化时点人名跳到别的条目，显示那条的页面，不再显示它的「新增变化」表单 |
| 6 | AI 记录只建看得见的行 | 「‹ 本作品用量」和筛选那一行留在上方，记录在下面滚动 |
| 6 | 写作页和检查页共用一棵章节树 | 只在软件外面改了章节、又开着检查页时看得出：检查项按检查页刚读的树显示；回到写作页，两秒内显示的也是这棵树 |
| 6 | 章名和跳到正文的目标只在读进来的树里找 | 章节刚在软件外面加上、两秒内还没重读时，它暂时没有标题，跳过去也找不到 |
| 6 | 打开「提醒与检查」不再重读章节树和标记 | 软件外面改了章节或标记，只开这一页看不到；进写作页、换章或进检查页时就有 |
| 6 | 讨论记下的设定变化每条刷新一次 | 看不出区别：同一回合的几条以前并成一次刷新，现在在同一帧里各刷新一次 |

第 7 批没有改行为的地方。

### 不在这次里的

- 每 2 秒看一次磁盘。它有自己的提案[少看几次磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)，作者说先放着。第 6 批的书模型是它以后落地的地方，这次没改看盘的频率和做法。
- 文件格式：`works.json`、`work.yaml`、`desk.json`、`providers.yaml` 的内容都不变。只有第 4 批把 `desk.json` 的读写搬进 `prefs` 包。
- 把 `desk` 整个拆成子包。

### 和其他笔记的关系

- [AI 从核心里拆出去](../../implemented/architecture/2026-10-14-ai-out-of-the-core.md)：
  - 第 5 批挪了 `aiport` 的位置。那篇的理由（接口单独成包，`desk` 的测试才能用真的 `ai`）不变，路径和接口清单已跟着改。
  - 那篇没要单独的零件包（当时叫 `desk/look`），是因为界面不拆。第 7 批要了它（`desk/kit`），是为了让几个独立的页面拆出去，理由不同，两篇互链。
- [纯 Go 的编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md)：第 1 批改了排版缓存，第 3 批删了演示，那篇已就地更新。
- [手机预览](../../implemented/feature/2026-10-21-phone-preview.md)：第 1 批让它和编辑器共用排版，第 4 批搬走分页，第 7 批搬进 `desk/phone`，那篇已就地更新。
- [导出成 Word](../../implemented/feature/2026-10-16-export-to-word.md)、[大纲页](../../implemented/feature/2026-10-19-outline-cards-and-timeline.md)：第 7 批搬进 `desk/exportdialog`、`desk/outline`，两篇已就地更新。
- [`desk` 的内部状态](../../implemented/architecture/2026-10-22-desk-state.md)：第 6 批的提案和落地。
- [少看几次磁盘](../../implemented/architecture/2026-10-13-fewer-looks-at-the-disk.md)：只是部分重叠，见上面「不在这次里的」。

### 进度

| 批次 | 状态 |
| --- | --- |
| 1 编辑器 | 2026-10-21 合入 |
| 2 存取层和共用小工具 | 2026-10-21 合入 |
| 3 整理 | 2026-10-22 合入 |
| 4 把规则搬出 `desk` | 2026-10-22 合入 |
| 5 AI 层 | 2026-10-22 合入 |
| 6 `desk` 的内部状态 | 2026-10-23 合入，见[那篇](../../implemented/architecture/2026-10-22-desk-state.md)；作者交代不等审，提案和结果一起看 |
| 7 拆子包 | 2026-10-24 合入；作者交代不等审，做完一起看 |

作者 2026-10-24 看完 7 批，提出的问题和改动见[那篇](../bug-fix/2026-10-24-after-reviewing-the-reorganization.md)。

## Alternatives considered

- **一次把 `desk` 拆成十来个子包**：
  - 最强的理由：边界由编译器守着，谁也越不过去，拆完一眼看得出谁依赖谁。
  - 不选：写作页那一团共用打开的章节、讨论状态和右栏，硬拆要一个四十多个方法的接口，只是把 `App` 换了个名字；一次挪六七千行也没法审。所以先把状态理顺（第 6 批），再只拆真正独立的几块（第 7 批）。
- **每处每帧重算的地方各自加缓存**：
  - 最强的理由：改动小，每处都马上见效，能和别的批次并行。
  - 不选：七八份缓存各管各的失效，正是现在乱的原因之一。书模型带版本号，缓存放在一处，失效也只在一处。编辑器内部和存取层的那几处不依赖书模型，所以放在第 1、2 批先做。
- **不挪文件，只加注释和文档讲清楚**：
  - 最强的理由：零风险，`git blame` 不断。
  - 不选：项目的规矩是不写读了也不改做法的说明；名字对不上内容，加注释只是多一层要读的东西。
- **`aiport` 留在 `desk/` 下面**：
  - 最强的理由：接口跟着用它的一方走，是 Go 常见的做法，少一次搬家。
  - 不选：`ai` 引用 `desk/aiport`，读起来像 AI 依赖界面；挪到顶层同样不成环，2026-10-14 那篇的理由照样成立。
- **全部交给一个 Codex 任务**：
  - 最强的理由：一次做完，中间不用等。
  - 不选：几万行的 diff 没人能审，作者要求做完一批审一批。
- **入口保留 `cmd/poc`**：
  - 最强的理由：启动命令和文档都不用改。
  - 不选：作者 2026-10-21 同意改名；「这是不是正式程序」从名字上就该看得出来。

## Consequences

- `desk` 下的边界由编译器和 `TestImports` 守着：页面包碰不到 `App`，`kit` 和 `desk/book` 碰不到 `desk`，谁都碰不到会发送的包。审一个页面的改动，看它的包和它的 `Host` 就够。
- 书的数据有了主人：只有 `desk/book` 读 `Store`，派生的数据按版本号缓存在一处，不变的帧不重算（第 6 批）。
- 规则搬出了 `desk`（第 4 批），在各自的包里测试，不用建 `App`。
- 代价：
  - 搬家处 `git blame` 断开。只挪的提交是单独的，提交说明写了从哪搬到哪，`git log --follow` 接得上。
  - 测试改动很大。读字段的地方改成经由 helper 或页面导出的读方法（`Panel.Preferences`、`Dialog.Plan`、`Dialog.Selection`）；页面的测试搬进页面包，用一个小的测试宿主驱动真的页面，`desk` 里留下导航、落盘、刷新和页面之间的契约。
  - 页面包多一层：每个包一个 `Host` 接口，`desk` 里一个 `appHost` 实现它们。
  - `TestOutlineFrameCost` 搬进 `desk/outline` 后只量大纲页本身，不再连窗口别处一起量；数字没改。
  - 第 2 批起作者没有逐批审，第 6 批的提案也没有先审，都和结果一起看。
- 增量校验日志保住了严格校验和「尾巴不完整时恢复」，外部改掉、截短、换掉的情形补了测试（见下表第 2 批）。

## Testing

| 验收项 | 守着它的测试，或「无」和怎么查的 |
| --- | --- |
| 每批 `npm run verify` 全过 | `npm run verify`。每批合入前在检查用的工作树里跑过，每个提交也单独构建、`go vet`、跑 `desk` 下的测试 |
| 布局截图前后逐张对比 | 无。每批合入前用 `SHOT=<目录>` 跑 `desk`、`editor`、`cmd/inkgo` 的截图测试，和上一批比，超出噪声的逐张看过 |
| 只挪的提交里没有逻辑改动 | 无。审查时按声明比较搬家前后，去掉首字母大小写、包名前缀和接收者名后只差名字 |
| 「进度」和相关笔记同一次提交更新 | 无。`npm run verify` 的三项笔记检查只管格式和链接，内容由审查看 |
| 第 1 批：一段 1.2 万字时一帧、移光标的耗时 | `editor/bench_test.go` 的 `BenchmarkFrameLongPara`、`BenchmarkLayoutAfterCaretMove`、`BenchmarkLayoutNoChange`；前后数字在第 1 批的交付里，基准测试不设上限 |
| 第 1 批：改标记以后不再重新排版 | `editor/doc_test.go` 的 `TestMarkChangesKeepLayout` |
| 第 2 批：讨论搜全书批量读正文 | `workspace/reads_test.go` 的 `TestTextsAreWhatReadingEachGives`；不随章数重建章节树没有测试，审查时看的调用 |
| 第 2 批：追加一条 AI 记录只读新增的一截 | `workspace/record_test.go` 的 `TestRecordCursorExternalChanges`、`TestRecordCursorRevalidatesAfterFailedTail`，`workspace/workspace_test.go` 的 `TestSessionCursorExternalChanges`；耗时本身没有测试 |
| 第 2 批：写文件只剩一份实现 | `internal/atomicfile` 的测试；只剩一份是合入时 `rg` 查的。导出写到作者选的文件夹那一处（`desk/exportdialog/export.go`）不在当时数的 5 份里，作者看完以后也改用 `atomicfile.Write`，见[那篇](../bug-fix/2026-10-24-after-reviewing-the-reorganization.md) |
| 第 3 批：没有 `a.page == <数字>`，`cmd/poc` 不在，`workspace/chapters.go` 拆完 | 无。合入时 `rg` 和 `ls` 查的；页码是有名字的类型常量 |
| 第 3 批：`desk/doc.go` 列出每个文件 | 无。第 7 批后对着目录重新核过一遍 |
| 第 4 批：上表的函数不在 `desk` 里，各有不用建 `App` 的单元测试 | 各目标包的测试（`internal/textdiff`、`export`、`backup`、`shelf`、`prefs`、`workspace`、`editor`）；不在 `desk` 里是合入时 `rg` 查的 |
| 第 5 批：`ai` 不依赖 `desk`，`writing` 不引用 `discussion` | `ai/imports_test.go`、`writing/imports_test.go` 的 `TestImports`，作者看完以后加的，见[那篇](../bug-fix/2026-10-24-after-reviewing-the-reorganization.md)；合入时是 `go list -deps` 查的。`desk` 这边不依赖 `ai` 由 `desk/imports_test.go` 守着 |
| 第 5 批：跨包约定有导出的解析函数 | `workspace/mentions_test.go` 的 `TestEntryNumbers`、`workspace/events_test.go` 的 `TestDigestSectionsWhitespace`、`workspace/types_test.go` 的 `TestModelRef` |
| 第 6 批：只有书模型调用 `Store` 的读接口 | `desk/book_guard_test.go` 的 `TestDeskLeavesTheBooksContentToBook` |
| 第 6 批：`sheet`、`write.entered` 不在 | 无。合入时 `rg` 查的 |
| 第 6 批：不变的帧不重建派生数据 | `desk/frames_test.go` 的 `TestUnchangedFramesBuildNoCache` |
| 第 7 批：子包通过依赖检查 | `desk/imports_test.go` 的 `TestImports`。合入前故意让 `kit` 引用 `desk/book`、大纲页引用 `desk/phone`、`desk/book` 引用 `editor`、手机预览引用 `writing`，四条都报了出来 |
| 文件格式和模型请求字节不变 | 现有的格式测试，和 `TestTalkRequestBytes`、`TestDigestRequestBytes`、`TestCheckRequestBytes`、`TestPredictionRequestBytes` |
