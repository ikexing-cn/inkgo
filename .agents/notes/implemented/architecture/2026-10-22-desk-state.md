# Agent Note: `desk` 的内部状态：书模型、打开的章节、页面和命令

Status: implemented

后续细化：[阅读入口、页面权限和服务表单状态](2026-10-27-reading-entry-and-owned-state.md)。其余候选整理见[维护可读性的建议](../../proposed/architecture/2026-10-27-maintainable-project-review.md)。

## Problem

写作页和检查页共用一本书的内容，章节树、字数、标记、纲和检查结果需要一致。打开的章节要跨标签保存编辑器、草稿和磁盘冲突状态。

页面进出、导航和命令也有共同规则。各处分别维护，会漏掉保存、刷新、快捷键提示或关 AI 时的收束动作。

这是[理一遍代码](../../implemented/architecture/2026-10-21-code-reorganization.md)的第 6 批。

## Decision

### 书模型：`desk/book`

- `Book` 持有 `*workspace.Store`、一棵章节树、字数、标记、设定、大纲、各章的纲和存着的检查结果。它不引用界面工具包、`editor` 或 `desk`。
- 书内容的读写经过 `Book`。它写盘后更新自己的副本，`Version()` 随内容的读取和写入递增；重复读树没有变化、打字后字数没有变化时不增加。
- 作品打开时读一次。写作页进入时读树和字数，每 `book.CheckEvery` 看盘时读树；换章时读标记；检查页进入时读树和检查；讨论交回设定变化时重读设定。`nodeTitle` 只查内存中的树。看盘频率见[少看几次磁盘](../../proposed/architecture/2026-10-13-fewer-looks-at-the-disk.md)。
- 坏的 `outline.json` 由 `PlanErr` 保留错误，不阻止打开作品；大纲页显示错误并按看盘间隔重读。
- 正文保存后更新这一章的字数，并从盘上重读这一章的标记。存储层会夹取标记位置和重写引文，不能直接把传入值当成写入结果。检查和章纲写后只更新对应章节。
- 新建、改名、移动、删除和恢复后重读树；删除到回收站和恢复还重读字数。卡片和情节线写后重读大纲，设定修订和删分类后重读设定。
- 页面持有自己的编辑、选择、滚动和队列状态。检查队列不持有检查结果副本，`Keep`、`EnqueueStale`、`AfterPause`、`DueChapters` 和 `PrunableChapters` 接收 `Book` 中的结果。
- `App.Store` 负责正文读取、讨论、AI 记录、快照、回收站列表、`work.yaml` 和偏好等未在 `Book` 缓存的内容。内容守卫覆盖 desk 及其子包（Book 自身除外），按 Go 类型识别受保护的 Store 方法，不依赖变量名。
- `Book` 只在界面线程用。后台讨论和检查拿 `Store`，界面线程接收结果后刷新 `Book`。

### 派生数据

- `cached[K,V]` 只保留上一组键和值。`get` 在键变化时重建，`builds` 供帧测试计数。缓存放在使用者旁边，不放进 `Book`。
- 键覆盖实际依赖：书指针和版本、编辑器和正文版本、章 ID、AI 开关、本机显示偏好、提醒间隔、忽略列表版本、备份失败状态和分钟、配色及页面。
- 缓存的有伏笔、检查项、检查列表、灵感、待办分组、人名链接、文件夹字数、检查标注、分类计数、全书字数和右栏角标。
- 调用方不改缓存值。排序、过滤或追加前复制；`checkDetail`、`markDoubts` 和待办列表使用 `slices.Clone`。

### 打开的章节和纲草稿

- `openChapter` 持有编辑器、已保存版本、盘上时间戳、快照时间、外部变化、丢失状态和这一章的 `book.OutlineDraft`（`desk/book/outline_draft.go`）。看盘、保存、冲突处理、存为新章和标记转换是它的方法。方法接收 Store、Book 和时间，不接收 App。
- `writeView.open` 按 ID 保存各标签的 `*openChapter`，`node` 指向当前标签。`cur()` 从映射取得当前章；没有打开的章节时给空值。偏好恢复的标签在第一次显示时创建编辑器。
- 字号、续写方式和 AI 开关改变时设给每个已打开的编辑器。只检查当前章的磁盘状态，切回标签时也比较一次。`changed` 随章节走，文件夹摘要 `gists` 留在写作页。
- 写作页当前章和大纲页编辑格各持有一个 `book.OutlineDraft`。两者都在停顿 `book.SaveAfter` 后存，换章、离开页面和关作品时立即存，写入经过 Book。`book.SaveAfter` 和 `book.CheckEvery` 位于 `desk/book/timing.go`，均为可缩短的变量；`snapEvery` 位于 `desk/save.go`。

### 页面和导航

- `pages.go` 的 `a.pages` 是绑定方法的函数表。每项有 `enter`、`leave`、`view` 和导航表项；计数在导航表项里。空动作表示无事可做。大纲页直接绑定结构体的方法。
- `turnPage` 按离开、进入、存偏好、更新 `shown` 的顺序执行。`View` 在构建界面前调用它，`pageTurn` 在构建页面前再调用一次，接住同一帧里的换页。`shown` 保证一次换页只进出一次。
- 写作页进入读树和字数，离开保存并退出检查；大纲页进入标记重读，离开收束纲和情节线；检查页进入读取并清理检查。跳到正文的动作自己打开目标章节，`liveChapter` 判断是否处于写作页。
- `navigation.go` 的 `tabEntry` 统一页面、右栏和设置分页的名称、图标、AI 归属、作品要求和计数。关 AI 后，页面回到正文，无作品时回讨论欢迎页；右栏显示设定而保留选中的标签；设置回到第一项。提醒与检查的关闭态名称是「提醒」。
- 待办项带 `open func(ui.Services)`。点击时先收列表，再交给它当前帧的服务。缓存闭包只保留宿主和条目数据，不保留帧内对象。`here` 供本章过滤和排序。

### 命令、状态和后台任务

- 命令表持有名称、快捷键、帮助行、帮助分组、AI 归属、可用条件和动作。标题栏、正文菜单、标签菜单和帮助从表取名称及提示。标签菜单的目标由该行决定。
- `View` 在页面准备之后、编辑器构建之前按表分派当前页面的快捷键。编辑器自己的键只在表中供菜单和帮助读取，桌面不重复分派；剪贴四项只供菜单提示。输入框的 Ctrl+Enter 和局部 Esc 留在各自控件。
- `kit.Status`（`desk/kit/status.go`）持有错误、连错次数、重试动作、完成通知和提示；toast 读取它。`App.SetError` 和 `warn` 转交状态处理。
- `jobs` 运行后台任务，把返回函数按交回顺序放入带锁队列；下一帧在界面线程执行。交回不等待界面，关作品等待备份时也不阻塞后台交回。备份在交回前关闭自己的 `done`。
- 讨论、导出、选目录、备份和设定刷新使用 `jobs`。讨论保留命名、取消和一次只运行一个任务的规则；`checkRunner` 独立管理检查取消和交回。
- `New` 登记关 AI 的收束动作，顺序是讨论、提议、检查、起草和改写、设定待确认、状态。`turnAI` 另管开关存储、开启时加载配置、页面去留和编辑器开关。

### 右栏和 AI 记录

- `settingRoute` 持有列表、待确认或已删除的底层页面，以及条目页、条目或变化表单、修改记录的覆盖层。每帧先校正已不存在的目标；表单文字留在 `docView`。
- `recordsView` 用 `ui.List` 只构建可见行，键是记录下标。「‹ 本作品用量」和筛选固定在上方。筛选下标在重读记录或改变筛选时重建。

### 设置、记录和备份的归属

- `settingsView`（`settings.go`）持有设置窗口的开关、所选页和 AI 开关确认；`backupView` 只持有备份任务、策略、位置和备份界面状态。
- `recordView`（`record.go`）持有读入的 AI 记录、是否查看明细、功能筛选、展开行、筛选下标和列表滚动。记录的加载与重置、打开列表和筛选下标计算由它自己的方法处理。
- `openPrefs/showPref` 选择设置页，进入 AI 页时从 Store 读一份记录；`closePrefs` 关闭窗口并取消 AI 确认，但保留所选页和同一作品的草稿。关闭按钮、Esc/遮罩和重新启动路径使用同一收尾入口。
- `resetPrefs` 在换书、回书架和关闭应用时清空设置、记录及服务表单；磁盘上的记录、模型配置和 key 不变。备份仍按原先的关书顺序等待任务、执行备份，再由作品重置清空备份视图。
- 这次分离不新增包或 Host：状态各有主人，跨模块协调留在 App。结构体定义与加载、清理放到对应功能文件，避免只换字段前缀。

### 各功能的结构体

- `phone.Panel`（`desk/phone/phone.go`）持有分页、排版偏好、正文缓存、阅读位置和命中几何。右栏顶行由 `settingColumn` 绘制，`commands.go` 的 `openPhone` 负责打开右栏和保存偏好。
- `exportdialog.Dialog`（`desk/exportdialog/export.go`）持有打开时的正文快照、范围、笔名、缩进和导出计划，放在 `App.exporting`，作品重置时清空。
- `outline.Page`（`desk/outline/plan.go`、`plan_layout.go`）持有索引、布局缓存、纲草稿、卡片和情节线编辑、搜索和滚动。页面表直接绑定 `Enter`、`Leave`、`View`，正文旁调用它的 `ChapterStory`。
- 三个页面各在自己的包里声明 `Host`，只列它调用的方法。`desk/host.go` 的 `appHost` 显式持有 `*App`，用大写方法实现这三个接口，`App` 不增加导出方法。`New` 绑定适配器；大纲和导出重置时重新绑定。手机保留跨作品的排版缓存。写作、讨论、右栏、伏笔、待办和提议留在 App。
- `appHost` 给出 `Colors/PhoneColors` 配色、`Content` 书内容、`Notices` 状态、`ShelfPrefs/SaveShelf` 本机偏好，以及 `Chapter` 的章 ID、编辑器和有效开章标志。章 ID 在写作页外为空。只有导出 Host 使用 `Workspace` 取得存取层和路径；手机和大纲 Host 只取 `WorkPath`。
- `Flush` 保存导出前的正文；`SyncPlanOutline` 把已保存的纲同步到同 ID 当前写作草稿的 Text、Saved、Seen；`OpenWriting` 打开指定章；`ShowOutline` 切到大纲页。
- `StartJob` 运行后台选文件，`Dialogs` 给 SaveFile 和 Reveal。`PickInto` 与书架共用后台选目录的实现，返回是否有选目录回调。单文件选文件，多文件选目录，相应回调缺席时写作品的「导出」目录。
- `DropMenu` 把宿主的 `kit.MenuState` 和配色交给 `DropMenu`，`SettingColumn` 绘制共享右栏。配色、状态和书内容的重复访问在方法内取一次。三个 `Host` 的签名只用 `kit.Palette`、`kit.Menu`、`kit.Status` 和 `desk` 以外的类型；`Modal` 和 `PaneEmpty` 是 `kit.Palette` 的方法，`laterLines` 在 `desk/exportdialog/export.go`，只供设定导出使用，显式接收 Book 和条目。
- `desk` 直接使用的边界是：导出的 `Show`、`Replan`、`View` 和 `Open`；手机的 `View`；大纲页的 `Enter`、`Leave`、`View`、`Dialog`、`ChapterStory`、`FinishOutline` 和 `Pane`。AI 起草直接调用 Book.ReloadPlan 统计卡片，不改变大纲页的加载和几何状态。

- `phone.Panel.Preferences` 返回逐字段默认后的本机偏好，不落盘；`exportdialog.Dialog.Plan` 和 `Selection` 返回当前持有的计划和范围，不重算计划。页面内部规则在自己的包测试，菜单、导航、落盘和刷新契约由 `desk` 测试。

### 会改行为的

服务编辑的输入、展开状态、模型行和加载代次由 `serviceForm` 持有；加载、关闭、解析在 `serviceform.go`，保存、目录请求和绘制在 `services.go`。关闭或换书时清空草稿，后台目录仅交回原代次的表单，详见上方后续细化笔记。

| 改动 | 作者看得到的 |
| --- | --- |
| 大纲页的章纲停顿后保存 | 和写作页一样：停笔片刻就存，离开大纲页时立刻存 |
| 字号、续写方式、AI 开关设给每个打开的编辑器 | 看不出区别：现在是切回标签时补设 |
| 「停笔后自动续写」改到已经打开的这一章 | 勾上或取消，当前这章立即按新方式续写，不再等切回来 |
| 从人名链接或讨论跳到设定条目时，一并关掉变化的表单 | 正在给某条新增变化时点人名跳到别的条目，显示那条的页面，不再显示它的「新增变化」表单 |
| AI 记录改用只建看得见的行的列表 | 「‹ 本作品用量」和筛选那一行留在上方，记录在下面滚动，不再跟着记录滚走 |

## Alternatives considered

- **把设置和记录继续合进 backupView，只靠注释分区**：少改字段路径，但看名字不能知道备份还保管 AI 记录，重置也依赖备份的偶然归属。采用同包具体类型分离，保持直接调用。
- **设置和记录各拆成子包**：编译器可以限制互相读字段；但这两处需要配色、AI 用量、作品标题和各设置页回调，新增 Host 的转发成本大于这次状态收拢的收益。

- **书模型放在 `desk` 包里**：
  - 最强的理由：少一个包，也不用为它导出名字。
  - 不选：第 7 批拆出去的手机预览、导出、大纲页都要读书，子包不能引用 `desk`。到时候再挪，就是同一批代码搬两次。
- **缓存放进 `workspace.Store`**：
  - 最强的理由：读盘和缓存在一处，谁调 `Store` 谁就有缓存。
  - 不选：
    - `Store` 也给讨论和检查的后台任务用，带着锁；版本号和派生数据是界面线程的事。
    - 放进去，后台任务会读到不该由它管的缓存，`Store` 也就不只是磁盘了。
- **派生数据缓存在 `Book` 上**：
  - 最强的理由：那篇原来就这么写，所有缓存在一处。
  - 不选：待办分组、人名链接是 `desk` 和 `editor` 的类型，放进 `Book` 它就要引用界面。那篇要的是「失效只在一处」：只认书的版本号，就做到了。
- **`Store` 的所有读都经过 `Book`，包括讨论、AI 记录、快照**：
  - 最强的理由：那篇的验收写的是「只有书模型调用 `Store` 的读接口」，一句话就能检查。
  - 不选：这几样各只有一处显示，包一层每个多一个转手的方法，什么都不缓存。守住的是书的内容，测试就守书的内容。
- **每个功能都改成自己的结构体**：
  - 最强的理由：那篇这么写，所有方法都有了明确的主人。
  - 不选：写作页那一团共用打开的章节、讨论状态和右栏，拆开要一个四十多个方法的接口，那篇自己否掉过。只给要拆出去的三个页面做，其余的结构体只是换了接收者。
- **一次做完，不分段**：
  - 最强的理由：中间状态少，不用一段段对齐。
  - 不选：这一批改的是 `desk` 一半以上的代码，一次的 diff 没人审得动。

## Consequences

- 书内容和打开的章节各有一个主人，页面进出和命令的规则各在一张表里。派生数据只认输入的版本，不变的帧不重建。
- `Book` 的版本必须随书内容变：绕过它写盘，缓存就是旧的。测试直接用 `Store` 摆场景后，要让 `Book` 重读。
- `Book` 只在界面线程用，这一条没有测试守，靠后台任务只拿 `Store`、交回后在界面线程刷新。
- 设置、AI 记录与备份的视图状态分离，换书时必须经 `resetPrefs` 一并清空设置与记录；备份仍自己收尾。关闭设置保留同一作品的服务草稿和所选页，不等于离开作品。AI 记录依旧在进入 AI 设置时读取一次，不每帧读盘，也不因此增加数据缓存层。
- 手机预览、导出和大纲页各经自己消费的接口访问 `appHost`。三个页面包不引用 `desk` 或别的页面包，接口签名不带 `desk` 的私有类型；这由 `desk/imports_test.go` 的 `TestImports` 守着。

## Testing

- `TestSettingsCloseKeepsTheWorkAndBackup` 覆盖关闭按钮、Esc、遮罩和重新启动入口：设置关掉，未确认的 AI 开关问题取消，同一作品的服务草稿与备份状态保留，重开仍是原设置页。
- `TestLeavingWorkClearsSettingsAndRecords` 用有 50 条记录的作品，实际滚动列表并选择筛选和展开项后，分别切换作品、回书架、关闭应用；设置、记录和服务草稿清空，旧作品的 50 条磁盘记录仍在，新作品只加载自己的记录。
- 设置状态分离前后运行 `TestShotBackup`、`TestShotRecord`：外观、模型、服务设置、讨论和空记录截图逐像素一致；记录与备份截图的差异仅为测试时间及临时路径，差异图已目视核对。

- `state_test.go` 的 helper 读共享状态；测试改了状态路径时只改 helper。
- 打开的章节、丢失和外部改动：`TestOneOpenChapterPerTab`、`TestALostChapterKeepsItsTab`、`TestOutsideChangeAsksTheAuthor`、`TestSavingDoesNotWriteOverAnOutsideChange`。
- 关 AI 的次序和收束：`TestAISwitchCallsPartsInOrder`、`TestTurnTheAIOffAndOn`。
- 发给模型的字节：`TestTalkRequestBytes`、`TestDigestRequestBytes`、`TestCheckRequestBytes`、`TestPredictionRequestBytes`。存储格式和帧耗时测试的阈值不变，大纲页那项在 `desk/outline/plan_perf_test.go`。

各项要求由哪个测试守：

| 要求 | 测试 |
| --- | --- |
| `desk` 不绕过 `Book` 读写它持有的书内容 | `TestDeskLeavesTheBooksContentToBook` |
| 写作、章纲、检查各十个不变的帧不重建派生数据 | `TestUnchangedFramesBuildNoCache` |
| 大纲页停顿后保存，关作品时保存 | `TestOutlineSavesAfterAPause`、`TestOutlineDraftSavedOnClose` |
| 菜单提示和有快捷键的命令的帮助出自命令表 | `TestMenuHintsComeFromCommands`、`TestTableHelpListsEachKeyedCommand`、`TestHelpShowsTheTableRows` |
| 右栏设定的页面由 `settingRoute` 决定 | `TestSettingRouteScreenOrder`、`TestSettingRouteOneBase`、`TestSettingRouteDropsWhatPointsAtNothing` |
| 一万条 AI 记录只建看得见的行 | `TestTheRecordBuildsOnlyWhatShows` |
| 三个功能的源码不提 `App` | 编译器：它们在 `desk` 下的包里，引用不了 `desk` |
| `desk/book` 的全部依赖不含 `mygo`、`editor` 和 `desk` | `desk/imports_test.go` 的 `TestImports` |
| `write.entered`、`sheet`、`park`、`unpark`、`keyGroups` 已删 | 无；用 grep 查过 |
| 前后截图一致 | 无；截图测试出图，前后对比在审查时做 |
