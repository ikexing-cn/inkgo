# Agent Note: 每条界面规则、每个名字的用处只写在一处

Status: implemented

## Problem

作者 2026-10-15 问：「我们的 AGENTS.md 和 DESIGN.md 会不会太多冗余导致模型漂移？检查一下」。查下来篇幅不算长，`DESIGN.md` 点名的测试、走查步骤、token 和代码里的名字也都在；问题是同一件事写了两三份，有几份已经对不上了：

- 按钮的高度：`DESIGN.md`、原型的 `.btn`、代码的 `primary` / `secondary` / `danger` 都用 `--h-control-sm`（28）；`desk/tokens.css` 和 `desk/theme.go` 的注释却说 `--h-control`（32）是给「按钮」的，28 是给「图标按钮、文字按钮」的。2026-10-12 按钮照原型改成 28 时，两处注释没跟着改。照注释做新按钮，会做成 32。
- 「按钮」一节写「例外只有三种」，下面列了四条。
- `AGENTS.md` 界面那节的标题是「以原型为准」，到第 4 条才说 `DESIGN.md` 和原型对不上以 `DESIGN.md` 为准。
- `desk/tokens.css` 开头说「代码里的 desk/tokens.css 是这一份的原样副本」，它自己就是那一份。
- 全套检查要多久：`AGENTS.md` 写四十秒左右，我在本机记的是半分钟。

还没对不上、但迟早会的：

- `DESIGN.md` 怎么写、怎么改：`AGENTS.md` 界面第 4 条的四个小项，和 `DESIGN.md` 开头的说明加「怎么维护」，几乎一句一句地各写一遍；`verify-design` 出现三次。
- 每个名字管什么写在三处：`DESIGN.md`、`desk/tokens.css` 的注释、`desk/theme.go` 的注释。
- `DESIGN.md` 说「只写名字，不写数字」，「尺寸」一节却写着 28、32、30……，「定过的两样」又和开头的 `components` 重复，这些数字没有测试在查；Colors 里对比度的比值是 `verify-design` 每次都会打印的。
- Do's and Don'ts 九条里有五条复述前面各节（后面写着「见 Overview」「见 Components」），又都写成「不要……」，这五条就比别的规则重了一倍。

模型读到对不上的几份，每次挑哪一份不一定，这就是漂移。

## Decision

一条规则、一个名字的用处只写在一处，别处要提到它只指过去。这收窄了[界面规范写在 DESIGN.md](2026-10-13-design-md.md) 里的分工：`DESIGN.md` 管规则，也管它自己怎么改；`AGENTS.md` 只管做界面的流程。

- `DESIGN.md` 怎么写、怎么改，只写在它的「怎么维护」里。`AGENTS.md` 界面那节只留两处：节首一句「动界面之前先读 `DESIGN.md`」；第 4 条「作者提了一条不只管一处的要求，照 `DESIGN.md` 的「怎么维护」写进去」。格式、以谁为准、`verify-design`、`TestDesignMD` 不再在 `AGENTS.md` 里复述。第 1 至 3 条的号不变，笔记里「界面那节第 2 条」的引用照旧对得上。
- 「怎么维护」加第 5 条：一条规则只写在一处，别的节、`AGENTS.md`、代码的注释要提到它只指过来；Do's and Don'ts 只放前面各节放不下的。
- 每个名字管什么只写在 `DESIGN.md`：
  - `desk/tokens.css` 的各块里、`desk/theme.go` 的调色板和尺寸常量里只写值，不写注释。`go test ./desk` 的 `TestUsesLiveInDesignMD` 查，写在「数值只写名字」那条规则下面。
  - `--page`、`--r-window-win`、`--shadow-board` 只属于原型的页面（窗口外面的底、窗口的外框），不是软件的界面，`DESIGN.md` 里没有。它们单放一块，块前一句注释说是做什么的；检查只查块里面，不查块前面。
- `DESIGN.md` 的正文只写名字：
  - 「尺寸」一节不再写数字；勾选框和图标按钮只说是见方的，多大看开头的 `components`。
  - 对比度只列是哪几对，比值看 `verify-design` 的输出。
- Do's and Don'ts 只留文案的四条和「不留不再用的东西」。
- 对不上的几处：
  - 按钮的高度照 `DESIGN.md`、原型和代码，写错的两处注释随用处注释一起删了。
  - 「例外只有四种」。
  - `AGENTS.md` 界面那节的标题改成「规则在 `DESIGN.md`，样子在原型」，谁为准只在 `DESIGN.md` 说。
  - `desk/tokens.css` 开头重写：只写值，用处在 `DESIGN.md`，由测试和另两处核对。
  - 检查要多久只写在 `AGENTS.md`。
- `AGENTS.md` 的「提交前」不再逐条列 `npm run verify` 跑的几项（`scripts/verify.ts` 开头就写着），只留要留意的：`verify-ui` 那条提醒、原型的走查要浏览器、`verify-design` 要联网。
- `AGENTS.md` 不再写 Design artifact 的链接。仓库里只有这一处提到它，写它只是为了说「别用」；它只作存档的决定在[界面以仓库里能点的原型为准](2026-10-12-local-prototype.md)。
- 「标题栏」一节开头那句来历（哪天做的、谁确认的）删掉：照「怎么维护」第 4 条，来历写在笔记里。
- 顺带删掉 `desk/tokens.css` 里旧画板时期没有人用的 `--fs-board-title`、`--fs-board-h2`、`--r-window`、`--shadow-inset`；`TestDesignMD`、`TestSizeTokens` 里放过它们的例外只剩 `--r-window-win`。见[原型和代码共用一份基准](../architecture/2026-10-09-design-color-tokens.md)。

## Alternatives considered

- **用处留在 `desk/tokens.css` 的注释里，`DESIGN.md` 只指过去** — 改值的人一眼就能看到用处，不用翻另一份文件；原型和代码也都直接读 `desk/tokens.css`。没有用：用处是规则的一部分（哪种控件用哪一级高度），`DESIGN.md` 是给作者看、给格式的检查工具读的规范，用处拆到 CSS 注释里，规范就不全了；`desk/theme.go` 那份英文注释也还是要么删、要么再写一遍。
- **几份都留着，加检查让它们一致** — 每处都能就地读到，对不上也查得出来。没有用：散文和散文没法机械比对，能核的只有值；要查就得给每条用处定一个能比的写法，比删掉麻烦。
- **Do's and Don'ts 整节留着当速查** — 作者反复说过的几条（按钮靠右、只写名字）在末尾再提一遍，读到最后的人不会漏。没有用：复述让这几条显得比「就地输入」「标签页」这些同样要守的规则更重；都写成「不要……」，反而把不要的做法摆到眼前。这些规则本来就在它们该在的那一节。
- **`AGENTS.md` 里原型各文件管什么的那张表也去掉** — 那是目录结构，严格说也是复述，列一下目录就看得到。没有用：它说的是哪份文件管哪一块界面，光看文件名猜不出来（右栏在 `pane.js`，四个页面在 `pages.js`）；留着能让模型直接找到要改的文件，只占八行。

## Consequences

- **收益**：
  - 改一条规则、一个用处只要改一处，不会有哪一份落下。
  - `AGENTS.md` 每次都会读进来，现在短了，界面那节只剩做界面的流程。
  - 名字的用处要是又写回 `desk/tokens.css` 或 `desk/theme.go`，`TestUsesLiveInDesignMD` 会报。
- **代价与已知上限**：
  - 读 `desk/tokens.css`、`desk/theme.go` 时看不到每个名字是做什么的，要翻 `DESIGN.md`。
  - `DESIGN.md` 正文不再写高度、图标的数字，要知道具体多大，看开头的数值或 `desk/tokens.css`。
  - 「一条规则只写一处」在 `DESIGN.md`、`AGENTS.md`、笔记之间只能靠写的时候守，没有检查；有检查的只是名字的用处那一部分。

## Verification

- `go test ./desk -run TestUsesLiveInDesignMD`：通过；在 `desk/tokens.css` 的一块里、`desk/theme.go` 的调色板里和尺寸常量旁各加一条注释，三处都报。
- `npm run verify`（`DESIGN.md` 改了，带上 `verify-design`）：六项全过，32.6 秒；新工作区第一次跑、缓存还是冷的时候 73 秒。
