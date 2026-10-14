# Agent Note: 把 AI 从核心里拆出去：核心包不引用模型、讨论、写作，也不联网

Status: proposed

## Problem

[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)要求关着 AI 时界面上看不见任何 AI 功能，也不发送任何内容。作者答「整个不显示」那一题时补了一句「因此，架构设计要做好」；问到架构怎么保证关得干净，选了拆成单独的包：「C 好一点，B还是会有一点workaround和review的复杂度考量」。

现在 AI 和别的部分在同一个包 `desk` 里，分不开：

- `desk` 直接引用 `model`、`discussion`、`writing`，还有渲染讨论回复用的 `goldmark`。联网的只有 `model`（[模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)），它经 `desk` 进了整个程序。
- AI 的界面在 7 个文件里（`ai.go`、`chat.go`、`check.go`、`summary.go`、`beside.go`、`markdown.go`、`settings.go`，约 3,500 行），都是 `*App` 的方法，和别的页面共用一个 `App`。`App` 有 91 个字段：15 个只有 AI 的文件用，37 个两边都用。
- 核心有 12 个文件直接调 AI 文件里的函数（下面的清单）；AI 的文件也反过来直接读写核心的状态：调色板 `a.pal` 128 处，正文页 `a.write` 51 处，作品 `a.Store` 44 处。
- 模型客户端在 4 处新建：`desk/ai.go:30`、`desk/app.go:352`、`desk/app.go:376`、`desk/settings.go:432`。

在这上面直接加开关，只能在每个入口加一句「AI 开着才显示」：漏一处就露一处，也没有东西能查出漏在哪。

## Proposal

### 包怎么分

| 包 | 放什么 | 引用 |
| --- | --- | --- |
| `desk/look`（新） | 画界面的零件：调色板和数值、图标、按钮、标签、分组、菜单、对话框的头、空白页、右栏的行、拖动条 | 只有 mygo 的 `ui` |
| `desk`（核心） | `App`；书架、正文页、章节树、标签页、设定、伏笔、作品分类、名字表、快照、备份、导出；设置里的外观、提醒、备份和 AI 开关 | `desk/look`、`workspace`、`editor`、`backup`、`shelf` |
| `desk/ai`（新） | 讨论、检查、摘要、续写、改写、起草、设置里的模型服务、Markdown 渲染 | `desk`、`desk/look`、`model`、`discussion`、`writing`，加上核心引用的那几个 |
| `cmd/poc` | 把两边接起来 | `desk`、`desk/ai` |

- 接口由核心定：`desk` 里声明 `AI`（核心问 AI 要的）和 `Host`（AI 问核心要的）。`desk/ai` 实现 `AI`，只通过 `Host` 碰核心。核心不知道 `desk/ai` 存在。
- `cmd/poc` 把造 AI 的函数交给核心（`d.MakeAI = ai.New`）。AI 开着时核心用它造一份，关着时这一份是 nil；关掉时先 `Stop()` 再扔掉，打开时重新造。
- 开关存在 `works.json`，核心自己读（见那一篇的「开关存在哪」）。核心现在靠 `model.ConfigPath()` 找本机放设置的文件夹，以后自己找（还是 `os.UserConfigDir()` 下的 `writer`，位置不变）；`providers.yaml` 由 AI 自己读。

### 接口清单

#### 核心问 AI 要的（`desk.AI`）

| 地方 | 现在核心调的（在哪个文件） | 以后 |
| --- | --- | --- |
| 打开、关上作品 | `stopSummaries`、`stopChecks`、`halt`、`kick`，后台整理的 `worker`，`loadWork` 里各用哪个模型（`app.go`） | `Open(store)`、`Close()` |
| 关掉 AI | 没有 | `Stop()`：停讨论回复、起草、续写、改写，扔掉后台任务 |
| 每一帧 | `keepPen`、`pollAI`、`checkWhilePausing`、`markDoubts`（`write.go`），`pollSummaries`、`pollChecks`（`shell.go`） | `Frame(c)` |
| 模型在忙什么 | 读 `a.busy`，把几个按钮变灰（`categories.go`、`setting.go`、`shell.go`、`toast.go`） | `Busy()` |
| 侧栏的页 | 讨论页、检查页，`switchSession`、`leaveCheck`、`loadChecks`、`pruneChecks`、`checkItems`（`shell.go`） | `Pages()`：名字、图标、怎么画、离开时做什么 |
| 正文的右键菜单 | 续写一句、AI 改写…、引用到讨论（`write.go`） | `ProseMenu(m, 选区)` |
| 编辑菜单 | 续写一句、起草本章（`titlebar.go`） | `EditMenu(m)` |
| 工具栏、选中后的按钮 | `openRewrite`（`toolbar.go`） | 并进 `ProseMenu` 那一组 |
| 章节的摘要框 | `summaryPanel`、`summaryShown`、`summaryState`、`loadSummary`、`saveSummary`（`write.go`） | `ChapterPanels()` |
| 正文页的底栏、起草 | `footBar`、`aiBar`、`draftView`（`write.go`） | `Foot(c)`、`Drafting(c)` |
| 灰字续写 | `pen`、`keepPen`（`write.go`、`titlebar.go`） | `Predictor()` 交给编辑器；关着时编辑器没有它 |
| 换标签页、全书替换之前 | `stopAI`（`tabs.go`、`findbook.go`），`leaveChapter`（`write.go`） | `LeaveChapter()` |
| 快照里 AI 那一次的记录 | `aiRecord`（`write.go`） | `SnapshotRecord(c, 记录)` |
| 右栏 | 讨论标签 `talkPanel`、`quoteToTalk`（`write.go`），设定的「查看原文」，修改记录的来源 `modelLabel`、`thought`（`setting.go`） | `PaneTabs()`、`Quote(文字)`、`Source(记录)` |
| 待处理 | `checkPending`、`openCheck`（`pending.go`） | `Pending()` |
| 设置 | 模型那一节 `modelsView`（`prefs.go`），提醒里检查的那一半 `whenRows`（`foreshadow.go`） | `Settings(c)`、`Reminders(c)` |
| 快捷键 | Ctrl+J、Ctrl+L、讨论里 Ctrl+Enter 那几行（`keys.go`） | `Keys()` |

一共二十来个方法，第二步搬的时候能并的再并。

#### AI 问核心要的（`desk.Host`）

- 作品：`Store`、设定文档、正在看的那一章（章节树里的位置、编辑器）、存盘（`flush`、`saveText`）、字数、打开某一章、进正文页。
- 提示：`notice`、`warn`、`setError`，让某一行闪一下（`flash`）。
- 界面：现在在哪一页和换页；右栏（宽度、拖动、开着哪个标签、收没收起）；打开设置的某一节（讨论里的「模型设置…」、提示条的「打开设置」）；叫界面重画。
- 作品的偏好：`.writer/desk.json` 里 AI 的那几项，见下面。
- 画界面用 `desk/look`，不经 `Host`。

#### 两边都用、要分家的

- **作品的偏好 `.writer/desk.json`**：一个文件里两家的东西。AI 的是 `auto_predict`、`no_auto_summary`、`check_when`、`ignored`、`talk_tab`、`no_attach`、`shelved_sessions`，其余是核心的。文件的样子不变：核心读写整个文件，自己不认识的项原样留着，AI 关着时存一次也不会把 AI 的几项冲掉；AI 通过 `Host` 读写自己那几项。
- **作品各用哪个模型**（`work.yaml` 里讨论、整理、写作的模型和 `TokenBudget`）：现在核心的 `loadWork` 读进 `App`，以后 AI 在 `Open` 里自己从 `Store` 读。
- **讨论的状态**（`busy`、`talking`、`cancel`、`live`、`input`、`source`、`session`、`sessions`、`events` 等）：核心只在 `resetView` 里清空它们、在几处读 `busy`；以后都归 AI，核心问 `Busy()`。
- **正文页的状态 `writeView`**：摘要（`sum`、`summary*`、`autoSummary`、`gists`、`unsummarized`、`recapStale`）、续写（`autoPredict`、`penKey`、`run`）、检查（`checked`、`ignored`）这几组搬进 AI；章节树、编辑器、标签页留在核心。
- **只给 AI 用的 15 个字段**（模型服务、key、讨论的输入框、引用、Markdown 缓存、思考过程的开合等），和测试换假模型用的 `Book`、`Pen`，整个搬进 AI。
- **页的编号**：现在是数字（0 讨论、2 作品分类、3 正文、5 检查），AI 的页由 `Pages()` 给出以后，改成按名字认。

#### 核心自己看开关的地方

AI 留在作品里的东西由核心画，关着时核心自己不画：设定条目的「查看原文」和修改记录的来源、后台整理的那几条修改记录、「待确认」，快照的「AI 改动前」和 AI 起草的那张卡，正文里 AI 起草的下划线（编辑器加一个不画这类标记的选项）。判断只有一个：`a.ai == nil`。

### 三道检查（第三步加，都在 `npm run verify` 里）

1. **核心不引用 AI，也不联网**：`go test ./desk` 里查 `writer/desk` 的全部依赖（`go list -deps`），不能有 `writer/model`、`writer/discussion`、`writer/writing`、`writer/desk/ai`、`net`、`net/http`。核心留下的那几个包（`workspace`、`editor`、`backup`、`shelf`、mygo 的 `ui`）都不引用 `net/http`，查过了。
2. **关着 AI 走一遍**：AI 关着，用测试的窗口把每一页、每个菜单、右键菜单、快捷键表、待处理、快照、设置走一遍，`tt.Texts()` 里除了正文没有和 AI 有关的字；造 AI 的函数一次也没被叫过，所以也没建过模型客户端。
3. **开着照常**：搬到 `desk/ai` 的测试不改断言照常过；再加开、关各一遍的测试（问句、取消、关的那一刻停下的东西、关了再开文件一个没变）。

### 第二步（搬家）怎么验

只搬，不改行为：`desk/shot_test.go` 和 `cmd/poc` 的布局测试在搬之前、搬之后各截一套图，逐像素比，一张不差；测试不改断言；`npm run verify` 全绿。

## 待作者确认

1. **零件放进 `desk/look`，还是直接从 `desk` 导出**：AI 的包要用调色板和零件，它们得导出。我倾向单独一个 `desk/look`：零件和 `App` 无关、两边都用，放进去以后 `desk` 导出的只有 `App` 和接口。代价是 `desk/tokens.css` 跟着搬到 `desk/look/`（`go:embed` 不能往上一层找文件），原型引用它的路径、`DESIGN.md` 的「数值只写名字」、`AGENTS.md` 里的路径和几个查数值的测试都要跟着改，同一次提交。
2. **接口清单本身**：哪一处放错了边，哪一处漏了。界限一旦划定，第二步搬完就很难再改。

## Alternatives considered

- **A：每个入口加一句「AI 开着才显示」** — 改得最少，今天就能做完，代码留在原地，读的时候不用跨包。没有选：漏一处就露一处，也没有东西能查出漏在哪；「不发送」也只能靠每一处都记得。
- **B：同一个包里，把 AI 的状态和界面收进一个对象，外面只在固定几个接入点问它，关着时这个对象不存在，再配三道检查** — 我原来推荐这个：不用导出，搬动比拆包小；AI 的代码集中、只经几个接入点进来以后，再走到拆包只剩机械搬家。没有选：作者说「C 好一点，B还是会有一点workaround和review的复杂度考量」——同一个包里什么都看得见，「只经接入点」只能靠审查守着，编译器不管。
- **零件直接从 `desk` 导出，不另建 `desk/look`** — 少一个包，`tokens.css` 和查数值的测试都不用挪。暂不选（见「待作者确认」第 1 条）：`desk` 导出的名字会多出八十来个零件，和 `App`、接口混在一起，而零件本来就不依赖 `App`。
- **用编译标签把 AI 编进或编出，不拆包** — 同样由编译器保证，搬动最少。没有选：开关要在运行时改、马上生效，编译标签做不到；编出两种程序又是作者否掉的「做两套」（[关掉 AI](../feature/2026-10-14-disable-ai.md) 的 Q15）。
- **AI 做成单独的进程或插件，核心通过管道和它说话** — 隔得最干净，核心的进程里连 AI 的代码都没有。没有选：Go 的 `plugin` 在 Windows 上不能用；另起进程要管启动、崩溃，界面还得跨进程传，比一个包贵太多，而包的界限已经由编译器和检查守着。

## Acceptance criteria

- `go list -deps writer/desk` 里没有 `writer/model`、`writer/discussion`、`writer/writing`、`writer/desk/ai`、`net`、`net/http`，有测试守着，在 `npm run verify` 里。
- `model.NewClient` 只在 `desk/ai` 里出现。
- 第二步搬完，截图和搬之前逐像素一样，测试不改断言照过。
- 第三步：关着 AI 时造 AI 的函数一次也没被叫过；走一遍查不到 AI 的字；关了再开，作品文件夹里的文件一个没变。
- `desk` 的测试在没有 AI 的情况下跑（它们引用不了 `desk/ai`），用到 AI 的测试都在 `desk/ai` 里。
- 这一篇和[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)落地时一起转 `implemented/`。

## Risks

- **一次改动很大**：约 3,500 行 AI 代码和二十来个测试换包；零件导出以后，四百来处 `a.pal.ink` 这样的写法要改成导出的名字。都是机械搬动，但审起来量大，靠截图逐像素比和测试不改断言兜着。
- **接口定宽了等于没拆，定窄了方法太碎**：`Host` 给整块的东西（`Store`、正在看的那一章），不是一个字段一个方法；`AI` 按地方分，一处一个方法。
- **`desk` 的测试碰不到 AI**：Go 不许 `desk` 的测试引用 `desk/ai`（会成环），两边一起的测试只能放在 `desk/ai` 或 `cmd/poc`。
- **mygo vet**：接口的方法每次拿当帧的 `c`；`AI` 的实现不能把 `*ui.Context`、`ui.Element` 存进结构体，和现在的要求一样。
- **偏好文件两家写**：核心存的时候要留着不认识的项，否则 AI 关着时存一次就把 AI 的设置冲掉；要有测试守着。
