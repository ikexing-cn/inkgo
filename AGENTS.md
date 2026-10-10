# AGENTS.md

写作软件：纯 Go（`CGO_ENABLED=0`），UI 基于 `github.com/egoist/mygo`。编辑器在 `editor/`，可执行入口在 `cmd/poc/`。

## 提交前

```sh
go vet ./...
go test ./...
npm run verify-notes
```

三条都绿才算完成。界面改动在 `cmd/poc` 的布局测试里渲染截图核对（`SHOT=<path> go test -run TestLayout ./cmd/poc`）。

要一本有内容的书来试功能：`go run ./cmd/sample <目录>` 写出两本测试用的书（各 15 章，带设定、卷、章纲、伏笔，正文里故意埋了和设定对不上的地方，清单在 `cmd/sample/books/` 各文件开头），再用 `go run ./cmd/poc -work <目录>/<书名>` 打开。

## 界面直接在代码里改

不再先画设计稿。原来的 Design artifact <https://claude.ai/artifact/RAP7UK4pSfnGArETpVtPSj> 只作存档，不再更新，也不再以它为准；里面还没做的「导入稿件」「全书规划」可以拿来参考。

1. 界面改动直接改代码。改完渲染截图核对（`SHOT=<目录> go test -run TestShot ./desk`），把改了什么、截图在哪告诉作者；作者用测试书打开软件试，不合适再改。
2. 新页面、新的交互方式，动手前先用几句话和作者说清楚要做成什么样；有两三种做法拿不准时，列出来让作者选。
3. 界面上的字照这几条写：一眼看懂，用日常说法，不用术语；名称 2–4 个字，描述不超过 20 个字。
4. 颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度只写名字，不写数字；间距只用 `--space`、`--space-board` 刻度上的数，半透明只用 `--alpha` 刻度上的数，和别的东西对齐的距离写成算式：
   - 基准是 `desk/tokens.css`。代码用 `desk/theme.go` 里的同名常量（`fsBody`、`rControl`、`lhBody`、`iconMd`、`hControl`、`opacityDisabled`）和调色板里的阴影、遮罩（`a.pal.pop.on(e)`、`a.pal.scrim`）。
   - `go test ./desk` 会核对常量和 `tokens.css` 是否一致，并且检查界面代码里有没有直接写 `FontSize`、`Radius`、`LineHeight`、`glyph` 大小、按钮高度的数字、`ui.RGBA(` 或 `Shadow(`，`Padding`、`Gap`、`Margin` 和透明度的数在不在刻度上。
   - 要加一级或改一个值，先和作者说，再改 `tokens.css` 和 `theme.go` 里的同名常量。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
