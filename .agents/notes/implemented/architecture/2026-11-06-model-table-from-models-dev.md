# Agent Note: 模型的上限和价格从 models.dev 后台拉取

Status: implemented

## Problem

`model/pricing.go` 的 `table` 是手抄的：十个模型的上下文上限、最大输出和价格，2026-10-15 照各家文档核实。[模型上限和用量](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)定的是「随软件发布更新，不联网拉」，备选里否掉过联网拉 models.dev，理由是要联网、服务商接口不一定返回窗口和价格、不好测，并留了一句「以后可以在内置表上叠加联网覆盖」。

手抄表的问题：

- 新模型出来，要等下一次发布才认得。表里没有的模型用保守值（上下文 32000、最大输出 4096），作者得自己去设置里改。
- 价格和上限变了，表不会跟着变，估算的花费会错。
- 每次发布前都要有人逐家核对。

作者在[盘点](../../proposed/architecture/2026-11-06-reinvented-wheels.md)第 3 条上说：「原来的决定我可能要放弃，这个可以先弄个新笔记，因为可以放后台拉取，就算拉取失败也没有副作用，拉取成功反而能拿到更多新的模型」。

## Decision

在内置表上叠一层从 models.dev 拉来的目录。内置表留着，拉取失败或从没拉到时照旧用它。

### models.dev 的数据

2026-11-06 查过 `https://models.dev/api.json`：

- 约 6 MB，235 家服务商。响应头是 `Cache-Control: public, max-age=0, must-revalidate`，带 `ETag`，可以用 `If-None-Match` 问有没有变，没变时不必重新下载。
- 表里的四家都在：`deepseek`、`moonshotai`、`openai`、`anthropic`，内置表的十个模型都按 ID 原样查得到。Claude 头一次查时筛选写错了，以为 `deepseek-flash` 不在，其实在；`deepseek-v4-flash` 是另一个、已标为 `deprecated` 的条目。
- 有几处数和内置表不一样：kimi-k3 的最大输出 models.dev 写 1048576，内置表是猜的 128K；gpt-6-astra 有缓存写价格 12.5，内置表按输入价算；DeepSeek 的上下文写 1048576，内置表写 1M。
- 只留 `limit`、`cost` 两项时，默认的四家约 12 KB，全部 235 家约 1.2 MB。
- 带上次的 `ETag` 再请求，服务器回 304、不带正文（2026-11-06 试过）。
- 每个模型有 `limit`（`context`、`output`，部分有 `input`）和 `cost`（每百万 token 的 `input`、`output`、`cache_read`、`cache_write`；分档的有 `tiers`，按上下文长度分，比如 gpt-6-astra 超过 272000 一档）。内置表的 `above`、`long` 两档能对上 `tiers`。
- 还有能力字段：推理、工具调用、结构化输出。这一篇先不用。

### 作者定的

- 内置表和拉来的目录都用 JSON：「models.dev 拉取弄成个 json，这样云端拉取完替换和每次更新都很方便」。
- 数对不上时：「以 model.dev 为主」。
- 这一批不加开关，界面不动。

### 怎么拉、怎么用

- **一种格式**：models.dev `api.json` 的子集，`{服务商: {"models": {模型 ID: {"limit": …, "cost": …}}}}`，字段名照 models.dev 原样，分档价格照它的 `tiers`。
- **内置的那份**：`model/models.json`，用 `go:embed` 编进程序，只留默认的四家（约 12 KB）。`model/pricing.go` 的手写 `table` 删掉，价格和上限都从这份 JSON 读。
- **更新内置的那份**：一个命令拉 models.dev、用和运行时同一个过滤函数重写 `model/models.json`。发布前跑一次，换文件即可。
- **后台拉**：启动后在后台拉一次，不挡启动，不挡任何请求；之后一天最多一次。带上次的 `ETag`，没变就只是一次很小的请求。
- **缓存在本机**：拉到的目录过滤成同样格式的 JSON，留全部服务商（约 1.2 MB），中转和自定义端点上的模型也查得到。整文件原子替换写进配置目录，`ETag` 存在旁边。
- **失败没有副作用**：断网、超时、返回的不是合法 JSON、字段对不上，都只写日志，不弹提示，接着用上一次的缓存或内置那份。认不出的单个模型跳过，不让整份作废。
- **查的时候不联网**：`Builtin(id)` 在读 `providers.yaml` 时就调，是同步的，只读内存里的目录。启动时读缓存（没有就用内置的），后台拉到新的以后换掉，之后的查表用新的；已经读进来的配置下次读时才更新。
- **怎么查**：只按模型 ID，在四家官方依次查（`deepseek`、`moonshotai`、`openai`、`anthropic`），查不到再按其余服务商 ID 排序查。默认服务商就是这四家；中转服务商 ID 与 models.dev 通常对不上，按服务商查只在同一 ID 在两家官方数值不同时才有用，不值得为此改签名或在 YAML 里解码两遍。缓存优先于内置目录。
- **优先级**：作者在 `providers.yaml` 里写了的 > models.dev 的缓存 > 内置的 JSON > 保守值。作者写的永远不被覆盖。

### 实现边界

- `model/directory.go` 共用过滤函数与内存目录；缓存通过 `atomic.Pointer` 整份替换，第三方重复 ID 按服务商 ID 排序以保证结果稳定。
- `StartModelUpdates` 在配置解码前同步读本机缓存，网络请求完全在后台；入口放在 `cmd/inkgo/main.go`。缓存目录是 `providers.yaml` 所在目录，`models-meta.json` 保存 ETag 和时间。失败尝试也计入每日限额，避免反复启动时重复请求。
- 请求超时 30 秒，响应上限 32 MB，使用环境代理；目录和元数据分别用 `internal/atomicfile` 原子替换。两文件不构成事务，ETag 非空且缓存能解析时才发送条件请求，缺失或损坏的目录不发送条件请求，下一次完整下载可恢复。
- `go run ./cmd/modelsjson` 从仓库根目录运行，调用同一个过滤函数生成两格缩进、键排序的四家内置数据。初始数据是 Claude 从 models.dev 过滤的初始数据，本次用共享过滤函数从现有内容离线重新生成，去掉 `reasoning`、`context_over_200k` 等多余字段；生成命令以后在有网的地方再跑。
- `Builtin`、`Lookup`、`Cost` 只接收模型 ID，YAML 解码一次，目录补缺省，作者显式填写的字段优先；已解码的配置不回写。
- 目录填值时，若 `0 < limit.input < limit.context`，输出取 `min(limit.output, context-input)`；否则 `output >= context` 视作输入输出共用窗口，输出取 `context/4`，其余沿用目录输出。共用窗口给回复留四分之一是 Claude 定的实现规则，不是作者的决定。kimi-k3 因此为上下文 1048576、输出 262144、输入预算 786432。作者在配置里写的最大输出仍须严格小于上下文（`Output >= Window` 报错），原校验和测试恢复。
- 只要 `limit.context`、`limit.output` 合法就保留模型。缺少或无法解析的价格不丢上限，基础价格不全时 `Cost` 返回 `ok=false`；坏分档逐档丢弃，保留其余分档。
- 启动时的缓存读取直接做有大小限制的文件读取和一次 `parseDirectory`，不经过过滤、序列化和再解析；此本机读取同步占用启动时间。`BenchmarkLoadModelCache` 使用 235 家服务商、3525 个模型生成的 1.2 MB 假缓存（含空白补齐），测完整读文件与解析、内存替换；报告注明本机热文件缓存下的测量值，不代表冷磁盘启动。

### 和模型列表的关系

`model/catalog.go` 的 `ListModels` 请求作者自己的端点的 `/models`，列的是这个端点实际提供的模型，models.dev 替代不了它：中转和自定义端点上有什么，models.dev 不知道。两者配合：`/models` 给出有哪些模型，models.dev 给这些模型补上限和价格。官方服务商的 `/models` 拉不到时，可以用 models.dev 的列表作候选，这一条可选。

## Alternatives considered

- **维持原决定：只用随发布更新的内置表。** 最强的理由是完全离线、行为可复现、测试简单。作者说「可能要放弃」，理由是后台拉取失败没有副作用，成功了能认得更多新模型；内置表仍是兜底，离线时行为和现在一样。
- **发布时从 models.dev 生成内置表，运行时不联网。** 最强的理由是省掉手抄，又不改「不联网」的决定，[盘点](../../proposed/architecture/2026-11-06-reinvented-wheels.md)第 3 条起初就是这样推荐的。没选为主方案：新模型照样要等发布。可以和本方案并用，发布时顺手刷新内置表，让兜底的数不太旧。
- **请求服务商自己的接口拿上限和价格。** 没选：原篇已经查过，服务商的 `/models` 大多不返回窗口和价格。

## Testing

- 断网启动：不报错、不弹提示，上限和价格与内置表一致；有缓存时与缓存一致。
- 拉到一个内置表里没有的新模型以后，设置里这个模型显示 models.dev 给的上下文上限和最大输出，不再是 32000 和 4096。
- 作者在 `providers.yaml` 里写过的值，拉取前后都不变。
- 返回坏 JSON、字段缺失、超时，都只写日志，接着用旧的数。
- `ETag` 没变时不重新下载整个文件。
- 测试不联网：用 httptest.ResponseRecorder 和内存 transport 给出成功、304、坏 JSON、超时与 HTTP 错误；不用监听端口，客户端超时通过请求 context 验证。
- [模型上限和用量](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)里「内置模型表」和备选项指向本篇，两篇互链。

## Consequences

- models.dev 是社区维护的，数可能填错。作者写的值优先，内置表兜底，减轻但不消除。
- 字段格式以后可能变。解析要宽松：认不出的模型跳过，不让整份目录作废。
- 6 MB 的下载在慢网上要一会儿；有 `ETag` 以后，平时只是一次很小的请求。
- 多了一处后台网络请求，会让 models.dev 看到本机的 IP，不带作者的任何数据。关于或隐私说明的文字属于后续界面工作，本次不改。

## 相关笔记当场审计

- [模型上限和用量](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)：部分重叠。本篇取代它「不联网拉」的决定，原篇保留模型配置、请求预算和用量的约束，两篇互链。
- [模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)：部分重叠，那篇记着 goai 不管模型目录、`/models` 由我们自己请求。本篇不改这一点。
- [盘点](../../proposed/architecture/2026-11-06-reinvented-wheels.md)：第 3 条由本篇接手。
- 没有要删或归档的笔记。
