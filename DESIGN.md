---
version: alpha
name: 写作软件
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

# 写作软件的界面规范

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
- 流程（先改原型再写代码、改完怎么验证）在 `AGENTS.md`。

### 怎么维护

1. 作者提了一条不只管一处的要求（「按钮应该统一右对齐」这种），先写进这里，再改原型和代码，同一次提交。
2. 能自动查的就加检查，在那条规则下面写明谁在查；这样下一次新做的界面不守规则会自己报出来，不用作者再提一次。
3. 规则有例外就写在规则下面，只写现有的那几处；例外是我定的而作者没说过的，标「待作者确认」。
4. 规则改了，旧的那句直接改掉，不留「原来是……」；来龙去脉在 `.agents/notes/` 的笔记里。
5. 改完跑 `npm run verify-design`（要联网，第一次会下载格式自带的检查工具）：没有错误才算过；它报的警告照实留着，不为了消掉警告改写法。

### 数值只写名字

颜色、字号、圆角、阴影、行距、图标大小、控件高度、禁用透明度，在原型和代码里只写名字，不写数字。

- 值写在三处，同一个名字同一个值：
  - 这份文件开头的数值；
  - `desk/tokens.css`，原型的页面直接用它，写 `var(--ink)`、`var(--fs-body)`、`var(--r-control)`；
  - `desk/theme.go` 里的同名常量（`fsBody`、`rControl`、`lhBody`、`iconMd`、`hControl`、`opacityDisabled`）和调色板（`a.pal.ink`、`a.pal.pop.on(e)`、`a.pal.scrim`），代码用它。
- 名字的对应：这里的 `colors.ink` 是 `--ink`，`colors.ink-dark` 是 `.dark` 里的 `--ink`；`typography.body` 的字号是 `--fs-body`；`rounded.control` 是 `--r-control`；`spacing` 是 `--space` 那一排数。
- 这个格式里没有位置放的几样（阴影、遮罩、半透明的几级、行距的几级、禁用透明度），值只在 `desk/tokens.css`，这里只写名字和用处。
- 改一个值或者加一级，三处一起改，从这里改起。

检查：

- `go test ./desk` 的 `TestDesignMD` 核对这里的数值和 `desk/tokens.css` 一致，八节的名字和先后对不对；`TestTokens`、`TestSizeTokens` 核对 `desk/theme.go` 和 `desk/tokens.css` 一致。
- `go test ./desk` 的 `TestNoLiteralSizes` 查界面代码里有没有直接写 `FontSize`、`Radius`、`LineHeight`、`glyph` 大小、按钮高度的数字、`ui.RGBA(` 或 `Shadow(`，`Padding`、`Gap`、`Margin` 和透明度的数在不在刻度上。
- `npm run verify-prototype` 先对着 `desk/tokens.css` 查原型的样式和脚本。

## Colors

名字说的是用处，不是颜色；同一个名字，浅色的值在 `colors.<名字>`，深色的在 `colors.<名字>-dark`。

- **墨**：`ink` 正文、主按钮、链接；`ink-2` 稍弱的正文；`muted` 次要文字、图标；`faint` 提示、占位、伏笔的点线；`on-ink` 墨色按钮上的字。`primary` 就是 `ink`，是格式要有的名字，界面里不单独用。
- **纸**：`bg` 正文区的底；`side` 侧栏、右栏的底；`raised` 浮层、弹出框的底；`hover` 悬停、选中的标签；`active` 当前项、按下。
- **线**：`line` 分隔线；`border` 卡片、输入框的边；`control` 复选框、单选框、小标签的边；`dim` 占位的圆点、计划中的格子。
- **有事的时候**：`amber` 等作者处理的事；`warn` 警告的字（琥珀色在白底上太浅），`warn-bg`、`warn-bd` 是警告条的底和边；`danger` 删除、出错；`success` 成功；`sel` 选中的文字。

半透明（颜色的透明度、装饰用的 `opacity`）只用 `--alpha` 那几级，不另起名字；不能点的控件用 `--opacity-disabled`（代码里是 `pale(b)`）。

对比度不到 4.5:1（WCAG AA）的几对，`npm run verify-design` 每次都会报出来，改不改待作者定：

- 提示和占位的字：`faint` 在 `bg` 上，浅色 2.54:1，深色 3.48:1。
- 警告条：`warn` 在 `warn-bg` 上 3.35:1；警告条里的主按钮，`on-ink` 在 `warn` 上 3.64:1。
- 成功的字：`success` 在 `bg` 上 4.28:1。

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

## Elevation & Depth

平的界面：区域之间靠底色（`bg`、`side`、`raised`）和 `line` 的细线分开。阴影只有这几级，写名字：

- `--shadow-raise`：分段选择里选中的那一段。代码里 `a.pal.raise.on(e)`。
- `--shadow-pop`：菜单、下拉、提示、待处理面板。代码里 `a.pal.pop.on(e)`。
- `--shadow-dialog`：对话框。代码里 `a.pal.dialog.on(e)`。
- `--shadow-focus`：聚焦的输入框。
- `--scrim`：对话框、设置窗口后面的遮罩。代码里 `a.pal.scrim`。

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

- 高度六级：按钮、图标按钮 28（`--h-control-sm`）；输入框、下拉框，以及和输入框并排的按钮 32（`--h-control`）；菜单里的一行 30（`--h-row`）；分段选择里的一段、工具栏里的小按钮 26（`--h-seg`）；小标签、计数 20（`--h-tag`）；标题栏、工具栏 40（`--h-bar`）。
- 图标四级：12（`--icon-xs`）箭头、行内的小记号；14（`--icon-sm`）按钮和列表行里的图标；16（`--icon-md`）默认图标；24（`--icon-lg`）空状态的大图标。
- 定过的两样：勾选框 16 见方，图标按钮 28 见方，章节树标题旁的两个也是。

### 按钮

一排按钮靠右，主按钮在最右：「取消」之类次要的在它左边，文字样式的操作（「今日不再提醒」「复制详情」）在更左。对话框、浮层、提示条、卡片、表单都一样。

例外只有三种：

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

- 有边有底的换成 `hover` 的底色；墨色实心的浅一级（`ink-2`）；`danger` 实心的淡一级；只有字或图标的变成墨色；文字按钮见上。
- 一组选项里已经选中的那一个不用再变。禁用的不回应。

检查：

- `npm run verify-prototype` 每点一下，把窗口里能点的都查一遍：样式表里要有对得上它的 `:hover` 规则，指针要是手形。
- `go test ./desk` 的 `TestButtonsAnswerThePointer` 读界面代码：每个手写的 `ui.ButtonBase` 都要在画它的那几行里读 `Hovered()`。

### 对话框

- 只问一句的对话框（标题、一两句说明、一排按钮，如「保存对……的修改？」「全书替换」）用窄的一档，宽 360，不摊成扁扁的一条。
- 对话框的宽不超过高的 2.5 倍。`npm run verify-prototype` 每点一下都量。

### 标题栏

2026-10-13 在原型里做成这样，作者当天确认，软件同一天照它改了。

- 标题栏自己占窗口最上面一行，高 40（`--h-bar`）：左边是侧栏的开关和菜单，右边是窗口的三个按钮，中间空着的地方按住拖窗口。
- 它下面各栏（侧栏、工具栏、右栏的标签行）不给窗口按钮留位置。
- 侧栏的开关只这一个，每一页都在同一个地方。书架没有侧栏可收，它在那里是灰的，不拿掉。
- 专注写作时标题栏也收起，只剩正文和它的工具栏；窗口的三个按钮回到工具栏那一行的右端，工具栏给它们让出位置。菜单这时点不到，快捷键照常；按 F11 或工具栏上的「退出专注」回来。
- 侧栏顶上一行只放作品名，和工具栏一样高；题材写在点开的菜单里，正在写的这一部排第一。
- 菜单四个：「文件」「编辑」「视图」「帮助」。每个命令在菜单里有一行，带它的快捷键；这一页做不了的是灰的，不拿掉。
- 「编辑」最上面是「撤销」「重做」，没有可退、可重来的一步时是灰的。提示里说「可撤销」的改动（替换、采纳续写或改写、起草），撤销一次就整个退回。
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

软件里重命名时还不能把名字全选上（工具包没有这个接口），光标在名字末尾。

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
- 思考程度的几档要留英文（Default、Low、Medium、High）；不要在前面加「思考：」。
- 软件还没有名字，界面和文档里要写「写作软件」；不要把测试用的作品名当成软件名。

别的：

- 不要在原型和代码里直接写颜色、字号、圆角、阴影的数字，写名字（见 Overview）。
- 不要把一排按钮放在左边或者中间（见 Components 的「按钮」）。
- 不要让能点的东西看不出能点：文字按钮只有一种样子，指针移上去都有变化（见 Components 的「文字按钮」「能点的都回应指针」）。
- 不要用系统的菜单。
- 不要留不再用的控件、样式、文案、代码：当次删掉，不留着备用，留着的东西每次审查都要再看一遍。
