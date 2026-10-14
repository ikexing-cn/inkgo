# Agent Note: 把会发送的部分从界面里拆出去：desk 不引用模型、讨论、写作三个包

Status: proposed

## Problem

[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)要求关着 AI 时界面上看不见任何 AI 功能，也不发送任何内容。作者答「整个不显示」那一题时补了一句「因此，架构设计要做好」；问到架构怎么保证关得干净，选了拆成单独的包：「C 好一点，B还是会有一点workaround和review的复杂度考量」。

现在会发送的东西和界面混在同一个包 `desk` 里：

- `desk` 直接引用 `model`、`discussion`、`writing`。联网的只有 `model`（[模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)），讨论和写作两个包拿到 `model` 的客户端才会发送；客户端在 `desk` 里 4 处新建：`desk/ai.go:30`、`desk/app.go:352`、`desk/app.go:376`、`desk/settings.go:432`。
- 读写模型服务的配置（`providers.yaml`）、建讨论引擎、开后台整理、建续写改写用的 `writing.Writer`，都是 `*App` 的方法，夹在画界面的代码中间。

## Proposal

### 审过的方向

第一稿要把讨论、检查、摘要这些界面也一起搬进 `desk/ai`，核心和 AI 之间定二十来个接口方法。作者审过以后说：「画界面的还是同一套啊，只是我们可以切换而已，整体没有区别，不要维护两套逻辑」「关掉后，禁用就好了，同样开启就是启用，不要那么极端」，问清楚以后选了「只拆会发送的部分」：

- 界面代码不搬：讨论、检查、摘要的界面也留在 `desk`，用同一套零件，开关决定显示与否。
- 会发送的部分搬进单独的包：读写模型服务的配置、建模型客户端、讨论、后台整理、续写、改写、起草、摘要、检查。
- 这个包一直都在：关掉 AI 就是把它停用，停用时不建客户端；打开就是启用。
- 不加「核心不许联网」：开关管的是 AI，不是联网。

### 包怎么分

| 包 | 放什么 | 引用 |
| --- | --- | --- |
| `desk` | 全部界面，和现在一样 | `desk/aiport`、`workspace`、`editor`、`backup`、`shelf` |
| `desk/aiport`（新） | `desk` 要 AI 做的事：几个接口，和过界要用的两三个小结构 | 只有 `workspace` |
| `ai`（新） | 实现 `aiport`：读写 `providers.yaml`、建客户端、讨论、后台整理、续写改写起草、摘要、检查 | `desk/aiport`、`model`、`discussion`、`writing`、`workspace` |
| `cmd/poc` | 建一个 `ai` 交给 `desk` | `desk`、`ai` |

`aiport` 单独成包，是因为 `desk` 的测试要用真的 `ai`（配假的模型）：接口要是放在 `desk` 里，`ai` 就得引用 `desk`，`desk` 的测试再引用 `ai` 就成环了，Go 不让。

### 接口清单（`aiport.Service`）

| 用处 | 方法 | 现在 `desk` 里是 |
| --- | --- | --- |
| 模型服务的设置 | `ConfigFile`、`Providers`、`Reload`、`SaveProvider`、`SetKey`、`Catalog`（给出能 `ListModels` 的对象） | `a.Config`、`a.ConfigFile`，`model.Load`、`Save`、`SetKey`、`NewClient` 加 `ListModels`（`settings.go`、`app.go`） |
| 推理等级的名字 | `Efforts`、`EffortLabel` | `model.Efforts`、`model.Effort(x).Label()`（`chat.go`、`settings.go`、`ai.go`） |
| 讨论 | `Talk(作品, 正文旁, 进度)` 给出能 `Say`、`Retry` 的对象；`Extract` | `a.engine`、`eng.Say`、`eng.Retry`、`eng.Extract`（`app.go`、`chat.go`） |
| 后台整理 | `Bookkeep(作品, 有变化时)` 给出 `Kick`、`Busy`、`Stop`；`Book(作品)` 给出能 `Redo` 的对象 | `a.Book`、`a.bookkeeper`、`discussion.NewWorker`、`startWorker`、`kick`、`halt`、`redo`（`app.go`、`setting.go`） |
| 续写、改写、起草、摘要、检查 | `Pen(作品)` 给出一支笔：`Name`、`Effort`、`Predict`、`Rewrite`、`Draft`、`Summarize`、`Check`；改写和起草给出 `Job` | `a.Pen`、`a.newWriter`、`writing.Writer` 的各个方法（`ai.go`、`summary.go`、`check.go`） |
| 不发送的 | `Unsummarized`、`MinSummary`、`MinCheck` | `writing.Unsummarized`、`writing.MinSummary`、`writing.MinCheck`（`summary.go`、`check.go`、`ai.go`） |
| 开关（第三步） | `SetOn` | 没有 |

`Catalog`、`Talk`、`Book`、`Pen` 都和现在一样分两段：先在界面的线程上按作品现在的设置建好，配置上的错（没选模型、没有 key）马上报；再在后台发送。

过界的小结构四个，`ai` 来回转换：

- `Provider`：和 `model.Provider` 一样的六项，设置页要编辑它。
- `Beside`：和 `discussion.Writing` 一样的六项，正文旁的讨论带上的正文。
- `Scene`：和 `writing.Scene` 一样的三项。
- `Job`：一次改写或起草，`Record`（快照和标记要用）加一个 `Run`。`desk` 现在只用到 `writing.Job` 的 `Record`。

别的都用 `workspace` 的类型、字符串和函数。

### 怎么建、测试怎么换假模型

- `ai.New(配置文件)` 建 `ai.Service`，`desk.New(它)` 建界面；`cmd/poc` 把两个接起来。作品清单 `works.json` 照旧放在配置文件旁边，位置问 `ConfigFile()`。
- 测试现在换 `App` 上的 `Pen`、`Book` 两个钩子，搬到 `ai.Service` 上，叫 `NewWriter`、`NewBookkeeper`，换法不变。`desk` 的测试引用 `ai` 和 `model` 来做假模型，不算 `desk` 的依赖（`go list -deps` 不看测试）。

### 关掉 AI 时（第三步）

- `ai` 停用：`SetOn(false)` 以后，凡是要建客户端的（讨论、后台整理、续写改写起草、摘要、检查、读模型目录）都直接返回「AI 已关闭」，一个客户端也不建；正在跑的停下。
- 界面照原型，用到 AI 的地方看开关，关着就不显示。

### 三道检查（第三步加，都在 `npm run verify` 里）

1. **desk 不引用会发送的包**：`go test ./desk` 里查 `writer/desk` 的全部依赖（`go list -deps`），不能有 `writer/ai`、`writer/model`、`writer/discussion`、`writer/writing`。
2. **关着 AI 走一遍**：AI 关着，用测试的窗口把每一页、每个菜单、右键菜单、快捷键表、待处理、快照、设置走一遍，`tt.Texts()` 里除了正文没有和 AI 有关的字，`ai` 一个客户端也没建过。
3. **开着照常**：现有的测试不改断言照常过；再加开、关各一遍的测试（问句、取消、关的那一刻停下的东西、关了再开文件一个没变）。

### 第二步（搬家）怎么验

只搬，不改行为：`desk/shot_test.go` 和 `cmd/poc` 的布局测试在搬之前、搬之后各截一套图，逐像素比；测试只改换假模型的写法，不改断言；`npm run verify` 全绿。

只有一处极少见的情况会不一样：第一次启动时配置文件还没有、默认配置又存不下（配置目录写不进去）。现在启动时会列出没存下的默认服务，「重新加载配置」碰到同样的错却什么都不换；搬了以后启动和「重新加载配置」走同一条路：读不成就不换，报的错一样。后台整理碰到这种情况本来就是报错不干活。

## Alternatives considered

- **AI 的界面也搬出去（`desk/ai` 放讨论、检查、摘要的界面和逻辑，核心留二十来个接口方法）** — 我第一稿推荐这个：核心里根本没有 AI 的代码，「看不见」也由编译器保证，以后把功能分成「只有 AI 才有」和「两边都能用」时，清单现成。没有选：作者要的是同一套界面靠开关切换，「不要维护两套逻辑」；接口二十来个、约 3,500 行换包、两边共用的状态要一样样分家，作者说「不要那么极端」。「看不见」由走一遍的检查守着。
- **零件单独建一个 `desk/look` 包** — 第一稿为了两个包都能用零件才要它。界面不拆以后用不着了，零件留在 `desk`。
- **核心的依赖里也不许有 `net`、`net/http`** — 第一稿我加的：核心连联网的能力都没有，「不发送」由编译器兜底。没有选：开关管的是 AI，以后核心自己要联网的功能（比如检查更新）不该被它挡住；作者说「不要那么极端」。
- **关着时 AI 那部分不存在（nil），打开时重新造** — 第一稿这么写的：关着时连对象都没有，最干净。没有选：作者说「关掉后，禁用就好了，同样开启就是启用」；一直在、只是停用，开关前后的状态也不用重建。
- **接口放在 `desk` 里，`ai` 引用 `desk`** — 接口跟着用它的一方走，Go 里常见，少一个包。没有选：`desk` 的测试要用真的 `ai` 配假模型，这样就成环了，测试得另写一份替身，等于把 `ai` 的那点胶水在测试里再写一遍。
- **每个入口加一句「AI 开着才显示」，不拆包** — 改得最少。没有选：「不发送」只能靠每一处都记得；作者选了拆包（C）。
- **同一个包里收进一个对象加一道闸（B）** — 不用新包。没有选：作者说「B还是会有一点workaround和review的复杂度考量」。
- **用编译标签编出没有 AI 的程序** — 编译器保证。没有选：开关要在运行时改、马上生效；出两种程序是作者否掉的「做两套」（[关掉 AI](../feature/2026-10-14-disable-ai.md) 的 Q15）。

## Acceptance criteria

- `go list -deps writer/desk` 里没有 `writer/ai`、`writer/model`、`writer/discussion`、`writer/writing`，有测试守着，在 `npm run verify` 里。
- `model.NewClient` 只在 `ai` 里出现。
- 第二步搬完，截图和搬之前逐像素一样，测试不改断言照过。
- 第三步：关着 AI 时 `ai` 一个客户端也没建；走一遍查不到 AI 的字；关了再开，作品文件夹里的文件一个没变。
- 这一篇和[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)落地时一起转 `implemented/`。

## Risks

- **「看不见」靠检查，不靠编译器**：界面里用到 AI 的入口各自看开关，漏一处就露一处；走一遍的检查只认它知道的字和它走到的地方。新做 AI 功能时要把入口和名字加进检查。
- **三个小结构抄了一份**：`aiport.Provider`、`aiport.Beside`、`aiport.Scene` 和 `model`、`discussion`、`writing` 里的长得一样，那边加一项这边要跟着加；`ai` 的转换函数在一处，测试覆盖它。
- **偏好和模型选择照旧由 `desk` 读写**：`work.yaml` 里各用哪个模型、`.writer/desk.json` 都不变，`ai` 只从 `Store` 读模型选择。
