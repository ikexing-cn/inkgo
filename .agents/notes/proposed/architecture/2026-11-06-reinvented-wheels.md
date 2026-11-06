# Agent Note: 自写实现和现成方案的盘点

Status: proposed

## Problem

作者要在动手修扩展契约（[契约审查](../../implemented/architecture/2026-11-06-extension-contract-review.md)）之前，先盘点项目里有多少是「重复造轮子」。作者定的标准：市面上已经有成熟、更完整的方案，就算；star 少但维护勤的库，也应当拿来作参照。作者说讨论过程不重要，但 Claude 和 Codex（astra）任何一方提到的轮子都要列出来。

本篇只是盘点，不代表要换。哪项换、哪项留、哪项重议，由作者逐项定。

作者 2026-11-06 逐项给了意见，见下面「作者的决定」，并说：「这期我确定的都写到一个笔记，暂时搁置」。之后看过推进计划，作者定「第一第二同步推进」：[契约审查](../../implemented/architecture/2026-11-06-extension-contract-review.md)的修复，和本篇的 19、20、5、6、7、3 同时施工；查阅不逼答、文件监视扩展「先搁着，这个我后面再定夺」；编辑器换 richtext、增量云同步照旧搁置。

## Proposal

清单如下。每一项都标出是谁提的、有没有已有决定；作者逐项定换、留还是重议。定下来要换的，先核对能不能换（见「换之前要核对的」），再动代码。

### 怎么盘的

- Claude 和 astra（gpt-6-astra，high，只读）各自读代码列出，合并去重。「提出」一栏里，C 是 Claude，A 是 astra。
- 维护情况是 2026-11-06 用 gh 查的：star 数、最近一次推送、许可证。超过一年没有推送的标「停更」。
- 判断分三种：
  - **是**：通用能力，我们自己又写了一遍；
  - **部分**：有通用的部分，也有产品自己的规则；
  - **不是**：已经在用现成的，或者只是一层薄封装。
- 「已有决定」一栏链到当初做决定的笔记。那里写过取舍的，重议要先读原篇的理由。
- 判断时守的约束：纯 Go，`CGO_ENABLED=0`；界面是 mygo；`scripts/` 不装 npm 依赖；存储是有意不建索引的纯文件。

### 清单

| # | 位置 | 我们自己写的 | 候选（★ / 最近推送 / 许可） | 判断 | 已有决定 | 提出 |
|---|---|---|---|---|---|---|
| 1 | `ext/talk/engine/engine.go` 229 行起 | 多步工具循环 | `zendev-sh/goai`（234 / 2026-10-31 / MIT）。它已经是依赖，有 `WithMaxSteps`、`WithStopWhen` 和工具执行前后的钩子，项目只用了单步的 `StreamText` | 是。要核对钩子能不能保住逐步落盘、推理内容回传、预算用完强制总结 | [讨论用工具查全书](../../implemented/feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)写了「循环由讨论自己跑」，但没比较过 goai 的多步执行；[模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)记着作者当初不想自己补工具循环 | C |
| 2 | `inference/` | 从回答里严格解析 JSON；`Locate` 忽略空白定位引文 | goai 的 `GenerateObject`、`StreamObject`；`kaptinlin/jsonrepair`（126 / 2026-10-25 / MIT）；`RealAlexandreAI/json-repair`（385 / 2026-11-02 / **GPL-3.0**）；`sergi/go-diff` 的模糊匹配作参照 | 部分。JSON 修复会把「拒绝坏回答」变成「容错」，语义不同；`Locate` 没有现成替代 | — | C、A |
| 3（已施工） | `model/limits.go`、`model/directory.go` | 上下文上限目录（原手抄表） | models.dev（`anomalyco/models.dev`，7523 / 2026-11-06 / MIT） | 已后台拉取；价格统计取消，只留上限 | [模型上限和用量](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)：已由 models.dev 后台更新决定取代 | C |
| 4 | `prompt/prompt.go` 的 `Estimate`、`ClipTokens` | 按字符比例估 token | `pkoukk/tiktoken-go`（960 / 2026-05-12 / MIT）、`tiktoken-go/tokenizer`（463 / 2026-10-21 / MIT），以及各家的 count tokens 接口 | 是。但没有一个精确计数器能覆盖所有模型 | [按字符估 token](../../implemented/architecture/2026-10-08-token-estimate-by-character.md)和 [M1 讨论](../../implemented/feature/2026-10-07-m1-discussion.md)：不接供应商 tokenizer，自定义端点未必提供 | C |
| 5 | `editor/wrap.go` 63 行 `breakLines` | 断行、英文按词断、避头尾 | `rivo/uniseg`（740 / 2026-11-05 / MIT）；`go-text/typesetting` 的 segmenter（161 / 2026-11-02，已是间接依赖） | 已施工（segmenter v0.3.5）。是。库给出合法断点，字宽、缩进、AI 插入的间隙仍归本地 | [纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md)：编辑器自己断行；[两端对齐](../feature/2026-11-02-justified-prose-and-adjustable-spacing.md)否掉过换成 mygo 的断行 | C |
| 6 | `internal/textclip` | 按 rune 截断，会切开表情和组合字符 | `rivo/uniseg` | 已施工（segmenter v0.3.5）。是。界面摘要可以改按字素截；业务上约定按 rune 计长度的地方不能一起改 | — | C |
| 7 | `ext/snaps/textdiff` | 盘点时为段落、字符两级 LCS；已施工：两层换成 go-diff | `sergi/go-diff`（2092 / 2025-06-05 / MIT）、`aymanbagabas/go-udiff`（235 / 2026-07-16 / BSD-3）、`hexops/gotextdiff`（7 / 2023-09，停更）、`pmezard/go-difflib`（429 / 2023-05，停更） | 是。另外 astra 发现，400 万格的上限只用在字符那一层（`diffRunes`），段落那一层的 LCS 没有上限，Claude 已核实 | — | C、A |
| 8 | `internal/atomicfile` | 临时文件、Sync、改名 | `google/renameio`（681 / 2026-01-10 / Apache-2.0，**在 Windows 上不导出任何函数**）、`natefinch/atomic`（212 / 2022-05，停更，支持 Windows） | 是，但没有合适的替代 | [纯文件存储](../../implemented/architecture/2026-10-08-m1-storage-without-index.md)：写文件只留这一份实现 | C |
| 9 | `ext/export/format/docx.go` | 手写 OOXML | `gomutex/godocx`（273 / 2025-08-11 / MIT）、`fumiama/go-docx`（307 / 2025-05 / **AGPL-3.0**）、`unidoc/unioffice`（4941 / 2026-09-21，**商业许可**） | 是。候选要么不常维护，要么许可证不合适 | [导出 Word](../../implemented/feature/2026-10-16-export-to-word.md)：要的几样用标准库一百多行就够，不多加依赖 | C |
| 10 | `internal/strictjson` | 36 行：`DisallowUnknownFields`，再查是否读到 EOF | `encoding/json/v2`（go1.27.3 里仍要 `GOEXPERIMENT=jsonv2`） | 不是，只是薄封装。另外它不拒绝重复键 | — | C、A |
| 11 | `tokens/`、`desk/kit/when.go` | K/M 格式化；「今天」「昨天」「10-06 21:40」 | `dustin/go-humanize`（4856 / 2026-10-13） | 部分。精确往返和中文日期说法是产品规则 | — | C、A |
| 12 | `workspace/numbering.go` | 汉字数字和章节序号互转 | `duke-git/lancet/v2` 的 convertor（5300 / 2026-11-02 / MIT） | 部分。反向转换和「两」「〇」的覆盖待核 | — | C、A |
| 13 | `ext/` 宿主 | 装配、依赖排序、生命周期、服务注入、注入链 | `uber-go/fx`（7725 / 2025-12-27）、`uber-go/dig`（4509 / 2025-05-13）、`samber/do`（2834 / 2026-11-05）。外部插件层：`hashicorp/go-plugin`（6119 / 2026-11-02 / MPL-2.0）；`extism/extism`（5827 / 2026-10-09），它的 `go-sdk`（186 / 2025-05）建在 wazero 上，纯 Go，不要 CGO；`wazero/wazero`（6455 / 2026-10-19）；`traefik/yaegi`（8437 / 2026-11-05）；`dop251/goja`（7188 / 2026-11-05）；`yuin/gopher-lua`（7004 / 2026-04-01） | 部分。DI 库不管书级开关、注入顺序和帧线程；插件运行时是外部扩展层的参照 | [扩展模块](../../implemented/architecture/2026-11-02-extension-modules.md)、[扩展 API](../../implemented/architecture/2026-11-03-extension-api.md) | C、A |
| 14 | `internal/extlocales` | 检查翻译表，生成带类型的取词函数 | `golang.org/x/text/message` 和 gotext（golang/text 810 / 2026-10-21）、`nicksnyder/go-i18n`（3552 / 2026-11-01） | 部分。运行时已经用 mygo | [多语言](../../implemented/architecture/2026-11-02-i18n-on-mygo.md)：否掉过 go-i18n，以免出现两个「当前语言」 | C、A |
| 15 | `workspace/` 的 JSONL、游标、修订历史；`ext/snaps`；`ext/backup` | 追加日志、修订历史、快照 | `modernc.org/sqlite`（主仓库在 GitLab cznic/sqlite，515 / 2026-11-04）、`etcd-io/bbolt`（9804 / 2026-10-16）、`go-git/go-git`（7846 / 2026-11-05） | 部分。ZIP 用的是标准库，不算自写 | [纯文件存储](../../implemented/architecture/2026-10-08-m1-storage-without-index.md)：可以手改、可以带走的纯文件；[备份](../../implemented/feature/2026-10-09-backup.md) | C、A |
| 16 | `scripts/prototype/browser.ts` | 自己写的 DevTools 协议驱动 | Playwright（99414 / 2026-11-06）、puppeteer（95806 / 2026-11-05）、`chromedp/chromedp`（13356 / 2026-11-05）、`go-rod/rod`（7145 / 2026-08-11） | 是。前两个违反「不装 npm 依赖」；后两个要把驱动改写成 Go | [本地原型](../../implemented/process/2026-10-12-local-prototype.md)：只用 Node 自带的 WebSocket 和 fetch | C |
| 17 | `scripts/verify-agent-note-*.ts`、`check-note-anchors.ts` | 检查笔记格式和链接 | `lycheeverse/lychee`（4097 / 2026-11-02）、`remarkjs/remark-validate-links`（125 / 2025-02-21）、`DavidAnson/markdownlint`（6416 / 2026-11-06） | 部分。生命周期目录、必备的节、锚点是仓库自己的规则 | — | C |
| 18 | `editor/` | 撤销栈、按键合并、文本缓冲 | `gioui/gio` 的 `widget.Editor`（2294 / 2026-11-02，只能参照，嵌不进 mygo）、`zyedidia/rope`（8 / 2021-06，停更） | 部分 | [纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md) | C、A |
| 19 | `editor/edit.go` 149 行起、`wordAt`、`wrap.go` 的 `runeClass` | 退格和 Delete 一次删一个 rune；双击按自定义的字符类选词 | `rivo/uniseg`、go-text 的 segmenter | 已施工（segmenter v0.3.5）。是。**退格会把表情或组合字符删掉一半，作者看得见**（155 行 `a--`，Claude 已核实） | [纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md) | A |
| 20 | `workspace/reads.go` 59 行 `Language.Count` | 字数统计 | `rivo/uniseg`、go-text 的 segmenter | 已施工（segmenter v0.3.5）。部分。统计口径是产品规则，换库不能悄悄改了字数 | — | A |
| 21 | `editor/doc.go` 115 行、`ext/find/findbook.go` | 逐位置比较的查找；全书查找 | `strings.Index`（标准库）、`blevesearch/bleve`（11251 / 2026-11-04）、SQLite FTS | 部分。全书索引受「不建索引」限制 | [纯文件存储](../../implemented/architecture/2026-10-08-m1-storage-without-index.md) | A |
| 22 | `editor/names.go` 33 行、`editor/complete.go` 26 行 | 每个名字各扫一遍全章；逐个查前缀 | `cloudflare/ahocorasick`（728 / 2026-04-24）、`armon/go-radix`（948 / 2024-07，停更；astra 写成 `hashicorp/go-radix`，没有这个仓库）、`hashicorp/go-immutable-radix`（1106 / 2026-10-26 / MPL-2.0） | 部分。名字少时现在的写法够用 | — | A |
| 23 | `desk/save.go` 64 行、`workspace/tree.go` | 定时检查文件在外面被改过没有 | `fsnotify/fsnotify`（10809 / 2026-05-11）、`radovskyb/watcher`（1506 / 2023-10，停更） | 部分。文件事件代替不了写之前的版本核对 | — | A |
| 24 | `desk/cache.go`、`ext/check/cache.go` 等 | 只记最后一个键的缓存，几个包各一份 | `hashicorp/golang-lru`（5136 / 2026-10-26 / MPL-2.0） | 部分。更该做的是把几份合成一份，不必引 LRU | — | A |
| 25 | `desk/jobs.go`、`workspace/checkqueue.go` | 任务排队、去重、一次只跑一个 | `golang.org/x/sync/errgroup`（golang/sync 936 / 2026-10-15）、`panjf2000/ants`（14523 / 2026-09-19） | 部分。回到 mygo 的下一帧、同一段正文只试一次，是本地规则 | — | A |
| 26 | `ext/backup/archive/policy.go` | 每日、每小时调度，失败后冷却 | `robfig/cron`（14198 / 2024-07，停更）、`cenkalti/backoff`（4093 / 2026-11-01） | 部分。几个时间比较可能已经够用 | [备份](../../implemented/feature/2026-10-09-backup.md) | A |
| 27 | `model/catalog.go` | 自己请求 `/models`，处理 Anthropic 的分页 | `openai/openai-go`（3526 / 2026-11-06）、`anthropics/anthropic-sdk-go`（1239 / 2026-11-05） | 部分。跨供应商的接口、大小和页数上限、凭证隔离要保留 | [模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)：goai 不管目录，自己发 GET | A |
| 28 | `model/config.go`、`prefs/`、`shelf/` | 配置文件路径、默认值、读写、迁移 | `knadh/koanf`（4231 / 2026-11-02）、`spf13/viper`（30501 / 2026-01-12）；YAML 已经在用 `go.yaml.in/yaml/v3`（yaml/go-yaml 545 / 2026-11-04） | 部分 | — | A |
| 29 | `scripts/prototype/browser.ts` 55 行的 `CLOCK` | 手写的假时钟 | `@sinonjs/fake-timers`（860 / 2026-09-10）、Playwright 的 Clock | 是。受 npm 约束 | [本地原型](../../implemented/process/2026-10-12-local-prototype.md) | A |
| 30 | `scripts/design/check.ts`、`rules.ts` | 用正则从 CSS 里取颜色、尺寸再检查 | `stylelint/stylelint`（11542 / 2026-11-04）、`postcss/postcss`（28963 / 2026-11-02）、`csstree/csstree`（2128 / 2026-09-16） | 部分。受 npm 约束 | — | A |
| 31 | `scripts/notes-board.ts` 26 行等 | 用正则解析 Markdown 的链接和标题 | `remarkjs/remark`（9042 / 2026-09-27）、`yuin/goldmark`（5093 / 2026-11-02） | 部分 | — | A |
| 32 | `scripts/verify.ts` | 并行跑各项检查、收集输出、给 Go 测试分片 | `gotestyourself/gotestsum`（2735 / 2026-04-15）、`go-task/task`（16323 / 2026-11-05） | 部分 | — | A |
| 33 | `scripts/agent-note-tree.ts` 9 行、`notes-board.ts` 14 行 | 手写递归枚举文件、读命令参数 | Node 标准库的 `node:fs` glob、`node:util.parseArgs` | 是，但很小，换了也不违反 npm 约束 | — | A |
| 34 | `ext/backup/archive/backup.go` | 整套备份流程 | `restic/restic`（37020 / 2026-11-05），独立工具，只能参照 | 部分。ZIP 作者能直接打开，这个优点要留 | [备份](../../implemented/feature/2026-10-09-backup.md) | A |
| 35 | `desk/kit/markdown.go`、`ext/talk/markdown.go` | — | 已经在用 mygo 的 Markdown | 不是 | — | A |
| 36 | 日志、限流、网络重试、模板 | — | — | 没发现自写的框架 | — | A |

第 3 条结局：后台拉取已实现；价格统计随后由作者取消，只保留上限，见[models.dev](../../implemented/architecture/2026-11-06-model-table-from-models-dev.md)。

### 和换不换无关、先修的

这两条是盘点时顺带发现的缺陷，不管换不换库都该修：

- **19（已施工）**：退格和 Delete 会切开字素簇。
- **7**：盘点时段落那一层的 LCS 没有上限，两份很长的稿子对比时，表会很大；第 7 条已施工，两层都交给 go-diff 的默认超时。

### 值得先看的

- astra 建议先核对 1、5、6、7、8、9、19、29。
- Claude 的看法：
  - 19、6、5 是作者碰得到的正确性问题，uniseg 维护很勤，最值得先做；
  - 1、3、4 主要是少维护：goai 已经是依赖，models.dev 天天有人更新；
  - 8、9 没有合适的替代（Windows 支持、许可证、维护频率），保留自写；
  - 13、14、15、16 牵涉已有决定或「不装 npm 依赖」的约定，要不要重议由作者定。

### 作者的决定（2026-11-06）

19、20、5、6 已施工（2026-11-06），现行事实见[纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md)和[字数口径](../../implemented/architecture/2026-11-02-i18n-on-mygo.md)。其余决定尚未全部落实，因此盘点整篇保留 proposed，不把未完成项标成 implemented。

下面只记作者说过的话；后面跟着的是 Claude 为这一条查到的事实，不是作者的决定。表里没提到的项，作者还没表态。

- **19**：「19 同意」；Claude 解释过以后，作者又说了一次「19 同意」。退格、Delete、普通左右键及 Shift 扩选按字素簇；双击保留 runeClass 产品分类，只将两端扩到字素簇边界。go-text/typesetting 的 segmenter 已经经 mygo 链进来了，有按 rune 的接口和字素、断行两种迭代器，不必新加依赖。
- **5（已施工）**：「5 同意」。断点改用 segmenter 的 UAX #14，字宽、缩进、AI 插入的间隙和避头尾的产品规则仍归本地。
- **7**：「7 同意，但是mygo好像也有个diff … 参考一下」。查过 mygo v0.7.4：
  - 算法在 `internal/diff`：git 的 histogram 算法，每行都重复太多时退回带代价上限的 Myers；另有 `Words`、`Chars` 两种行内比较，`ui/editor` 的 `alignBySimilarity` 按相似度给改过的行配对。这几样正是我们缺的：有上限而不是硬切成「整段替换」，段落配对也更像人想的。
  - `internal/diff` 本身导入不了，但段落那一层能经公开的 API 调到：`editor.DiffState{Old: editor.NewBuffer(旧), New: editor.NewBuffer(新)}` 不画界面，直接调 `Hunks()`，得到按行（我们的段落）的改动范围，`Stats()` 得到增删行数。Claude 写了个不开窗口的小程序试过：三千段、每十段改一段，连建缓冲带比较约 0.11 秒。`ui/editor` 已经链进我们的包里，不增加体积。
  - 调不到的：段落里面按字比较（`Words`、`Chars`、`inlineDiff`）和改过的段落按相似度配对（`alignBySimilarity`）都是私有的。DiffView 界面是代码审阅的样子（行号、槽位），快照对比以原型为准，不拿来用。
  - `Words` 把连续的字母当一个词，一整句中文会算一个「词」；中文要用 `Chars`。
  - Claude 2026-11-06 实测三种做法（随机汉字的稿子，不开窗口）：

    | 情况 | 现在的 | `sergi/go-diff` v1.4.0 | mygo 段落 + 现在的按字 |
    |---|---|---|---|
    | 一章五千字，改 10 处 | 0.5 ms，0.3 MB | 0.2 ms，几乎不占 | 0.7 ms，0.2 MB |
    | 一章四万字，改 40 处 | 9.7 ms，2.7 MB | 1.7 ms，0.4 MB | 5.3 ms，1.2 MB |
    | 一段 1900 字改一处 | 1.7 ms，3.2 MB | 0.15 ms | 1.9 ms，3.2 MB |
    | 一段 1900 字整段重写 | 52 ms，16 MB | 102 ms，1.1 MB | 51 ms，16 MB |
    | 一段 2000 字以上整段重写 | 超过 400 万格，整段算替换 | 照样逐字比（112 ms） | 同现在的 |
    | 两份完全不同的稿子，各 3000 段 | 671 ms，132 MB | 633 ms，48 MB | 279 ms，92 MB |

    - 平常的稿子三种都在 10 ms 以内，快照对比只在打开时算一次，速度差作者感觉不到。go-diff 的好处主要在内存小、没有「超过上限整段算替换」。
    - 体积：Windows/amd64、去掉符号，go-diff 比基线多约 64 KB，现在的 textdiff 约 20 KB，换了净增四十多 KB。它用到的 `regexp`、`html`、`net/url` 等标准库我们本来就链着。
    - go-diff 不做语义整理时切得更碎，要再调 `DiffCleanupSemantic`。
  - 所以可以：段落那一层换成 `DiffState.Hunks()`，顺带修掉段落那一层没有上限的问题；段落里面按字比较先留着现在带 400 万格上限的那一份，或者换 `sergi/go-diff`。要连按字比较也用 mygo 的，得请 mygo 把它公开，这是对外的事，由作者去提或让 Claude 起草。
  - 作者看了实测说：「7 不错 这个保留」；看推进计划时又说「第二的 diff 不是说了换 go diff 吗」。所以两层都换成 `sergi/go-diff`：段落一层用它的按行比较，改过的段落再按字比较，做语义整理，用它的超时代替 400 万格的上限。上面 mygo `Hunks()` 那条路不走。
  - **已施工（2026-11-06）**：`ext/snaps/textdiff` 两层均使用 go-diff v1.4.0；删除自写 LCS 和 `maxCells`，保留默认每次 1 秒超时、按序配对段落及换行拼接，按 rune 比较后做语义整理。实现约定及验证覆盖已就地更新到[正文页与快照笔记](../../implemented/feature/2026-10-08-writing-page-and-chapters.md)。本篇还有未落地或搁置的其他项，仍留 proposed。
- **3（已施工）**：「3 同意，原来的决定我可能要放弃，这个可以先弄个新笔记」。新笔记是 [models.dev 后台拉取](../../implemented/architecture/2026-11-06-model-table-from-models-dev.md)。推进计划里作者又定：数对不上「以 model.dev 为主」，内置表和拉来的目录都是 JSON（「models.dev 拉取弄成个 json，这样云端拉取完替换和每次更新都很方便」）。
- **1**：作者让 Claude 用 haiku 先做一次快速调研。结论是**部分可替换，不值得整体替换**：
  - goai 能接管工具执行、推理回传和每步用量。
  - 接管不了的：goai 在第一步以后清空 `ToolChoice`（`generate.go` 1151 行，Claude 已核实），预算或步数用完时没法在它的循环里发 `tool_choice: none`，只能用 `WithStopWhen` 让它停下，再在循环外自己发最后一次请求。请求数和现在一样，但收尾、超预算换提示、关掉后续工具这些控制仍要自己写（[讨论用工具查全书](../../implemented/feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)记着为什么收尾要发 `tool_choice: none`、不拿掉工具）。
  - goai 的钩子在它自己的 goroutine 里跑，我们的 Progress、Proposing、Step 约定在调用方的 goroutine 上，写记录出错也没法中止循环。
  - 粗估换完代码反而多四十行左右。
  - 作者看了现在的收尾方式，说：「1 的设计感觉不好，你看看 codex dsh pis 怎么做的，这个会导致模型没有足够的上下文乱编造内容」，看完调研后说：「第一条记录笔记，后面再改」。新笔记是[查阅到头时不逼模型作答](../feature/2026-11-06-talk-stops-without-forcing-an-answer.md)。
- **18**：作者说「考虑一下mygo的编辑器」。mygo 的 `ui/richtext` 有 rope（编辑 O(log n)）、只排版看得见的段落、输入法、无障碍、首行缩进、Tags（挂在文字上的应用标记）、`VisibleParagraphs`（给页边批注取位置）。我们的编辑器没有无障碍，改一个字整章重新断行，这两处它都有。作者让 haiku 评估了一次，结论是**现在不值得换，也不值得建在它上面**：
  - 卡住的是公开 API：`EditorOptions` 只有标签、占位、字体、只读、链接、Tag 样式、主题几项（`ui/richtext/view.go` 18–46 行，Claude 已核实），没有按键回调、没有绘制回调；按字取屏幕位置的 `BoundsForRange` 是私有的（`ime.go` 186 行），公开的只有按段落的 `VisibleParagraphs`。
  - 所以 AI 灰字、行内建议（原文划线加新字）、补全弹窗、名字悬停、查找高亮、按字定位的页边批注、滚动锚点（手机预览在用）、Tab 接受灰字，都做不出来，或者只能在上面再叠一层、自己复刻它的排版算光标位置。
  - 它做得比我们好的：无障碍（我们没有）；超长的章编辑更快。输入法和撤销我们也有，它的撤销多管格式，我们这边用不着。
  - 快慢是 Claude 2026-11-06 实测的（不开窗口）：我们插一个字再排版，100 段约 0.4 ms，1000 段约 2.5 ms，10000 段约 26 ms，随段数线性变慢，因为正文是一整个 `[]rune`，每次编辑整份复制（`editor/doc.go` 的 `replace`），排版也每段过一遍；richtext 打一个字连画一帧，1000 段和 10000 段都在 5 ms 上下。两边量的东西不完全一样（它含绘制），但趋势清楚：普通长度的章我们不慢，上千段的章才落后。
  - 粗估整体换要两到三个月，核心的 AI 功能会倒退。
  - 和[两端对齐](../feature/2026-11-02-justified-prose-and-adjustable-spacing.md)、[纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md)两篇当初否掉 richtext、`TextArea` 的理由一致。
  - 往后的路：请 mygo 给 richtext 开按键、装饰、按字取位置的接口（对外的事，由作者定）；在那之前，第 5、19 条在现在的编辑器里修不算白做。
  - 作者说：「mygo 这个设计也保留笔记，也是后面再换」。新笔记是[编辑器以后换到 mygo 的 richtext](2026-11-06-editor-on-mygo-richtext.md)。
- **23**：「这个感觉也是个有需要的，后续可能维护一个新的插件吧，先记一下笔记」。新笔记是[文件监视扩展](../feature/2026-11-06-watch-files-extension.md)。盘点时漏了一个候选：mygo 自己就有 `mygo.WatchFiles`（`watch.go`），能递归监视、合并短时间内的多次改动、把原子保存报成一次写入，比 fsnotify 更合适，也不加依赖。
- **20（已施工）**：「20 这个无伤大雅 用库吧」。字数统计改用 go-text/typesetting 的 segmenter（已经链进来了），按字素簇数，表情不再被拆成几个字。英文按词、中文按字的口径不变。
- **6（已施工）**：作者先说没看懂，Claude 解释过以后作者说「改」。界面上截短文字（`internal/textclip`）改成不切开字素簇。
- **8**：作者问原子写文件的好处，说后续可能有增量云同步，「原子写文件和增量云同步都先记笔记」。新笔记是[增量云同步](../feature/2026-11-06-incremental-cloud-sync.md)。Claude 的看法仍是第 8 条保留自写，作者没表态。
- **27**：作者问「model.dev应该有吧」。models.dev 有官方服务商的模型列表，但 `ListModels` 列的是作者自己的端点实际提供的模型，中转和自定义端点上有什么，models.dev 不知道。两者配合，见 [models.dev 后台拉取](../../implemented/architecture/2026-11-06-model-table-from-models-dev.md)「和模型列表的关系」。

### 换之前要核对的

- 行为：换完以后断行、字数、diff 的分段、JSON 拒收的规则，和现在一样还是不一样；不一样的，作者要先看。
- 体积：`CGO_ENABLED=0` 下的 Windows/amd64 包要量一遍。2026-10-08 换模型库时，体积从 13.6 MB 涨到 60.8 MB，那次就否掉了。
- 许可证：GPL、AGPL、商业许可的不用。
- 平台：Windows、macOS、Linux 都要能用（第 8 条就卡在这里）。
- 维护：看最近一年的提交和 issue 处理，不只看 star。

## Alternatives considered

- **只列能直接拿来替换的。** 最强的理由是清单短，每一项都能动手。没选：作者要的是双方提到的全部轮子，只能作参照的也要列，参照本身就能帮着改进自写的版本。

## Acceptance criteria

- 作者对每一项给出换、留或重议。重议的，先读「已有决定」里的原篇。
- 定下来要换的，换之前做完上面的核对，结果写回本篇或相应的笔记。
- 第 19、7 两条修好，并且有测试：表情和组合字符退格一次删完；超长段落对比时有上限。
- 换掉的项，原篇（「已有决定」那一栏）同步改，本篇对应的一行注明已换。

## Risks

- 换库会悄悄改掉行为：断行、字数口径、diff 的分段、JSON 的容错。用户看得见，不能只看测试通过。
- 多一个依赖，就多一份要跟着升级、要审的代码，体积和许可证也可能变。
- 表里的维护情况会过时，作者定的时候按当时的情况重查。
- astra 的部分判断是凭记忆，没有联网核实。Claude 查过它提到的仓库，改正了两处（Extism 的 Go SDK 是纯 Go；没有 `hashicorp/go-radix` 这个仓库）。没逐个核对的，是各库在功能细节上能不能替换。

## 相关笔记当场审计

- [契约审查](../../implemented/architecture/2026-11-06-extension-contract-review.md)：无关，那篇是扩展失败路径的修法。本篇是它施工前的另一项盘点，互不取代。
- 「已有决定」一栏链到的笔记：部分重叠。本篇引用它们；19、20、5、6 的现行事实已同步原篇，其余项落实后再同步。这些笔记是：
  - [讨论用工具查全书](../../implemented/feature/2026-10-15-talk-looks-up-and-notes-the-setting.md)
  - [模型层](../../implemented/architecture/2026-10-08-model-layer-on-goai.md)
  - [模型上限和用量](../../implemented/architecture/2026-10-15-model-limits-and-usage.md)
  - [按字符估 token](../../implemented/architecture/2026-10-08-token-estimate-by-character.md)
  - [M1 讨论](../../implemented/feature/2026-10-07-m1-discussion.md)
  - [纯 Go 编辑器](../../implemented/architecture/2026-10-07-pure-go-native-editor.md)
  - [两端对齐](../feature/2026-11-02-justified-prose-and-adjustable-spacing.md)
  - [纯文件存储](../../implemented/architecture/2026-10-08-m1-storage-without-index.md)
  - [导出 Word](../../implemented/feature/2026-10-16-export-to-word.md)
  - [备份](../../implemented/feature/2026-10-09-backup.md)
  - [本地原型](../../implemented/process/2026-10-12-local-prototype.md)
  - [扩展模块](../../implemented/architecture/2026-11-02-extension-modules.md)
  - [扩展 API](../../implemented/architecture/2026-11-03-extension-api.md)
  - [多语言](../../implemented/architecture/2026-11-02-i18n-on-mygo.md)
- 这期作者定下来、另开的五篇，部分重叠，互链；本篇的表照旧留着那几行：
  - [models.dev 后台拉取](../../implemented/architecture/2026-11-06-model-table-from-models-dev.md)接手第 3 条，已施工；
  - [文件监视扩展](../feature/2026-11-06-watch-files-extension.md)接手第 23 条；
  - [增量云同步](../feature/2026-11-06-incremental-cloud-sync.md)接手第 8 条引出的讨论；
  - [查阅到头时不逼模型作答](../feature/2026-11-06-talk-stops-without-forcing-an-answer.md)由第 1 条的调研引出；
  - [编辑器以后换到 mygo 的 richtext](2026-11-06-editor-on-mygo-richtext.md)接手第 18 条。
- [项目架构审查](2026-10-27-maintainable-project-review.md)：部分重叠，那篇管代码组织，不评依赖选择。本篇不重复它的内容。
- 没有完全被本篇吸收、要删或归档的笔记。
