# Agent Note: 把会发送的部分从界面里拆出去：desk 不引用模型、讨论、写作三个包

Status: implemented

## Problem

[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)要求关着 AI 时界面上看不见任何 AI 功能，也不发送任何内容。作者答「整个不显示」那一题时补了一句「因此，架构设计要做好」；问到架构怎么保证关得干净，选了拆成单独的包：「C 好一点，B还是会有一点workaround和review的复杂度考量」。

拆之前（`caaa5f5`），会发送的东西和界面混在同一个包 `desk` 里：

- `desk` 直接引用 `model`、`discussion`、`writing`。联网的只有 `model`（[模型层](2026-10-08-model-layer-on-goai.md)），讨论和写作两个包拿到 `model` 的客户端才会发送；客户端在 `desk` 里 4 处新建：`desk/ai.go:30`、`desk/app.go:352`、`desk/app.go:376`、`desk/settings.go:432`。
- 读写模型服务的配置（`providers.yaml`）、建讨论引擎、开后台整理、建续写改写用的 `writing.Writer`，都是 `*App` 的方法，夹在画界面的代码中间。

## Decision

### 审过的方向

第一稿要把讨论、检查、摘要这些界面也一起搬进 `desk/ai`，核心和 AI 之间定二十来个接口方法。作者审过以后说：「画界面的还是同一套啊，只是我们可以切换而已，整体没有区别，不要维护两套逻辑」「关掉后，禁用就好了，同样开启就是启用，不要那么极端」，问清楚以后选了「只拆会发送的部分」：

- 界面代码不搬：讨论、检查、摘要的界面也留在 `desk`，用同一套零件，开关决定显示与否。
- 会发送的部分在单独的包里：读写模型服务的配置、建模型客户端、讨论、后台整理、续写、改写、起草、摘要、检查。
- 这个包一直都在：关掉 AI 就是把它停用，停用时不建客户端；打开就是启用。
- 不加「核心不许联网」：开关管的是 AI，不是联网。

### 包怎么分

| 包 | 放什么 | 引用 |
| --- | --- | --- |
| `desk` | 全部界面，和拆之前一样 | `desk/aiport`、`workspace`、`editor`、`backup`、`shelf` |
| `desk/aiport` | `desk` 要 AI 做的事：几个接口，和过界要用的几个小结构 | 只有 `workspace` |
| `ai` | 实现 `aiport`：读写 `providers.yaml`、建客户端、讨论、续写改写起草、摘要、检查（原来还有后台整理和它的日志，2026-10-15 随记账员删了，见[讨论用工具查全书、当场记设定](../feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)） | `desk/aiport`、`model`、`discussion`、`writing`、`workspace` |
| `cmd/poc` | 建一个 `ai` 交给 `desk` | `desk`、`ai` |

`aiport` 单独成包，是因为 `desk` 的测试要用真的 `ai`（配假的模型）：接口要是放在 `desk` 里，`ai` 就得引用 `desk`，`desk` 的测试再引用 `ai` 就成环了，Go 不让。

`go list -deps writer/desk` 里自己的包只有 `backup`、`workspace`、`desk/aiport`、`editor`、`shelf`；`model.NewClient` 只在 `ai/ai.go` 里。

### 接口清单（`aiport.Service`）

| 用处 | 方法 | 拆之前 `desk` 里是 |
| --- | --- | --- |
| 模型服务的设置 | `ConfigFile`、`Providers`、`Reload`、`SaveProvider`、`SetKey`、`Catalog`（给出能 `ListModels` 的对象） | `a.Config`、`a.ConfigFile`，`model.Load`、`Save`、`SetKey`、`NewClient` 加 `ListModels`（`settings.go`、`app.go`） |
| 推理等级的名字 | `Efforts`、`EffortLabel` | `model.Efforts`、`model.Effort(x).Label()`（`chat.go`、`settings.go`、`ai.go`） |
| 讨论 | `Talk(作品, 正文旁, 进度, 每一步)` 给出能 `Say`、`Retry` 的对象；`Extract`。「每一步」是 2026-10-15 加的，回复每写一条工具调用或结果就交给界面 | `a.engine`、`eng.Say`、`eng.Retry`、`eng.Extract`（`app.go`、`chat.go`） |
| 后台整理 | 原来是 `Bookkeep(作品, 有变化时)` 给出 `Kick`、`Busy`、`Stop`，`Book(作品)` 给出能 `Redo` 的对象；2026-10-15 随记账员删了 | `a.Book`、`a.bookkeeper`、`discussion.NewWorker`、`startWorker`、`kick`、`halt`、`redo`（`app.go`、`setting.go`） |
| 续写、改写、起草、摘要、检查 | `Pen(作品)` 给出一支笔：`Name`、`Effort`、`Predict`、`Rewrite`、`Draft`、`Summarize`、`Check`；改写和起草给出 `Job` | `a.Pen`、`a.newWriter`、`writing.Writer` 的各个方法（`ai.go`、`summary.go`、`check.go`） |
| 不发送的 | `Unsummarized`、`MinSummary`、`MinCheck` | `writing.Unsummarized`、`writing.MinSummary`、`writing.MinCheck`（`summary.go`、`check.go`、`ai.go`） |
| 开关 | `SetOn` | 没有 |

`Catalog`、`Talk`、`Book`、`Pen` 都分两段：先在界面的线程上按作品现在的设置建好，配置上的错（没选模型、没有 key）马上报；再在后台发送。

过界的小结构四个：

- `Provider`：和 `model.Provider` 一样的六项，设置页要编辑它。
- `Beside`：和 `discussion.Writing` 一样的六项，正文旁的讨论带上的正文。
- `Scene`：和 `writing.Scene` 一样的三项。
- `Job`：一次改写或起草，`Record`（快照和标记要用）加一个 `Run`。`desk` 原来只用到 `writing.Job` 的 `Record`。

前三个和那边的项、类型、先后都一样，`ai` 用 Go 的类型转换来回换（`aiport.Provider(p)`、`discussion.Writing(*beside)`、`writing.Scene(scene)`）：哪边多一项、少一项，`go build` 就过不去。别的都用 `workspace` 的类型、字符串和函数。

### 怎么建、测试怎么换假模型

- `ai.New(配置文件)` 建 `ai.Service`，不读盘；`desk.New(它)` 建界面：先读作品清单 `works.json`，把开关告诉 `ai`（`SetOn`），AI 开着而且有配置文件时 `Reload` 一次，读不成的错照旧显示在界面上。`cmd/poc` 把两个接起来，找不到配置目录的错也由它交给界面。`works.json` 照旧放在配置文件旁边，位置问 `ConfigFile()`。
- 测试换 `ai.Service` 上的 `NewWriter` 钩子（拆之前是 `App` 上的 `Pen`；原来还有换记账员的 `NewBookkeeper`，2026-10-15 删了）。`desk/app_test.go` 里两个小函数：`newApp(配置文件)` 建界面，`aiOf(a)` 拿到它的 `ai.Service` 好换钩子。`desk` 的测试引用 `ai` 和 `model` 来做假模型，不算 `desk` 的依赖（`go list -deps` 不看测试）。要数 `desk` 向 AI 要了什么，`desk/aiswitch_test.go` 的 `spy` 包住一个 `aiport.Service`，记下 `Catalog`、`Talk`、`Pen` 的每一次。

### 关着 AI 时

- `ai` 停用：`SetOn(false)` 以后，`Catalog`、`Talk`、`Pen` 头一件事就是返回 `ai.ErrOff`（「AI 已关闭」），一个客户端也不建。（原来 `Book` 和后台整理要用的那支笔也这样，2026-10-15 随记账员删了。）正在跑的回复、起草、摘要、检查由 `desk` 在开关的那一刻停下（取消各自的 context），`ai` 不记它们。
- 界面照原型，用到 AI 的地方看 `a.aiOn()`，关着就不显示；细节见[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)的「代码里怎么做」。
- `desk` 里只有 AI 的活走 `a.run`（讨论、读模型目录；原来还有后台整理的重做），关着时这条路上到达的报错不显示。

### 搬家时的小变化

第二步只搬、不改行为，下面几处和搬之前不一样：

- 推理等级的名字要问 `ai`，原来不挂在 `App` 上的 `modelLabel`、`effortSelect` 成了 `App` 的方法，前者改叫 `replyLabel`（`ai.go` 里已经有一个 `modelLabel` 方法）；续写还没有笔时报 `desk` 自己的错，编辑器本来就不显示续写的错。
- 第一次启动时配置文件还没有、默认配置又存不下（配置目录写不进去）：原来启动时会列出没存下的默认服务，「重新加载配置」碰到同样的错却什么都不换；现在启动和「重新加载配置」走同一条路：读不成就不换，报的错一样。后台整理碰到这种情况本来就是报错不干活。
- 顺带改正：原来 `engine` 和 `startWorker` 把那一帧的 `*ui.Context` 留进了在后台跑的回调（回复一段段到、后台整理写完一次），`mygo vet` 只查 `go` 语句和存下来的，没查出来；现在用起头那一帧取的 `c.Services()`。

## Alternatives considered

- **AI 的界面也搬出去（`desk/ai` 放讨论、检查、摘要的界面和逻辑，核心留二十来个接口方法）** — 我第一稿推荐这个：核心里根本没有 AI 的代码，「看不见」也由编译器保证，以后把功能分成「只有 AI 才有」和「两边都能用」时，清单现成。没有选：作者要的是同一套界面靠开关切换，「不要维护两套逻辑」；接口二十来个、约 3,500 行换包、两边共用的状态要一样样分家，作者说「不要那么极端」。「看不见」由走一遍的检查守着。
- **零件单独建一个 `desk/look` 包** — 第一稿为了两个包都能用零件才要它。界面不拆以后用不着了，零件留在 `desk`。
- **核心的依赖里也不许有 `net`、`net/http`** — 第一稿我加的：核心连联网的能力都没有，「不发送」由编译器兜底。没有选：开关管的是 AI，以后核心自己要联网的功能（比如检查更新）不该被它挡住；作者说「不要那么极端」。
- **关着时 AI 那部分不存在（nil），打开时重新造** — 第一稿这么写的：关着时连对象都没有，最干净。没有选：作者说「关掉后，禁用就好了，同样开启就是启用」；一直在、只是停用，开关前后的状态也不用重建。
- **接口放在 `desk` 里，`ai` 引用 `desk`** — 接口跟着用它的一方走，Go 里常见，少一个包。没有选：`desk` 的测试要用真的 `ai` 配假模型，这样就成环了，测试得另写一份替身，等于把 `ai` 的那点胶水在测试里再写一遍。
- **每个入口加一句「AI 开着才显示」，不拆包** — 改得最少。没有选：「不发送」只能靠每一处都记得；作者选了拆包（C）。
- **同一个包里收进一个对象加一道闸（B）** — 不用新包。没有选：作者说「B还是会有一点workaround和review的复杂度考量」。
- **用编译标签编出没有 AI 的程序** — 编译器保证。没有选：开关要在运行时改、马上生效；出两种程序是作者否掉的「做两套」（[关掉 AI](../feature/2026-10-14-disable-ai.md) 的 Q15）。

## Consequences

**收益**

- 「不发送」有两道：界面关着时不发起，`ai` 关着时什么都不建。界面漏了一处入口，到 `ai` 那里也只得到 `ErrOff`。
- `desk` 引用不到 `model`、`discussion`、`writing`，由测试守着；界面里再也新建不了模型客户端。
- 会发送的代码集中在 `ai/ai.go` 一个文件，审隐私时只看它。

**代价与已知上限**

- **「看不见」靠检查，不靠编译器**：界面里用到 AI 的入口各自看开关，漏一处就露一处；走一遍的检查只认它知道的字和它走到的地方。新做 AI 功能时要把入口和名字加进检查。
- **三个小结构抄了一份**：`aiport.Provider`、`aiport.Beside`、`aiport.Scene` 和 `model`、`discussion`、`writing` 里的长得一样，那边加一项这边要跟着加。漏不掉：`ai` 用类型转换来回换，两边对不上就编译不过；只是改的时候要两边一起改。
- **偏好和模型选择照旧由 `desk` 读写**：`work.yaml` 里各用哪个模型、`.writer/desk.json` 都不变，`ai` 只从 `Store` 读模型选择。
- **`ai` 不记谁在跑**：停下正在跑的回复、起草、摘要、检查，靠 `desk` 记着各自的 cancel；以后 `ai` 里自己起后台任务，要像原来的后台整理那样每回先查开关。

## Verification

- `desk/aiswitch_test.go` 的 `TestDeskImportsNothingThatSends`：`go list -deps writer/desk` 里有 `writer/desk/aiport`，没有 `writer/ai`、`writer/model`、`writer/discussion`、`writer/writing`。在 `go test ./...` 里，也就在 `npm run verify` 里。
- 同一个文件的 `TestAIOffShowsNothingOfIt`、`TestTurnTheAIOffAndOn`：关着 AI 时替身一个请求也没收到；重开软件开关还是关着，`Catalog` 返回 `ai.ErrOff`。`ai/ai_test.go` 的 `TestOffBuildsNothing`：关着时 `Catalog`、`Talk`、`Book`、`Pen` 都返回 `ErrOff`，没建一个客户端。
- 第二步：`desk/shot_test.go` 和 `cmd/poc` 的布局测试在搬之前截两套、搬之后一套（`desk` 85 张、`cmd/poc` 12 张）。搬之前的两套之间就有 28 张不一样；搬之后和两套都不一样的 37 张逐张看过，全是会随时间变的：消息、快照、书架上的钟点、测试临时文件夹的名字、提示条的倒计时条、后台整理刚改的设定那 3 秒的淡出（按真实时间算，`desk/setting.go` 的 `flashFor`）。测试只改了建 `App`、换假模型和 `live.add` 的写法，断言没动。
- 第三步的截图对比见[可以整个关掉 AI](../feature/2026-10-14-disable-ai.md)的 Verification。
