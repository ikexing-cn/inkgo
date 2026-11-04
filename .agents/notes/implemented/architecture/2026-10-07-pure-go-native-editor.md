# Agent Note: 纯 Go 原生 UI，正文编辑器自己画

Status: implemented

## Problem

写作工具需要的不只是纯文本框：伏笔标记要跟着文字移动，人物/地名要在光标处弹补全，AI 预测要以灰字显示在光标后、按 Tab 才进正文，还要能做 diff 式审阅。mygo 自带的 `ui.TextArea` 只编辑纯文本，拿不到光标坐标，也画不了行内装饰。

编辑器已接入[正文页](../feature/2026-10-08-writing-page-and-chapters.md)。[AI 辅助写作](../feature/2026-10-08-ai-assisted-writing.md)改动了下文两处：灰字可以只在作者要时出现（正文页默认如此），正文页不开补全。

## Decision

- 全部代码是 Go，`CGO_ENABLED=0` 可编译；窗口与绘制用 `github.com/egoist/mygo` 的原生 UI（Windows 走 Direct3D 11，macOS 走 Metal，Linux 走 OpenGL），不用 WebView。
- 程序入口是 `cmd/inkgo`，只打开作品窗口；编辑器通过 `ui.Tester` 无窗口测试，预测替身只放在测试文件里。
- 正文编辑器在 `editor/` 自己实现，不用 `ui.TextArea`：用 `HandleInput` 取按键、文字与输入法组字，`TextCaret` 告诉系统光标位置，`ui.Shape` 排字形后自己按宽度断行、自己画光标、选区、标记、灰字与补全弹层。
- 文档模型 `editor.Doc` 只存纯文本（rune 下标）加一组 `Mark`；标记在每次替换时按位置映射移动，在标记边上打字不扩大标记，标记内文字被删光时置 `Orphaned` 而不是静默删除。`Version()` 记录文字和标记的变化；内部文字版本只使排版、名字匹配和预测失效。
- 段落缓存保留旧 rune 切片，先从头按序号比较，未命中再按距末尾的段数比较；回车、合段和粘贴后，未改的前后段仍复用整形。每次重排多扫一遍正文数段，只在缓存未命中时生成字符串。
- 整形时建立 rune 边界的字形索引，正文和手机预览按行切片；上升高度每份排版只计算一次。
  - 2026-11-04 起不再假定 `ui.Shape` 返回的簇号递增。Linux 上从右往左的文字（希伯来文、阿拉伯文）按视觉顺序返回，簇号在那一段里递减，按下标切片会取错字形；Windows 上没见过。
  - 簇号递增时照旧走索引和切片。
  - 不递增时（`editor/visual.go`）：
    - 断行按逻辑宽度累计；
    - 每行另排一份视觉布局并缓存，光标、点击、选区、标记、悬停、预览都用它；
    - 一段逻辑范围跨过方向边界时，画成几块。
  - 测试用手造的字形序列（`editor/visual_test.go`），不靠机器上装的字体。没有组字或灰字插入时，显示借用原文，光标移动不重排。
- 行和范围用二分查找；名字重叠按输入名单优先，用扫线合并。名字框每次排版计算一次，命中和绘制共用；标记、选区和查找的遮盖仍即时判断。
- 预测状态放在 `editor/predict.go`。滚动更新仍在绘制回调，保持输入法光标报告的时序；标记添加统一走 `Doc.Add`，撤销记录用 `pushMarkStep`。
- AI 预测只显示、不写入：`Predictor` 的结果作为灰字插在显示层，按 Tab 才作为一次可撤销的编辑写入，并打 `MarkAIAccepted` 标记留下来源（`Options.GhostNote` 给出标记的备注，正文页记模型名）。`Options.PredictOnDemand` 打开时，只在 Cmd+J 或 `RequestPrediction` 时预测，光标在哪儿都行；关着时停止输入、光标在段尾才预测。补全候选只来自传入的已确认实体，不调用模型；正文页不传实体，补全不出现。
- 改写建议和起草也在编辑器里：`Propose(a, b, text)` 把原文划掉、新文着色显示在原文后，文字一变就失效，`AcceptProposal` 一步替换并打 `MarkAIAccepted`；`BeginDraft` / `DraftAppend` / `EndDraft` 让 AI 流式写入，期间只读，结束时整段打 `MarkAIDraft`，连同被覆盖的原文算一步撤销。2026-10-15 起起草也是提议：`ProposeDraft(a, b)` 开一个空提议，`ProposeMore` 接上流来的字，`AcceptProposal` 按提议记着的种类打 `MarkAIAccepted` 或 `MarkAIDraft`；`BeginDraft` 那三个删掉，见[AI 写的字等作者采纳，作品记下用量](../feature/2026-10-15-ai-text-waits-and-usage-is-kept.md)。
- 标记有三种：伏笔（`MarkForeshadow`）、AI 采纳（`MarkAIAccepted`）、AI 起草（`MarkAIDraft`）。替换正好覆盖一个标记的文字时，标记跟着新文字走。AI 起草标记例外：作者在它里面改字，改动的部分脱离标记，前后剩下的各自留着。
- 正文页用到的能力：
  - `Options.Prose`：首行缩进两字只在显示层，粘贴时去掉行首空白，Tab 不插入"　　"；
  - `Placeholder`：空文档的提示；
  - `OwnMenu`：设了，右键菜单由调用方自己画，编辑器只告诉它在窗口的哪里要菜单（见[自己画的菜单](../feature/2026-10-12-own-menus.md)）；原来是 `Menu`，调用方往系统菜单里追加项；
  - 查找替换：`Doc.Find`、`SetFound` 高亮、`Replace`、`ReplaceAll`。连续的替换在撤销记录里合成一步（`undoStep.joined`）。

## Alternatives considered

- **业务用 Go，正文编辑器单独一个 WebView 窗口内嵌 CodeMirror 6** — 装饰层、补全弹窗、行内灰字、并排 diff 都有成熟实现，几天就能做出来。但用户要求纯 Go、不走 Web；且三平台 WebView 行为不一致，mygo 近期提交还在修 WebView 崩溃。
- **保留独立的编辑器演示入口** — 最强的理由：不建作品就能试输入法、补全和灰字。没有用：正文页和编辑器的无窗口测试覆盖这些能力，演示的窗口、预测替身和专用布局测试增加了另一套入口。
- **直接用 `ui.TextArea`** — 撤销、输入法、长文本性能都现成。但它是纯文本，没有光标坐标和行内装饰，三个核心功能都做不了。
- **换 Wails、Tauri 或 Electron** — 生态比只有几周历史的 mygo 成熟得多。但它们都把界面交给 Web 技术，与纯 Go 的要求冲突。

## Consequences

- **收益**：单个 Go 二进制（去掉调试信息约 15 MB，见[模型层用 GoAI](2026-10-08-model-layer-on-goai.md)），交叉编译 Windows/macOS/Linux 不需要 C 工具链；编辑器每个行为都在自己代码里，可用 `ui.Tester` 无窗口测试（输入法组字、Tab 采纳与撤销、补全、点击定位）。
- **代价与已知上限**：中文断行、光标、选区、撤销、滚动、无障碍都要自己维护；目前还没有无障碍文本暴露，超长章节每次编辑会重新断全部行。mygo 处于 0.2.x、基本单人维护，API 变动或停更时需要重访这个决定。2026-10-10 从 v0.2.15 升到 v0.3.7 时就变了两处：元素从 `*ui.Element` 改成值 `ui.Element`；`*ui.Context` 只在画一帧的时候有效，帧外调用（输入回调里读写剪贴板、打开链接）不报错也不生效，所以编辑器另存一份 `c.Services()` 来读写剪贴板，`Invalidate` 仍可在任何时候调用。Linux 上回车/退格是否以按键事件到达、fcitx5/ibus 输入法兼容性尚未在真机验证。

## Verification

`go test ./...` 覆盖 `editor/doc_test.go`（标记映射、断行避头尾、补全匹配、撤销合并、全部替换一次撤销、`Prose` 不存缩进）、`editor/ui_test.go`（无窗口驱动真实界面）与 `cmd/inkgo` 的布局测试（讨论、正文、大纲和手机预览）。`SHOT=<目录> go test ./cmd/inkgo` 保存作品窗口的截图，`SHOT=<目录> go test ./desk -run Shot` 保存界面各状态的截图。`GOOS=darwin|linux CGO_ENABLED=0 go build ./cmd/inkgo` 可通过。
