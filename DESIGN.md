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
  phone-eye: "#cce8cf"
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
  phone-eye-dark: "#cce8cf"
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
    backgroundColor: "{colors.bg}"
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
    backgroundColor: "{colors.bg-dark}"
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

这份文件按 [DESIGN.md 格式](https://github.com/google-labs-code/design.md)（`version: alpha`）写：开头两条 `---` 之间是数值，给工具读；后面八节是怎么用这些数值、每一处界面都要守的规则。节的名字和先后是格式定的，用它的英文名，检查工具按名字认；节里面照常写中文。

## Overview

写长篇小说用的桌面软件。界面要安静，让位给正文：

- 只有纸和墨两种颜色，中间几级灰。别的颜色只在有事的时候出现：琥珀色是等作者处理的事，红色是删除和出错，绿色是成功。
- 正文的稿纸是主角，字最大、行距最宽；其余界面的字小而紧，让位给正文。
- 标题栏、侧栏、正文、右栏是同一张纸，靠细线分开；阴影只给浮在上面的东西（菜单、提示、对话框）。
- 浅色、深色各一套，同一个名字在两套里各有一个值。

这份文件和别处的分工：

- 这里只写做哪一处界面都要守的规则。每个界面具体的样子看 `prototype/` 的原型；一个功能自己的细节和来龙去脉，看 `.agents/notes/` 里它的笔记。和这里对不上，以这里为准。
- 这里只写作者定下来的。还在原型里等作者看的方案在 `.agents/notes/proposed/`，定了再写进来。
- 做界面的流程（先改原型再写代码、改完怎么验证）在 `AGENTS.md`；这份文件自己怎么改，只在下面的「怎么维护」。

### 怎么维护

每次动界面都要把这份文件整个读一遍，所以只写非写不可的。

1. 作者提了一条不只管一处的要求（「按钮应该统一右对齐」这种），先写进这里，再改原型和代码，同一次提交。只管一个功能的（某一页怎么排、各种情况怎么办、文案的原话），写进那个功能的笔记和原型，不写进来。
2. 能自动查的就加检查。这里不列测试和走查步骤的名字：不守规则，检查自己会报。
3. 规则有例外就写在规则下面，只写现有的那几处；例外是我定的而作者没说过的，标「待作者确认」。
4. 规则改了，旧的那句直接改掉，不留「原来是……」；来龙去脉在 `.agents/notes/` 的笔记里。
5. 一条规则只写在一处：放进它该在的那一节，别的节、`AGENTS.md`、代码的注释要提到它，只指过来。Do's and Don'ts 只放前面各节放不下的。
6. `npm run verify-design` 报的警告照实留着，不为了消掉警告改写法。

### 数值只写名字

颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度，在原型和代码里只写名字，不写数字。

- 值写在三处，同一个名字同一个值：这份文件开头的数值；`desk/kit/tokens.css`，原型直接用它（`var(--ink)`、`var(--fs-body)`、`var(--r-control)`）；`desk/kit/theme.go` 里的同名常量（`FsBody`、`RControl`、`LhBody`、`IconMd`、`HControl`、`OpacityDisabled`）和调色板（`a.pal.Ink`、`a.pal.Pop.On(e)`、`a.pal.Scrim`），代码用它。改一个值或者加一级，三处一起改，从这里改起。
- 名字的对应：`colors.ink` 是 `--ink`，`colors.ink-dark` 是 `.dark` 里的 `--ink`；`typography.body` 的字号是 `--fs-body`；`rounded.control` 是 `--r-control`；`spacing` 是 `--space` 那一排数。
- 格式里没有位置放的几样（阴影、遮罩、半透明的几级、行距的几级、禁用透明度），值只在 `desk/kit/tokens.css`，这里只写名字和用处。
- 每个名字管什么只写在这里；`desk/kit/tokens.css` 和 `desk/kit/theme.go` 只写值，不另写用处。

### AI 可以整个关掉

设置「AI」的最上面是「关闭 AI」：管本机的所有作品，默认开着，改了马上生效；关、开之前都问一句。关着时：

- 用到 AI 的功能和 AI 留下的痕迹整个不显示，不是变灰。
- 作品里的东西不删、不改（讨论、检查结果、AI 记录、模型服务和 key 都留着），重新开启后照旧。
- 不发送任何内容。

新做的界面，同时定下 AI 关着时它的样子：用到 AI 的，关着时整个不见。

### 模型看到的，作者也看得到

发给模型的东西，作者在界面上都找得到；不在作者背后往上下文里塞东西。

- 每条回复看得到这次 AI 查了什么、记了什么，发了哪几部分、各约多少 token，用了多少、花了多少。
- AI 改了作品里的东西（记进设定），修改记录里有一条。
- 讨论压缩过，压缩的地方看得到摘要。
- 每一次请求都记进「AI 记录」，失败、取消的也记；只记用量，发出去的内容不存。

### AI 写的字要作者采纳

AI 写的字，作者点了才进正文，没有例外：续写按 Tab；起草、改写和讨论里改正文，AI 都在正文里提议，点「采纳」才写进去。

- 提议和正文分得开：续写是灰字；提议里 AI 写的是绿色，要换掉的原文划红色删除线，后面跟「放弃」「采纳」。
- 没采纳的字不在正文里、不存盘，正文照常能打字。
- 采纳前存快照；采纳以后光标在 AI 写的字后面。
- 还有没决定的提议时离开作品，先问；关掉 AI，提议都丢掉。

### 等 AI 回复时照常用

讨论里 AI 在回复，别的地方照常用。只有两样要等它回完或者取消：再发一句、重试回复（一次只回一句），和切换作品。回复的那段讨论不在眼前时，右下角的提示条写「AI 正在思考…」，带「取消」。

## Colors

名字说的是用处，不是颜色；同一个名字，浅色的值在 `colors.<名字>`，深色的在 `colors.<名字>-dark`。

- **墨**：`ink` 正文、主按钮、链接；`ink-2` 稍弱的正文；`muted` 次要文字、图标；`faint` 提示、占位、伏笔的点线；`on-ink` 墨色按钮上的字。`primary` 就是 `ink`，是格式要有的名字，界面里不单独用。
- **纸**：`bg` 窗口的底，标题栏、侧栏、正文、右栏、设置都用它；`side` 纸上隔出来的一块：引的原文、摘要、大纲里卷的那一行和下面的托盘；`raised` 浮层、弹出框的底；`hover` 悬停；`active` 当前项、按下。
- **线**：`line` 分隔线；`border` 卡片、输入框的边；`control` 复选框、单选框、小标签的边；`dim` 占位的圆点、计划中的格子。
- **手机预览**：`phone-eye` 是预览里「护眼」那种背景；深浅两套是同一个值，手机上的背景不跟着软件换。
- **有事的时候**：`amber` 等作者处理的事；`warn` 警告的字（琥珀色在白底上太浅），`warn-bg`、`warn-bd` 是警告条的底和边；`danger` 删除、出错；`success` 成功；`sel` 选中的文字。

半透明（颜色的透明度、装饰用的 `opacity`）只用 `--alpha` 那几级，不另起名字；不能点的控件用 `--opacity-disabled`（代码里是 `pale(b)`）。

`npm run verify-design` 报的几对对比度不到 4.5:1（提示字、警告条、成功的字），改不改待作者定，先不动。

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
- 一列里每行都有的操作和次要的数，指针移到那一行上才出来，不常驻（章节树的字数、讨论的时间、每行的「编辑」「删除」）。点开着、键盘在里面时也在。
- 搜索和「新增」分开：搜索框只管找，新增是它旁边单独的按钮。
- 能滚动的内容不越过它上面固定的那一栏（标题栏、工具栏、对话框的标题），滚到那里就被裁掉。
- 菜单、浮层、对话框、提示条里的东西不比它们自己宽。
- 稿纸上说这一章的东西（章标题和章纲、AI 那一行）和正文一样宽，左边对齐。

## Elevation & Depth

平的界面：各栏之间只靠 `line` 的细线分开，不换底色。阴影和遮罩只有这几样，写名字：

- `--shadow-raise`：分段选择里选中的那一段。代码里 `a.pal.Raise.On(e)`。
- `--shadow-pop`：菜单、下拉、提示、待处理面板。代码里 `a.pal.Pop.On(e)`。
- `--shadow-dialog`：对话框。代码里 `a.pal.Dialog.On(e)`。
- `--shadow-focus`：聚焦的输入框。
- `--scrim`：对话框、设置窗口后面的遮罩。代码里 `a.pal.Scrim`。深色另有一个更深的：浅色那个和深色的底同色，盖上去看不出（待作者确认）。

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
- 勾选框和图标按钮都是见方的；标题栏两边的开关、侧栏顶上的加号也是图标按钮。

### 按钮

一排按钮靠右，主按钮在最右：「取消」之类次要的在它左边，文字样式的操作（「今日不再提醒」「复制详情」）在更左。对话框、浮层、提示条、卡片、表单都一样。

例外只有四种：

- 空页正中那一段的按钮，跟着那段话居中，主按钮仍在右。
- 跟在一句话后面的行内链接（「查看原文」「更改」），跟着那句话。
- 不做决定的那一个（「稍后再说」）单独留在最左，和其余的分开（作者 2026-10-13 确认）。
- 问「要不要保存」的三个按钮照系统的顺序。Windows 上是「保存」「不保存」「取消」，仍然靠右，主按钮「保存」在三个里最左，「取消」在最右（作者 2026-10-13 定）。macOS 上「不保存」单独在最左，「取消」「保存」靠右，主按钮在最右（待作者确认）。代码里这一行在行尾写 `// system order`。

### 文字按钮

没有边框和底色、只有字的按钮（「重新检查」「下一条」「查看原文」「稍后再说」）只有一种样子：

- `note` 字号，墨色的字，下面一条 `control` 色的细线。不可撤回的那一种（「清除已保存 key」）字用 `danger`。
- 指针移上去，出现 `active` 的底色，线变成字的颜色。
- 不用灰字、去掉线来表示次要：次要的靠位置（在主按钮左边），不能让人看不出可以点。
- 一排按钮里的「取消」「放弃」用普通按钮，不用文字按钮。

原型里是 `.link` 一个样式，软件里是 `a.pal.Ghost` 一个函数，别处不另写。

### 能点的都回应指针

凡是能点的（按钮、图标、列表行、标签、分段、文字按钮），指针移上去都有变化：

- 有边有底的换成 `hover` 的底色；只有底没有边的（AI 那一行和提议后面的「放弃」）深一级，用 `dim`；墨色实心的浅一级（`ink-2`）；`danger` 实心的淡一级；只有字或图标的变成墨色；文字按钮见上。
- 一组选项里已经选中的那一个不用再变。禁用的不回应。

### 动起来的地方

只有按下、出现、挪动三种，都在 0.25 秒以内：

- 按下：有边有底的按钮、列表行、标签，按下就换成 `active` 的底色，是点完以后当前项的样子；按钮再缩一点，松开弹回。
- 出现：菜单、下拉、提示、浮条从点它的那一边展开；对话框和遮罩淡入；提示条从右边滑进来。只在出现的那一刻动一次，收起不动。
- 挪动：收起、展开一栏，那一栏整块从它那一边的窗口边滑出去、滑进来，不淡，不分成几块各动各的；别的东西从原来的地方平移到新的地方，看得出是从哪里来、到哪里去的；正文的字不跟着拉宽。
- 系统设了减少动态效果，都直接到位。

### 对话框

- 只问一句的对话框（标题、一两句说明、一排按钮）用窄的一档，宽 360，不摊成扁扁的一条。
- 对话框的宽不超过高的 2.5 倍。
- 一个对话框上面再问一句，遮罩只盖一层：上面那一句的遮罩连页面带下面的对话框一起盖住，下面的对话框不再另盖。
- 关掉对话框，键盘回到打开它之前所在的地方：在正文里打着字，打开一个对话框再关掉，接着打的字还进正文，光标在原处。打开对话框和菜单的按钮按下时不拿走键盘（代码里 `KeepFocus`，原型里 `data-keep`），关的时候把焦点还回原处。别的按钮照常拿走键盘：正在改的名字，点别的按钮就算改完。

### 表单

- 要填的没填，主按钮是灰的。
- 填的会动到已有的东西，在表单下面用灰字说明。
- 填的用不了，在那一栏下面用红字写明为什么、怎么办，主按钮也是灰的。
- 按下去以前就知道的事都在表单里说，不等作者按了才弹提示条说不行。
- 要先在正文里选中文字才能做的，没选的时候按钮是灰的，和右键菜单一样；旁边那句「请先在正文中选中……」一直在，不跟着选区变。
- 几选一用一排单选圆点，几样里勾几样用一排勾选框，两种排法一样：每个只有它的字那么宽，横着排。不用分段：分段放进表单会撑满一行。分段只用在点了马上换样子、不用再按按钮的地方，也只有它的几段那么宽。

### 提示条

- 在右下角，底栏上面，新的排在最下面。
- 图标按提示条说的事配：做完了一件事（「已保存」「已加入设定」）带 `success` 的勾，出了错带 `danger` 的警告。告诉作者先做什么、或者这次没什么可做的不带图标：配了勾看着像做成了。
- 点了马上生效的处理（忽略、采纳、全部忽略），点错了在提示条上撤销。提示里说「可撤销」的，撤销一次就整个退回。

### 标题栏

- 标题栏自己占窗口最上面一行：最左是侧栏的开关，接着是菜单；正中换页；右边是这一页的工具、右栏的开关、窗口的三个按钮。macOS 上窗口的按钮是系统画在最左的红黄绿三个点，标题栏从它们右边排起，右端不再有窗口按钮。空着的地方按住拖窗口。它下面各栏不给窗口按钮留位置。
- 两边的开关每一页都在原处：这一页没有那一栏，它是灰的，不拿掉。右栏自己不另放收起的按钮。
- 只放这一页常用的几个，不另设「更多」：放不下的进菜单。时有时无的工具在左，总在的在右，谁出来、谁走了都挤不动它们；这一页用不了的是灰的。
- 顶上最深的当前项只有一处，是标签页（墨色、加粗、一道墨线）；换页和右栏的标签，正在的那个只换成墨色。
- 用过标题栏以后光标回到正文：开了菜单又收起、点了侧栏的开关以后，接着打的字进正文。点别处收起菜单的，焦点在点的那个地方。

### 菜单

- 菜单都是软件自己画的，不用系统的菜单。
- 每个命令在标题栏的菜单里有一行，带它的快捷键；这一页做不了的是灰的，不拿掉。
- 一组会跟着作品变多的项（卷、分类）收进下一级，菜单本身不跟着变长；下一级最多露十行，再多就滚。
- 有下一级的行，指针滑到就展开，不用点。开着一个菜单时，指针移到旁边的名字上就换过去；再点一下名字、按 Esc、点别处都收起。
- 菜单开着的时候位置不动：按方向键、Tab，或者指针移到别的行，它和旁边的下一级都留在原处。

### 侧栏

侧栏只列东西：顶上一行是作品名，右边一个加号；下面是章节树，讨论页上是讨论。别的都不放：换页在标题栏正中，备份在底栏，其余在「文件」菜单。

### 就地输入

在列表、树里直接改一行的名字（新建章节、新建文件夹、重命名）：

- 这一行还是这一行：箭头、缩进、行高都不变，只是名字那一段换成输入框，框里的字和上下各行的名字左边对齐。旁边不另放「确定」按钮。
- 一出来就有默认值，光标已经在里面：新建的章接着上一章的号，光标在号后面；别的全选上，打字就整个换掉。
- 回车确定；光标离开输入框（点了别处）也算确定，点的那一下照常生效。Esc 不改。空着确定等于没改。

### 保存

- 停笔片刻自动保存，Ctrl+S 马上保存。切到别的标签页、别的页面不问，先存了再走。没存的那一章，标签上关闭的叉换成一个圆点；标题栏不写「已保存」「未保存」。
- 关掉的标签页里还有没存的字，先问要不要保存（按钮照「按钮」的例外）；问着的时候不自动保存。

## Do's and Don'ts

文案：

- 要用书面语；不要口语化。
- 要一眼能看懂：名称 2–4 个字，说明不超过 20 个字。
- 要只在作者不读就会做错、或要知道去哪儿找的地方写说明（「key 保存在系统钥匙串中」）；不要写读了也不会改变怎么做的说明。控件说不清自己做什么，先改名字，不靠旁边加一句。
- 思考程度的几档要留英文（Default、Low、Medium、High），跟在模型后面（「DeepSeek/deepseek-flash · High」）；不要在前面加「思考：」「推理」。这一项在别处说起时叫「思考程度」。
- 软件叫 Inkgo（暂定），界面和文档里都写这个英文名，不另起中文名；不要把测试用的作品名当成软件名。
- 要用章标题指某一章（「第四章 钟表铺」），正开着的那一章写「本章」；不要另算「第 N 章」这种按位置编的号。标题在句子里加「」，单独摆着的不加；放不下就截断。讲距离的「已过 3 章」照旧按位置数。
- 要按 K、M、B 写 token 数，留一位小数（「9.6K」「1.7M」），不到一千的照写；填的时候也认这种写法。不要用「万」写 token，字数照旧写「万字」。
- 要按离今天多远写时间：今天、昨天直接写，今年别的日子写月-日，往年的写年-月-日，要几点就跟在后面（「今天 10:32」「10-06 21:40」「2025-10-06 21:40」）；不要单写「10-16 01:34」让作者自己算是哪天，也不要写「刚刚」。只说哪天的只写哪天。
- 要让文案里提到的地方和界面上的名字一字不差（设置左边那一栏叫「AI」，就写「设置中「AI」配置」）；不写界面上没有的「模型设置」。

留下的东西：

- 要当次删掉不再用的控件、样式、文案、代码；不要留着备用：留着的东西每次审查都要再看一遍。
