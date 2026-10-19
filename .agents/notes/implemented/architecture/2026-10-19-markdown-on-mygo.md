# Agent Note: 回复和摘要的 Markdown 改用 mygo 的 ui/markdown

Status: implemented

## Problem

回复和摘要原来用自己写的一套 Markdown 渲染（`desk/markdown.go`，约 450 行）：goldmark 解析，`mdDraw` 把相邻的段落、标题、纯文本列表合成一个 `RichText` 好跨段选中，`liveMarkdown` 在流式时把写完的块和正在写的那块分开排版。代价写在[回复跨段选中与流式排版](../feature/2026-10-08-reply-selection-and-smooth-streaming.md)里：列表没有悬挂缩进，段落之间只能空一整行，引用、代码、表格处选区断开。

mygo v0.5.5 发布了 `ui/markdown`：`View` 画写完的 Markdown，`Stream` 画流式的，选区跨过所有块（含代码和表格），复制时按选中的块还原成文字。作者要求用它替换自己的实现。

同时发布的 `ui/richtext`（富文本编辑器）没有拿来换正文编辑器：它不给光标和选区在屏幕上的位置、不能控制滚动、没有行距和首行缩进、不能画 AI 提议的那种插在正文里的文字和自定义下划线，这些正文编辑器都要用。

## Decision

- `desk/markdown.go` 只剩一层薄壳：
  - `markdown` 调 `markdown.View`，`liveMarkdown` 调 `markdown.Stream`。流式的分块冻结、渐显由 mygo 做。
  - `markdownOptions` 给回复（`replyText`）和摘要（`digestText`）各一份 `markdown.Options`，存在 `App.md` 里，地址不变（mygo 按它认视图的状态）；调色板换了才重做 `Theme`。
  - `markdownTheme` 把 mygo 的主题换成软件的字号、行距、颜色（`fsRead`/`lhRead`、`fsNote`/`lhBody`、`a.pal.side`、`a.pal.line`、`rControl`）。标题统一 650 字重，只按 1.35、1.2、1.08 三级放大。
  - **代码块自己画**（`codeBlock`）：mygo 自带的代码块按钮写的是英文 Copy / Copied，它的文案库只有英文。我们画同样的边框和底色，左上写语言，右上一个「复制代码」图标按钮；代码正文仍用 mygo 的 `Body`。
- **回复里的链接不跳转**：`OnLink` 什么都不做，和原来一样。
- **摘要里的「[N]」**：`digestMarkdown` 用 mygo 的解析器找到正文里（不在代码、不在链接里）的「[N]」，换成「名字」，名字是一个指向 `inkgo-entry:N` 的 Markdown 链接，名字里的 Markdown 符号转成 HTML 实体；摘要的 `OnLink` 认 `inkgo-entry:` 打开那条设定。没有这条设定的编号原样留着。
- 去掉 goldmark 依赖。

## Alternatives considered

- **留着自己的实现，只把列表缩进、段距补上** — 最强的理由是它已经调过性能，行为都有测试，换掉要重写测试。没有用：缺的是跨元素选区，这要在 mygo 里加一套选区模型，自己补不划算；mygo 现在已经做了，留着两套 Markdown 只是多维护 450 行。
- **代码块用 mygo 自带的，等它出中文文案** — 最省事。没有用：界面全是中文，代码块头上一个英文 Copy 很扎眼；自己画只要十几行，以后 mygo 有了中文可以再换回去。
- **摘要的「[N]」在 mygo 解析后的文字里替换（自定义行内渲染）** — 不改原文，最干净。没有用：`ui/markdown` 没有自定义行内元素的钩子，只有链接的 `OnLink`，所以把编号改写成链接。
- **把正文编辑器也换成 `ui/richtext`** — 见 Problem 最后一段。

## Consequences

- **收益**：列表有悬挂缩进，有序列表、引用、表格、分隔线都按 Markdown 的样子排；整条回复从头拖到尾都能选，代码和表格也在里面；少了约 400 行和一个依赖。
- **代价**：
  - 摘要原文里的「[N]」每次画都要先解析一遍再改写。摘要短，量不出差别。
  - 代码块的样子跟着我们自己的 `codeBlock` 走，mygo 以后改了它的默认代码块，这里不会跟着变。
- **性能**：`BenchmarkStreamingFrame` 改成每来一个字推进一帧的时间、只画一帧（Tester 的时钟不走时，渐显动画会让 `Frame` 一口气跑满 20 帧，原来的写法量不准；原来还停在正文页、没开讨论，根本没在流式）。同一写法下旧实现每帧 2.9～3.0 ms，新实现 2.9～3.5 ms，内存分配少了大半。

## Verification

- `desk/markdown_test.go`：摘要把编号换成能点的名字、代码里的编号不动、名字里的符号不会变成 Markdown、流式写完后按完整的 Markdown 画、主题跟着调色板。
- `desk/app_test.go` 的 `TestReplyTextSelectsAcrossBlocksAndCopies`：标题和列表不露出 Markdown 符号；从第一段拖到最后一段，复制出来的有中间的列表；「复制代码」只复制代码；「复制全文」复制 Markdown 原文。
- `desk/digest_test.go` 的 `TestTheTalkShowsWhereItWasCompacted`：摘要里的列表画成圆点，设定的名字能点开。
- `go test ./desk -run xxx -bench StreamingFrame -benchmem` 复现上面的数。
