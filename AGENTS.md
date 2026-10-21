# AGENTS.md

Inkgo（暂定名）是一个写作软件：纯 Go（`CGO_ENABLED=0`），UI 基于 `github.com/egoist/mygo`。编辑器在 `editor/`，可执行入口在 `cmd/inkgo/`。

## 提交前

```sh
npm run verify
```

四十秒左右，都绿才算完成。它同时跑哪几项写在 `scripts/verify.ts` 开头，各项也能单独跑。要留意的几项：

- `npm run verify-ui`：工具包自带的检查 `mygo vet`，查界面代码有没有把一帧里的东西（`*ui.Context`、`ui.Element`）留到帧外；后台任务要叫界面重画，用起头那一帧取的 `c.Services()`。
- `npm run verify-prototype` 要本机有 Edge 或 Chrome。
- `npm run verify-design` 要联网，只在 `DESIGN.md` 改了还没提交时跑；`npm run verify -- --design` 强制跑。

界面改动渲染截图核对：`SHOT=<目录> go test ./cmd/inkgo` 截整个窗口的几处布局，`SHOT=<目录> go test ./desk -run Shot` 截界面的各个状态。Go 测试里的 `SHOT` 都是目录，每个测试往里写自己命名的图。

要一本有内容的书来试功能：`go run ./cmd/sample <目录>` 写出两本测试用的书（各 15 章，带设定、卷、章纲、伏笔，正文里故意埋了和设定对不上的地方，清单在 `cmd/sample/books/` 各文件开头），再用 `go run ./cmd/inkgo -work <目录>/<书名>` 打开。

## 界面：规则在 `DESIGN.md`，样子在原型

动界面之前先读 `DESIGN.md`，里面是每一处界面都要守的规则。每个界面具体的样子在 `prototype/` 的原型里。

双击 `prototype/index.html` 就开：整个窗口都能点，内容是假的，正文能打字（不存盘，刷新就回到原样），AI 只回固定的几句。纯 HTML/JS，没有依赖，不用构建。各文件管什么：

- `index.html`：样式。
- `book.js`：那本假书。
- `base.js`：状态、重画、各处都用的小零件。
- `write.js`：侧栏、工具栏、正文。
- `pane.js`：右栏的设定、讨论、伏笔。
- `pages.js`：讨论、检查、作品分类、书架四页。
- `dialogs.js`：对话框和备份。
- `app.js`：把窗口拼起来，标题栏和它的菜单，和不管光标在哪都管用的按键、拖动。

1. 新的界面功能照原型做：布局、色值、字号、文案、交互；原型和现有代码冲突时先说出来，不要自己决定以哪边为准。
2. 原型里没有的界面功能，先在原型里做出来给作者看，确认后再开始写代码。
3. 改完原型跑 `npm run verify-prototype`，它在浏览器里把每个动作点一遍；加 `SHOT=<目录>` 会存下每一步的截图。动作或文案变了，`scripts/prototype/walk.ts` 里的步骤跟着改。平时碰不到的情况（出错的、软件外面动了稿子的）放进窗口上面「少见的情况」那个菜单，点一下就能摆出来看。
4. 作者提了一条不只管一处的要求（「按钮应该统一右对齐」这种），照 `DESIGN.md` 的「怎么维护」写进去。

## 重要改动必须留笔记

1. 非平凡改动（改了行为、架构、跨文件契约、流程与工具链、测试策略、落盘/网络/配置格式）前，按 `.agents/skills/write-notes-like-deepseek/SKILL.md` 写或更新 `.agents/notes/` 里的笔记；机械性小改（样式、格式化、打标、不改行为的补丁）直接提交。
2. 写之前先检索 `.agents/notes/` 里的同主题旧笔记：有归属就地更新；新想法先放 `proposed/`，落地时随同代码改动转 `implemented/`；新方案彻底取代旧决策时，同批归档旧篇并在新篇标明取代关系。
3. 被放弃的方案先写它最强的理由，再解释为什么不用。
4. 代码和笔记同一次提交。

笔记校验脚本在 `scripts/`，由 Node 直接运行 TypeScript（Node ≥ 22.18），不装 tsx 或任何 npm 依赖。
