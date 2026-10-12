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

## 界面设计以 `prototype/` 里的原型为准

双击 `prototype/index.html` 就开：整个窗口都能点，内容是假的，正文不能打字，AI 只回固定的几句。纯 HTML/JS，没有依赖，不用构建。各文件管什么：

- `index.html`：样式。
- `book.js`：那本假书。
- `base.js`：状态、重画、各处都用的小零件。
- `write.js`：侧栏、工具栏、正文。
- `pane.js`：右栏的设定、讨论、伏笔。
- `pages.js`：讨论、检查、作品分类、书架四页。
- `dialogs.js`：对话框和备份。
- `app.js`：把窗口拼起来，和不管光标在哪都管用的按键、拖动。

1. 新的界面功能都以原型为基准：布局、色值、字号、文案、交互照它来；和现有代码冲突时先说出来，不要自己决定以哪边为准。
2. 原型里没有的界面功能，先在原型里做出来给作者看，确认后再开始写代码。
3. 改完原型跑 `npm run verify-prototype`（要本机有 Edge 或 Chrome）：先查下面第 4 条的规则，再在浏览器里把每个动作点一遍；加 `SHOT=<目录>` 会存下每一步的截图。动作或文案变了，`scripts/prototype/walk.ts` 里的步骤跟着改。平时碰不到的情况（出错的、软件外面动了稿子的）放进窗口上面「少见的情况」那个菜单，点一下就能摆出来看。
4. 颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度只写名字，不写数字；间距只用 `desk/tokens.css` 里 `--space`、`--space-board` 刻度上的数，半透明只用 `--alpha` 刻度上的数，和别的东西对齐的距离写成算式：
   - 原型写 `var(--…)`，名字都在 `desk/tokens.css`，全仓库只有这一份。
   - 代码用 `desk/theme.go` 里的同名常量（`fsBody`、`rControl`、`lhBody`、`iconMd`、`hControl`、`opacityDisabled`）和调色板里的阴影、遮罩（`a.pal.pop.on(e)`、`a.pal.scrim`）。`go test ./desk` 会核对常量和 `desk/tokens.css` 是否一致，并且检查界面代码里有没有直接写 `FontSize`、`Radius`、`LineHeight`、`glyph` 大小、按钮高度的数字、`ui.RGBA(` 或 `Shadow(`，`Padding`、`Gap`、`Margin` 和透明度的数在不在刻度上。
   - 要加一级，先在 `desk/tokens.css` 里加，再在 `desk/theme.go` 里加同名常量。
5. 原来的设计稿在 Design artifact <https://claude.ai/artifact/RAP7UK4pSfnGArETpVtPSj>，只作存档：不再更新，也不以它为准。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
