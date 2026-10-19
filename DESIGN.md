---
version: alpha
name: Inkgo
description: 写长篇小说用的桌面软件。纸和墨两种颜色，浅色、深色各一套；名字以 -dark 结尾的是深色那一套。
colors:
  primary: "{colors.ink}"
  bg: "#ffffff"
  raised: "#ffffff"
  side: "#f7f7f5"
  hover: "#f3f3f1"
  active: "#ebebe8"
  line: "#ecece8"
  border: "#e4e4e0"
  control: "#c9c9c2"
  dim: "#dcdcd6"
  ink: "#1d1d1b"
  ink-2: "#3a3a37"
  muted: "#76766f"
  faint: "#a3a39c"
  on-ink: "#ffffff"
  amber: "#d9a441"
  warn-bg: "#fbf5e8"
  warn-bd: "#ecd9ac"
  sel: "#d6e4f7"
  danger: "#c4473a"
  warn: "#b7791f"
  success: "#2f8a5b"
  bg-dark: "#191919"
  raised-dark: "#222221"
  side-dark: "#1f1f1e"
  hover-dark: "#252524"
  active-dark: "#2c2c2a"
  line-dark: "#2a2a28"
  border-dark: "#3a3a37"
  control-dark: "#3a3a37"
  dim-dark: "#3a3a37"
  ink-dark: "#ededea"
  ink-2-dark: "#c9c9c4"
  muted-dark: "#a0a09a"
  faint-dark: "#6f6f69"
  on-ink-dark: "#191919"
  amber-dark: "#d9a441"
  warn-bg-dark: "#2a2518"
  warn-bd-dark: "#4a3d22"
  sel-dark: "#2b3a52"
  danger-dark: "#e4675b"
  warn-dark: "#d9a441"
  success-dark: "#5cb684"
typography:
  display:
    fontSize: 26px
    fontWeight: 700
  title:
    fontSize: 22px
    fontWeight: 700
  dialog:
    fontSize: 17px
    fontWeight: 700
  group:
    fontSize: 15px
    fontWeight: 600
  body:
    fontSize: 13.5px
    fontWeight: 400
  control:
    fontSize: 13px
    fontWeight: 400
  note:
    fontSize: 12.5px
    fontWeight: 400
  small:
    fontSize: 11.5px
    fontWeight: 400
  read:
    fontSize: 14.5px
    fontWeight: 400
    lineHeight: 1.8
  paper:
    fontSize: 17px
    fontWeight: 400
    lineHeight: 1.95
rounded:
  xs: 4px
  control: 6px
  card: 10px
  dialog: 12px
  round: 999px
spacing:
  "0": 0px
  "2": 2px
  "4": 4px
  "6": 6px
  "8": 8px
  "10": 10px
  "12": 12px
  "14": 14px
  "16": 16px
  "20": 20px
  "24": 24px
  "32": 32px
  "40": 40px
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
    typography: "{typography.control}"
    rounded: "{rounded.control}"
    padding: 14px
    height: 28px
  button-secondary:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.ink}"
    typography: "{typography.control}"
    rounded: "{rounded.control}"
    padding: 12px
    height: 28px
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "{colors.on-ink}"
    typography: "{typography.control}"
    rounded: "{rounded.control}"
    padding: 12px
    height: 28px
  button-text:
    textColor: "{colors.ink}"
    typography: "{typography.note}"
    rounded: "{rounded.xs}"
    padding: 6px
  button-text-hover:
    backgroundColor: "{colors.active}"
    textColor: "{colors.ink}"
  icon-button:
    rounded: "{rounded.control}"
    size: 28px
  icon-button-hover:
    backgroundColor: "{colors.active}"
  input:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.ink}"
    typography: "{typography.control}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 32px
  checkbox:
    rounded: "{rounded.xs}"
    size: 16px
  checkbox-checked:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
  segmented:
    backgroundColor: "{colors.active}"
    rounded: "{rounded.control}"
    padding: 2px
  segment:
    textColor: "{colors.muted}"
    typography: "{typography.control}"
    rounded: "{rounded.xs}"
    padding: 10px
    height: 26px
  segment-selected:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.ink}"
  chip:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.muted}"
    typography: "{typography.note}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 26px
  chip-selected:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
  tag:
    typography: "{typography.small}"
    rounded: "{rounded.xs}"
    height: 20px
  menu:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink}"
    typography: "{typography.control}"
    rounded: "{rounded.card}"
    padding: 4px
  menu-row:
    rounded: "{rounded.control}"
    padding: 10px
    height: 30px
  menu-row-hover:
    backgroundColor: "{colors.hover}"
  bar:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.ink}"
    height: 40px
  window-button:
    textColor: "{colors.ink-2}"
    height: 40px
  sidebar:
    backgroundColor: "{colors.side}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
  row-current:
    backgroundColor: "{colors.active}"
    textColor: "{colors.ink}"
  card:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink}"
    typography: "{typography.note}"
    rounded: "{rounded.card}"
  dialog:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.dialog}"
    padding: 24px
  paper:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.ink}"
    typography: "{typography.paper}"
  selection:
    backgroundColor: "{colors.sel}"
    textColor: "{colors.ink}"
  notice:
    backgroundColor: "{colors.warn-bg}"
    textColor: "{colors.warn}"
    typography: "{typography.control}"
    rounded: "{rounded.card}"
  notice-outline:
    backgroundColor: "{colors.warn-bd}"
  notice-button:
    backgroundColor: "{colors.warn}"
    textColor: "{colors.on-ink}"
  text-secondary:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.muted}"
  text-hint:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.faint}"
  text-danger:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.danger}"
  text-success:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.success}"
  divider:
    backgroundColor: "{colors.line}"
  outline:
    backgroundColor: "{colors.border}"
  outline-control:
    backgroundColor: "{colors.control}"
  dot-pending:
    backgroundColor: "{colors.amber}"
  placeholder:
    backgroundColor: "{colors.dim}"
  icon-xs:
    size: 12px
  icon-sm:
    size: 14px
  icon-md:
    size: 16px
  icon-lg:
    size: 24px
  button-primary-dark:
    backgroundColor: "{colors.ink-dark}"
    textColor: "{colors.on-ink-dark}"
  button-secondary-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.ink-dark}"
  button-danger-dark:
    backgroundColor: "{colors.danger-dark}"
    textColor: "{colors.on-ink-dark}"
  button-text-dark:
    textColor: "{colors.ink-dark}"
  button-text-hover-dark:
    backgroundColor: "{colors.active-dark}"
    textColor: "{colors.ink-dark}"
  icon-button-hover-dark:
    backgroundColor: "{colors.active-dark}"
  input-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.ink-dark}"
  checkbox-checked-dark:
    backgroundColor: "{colors.ink-dark}"
    textColor: "{colors.on-ink-dark}"
  segmented-dark:
    backgroundColor: "{colors.active-dark}"
  segment-dark:
    textColor: "{colors.muted-dark}"
  segment-selected-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.ink-dark}"
  chip-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.muted-dark}"
  chip-selected-dark:
    backgroundColor: "{colors.ink-dark}"
    textColor: "{colors.on-ink-dark}"
  menu-dark:
    backgroundColor: "{colors.raised-dark}"
    textColor: "{colors.ink-dark}"
  menu-row-hover-dark:
    backgroundColor: "{colors.hover-dark}"
  bar-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.ink-dark}"
  window-button-dark:
    textColor: "{colors.ink-2-dark}"
  sidebar-dark:
    backgroundColor: "{colors.side-dark}"
    textColor: "{colors.ink-dark}"
  row-current-dark:
    backgroundColor: "{colors.active-dark}"
    textColor: "{colors.ink-dark}"
  card-dark:
    backgroundColor: "{colors.raised-dark}"
    textColor: "{colors.ink-dark}"
  dialog-dark:
    backgroundColor: "{colors.raised-dark}"
    textColor: "{colors.ink-dark}"
  paper-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.ink-dark}"
  selection-dark:
    backgroundColor: "{colors.sel-dark}"
    textColor: "{colors.ink-dark}"
  notice-dark:
    backgroundColor: "{colors.warn-bg-dark}"
    textColor: "{colors.warn-dark}"
  notice-outline-dark:
    backgroundColor: "{colors.warn-bd-dark}"
  notice-button-dark:
    backgroundColor: "{colors.warn-dark}"
    textColor: "{colors.on-ink-dark}"
  text-secondary-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.muted-dark}"
  text-hint-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.faint-dark}"
  text-danger-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.danger-dark}"
  text-success-dark:
    backgroundColor: "{colors.bg-dark}"
    textColor: "{colors.success-dark}"
  divider-dark:
    backgroundColor: "{colors.line-dark}"
  outline-dark:
    backgroundColor: "{colors.border-dark}"
  outline-control-dark:
    backgroundColor: "{colors.control-dark}"
  dot-pending-dark:
    backgroundColor: "{colors.amber-dark}"
  placeholder-dark:
    backgroundColor: "{colors.dim-dark}"
---

# Inkgo 的界面规范

这份文件按 [DESIGN.md 格式](https://github.com/google-labs-code/design.md)（`version: alpha`）写：

- 开头两条 `---` 之间是数值（颜色、字号、圆角、间距、各控件的尺寸），给工具读。
- 后面八节是怎么用这些数值、每一处界面都要守的规则。节的名字和先后是格式定的，用它的英文名，检查工具按名字认；节里面照常写中文。

## Overview

写长篇小说用的桌面软件。界面要安静，让位给正文：

- 只有纸和墨两种颜色，中间几级灰。别的颜色只在有事的时候出现：琥珀色是等作者处理的事，红色是删除和出错，绿色是成功。
- 正文的稿纸是主角，字最大、行距最宽；其余界面的字小而紧，让位给正文。
- 层次靠底色的深浅和细线分开，阴影只给浮在上面的东西（菜单、提示、对话框）。
- 浅色、深色各一套，同一个名字在两套里各有一个值。

这份文件和别处的分工：

- 这里写每一处界面都要守的规则；`prototype/` 里的原型是每个界面具体的样子。两边对不上，以这里为准，改原型。
- 这里只写作者定下来的。还在原型里等作者看的方案在 `.agents/notes/proposed/`，定了再写进来。
- 做界面的流程（先改原型再写代码、改完怎么验证）在 `AGENTS.md`；这份文件自己怎么改，只在下面的「怎么维护」。

### 怎么维护

1. 作者提了一条不只管一处的要求（「按钮应该统一右对齐」这种），先写进这里，再改原型和代码，同一次提交。
2. 能自动查的就加检查，在那条规则下面写明谁在查；这样下一次新做的界面不守规则会自己报出来，不用作者再提一次。
3. 规则有例外就写在规则下面，只写现有的那几处；例外是我定的而作者没说过的，标「待作者确认」。
4. 规则改了，旧的那句直接改掉，不留「原来是……」；来龙去脉在 `.agents/notes/` 的笔记里。
5. 一条规则只写在一处：放进它该在的那一节，别的节、`AGENTS.md`、代码的注释要提到它，只指过来。Do's and Don'ts 只放前面各节放不下的。
6. 改完跑 `npm run verify-design`（要联网，第一次会下载格式自带的检查工具）：没有错误才算过；它报的警告照实留着，不为了消掉警告改写法。

### 数值只写名字

颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度，在原型和代码里只写名字，不写数字。

- 值写在三处，同一个名字同一个值：
  - 这份文件开头的数值；
  - `desk/tokens.css`，原型的页面直接用它，写 `var(--ink)`、`var(--fs-body)`、`var(--r-control)`；
  - `desk/theme.go` 里的同名常量（`fsBody`、`rControl`、`lhBody`、`iconMd`、`hControl`、`opacityDisabled`）和调色板（`a.pal.ink`、`a.pal.pop.on(e)`、`a.pal.scrim`），代码用它。
- 名字的对应：这里的 `colors.ink` 是 `--ink`，`colors.ink-dark` 是 `.dark` 里的 `--ink`；`typography.body` 的字号是 `--fs-body`；`rounded.control` 是 `--r-control`；`spacing` 是 `--space` 那一排数。
- 这个格式里没有位置放的几样（阴影、遮罩、半透明的几级、行距的几级、禁用透明度），值只在 `desk/tokens.css`，这里只写名字和用处。
- 每个名字管什么只写在这里；`desk/tokens.css` 和 `desk/theme.go` 只写值，不另写用处。
- 改一个值或者加一级，三处一起改，从这里改起。

检查：

- `go test ./desk` 的 `TestDesignMD` 核对这里的数值和 `desk/tokens.css` 一致，八节的名字和先后对不对；`TestTokens`、`TestSizeTokens` 核对 `desk/theme.go` 和 `desk/tokens.css` 一致；`TestUsesLiveInDesignMD` 查 `desk/tokens.css` 的各块、`desk/theme.go` 的调色板和尺寸常量里没有注释。
- `go test ./desk` 的 `TestNoLiteralSizes` 查界面代码里有没有直接写 `FontSize`、`Radius`、`LineHeight`、`glyph` 大小、按钮高度的数字、`ui.RGBA(` 或 `Shadow(`，`Padding`、`Gap`、`Margin` 和透明度的数在不在刻度上。
- `npm run verify-prototype` 先对着 `desk/tokens.css` 查原型的样式和脚本。

### AI 可以整个关掉

设置「AI」的最上面是「关闭 AI」：管本机的所有作品，默认不勾（AI 开着），改了马上生效。勾上以后：

- 用到 AI 的功能整个不显示，不是变灰：讨论、检查两页，右栏的讨论，引用到讨论，续写、改写、起草，设置「AI」里除了这个开关的全部，「提醒与检查」里检查的那一半（这一部分改叫「提醒」）。侧栏只有「正文」「大纲」「作品分类」。
- AI 留下的痕迹也不显示：设定里跳到讨论的「查看原文」，修改记录里的来源，快捷键表里 AI 的那几行（按了也不起作用），待处理里检查来的那几条。
- 作品里的东西不删、不改：讨论、检查结果、AI 记录、模型服务和 key 都留着，重新开启后照旧。
- 不发送任何内容。
- 关、开之前都问一句，标题是「是否关闭 AI」「是否开启 AI」。

新做的界面，同时定下 AI 关着时它的样子：用到 AI 的，关着时整个不见。

细处：

- 修改记录里讨论时 AI 记下的那几条整条不列，不只是不写来源。
- 快照不写「AI 改动前」，只写「改动前」。
- 正文里 AI 写的那几段不画下划线。
- 问句的标题末尾带问号，和「保存对……的修改？」一样；问句浮在设置上面，Esc 只关问句。
- 关的时候停在讨论页、检查页的，回到正文页，重新开启时还在正文页就回到那一页；右栏停在讨论的，显示设定，停在哪一页记着，重新开启后回到讨论。

检查：

- `npm run verify-prototype` 的「AI turned off」：关掉 AI 以后，把正文页、右栏、各个菜单、正文的右键菜单、选中后浮出的按钮、快捷键表、待处理、快照、作品分类、书架（有作品和没有作品）都看一遍，窗口里除了正文，不能有和 AI 有关的字（AI、讨论、检查、续写、改写、起草、摘要、模型、整理），提示和输入框里的字也算；按 Ctrl+J、Ctrl+L 不起作用。
- `go test ./desk` 的 `TestAIOffShowsNothingOfIt`：在软件里把同样的地方走一遍，查同样的字（读不到悬停提示和输入框的占位字），而且一个 AI 请求也没发出；`TestTurnTheAIOffAndOn` 查两个问句、关的那一刻停下的东西、关了再开作品文件夹不变；`TestTheAIComesBackToItsPage` 查重新开启后回到关的时候停的那一页；`TestDeskImportsNothingThatSends` 查界面的包引用不到会发送的包。

### 模型看到的，作者也看得到

发给模型的东西，作者在界面上都找得到；不在作者背后往上下文里塞东西。

- 讨论回复的上方，和「思考过程」一样折着一行，写这次 AI 查了什么、记了什么（「搜了全书「停电」，读了「第87章：停电」第 1–3 段」）。点开是每一项查到的内容；记进设定的那一条，点「查看设定」去看它。
- 回复下面的「AI 参考内容」列出这次发送的各部分各约多少 token、附了哪些设定（设定多到装不下时写「设定较多，本次只附目录」），和服务商算的实际用量。AI 查一回就多发一次请求，这里说的是最后一次，写出回复的那次；不止一次的，注明「这一轮共 N 次请求，这是最后一次。」
- AI 在讨论里记进设定的，修改记录里有一条，写着记在哪段讨论里。
- 讨论长了压缩过，压缩的地方隔一行「以上 N 轮已整理成摘要」，点开是摘要全文；只能看，不能改。压缩的时候，回复的地方写「正在压缩上下文…」。
- 摘要里提到设定，模型写的是设定的编号（「[2]」），给作者看换成那一条的名字，加「」，没有名字的取第一句的开头；点它在右栏打开那一条。摘要的排法和回复一样：模型写的列表画成圆点，不露出「-」。
- 用了多少也看得到：每条回复下面写这一轮所有请求一共用了多少 token 和按内置价格估算的花费（「· 42K token · 约 $0.08」，不到一分钱写「不到 $0.01」；这一轮用到的模型有一个不在价格表里，就只写 token）。设置「AI」的「本作品用量」按功能各一行（讨论、起草、改写、续写、检查），写请求次数、token、缓存命中多少和花费，没有价格的写「无价格」；下面「查看全部记录（N 次请求）」打开「AI 记录」。
- 「AI 记录」是本作品向模型发的每一次请求，一次一行，新的在上，失败、取消的也记：时间，功能和为了哪里（讨论写它的开头，起草、续写、检查写章名；删掉了的写「已删除的讨论」「已删除的章节」），token，花费，结果（完成、已取消、「失败：」和原因）。上面按功能筛选。点开一行是模型和思考程度、发了哪几部分各约多少 token、服务商算的实际用量。只记这些，发出去的内容不存，也就不显示；只能看，不能清零。

检查：

- `npm run verify-prototype` 的「what the AI looked at」「what was sent」「a long talk」「models and services」查原型：「a long talk」查摘要里的名字点了打开那一条、列表画成圆点，和发出去之前先压缩；「models and services」查「本作品用量」按功能的各行、「AI 记录」里失败和取消的行、点开的一行和筛选。
- `go test ./desk` 的 `TestTalkNotesTheSettingAsItReplies` 查软件里回复上方那一行：回复时展开、回复完折起、点开有查到的内容，「查看设定」打开记下的那一条，修改记录里那一条点「查看讨论」回到讨论；`TestWhatWasSent` 查「AI 参考内容」说的是最后一次请求、「这一轮共 N 次请求」，和回复下面这一轮一共的 token 数和花费；`TestTheWorkShowsWhatItSpent` 查「本作品用量」按功能的各行、没有价格的模型、「AI 记录」的先后、在哪、结果、筛选和点开的一行，关掉 AI 后都不显示；`TestTheTalkShowsWhereItWasCompacted` 查「以上 N 轮已整理成摘要」那一行和点开的摘要，摘要里设定的名字点了打开那一条，列表画成圆点；`TestCompactingShowsItself` 查压缩时写「正在压缩上下文…」，压缩完那一行就出来，作者刚发的话只出现一次。

### AI 写的字要作者采纳

AI 写的字，作者点了才进正文，没有例外：续写按 Tab；起草、改写和讨论里改正文，AI 都在正文里提议，点「采纳」才写进去。

- 起草、改写各开一段新的讨论，在正文旁边的右栏：第一句替作者说出要求（改写还引着选中的那段），标题写「起草：章名」「改写：那段的开头」，下面写「跟着「章名」」，附上的正文和设定都按这一章。想改哪里，在讨论里接着说。
- 别的讨论也能提议改正文。提议的那一章没开着，就在旁边开一个标签页，作者在哪还在哪。
- 提议和正文分得开：续写是灰字；提议里 AI 写的是绿色，要换掉的原文划红色删除线，后面跟「放弃」「采纳」。标题下面 AI 那一行写「本章有 N 处 AI 提议」，带「全部放弃」「全部采纳」。提议边写边显示，写着时「放弃」后面是「正在写…」，写完换成「采纳」；那一行的「全部采纳」写着时是灰的。写完那一刻按钮都不挪地方，想点「放弃」不会点到「采纳」。没采纳的字不在正文里、不存盘，正文照常能打字。
- 讨论里每处提议一张卡片：改哪一章，换掉一句、接在一句之后、接在章末还是换掉全章，多少字，后来怎样（等你决定、已采纳、已放弃、已被新的提议替换、正文已改动，提议已作废、作品关闭时已丢弃、关闭 AI 时已丢弃）。还没决定的带「在正文中查看」。
- 采纳前存快照，快照里写「采纳讨论「标题」里的提议」，一次采纳了几段讨论的就写几个标题；采纳算一步，撤销一次就退回去；采纳以后光标在 AI 写的字后面。
- 放弃一处还在写的提议，AI 跟着停下。采纳、放弃、作废了，作者下次在那段讨论里说话时告诉 AI。
- 同一章里新的提议和还没决定的提议改的是同一处，旧的换成新的。提议要换掉的原文被作者改了，提议作废，提示「1 处 AI 提议已作废」。
- 还有没决定的提议时离开作品（换作品、新建作品、回书架、关窗口），先问「还有 N 处 AI 提议没有处理」，点「丢弃并关闭」才走。关掉 AI，提议都丢掉，正文照旧。
- 点过「采纳」「放弃」「取消」，或者按 Esc 关掉改写要求，键盘回到正文。

检查：

- `npm run verify-prototype` 的「a sentence more」「a rewrite」「a proposal void, and one let go」「a draft」「leave with a proposal」「a proposal in another chapter」查原型：改写和起草开的讨论、边写边显示、新提议换掉旧的、刚写完马上点「采纳」不算、采纳以后按 Ctrl+Z、Ctrl+Y、快照写哪段讨论、改了原文提议作废、放弃还在写的提议 AI 停下、离开作品先问、提议改另一章在旁边开标签页。
- `go test ./desk` 的 `TestDraftAChapterThroughATalk`、`TestDraftOverWhatIsThere`、`TestRewriteASelectionThroughATalk`、`TestAProposalVoidAndOneLetGo`、`TestLeavingTheWorkAsksAboutProposals`、`TestAProposalInAnotherChapterOpensBeside`、`TestTakingTheDisksTextVoidsWhatItChanged` 在软件里查同样的几处，还查提议在采纳前正文和磁盘都不变、讨论的第一句和请求里写着跟着哪一章、放弃以后下次说话时 AI 被告知；`TestTurnTheAIOffAndOn` 查关掉 AI 时提议丢掉。`go test ./discussion` 的 `TestATalkOpenedToDraftKeepsToItsChapter`、`TestATalkOpenedToRewriteKeepsToItsChapter`、`TestAProposalFindsItsPlace`、`TestAProposalShowsAsItStreams` 查发给模型的和提议怎么找到它的位置。

### 等 AI 回复时照常用

讨论里 AI 在回复，别的地方照常用：换页、翻章节、写正文、看设定、改设置都行。只有两样要等它回完或者取消：再发一句、重试回复（一次只回一句），和切换作品。回复中途改了讨论模型，下一句起才用新的。

- 在回复的那段讨论不在眼前（看着别的讨论或讨论列表、在别的页、右栏收起或在别的标签、专注写作）时，右上角的提示条写「AI 正在思考…」，带「取消」；回到那段讨论，提示条收起，那里有它自己的进度。正在做的提示条没有关闭的叉，做完自己收起。

检查：

- `npm run verify-prototype` 的「the rest free while the AI replies」：回复中去「作品分类」再回来，回复照样到，在「作品分类」时右上角有「AI 正在思考…」，回来就没有；切换作品是灰的。
- `go test ./desk` 的 `TestTalkLeavesTheRestFree`：回复中换页、换到别的对话、打下一句、打开模型菜单都行；发送、重试回复、切换作品点了没反应；回复落在它自己的对话里；别的页、别的对话有「AI 正在思考…」，它自己的对话里没有。`TestTalkBesideShowsItsOwnProgress` 查正文旁边的讨论：右栏收起、看讨论列表、专注写作时有这个提示条，讨论回到眼前时没有。

### 设定随章节变化

一条设定可以从某一章或某一卷起变成另一段话。先后照侧栏里章节的顺序，章挪了位置，变化跟着章走。

- 正文页开着一章或一卷时，设定的列表和每一条都按那时的样子写；列表那一行的右边写它从哪起是这样（「第87章起」），那时还是原来的样子、后面才变的写「之后有变化」。别处按最后的样子写。
- 一条设定下面的「随章节的变化」按先后列出每一处，开着的那一章用的那一处标「本章适用」；那一章还是原来的样子，写「本章适用上面原来的样子。」。开着的章还没有变化的，有「从本章起有变化」（开着卷是「从本卷起有变化」），编辑框里先放这一条在它之前的样子。
- 一章（一卷）只有一处变化：保存到已经有变化的地方，替掉那一处。
- 变化所在的章进了回收站，这一处灰着，写「所在章节已删除 · 不起作用」；章恢复了就回来。
- AI 在讨论里推断出的变化进「待确认」，排在待确认的设定后面，写「关于 周祥 · 第87章起」；采纳才起作用，忽略就删掉，不进「已删除」。
- 模型看到的：起草、改写、续写、检查第 N 章，给第 N 章时的样子，后面的变化只说还有几处；普通讨论给原来的样子，下面每处变化一行「「第87章：停电」起：」；设定多到只给目录时，目录写最后的样子。「AI 参考内容」照发出去的写。
- 导出的设定先写原来的样子，下面每处变化一段「第87章起：……」，不起作用的不导。

检查：

- `npm run verify-prototype` 的「the change log」查 AI 建议的变化采纳后挂在那一条下面；「an entry that changes」查列表右边的字、「本章适用」、所在章节已删除、能从卷起、新记的一处从之前的样子起、保存和删除；「what was sent」查讨论发出去的是原来的样子加每处变化；「AI turned off」查关掉 AI 时变化的表单不提 AI。
- `go test ./desk` 的 `TestAnEntryChangesFromAChapter` 在软件里查同样的几处，和导出的几行；`go test ./workspace` 的同名测试查先后、卷、回收站；`go test ./writing` 的 `TestTheProseIsWrittenToTheSettingAsItStands` 查起草、检查用那一章时的设定，后面的变化只说几处；`go test ./discussion` 的 `TestATalkTellsAndNotesHowTheSettingChanges` 查讨论发出的几行、目录、记一处变化。

### 大纲：写的先后和故事的先后

大纲是一叠卡片，一张记一件事。卡片有两种先后：写在哪一章（写的先后），和故事里哪件事先发生（故事的先后）。两种先后都由作者自己排，软件不从正文里猜，也不用 AI。

- 卡片只有「发生了什么」必填；写在哪一章、哪条情节线、故事中的时间、说明都可以空着，以后再补。
- 「按章节」一章一行、一条情节线一列，卡片落在交叉的格子里；章名、章纲、卷纲在左边一列，就地改。没定写在哪一章的卡片在下面「未放进章节」。
- 「时间线」把同一叠卡片按故事的先后从左往右排开，一条情节线一行。故事中的时间可以不写，左右的先后就是故事的先后；挨着的、时间写得一样的几张并成一列，点一列的时间整列一起改。已放进章节的卡片是实线框，没放进的是虚线框。
- 一章里有比前面各章更早的事，这一章标「回溯」。
- 两种看法里，左边一列和表头滚动时都不动。停在格子上才显出「添加」；点一张卡片在对话框里改；把卡片拖到另一格就换到那一格的章、情节线或时间。新加的卡片挪到看得见的地方，标出来一会儿。
- 删掉一条情节线，它的卡片不删，只是不再属于哪条线。
- 正文标题下面写这一章「故事中」的时间（它的卡片里第一个写了时间的到最后一个），点了到时间线上看这一章的卡片。
- 搜索时工具栏那一句换成找到了多少；一个也没找到，整页只留「没有找到「…」」、这一种看法搜哪些字，和「清除搜索」。

检查：

- `npm run verify-prototype` 的「the outline by chapter」「the timeline」「the chapter in the story」查原型：添加、继续添加、改卡片、拖到另一格、改章纲、情节线的增删改和挪位置、改一列的时间、滚动时不动的那一列和表头、搜索的计数和什么都没找到的那一页、正文里「故事中」那一行跳到时间线。

## Colors

名字说的是用处，不是颜色；同一个名字，浅色的值在 `colors.<名字>`，深色的在 `colors.<名字>-dark`。

- **墨**：`ink` 正文、主按钮、链接；`ink-2` 稍弱的正文；`muted` 次要文字、图标；`faint` 提示、占位、伏笔的点线；`on-ink` 墨色按钮上的字。`primary` 就是 `ink`，是格式要有的名字，界面里不单独用。
- **纸**：`bg` 正文区的底；`side` 侧栏、右栏的底；`raised` 浮层、弹出框的底；`hover` 悬停、选中的标签；`active` 当前项、按下。
- **线**：`line` 分隔线；`border` 卡片、输入框的边；`control` 复选框、单选框、小标签的边；`dim` 占位的圆点、计划中的格子。
- **有事的时候**：`amber` 等作者处理的事；`warn` 警告的字（琥珀色在白底上太浅），`warn-bg`、`warn-bd` 是警告条的底和边；`danger` 删除、出错；`success` 成功；`sel` 选中的文字。

半透明（颜色的透明度、装饰用的 `opacity`）只用 `--alpha` 那几级，不另起名字；不能点的控件用 `--opacity-disabled`（代码里是 `pale(b)`）。

对比度不到 4.5:1（WCAG AA）的几对，`npm run verify-design` 每次都会连比值一起报出来，改不改待作者定：

- 提示和占位的字：`faint` 在 `bg` 上，浅色、深色都是。
- 警告条：`warn` 在 `warn-bg` 上；警告条里的主按钮，`on-ink` 在 `warn` 上。
- 成功的字：`success` 在 `bg` 上。

## Typography

原型用系统自带的中文黑体（PingFang SC、Microsoft YaHei、Noto Sans SC），软件用工具包默认的字体；规范只定字号、字重和行距。

- `display` 欢迎页的大字；`title` 页标题、设置窗口的标题；`dialog` 对话框的标题：这三级加粗（700）。
- `group` 页面里的分组标题、卡片标题（600）。
- `body` 列表项、表单项、说明的正文；`control` 按钮、输入框、下拉框里的字；`note` 次要说明、文字按钮；`small` 小标签、计数、快捷键。当前项、选中的那一段用 600，其余 400。
- `read` 要读的正文：讨论里的回复、快照的预览。
- `paper` 正文栏的稿纸，这是默认字号，作者可以在「外观」里调。

行距五级，名字是 `--lh-tight`（标题、按钮、单行的字）、`--lh-body`（列表项、表单项、说明）、`--lh-loose`（成段的说明、讨论气泡）、`--lh-read`（要读的正文）、`--lh-paper`（稿纸）。

## Layout

- 间距（`padding`、`margin`、`gap`）只用 `spacing` 里的数，不另起名字。`--space-board` 那几个更大的数只用于整页的大块留白。100 以上的是布局尺寸，不算间距。
- 和别的东西对齐的距离写成算式：一行的内边距加一级缩进，写 `calc(10px + 20px)`、`10+float32(depth)*20`，不写 `30`。
- 一块区域的功能操作放在它的右上角。
- 搜索和「新增」分开：搜索框只管找，新增是它旁边单独的按钮。
- 能滚动的内容不越过它上面固定的那一栏（标题栏、工具栏、对话框的标题），滚到那里就被裁掉。
- 菜单、浮层、对话框、提示条里的东西不比它们自己宽。`npm run verify-prototype` 每点一下都查。
- 稿纸上说这一章的东西（章标题和章纲、AI 那一行）和正文一样宽，左边对齐；查找条管整栏，是标签条下面横跨整栏的一条。`desk` 的 `TestTheHeadLinesUpWithTheProse` 在窄窗口和宽窗口各查一次。

## Elevation & Depth

平的界面：区域之间靠底色（`bg`、`side`、`raised`）和 `line` 的细线分开。阴影和遮罩只有这几样，写名字：

- `--shadow-raise`：分段选择里选中的那一段。代码里 `a.pal.raise.on(e)`。
- `--shadow-pop`：菜单、下拉、提示、待处理面板。代码里 `a.pal.pop.on(e)`。
- `--shadow-dialog`：对话框。代码里 `a.pal.dialog.on(e)`。
- `--shadow-focus`：聚焦的输入框。
- `--scrim`：对话框、设置窗口后面的遮罩。代码里 `a.pal.scrim`。深色另有一个更深的：浅色那个和深色的底同色，盖上去看不出（待作者确认）。

## Shapes

圆角五级，越大的东西越圆：

- `xs`：复选框、小标签、计数。
- `control`：按钮、输入框、下拉框、列表行。
- `card`：分组、卡片、菜单、提示。
- `dialog`：对话框、讨论气泡和输入框。
- `round`：圆点、细条。

## Components

各控件的底色、字、圆角、尺寸在开头数值的 `components` 里，深色的那一份名字以 `-dark` 结尾。原型和软件里同一种控件一样大。

### 尺寸

- 高度六级：`--h-control-sm` 按钮、图标按钮；`--h-control` 输入框、下拉框，以及和输入框并排的按钮；`--h-row` 菜单里的一行；`--h-seg` 分段选择里的一段、工具栏里的小按钮；`--h-tag` 小标签、计数；`--h-bar` 标题栏、工具栏、窗口按钮。
- 图标四级：`--icon-xs` 箭头、行内的小记号；`--icon-sm` 按钮和列表行里的图标；`--icon-md` 默认图标、勾选框；`--icon-lg` 空状态的大图标。
- 勾选框和图标按钮都是见方的；章节树标题旁的两个按钮也是图标按钮。

### 按钮

一排按钮靠右，主按钮在最右：「取消」之类次要的在它左边，文字样式的操作（「今日不再提醒」「复制详情」）在更左。对话框、浮层、提示条、卡片、表单都一样。

例外只有四种：

- 空页正中那一段的按钮，跟着那段话居中，主按钮仍在右。
- 跟在一句话后面的行内链接（「查看原文」「更改」），跟着那句话。
- 不做决定的那一个（「稍后再说」）单独留在最左，和其余的分开（作者 2026-10-13 确认）。
- 问「要不要保存」的三个按钮照系统的顺序：「保存」「不保存」「取消」。仍然靠右，主按钮「保存」在三个里最左，「取消」在最右。只有这一问照系统的顺序，别的对话框主按钮都在最右（作者 2026-10-13 定）。

检查：

- `npm run verify-prototype` 每点一下就把窗口里的按钮行量一遍：最后一个贴着这一行的右边，从第一个按钮起一个挨一个，主按钮后面没有别的按钮（问要不要保存的那一行反过来查：主按钮是第一个）。
- `go test ./desk` 的 `TestButtonsAtTheRight` 读界面代码：以按钮开头的 `ui.Row` 要写 `Justify(ui.End)`，按钮不在 `ui.Spacer(c)` 左边，`primary` 后面没有别的按钮。问要不要保存的那一行在行尾写 `// system order`，反过来查：三个按钮正好是「保存」「不保存」「取消」，全软件只有这一行。

### 文字按钮

没有边框和底色、只有字的按钮（「重新检查」「下一条」「查看原文」「稍后再说」）只有一种样子：

- `note` 字号，墨色的字，下面一条 `control` 色的细线。不可撤回的那一种（「清除已保存 key」）字用 `danger`。
- 指针移上去，出现 `active` 的底色，线变成字的颜色。
- 不用灰字、去掉线来表示次要：次要的靠位置（在主按钮左边），不能让人看不出可以点。
- 一排按钮里的「取消」「放弃」用普通按钮，不用文字按钮。

原型里是 `.link` 一个样式，软件里是 `a.pal.ghost` 一个函数，别处不另写。

### 能点的都回应指针

凡是能点的（按钮、图标、列表行、标签、分段、文字按钮），指针移上去都有变化：

- 有边有底的换成 `hover` 的底色；只有底没有边的（AI 那一行和提议后面的「放弃」）深一级，用 `dim`；墨色实心的浅一级（`ink-2`）；`danger` 实心的淡一级；只有字或图标的变成墨色；文字按钮见上。
- 一组选项里已经选中的那一个不用再变。禁用的不回应。

检查：

- `npm run verify-prototype` 每点一下，把窗口里能点的都查一遍：样式表里要有对得上它的 `:hover` 规则，指针要是手形。
- `go test ./desk` 的 `TestButtonsAnswerThePointer` 读界面代码：每个手写的 `ui.ButtonBase` 都要在画它的那几行里读 `Hovered()`。

### 对话框

- 只问一句的对话框（标题、一两句说明、一排按钮，如「保存对……的修改？」「全书替换」）用窄的一档，宽 360，不摊成扁扁的一条。
- 对话框的宽不超过高的 2.5 倍。`npm run verify-prototype` 每点一下都量。
- 一个对话框上面再问一句（如设置里「是否关闭 AI？」），遮罩只盖一层：上面那一句的遮罩连页面带下面的对话框一起盖住，下面的对话框不再另盖。`go test ./desk` 的 `TestAQuestionOverTheSettingsShadesOnce`、`npm run verify-prototype` 的「AI turned off」查。
- 关掉对话框，键盘回到打开它之前所在的地方：在正文里打着字，点齿轮、回收站、选区浮条、提示里的「打开设置」、讨论里的「摘取原文」或标题栏的菜单打开一个对话框，关掉以后接着打的字还进正文，光标在原处。打开对话框和菜单的按钮按下时不拿走键盘（代码里 `KeepFocus`，原型里 `data-keep`），关的时候把焦点还回原处。别的按钮照常拿走键盘：正在改的章名、设定这些，点别的按钮就算改完。

  检查：`go test ./desk` 的 `TestDialogsGiveTheKeyboardBack`（每个入口打开、按 Esc 或点「关闭」关掉）、`TestARenameEndsAtAnotherButton`；`npm run verify-prototype` 的「the title bar, and the sidebar put away on every page」。

### 表单

- 要填的没填，主按钮是灰的。
- 填的会动到已有的东西，在表单下面用灰字说明（作品分类的名称和已有的同名：「将修改已有的分类“人物”，现在的落点是“设定库”。」）。
- 填的用不了（新建作品的文件夹里已有文件，模型服务的 ID 和别的服务重了、地址不对、模型的数不合规矩，目标字数不是数），在那一栏下面用红字写明为什么、怎么办，主按钮也是灰的。
- 按下去以前就知道的事都在表单里说，不等作者按了才弹提示条说不行。
- 要先在正文里选中文字才能做的（右栏伏笔的「记为回收」「更换原句」），没选的时候按钮是灰的，和右键菜单一样；旁边那句「请先在正文中选中……」一直在，不跟着选区变，免得作者拖选时右栏跳动。
- 几选一用一排单选圆点（导出的「格式」「范围」，起草的「本章已有内容」），几样里勾几样用一排勾选框（导出的「内容」），两种排法一样：每个只有它的字那么宽，横着排。不用分段：分段放进表单会撑满一行，一长条灰底里只有左边几个字。分段只用在点了马上换样子、不用再按按钮的地方（检查的分类、待处理的全部和本章、外观的颜色），也只有它的几段那么宽。

检查：

- `npm run verify-prototype` 的「categories」「the shelf, where things go wrong」「models and services」「a goal for the day」「a draft」「export」「pay off with nothing picked」「a new sentence」。
- `go test ./desk` 的 `TestACategorySavesOnceFilledIn`、`TestANewWorkSaysWhereItGoes`、`TestModelLimitsInTheServiceForm`、`TestANewServiceSaysWhatIsWrongWhereItIs`、`TestSetTheDaysGoal`、`TestDraftAChapterThroughATalk`、`TestExportTheWork`、`TestDraftOverWhatIsThere`。

### 提示条

- 图标按提示条说的事配：做完了一件事（「已保存」「已加入设定」）带 `success` 的勾，出了错带 `danger` 的警告。
- 告诉作者先做什么、或者这次没什么可做（「各章均已检查，正文无新改动」）的不带图标：配了勾看着像做成了。

检查：`npm run verify-prototype` 的「the whole book checked」和复制全文那一步，`go test ./desk` 的 `TestAHintHasNoMark`。

### 标题栏

- 标题栏自己占窗口最上面一行，高 40（`--h-bar`）：左边是侧栏的开关和菜单，右边是窗口的三个按钮，中间空着的地方按住拖窗口。
- 它下面各栏（侧栏、工具栏、右栏的标签行）不给窗口按钮留位置。
- 侧栏的开关只这一个，每一页都在同一个地方。书架没有侧栏可收，它在那里是灰的，不拿掉。
- 专注写作时标题栏也收起，只剩正文和它的工具栏；窗口的三个按钮回到工具栏那一行的右端，工具栏给它们让出位置。菜单这时点不到，快捷键照常；按 F11 或工具栏上的「退出专注」回来。
- 侧栏顶上一行只放作品名，和工具栏一样高；题材写在点开的菜单里，正在写的这一部排第一。
- 菜单四个：「文件」「编辑」「视图」「帮助」。每个命令在菜单里有一行，带它的快捷键；这一页做不了的是灰的，不拿掉。
- 「编辑」最上面是「撤销」「重做」，没有可退、可重来的一步时是灰的。提示里说「可撤销」的改动（替换、采纳续写、采纳 AI 提议），撤销一次就整个退回。
- 工具栏上只放这一页常用的几个，不另设「更多」：放不下的进菜单。
- 开着一个菜单时，指针移到旁边的名字上就换过去；再点一下名字、按 Esc、点别处都收起。
- 用过标题栏以后光标回到正文：开了菜单又收起、点了侧栏的开关、从菜单打开的对话框关掉以后，接着打的字进正文。点别处收起菜单的，焦点在点的那个地方。

检查：

- `npm run verify-prototype` 的「the title bar, and the sidebar put away on every page」「the menus」。
- `go test ./desk` 的 `TestTheTitleBarHasItsOwnRow`、`TestTheSidebarIsPutAwayOnEveryPage`、`TestTheMenusDoWhatTheySay`、`TestMenuRowsOffThePageOfProse`、`TestAMenuOpensOnASlowClick`、`TestTheBarGivesTheCaretBack`、`TestShelfKeepsTheWorks`（作品的菜单）。

### 菜单

- 菜单都是软件自己画的，不用系统的菜单：样子和原型一致。
- 有下一级的行，指针滑到就展开，不用点。
- 菜单开着的时候位置不动：按方向键、Tab，或者指针移到别的行，它和旁边的下一级都留在原处。`go test ./desk` 的 `TestMenuStaysPut` 查。

### 就地输入

在列表、树里直接改一行的名字（新建章节、新建文件夹、重命名）：

- 这一行还是这一行：图标、缩进、行高都不变，只是名字那一段换成输入框；框里的字和上下各行的名字左边对齐。旁边不另放「确定」按钮。
- 一出来就有默认值，光标已经在里面：
  - 新建的章接着上一章的号（上一章是「第87章：停电」，框里就是「第88章：」），光标在号后面，等着写名字；什么都不写就叫「第88章」。
  - 没有号可接的叫「新章节」，新文件夹叫「新文件夹」，重命名是现在的名字：这三种全选上，打字就整个换掉。
- 回车确定；光标离开输入框（点了别处）也算确定，点的那一下照常生效。Esc 不改。空着确定等于没改。
- 写了什么，标题就是什么，给的号只是默认值：
  - 接着号写名字，得到「第88章：雨停」。
  - 把框里的字整个换掉（「楔子」「第一章：雨」），标题就是换成的字，前面不再加号。
  - 在给的号后面又写了一个带号的标题（「第88章：第九十一章 雪」），以后写的那个为准。
- F2 给正开着的那一章改名字，光标在哪里都行。这一行在折起来的卷里、在专注模式里，都先摆出来。
- 回车确定的是正开着的那一章（刚新建的就是），光标接着进正文。

检查：

- `npm run verify-prototype` 的「naming in the tree」：默认值、光标位置、和别的行对齐、点别处就确定、整个换掉的标题前面没有号、F2。
- `go test ./desk` 的 `TestLeavingTheNameSettlesIt`、`TestWritingSavesAsTheAuthorPauses`、`TestNameANewChapter`、`TestF2NamesTheOpenChapter`。

### 标签页和保存

- 标签页自己占一行，在顶栏下面，不和功能按钮挤在一起；标签之间有分隔线，正在看的那个和正文连成一片。
- 点章节树里的一章：已经开着就切过去，没开就新开一个标签页。
- 点章节树里的卷只是折起、展开，不开标签页；章节树只管左边的管理。卷纲从卷的右键菜单「编辑卷纲」打开。
- 每个标签页留着自己的撤销记录、光标和滚到哪；开着哪几个、正看哪个记在作品里，下次打开还在。
- 没存的那一章，标签上关闭的叉换成一个圆点，指针移上去换回叉；工具栏不写「已保存」「未保存」。
- 停笔片刻自动保存，Ctrl+S 马上保存。切到别的标签页、别的页面不问，先存了再走。
- 关掉的标签页里还有没存的字，先问：「保存」「不保存」「取消」（系统的顺序），主按钮是「保存」。问着的时候不自动保存；选「不保存」，这一章回到上次保存的样子。

检查：

- `npm run verify-prototype` 的「closing with words not saved」。
- `go test ./desk` 的 `tabs_test.go`：`TestChaptersOpenInTabs`（点章开标签、点卷不开）、`TestATabKeepsItsEditor`、`TestClosingAsksAboutUnsavedWords`（圆点、先问、问着不存、三个回答）、`TestTabsComeBackWithTheWork`。

## Do's and Don'ts

文案：

- 要用书面语；不要口语化。
- 要一眼能看懂：名称 2–4 个字，说明不超过 20 个字。
- 思考程度的几档要留英文（Default、Low、Medium、High），跟在模型后面（「DeepSeek/deepseek-flash · High」）；不要在前面加「思考：」「推理」。这一项在别处说起时叫「思考程度」。`go test ./desk` 的 `TestDraftAChapterThroughATalk` 和 `npm run verify-prototype` 的「a draft」查起草对话框。
- 软件叫 Inkgo（暂定），界面和文档里都写这个英文名，不另起中文名；不要把测试用的作品名当成软件名。
- 要用章标题指某一章（「第四章 钟表铺」），说的是正开着的那一章写「本章」；不要另算「第 N 章」这种按位置编的号，删一章、插一章它就和作者写的标题对不上了。标题在句子里加「」（「埋在「第四章 钟表铺」，已过 3 章」），单独摆着的不加（列表右边、选择框里那一行）；放不下就截断。讲距离的「已过 3 章」「相隔 3 章」「3 章前」照旧按位置数。`npm run verify-prototype` 的伏笔、检查、设定出处那几段查原型的说法，`go test ./desk` 的 `TestForeshadowBuryAndPay`、`TestCheckOnlyWhenAsked`、`TestSettingListAndPage` 查软件里的。
- 要按 K、M、B 写 token 数，K 是一千，留一位小数（「9.6K」「1.7M」），不到一千的照写；填上下文上限、最大输出时也认这种写法，保存后照这样写回（「128K」「1M」）。不要用「万」写 token，字数照旧写「万字」。`npm run verify-prototype` 的「what was sent」「models and services」查原型。
- 要按离今天多远写时间：今天、昨天直接写，今年别的日子写月-日，往年的写年-月-日，要几点就跟在后面（「今天 10:32」「昨天 23:15」「10-06 21:40」「2025-10-06 21:40」）；不要单写「10-16 01:34」让作者自己算是哪天，也不要写「刚刚」。只说哪天的只写哪天：讨论列表，书架上前天以前打开的作品，字数旁「最近 7 天」各天，最早的备份（「已存 12 份，最早 09-28」）。讨论里每一句话下面也这样写（「今天 00:50 · deepseek-flash · 8K token」）。`go test ./desk` 的 `TestWhenSaid` 查软件里的写法和最早的备份，`TestWhatWasSent` 查讨论里的，`TestSetTheDaysGoal` 查最近 7 天；原型里新做的事都记在「今天 10:40」，`npm run verify-prototype` 的备份、修改记录、「what was sent」「a goal for the day」几步查。
- 要让文案里提到的地方和界面上的名字一字不差：设置左边那一栏叫「AI」，指它一律写「设置中「AI」配置」（「请在设置中「AI」配置里选择续写和检查模型」），菜单里去那儿的一行叫「AI 配置…」；不写界面上没有的「模型设置」。`npm run verify-prototype` 的「no model to write with」查原型那一句，`go test ./desk` 的 `TestCopySaysWhereTheModelsAre` 查软件里的。

留下的东西：

- 要当次删掉不再用的控件、样式、文案、代码；不要留着备用：留着的东西每次审查都要再看一遍。
