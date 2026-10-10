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

## 界面设计以 Design artifact 为准

设计稿在 Design artifact <https://claude.ai/artifact/RAP7UK4pSfnGArETpVtPSj>，文件都在 `project/` 下：入口是 `Main.dc.html`（总览），另有 `Pending`（待处理）、`Settings`（设置）、`Consistency`（一致性检查）、`Titlebar`（标题栏）、`Foreshadow`（伏笔）、`SettingPane`（右栏：设定、伏笔、讨论）、`Planning`（全书规划）、`Import`（导入稿件）、`Backup`（备份）、`Toast`（提示）各一份 `.dc.html`，`System`（设计规范）定字号、圆角、阴影、间距和常用组件，公共组件 `Sidebar`、`WindowControls`，画布排布在 `canvas.json`。它由原 Claude Design 项目 <https://claude.ai/design/p/b415efa2-67ae-4711-8873-eafbaebc192a> 转来，原项目只作存档，不再更新。

1. 新的界面功能都以这份设计为基准：布局、色值、字号、交互照它来；和现有代码冲突时先说出来，不要自己决定以哪边为准。
2. 设计稿里没有的界面功能，先和作者在这份 artifact 里把设计方向定下来，确认后再开始写代码。
3. 可以改这份 artifact：作者要求的修改，或作者确认后把设计稿和代码实现的偏差同步过去。改之前先读最新版本，改完告诉作者改了哪几页；不删页面，不动分享设置。
4. 颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度只写名字，不写数字；间距只用 `tokens.css` 里 `--space`、`--space-board` 刻度上的数，半透明只用 `--alpha` 刻度上的数，和别的东西对齐的距离写成算式：
   - 设计稿页面写 `var(--…)`，名字都在 `project/tokens.css`。发布前把最新版本拉到本地，跑 `npm run verify-design -- <本地的 project 目录>`。设计稿的工具在 `scripts/design/`：`normalize.ts` 把写出来的数字换成变量，`system.ts` 照 `tokens.css` 重新生成「设计规范」页，`render.ts` 渲染各页、和旧版本比对；用法写在各文件开头。
   - 代码用 `desk/theme.go` 里的同名常量（`fsBody`、`rControl`、`lhBody`、`iconMd`、`hControl`、`opacityDisabled`）和调色板里的阴影、遮罩（`a.pal.pop.on(e)`、`a.pal.scrim`）。`desk/tokens.css` 是设计稿那份的原样副本；`go test ./desk` 会核对常量和它是否一致，并且检查界面代码里有没有直接写 `FontSize`、`Radius`、`LineHeight`、`glyph` 大小、按钮高度的数字、`ui.RGBA(` 或 `Shadow(`，`Padding`、`Gap`、`Margin` 和透明度的数在不在刻度上。
   - 要加一级，先在「设计规范」页和 `tokens.css` 里加，再复制过来用。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
