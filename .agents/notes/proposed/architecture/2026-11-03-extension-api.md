# Agent Note: 宿主给扩展的 API

Status: proposed

## Problem

[扩展模块](2026-11-02-extension-modules.md)定了模型和行为：核心只有编辑器、目录、存储、扩展宿主；两种扩展；必需依赖加连带开关；不连带的可选依赖；全局和按书的开关；替换和四种注入；详情页全部来自宿主的账。作者 2026-11-03 要求「API 设计要做好」。本篇定宿主交给扩展的接口：扩展怎么写、宿主怎么记账、哪些规矩由类型和测试来守。

写这篇时的接口是每个子包一个窄 `Host`：`desk/phone/host.go` 有 7 个方法，`desk/exportdialog/host.go` 有 12 个，都由 [desk/host.go](../../../desk/host.go) 的 `appHost` 实现（这两个包现在已搬成扩展，见下）。它们有三个缺口：

- 只能取，不能加：命令、菜单、右栏标签都写死在 `desk`。
- 宿主不知道谁用了什么，详情页没有账可读。
- 没有关掉、拆干净的路。

## Proposal

包 `writer/ext` 是宿主对扩展的全部 API：登记、开关、Session、注入、出错。宿主（`desk`）开的点和给的服务放在 `writer/ext/hostapi`，只有类型和键。扩展（`ext/` 下除这两个以外的包）的非测试文件只能 import 这几样：

- `ext`、`ext/hostapi`、`desk/kit`、`editor`、`locales`、mygo；
- 书的数据：`workspace`、`desk/book`（界面线程看到的内容）、`internal/atomicfile`；
- 自己名下的子包（`ext/export/format` 这类）；
- 自己在 Requires 或 Optional 里声明了的扩展的公开包（`ext/context/contextapi` 这类）。别的扩展的其余子包不行。

`desk` 实现宿主，非测试文件不 import 任何具体扩展，扩展也不 import `desk`、`shelf`。具体装哪些扩展由 `cmd/inkgo/modules.go` 的清单定。`ext/boundary_test.go` 读源码查这几条，Requires 从 `Module` 字面量里读。

### 扩展

```go
type Module struct {
	ID       string        // "phone"：[a-z][a-z0-9]*，也是它所有 id 的前缀、翻译表的命名空间
	Kind     Kind          // ext.Feature 或 ext.Library（前置）
	Name     func() string // 显示时才取，取自翻译表
	Desc     func() string
	Requires []string      // 必需依赖：连带开关
	Optional []string      // 可选依赖：不连带，对方没开时用到它的部分不生效
	Setup    func(r *Registrar)
}
```

### 公开包只放类型和键

扩展对别人公开的东西放在 `<id>api` 子包里：数据类型、接口，以及带类型的键。也可以放不带状态的小帮手（调用的适配、快照、规整、显示时才拼的标签，比如 `aiapi.ModelLabel`），但不放这个扩展开着时的状态。键只是「名字加类型」，不带状态：

```go
package contextapi

type Context interface{ Entries(kind Kind) []Entry }

var Service ext.ServiceKey[Context] = "context"
var Kinds ext.PointKey[Kind] = "context.kinds"
```

import 一个公开包只是编译期用到它的类型，不等于运行时依赖它。所以边界测试查的是：import 了谁的公开包，谁就必须出现在 Requires 或 Optional 里；取服务也只许取这两处声明了的。

### 生命周期：Setup 一次，Start、Stop 多次

- **Setup** 每装配一次宿主跑一次（应用里就是每个进程一次），不管扩展开没开：
  - 只许登记，不许读写文件、起后台任务、取服务的实现；
  - 宿主把它登记的一切记进账里，所以扩展关着也列得出加了什么（VS Code 的 Feature Contributions）；
  - Setup 里调 `Ref.Get` 直接 panic，测试能抓到。读写文件靠规矩：普通的 Go 文件操作宿主拦不住。
- **Start** 在扩展生效时跑：
  - 跑的时机：开着且依赖都已 Start、打开一本书、换书；
  - 用 `r.OnStart(func(s *ext.Session) error)` 登记，读写文件、建状态都在这里做。
  - 记在这本书界面上的偏好（不是 `ext.Setting` 那种设置）用 `r.Persist(ext.BookState{Load, Save})` 登记：Load 在 OnStart 之后、扩展可用之前跑，每次 Start 都跑，所以打开书时关着、后来才开的扩展也读得到；Save 只给成功到过「开着」的 Session 在 Stop 拆状态之前跑；Provide、OnStart 或 LoadState 失败只拆资源，不写回原书级状态，用着用着出错停下仍保存。宿主存自己的界面状态时也让开着的扩展各存一份。存在 `.writer/desk.json` 的 `extensions.<扩展 ID>` 下，每个扩展一段原样的 JSON，关着的扩展那段原样留着。这个文件读不出来（读失败或不是合法 JSON）时，这次按默认值开，提示一次；之后第一次保存前先把原文件改名留在旁边（`desk.json.unreadable`，替掉更早的同名文件），留不下就不保存。2026-11-05 以前是并进宿主的 `prefs.Prefs`，宿主不认识的键会丢，检查的「何时检查」就这样丢过；旧的平铺键不迁移。
- **Session** 的寿命是「这个扩展开着」乘「这本书开着」：
  - 换书时所有开着的扩展都先 Stop 再 Start，所以两者都要便宜，有测试计时；
  - 按书开关变了的，只 Stop 或只 Start；
  - 一次开启就是一个 Session 实例。`Session.ID()` 是扩展 ID，不能拿来认「是不是这一次开启」，认这个用 `Session.Active()`。
- **扩展的状态分三种**（2026-11-05 审查后定的规矩，起因是换书时宿主先关掉所有扩展，再调各扩展的 `ViewState.Reset`，这时它们都已不算开着，`Reset` 一个都没跑）：
  - 这一次开启的状态：后台任务、回调、收结果的信箱、临时的界面对象。Stop 以后一律作废。
  - 同一本书里关了再开能接着用的：未定的提议、讨论输入框里的字这类。记着属于哪本书（`*workspace.Store`），Start 时比一下，换了书就清掉；同一本书里关了再开照旧在。
  - 长期偏好：经上面的 `Persist` 存取。
  - `ViewState.Reset` 从宿主的约定里删了。
- **Stop** 按 Start 的相反顺序跑：
  - 成功到过「开着」的 Session 先存下书级状态（`Persist` 的 Save），启动失败不保存；
  - 再让这次 Session 作废、取消它的后台任务、等这些任务退出，跑 `Session.OnStop` 登记的资源清理（只跑一次；每一条都和 `r.OnStop` 一样在出错边界里跑，panic 记在这个扩展的账上，接着跑下一条）；
  - 最后跑 `r.OnStop` 登记的清理。
- **后台任务**归这次开启：扩展经 `Ref.Get` 取到的 `hostapi.Jobs` 是绑在取用方这次 Session 上的一份（宿主服务实现 `ext.SessionBinder`）。任务排到帧线程的回调，在真正执行时再查一次 Session 还在不在；不在就不执行它的结果、出错提示和「再试一次」，宿主只把忙碌状态结掉。后台任务不许等帧线程，因为 Stop 要等后台任务退出。停止同步等待，可能长时间阻塞。StartJob 的工作函数接收取消 ctx；Post、StartJob apply、Request apply 和对话框结果回调都在所属扩展的出错边界内，宿主结算和 jobs.poll 不 recover。Request 的 Session 和 epoch 归 App；每帧先装唤醒函数、Settle，再结算失效 Session 的 busy、处理错误通知，最后 poll。finished 在 apply 前清归属；App.work、resetView 清归属；Close 在 leaveWork 后结算再推进 epoch，不用取消提示覆盖扩展错误。
  - 一个扩展的工具在别的扩展的请求里跑时（改稿建议的 `propose_edit` 在讨论的请求里），提供方也要认：工具拿到时记下提供方的 Session（`ext.WithSession`、`ext.SessionOf`），交结果时比一下，提供方中途关了再开，旧结果不收，propose_edit 返回失败且不带建议。
  - AI 每次调用的记录交给 `aiapi.Calls` 时带着收记录那一方的这次开启（`aiapi.Call.Activation`），收记录的是哪几方、哪一次开启，在每次请求开始时取，不在建模型句柄时取：续写缓存着的笔照样记账，用量记录关了再开，旧调用不会记到新的那一次头上，新的那一次也收不到半截调用。
- **贡献的回调**（命令的 Run、标签的 Draw……）只在扩展生效时被调，读 Start 时建好的状态。宿主在扩展没生效时不调它们，也不经 `Declared` 去调关着的扩展的回调。
- **状态**有四种：关着、启动中、开着、出错。出错时带错误原文和出错的位置：哪个回调，在谁的注入链里。

Dialogs.SaveInto 和 PickInto 的原生操作归宿主管，不计入 Session 任务；结果回调只在原 Session 有效时执行。Reveal 是 Dialogs 的普通方法，不暴露裸函数。帮助行 HelpAfter、HelpBefore 使用稳定行 ID，显示函数只负责文字，语言切换不影响位置。

### 登记表

所有 id 由宿主加前缀：在 `phone` 里登记的 `open` 就是 `phone.open`。同名在 Setup 时就报错，不覆盖。

- **扩展点** `PointKey[T]`：
  - 宿主开的有命令、菜单项、右栏标签、整页、底栏项、正文工具条项、补全来源；
  - 别的点由扩展自己开，比如 AI 开「AI 工具」（`ai.tools`），导出开「导出格式」（`export.formats`）；
  - 扩展用 `ext.Declare(r, contextapi.Kinds)` 开自己的点，开的必须是自己名下的键；
  - 往点里放用 `ext.Contribute(r, point, "name", value)`。
- **服务** `ServiceKey[T]`：
  - 提供：`ext.Provide(r, key, func(*ext.Session) T)`。实现在 Start 时才造出来，所以能读这本书的数据。
  - 取用必需依赖的：`ref := ext.Use(r, key)`，返回 `*ext.Ref[T]`，用的时候 `ref.Get()`。这样关掉再开、换书之后，拿到的总是当前那份；Setup 时提供方可能还关着，也没有东西可返回。
  - 取用可选依赖的：`ref := ext.UseOptional(r, key)`，`ref.Get()` 返回 `(T, bool)`，对方没开时是 false。Optional 只指「装了但关着」：提供模块必须参与装配，缺席或依赖成环仍报错。两种引用共用服务解析和 SessionBinder 绑定，服务绑定到消费方的当前 Session。
  - 往可选依赖的点里放东西、注入它，写法和必需依赖一样；对方没开时宿主不生效这些登记，详情页写「X 没开，以下不生效」。对方中途开、关，宿主照账接上或拆下，依赖方不用重启。
- **宿主的能力也是服务**，键在 `hostapi` 包里：`BookService`（可信的书句柄：当前的书 `Store`、界面读的内容 `Content`；`Store` 能写，不是只读）、`EditingService`（会动编辑器、目录、字数、撤销的操作：选中的章和编辑器、先存下显示着的那一章、恢复快照、全书替换、内容刚变的闪烁……）、`NavigationService`（页、条目、选中的章、专注、打开某个扩展的详情页）、`NativeUIService`（菜单、动画、右栏里画的东西）、`MachinePathsService`（本机路径）、`PaletteService`（颜色）、`PaneService`（打开右栏的某个标签）、`NoticesService`（提示条）、`JobsService`（后台任务）、`DialogsService`（选文件夹、存文件、在文件夹里显示）。原来每个子包一个的窄 `Host` 已经换成这些。2026-11-05 审查后把迁移中越长越宽的 `Desktop`、`Book` 按能力拆成上面几个，`Desktop` 删了；`Store` 是给仓库里 Go 代码的可信能力，后台任务开始时拿住那一本书的，不在中途问「现在是哪本书」。宿主的服务由 `ext.Options.DeclareHost` 在装配时提供，一直在，不用写进 Requires；其中 `Jobs` 交给每个扩展的是绑在它这次开启上的一份，见上面「后台任务」。每个方法的 doc comment 写明在哪个线程调。
- **设置**：
  - `ext.Setting(r, "name", ext.Schema[T]{…})`，返回 `*ext.Pref[T]`，带 `Get()`、`Set()`，不提供变更监听；值按 JSON 存；
  - Schema 有标题、说明、默认值、可选项（每项带说明）、分组、顺序、作用范围（`Local` 本机、`BookScope` 按书、`Both` 两者都有）；
  - 存放：本机的在 `works.json` 旁边的 `ext.json`（键 `off` 本机关掉的扩展、`settings`、`injections` 作者调过的顺序和选择），按书的在作品根上的 `ext.json`（键 `switches`、`settings`；2026-11-04 从 `.writer/ext.json` 搬出来，好进备份）。都用 `internal/atomicfile` 写。文件读不出来时这一次按默认处理、提示条说一句（`locales.ExtFileUnreadable`），在真正存一次以前不覆盖它；
  - 宿主照声明用 `kit` 画在设置里这个扩展的那一节（`desk/settings_declared.go`）：现在只画开关（`Schema[bool]`）和带可选项的 `Schema[string]`，按分组、顺序排；别的类型扩展自己画。宿主开的具名分组（`ext.SettingsGroupChecks` 这类，几个扩展的项画在一页、每项记着是谁加的）没做：几个扩展合用一页改用下面登记同一个页；
  - 要自己画就往 `hostapi.SettingsSections` 里放一段，画在宿主画的页头下面，详情页标明「自定义设置页」；要在设置左栏占一页，往 `hostapi.SettingsPages` 里登记页名和排序数（AI 登记 10，伏笔、检查都登记「提醒」20，备份 30），同一个 `Key` 合用一页，页里每个扩展一段，段头是它的开关和说明。登记的页一直在，扩展关着也在，宿主经 `ext.Declared` 读没生效的登记（只读数据，不调回调）。
- **给宿主显示用的**：`Module.Group`（总览里的组，`hostapi.GroupFiles` 这类）、`Module.Off`（关着时多说的一句）。
- **数据目录**：`Session.BookDir()` 是作品里的 `ext/<id>/`（由 `workspace.Store.ExtDir` 给出，宿主经 `ext.Options.BookDir` 接进来），`Session.LocalDir()` 是 shelf 旁边的 `ext/<id>/`。宿主只在 Start 之后给。

### 注入

每个扩展点在声明时说明它的「调用」是哪个函数：命令是 Run，标签是 Draw，导出格式是 Write。点里每一项贡献的调用都是一个 `Hook[In, Out]`，`ext.HookOf(point, "phone.open")` 取到它。一项贡献有好几种会改状态的操作时（提议的 Begin、Follow、Finish、Drop、ShowProposal），每种操作单独声明成有名字的调用（`ext.DeclareOperation`，调用键是 `<贡献 id>@<点>:<操作>`），各自带类型，各自能注入；只读的查询不用声明。

一个扩展要用另一个扩展的能力，就把它写进必需依赖，经它的服务用；服务背后改状态的那几下仍走那个扩展自己点上声明过的操作，照样能注入、记账。起草、改稿建议开讨论就是这样（`talkapi.Service` 的 `Start`、`Open` 走 `talk.starters` 上的操作）。2026-11-05 以前「开一段讨论」是 AI 名下的单值点（`ext.DeclareSingle`），作者定起草、改稿建议必需讨论以后，单值点没人用了，连同账里的候选和冲突一起删了。核心另外声明一些点，比如 `ext.EditorKey`（编辑器处理按键）、`ext.PaintProse`（画正文）。

```go
ext.Before(r, hook, func(in *In))                              // 改参数
ext.After(r, hook, func(in In, out *Out))                      // 改结果
ext.Around(r, hook, func(in In, next func(In) Out) Out)        // 决定调不调原来的
ext.Overwrite(r, hook, func(in In) Out)                        // 整个换掉
ext.Replace(r, point, "export.docx", value)                    // 换掉一整项贡献
```

- Replace 就是对那项贡献的 Overwrite，换的是整个值（标题、快捷键一起换）。
- 注入的目标必须属于 Requires 或 Optional 里的扩展，或者是核心的点。
- 目标没生效时，注入什么也不做；详情页写「X 没开，以下不生效」。
- before、after、around 可以叠好几层：
  - 默认按装配清单的顺序排；
  - 作者在被注入那一处调过的顺序存进设置。
- overwrite（包括 Replace）同一处只能有一个：
  - 两个扩展都要时，两个都不生效，原来的照跑，那一处标冲突；
  - 作者选了哪个，存进设置。
- 宿主内部（登记表、开关、账、详情页）没有键，注入不到。
- 链在登记变化时（开关、换书、作者调顺序）建好存起来，不在每次调用时现拼闭包。画和按键这类每帧都跑的链，要过现有的帧耗时检查。
- `Host.Changes()` 是一个计数：生效的贡献、开关、链有任何变化时加一，读它不分配。desk 由扩展派生的表（右栏标签、命令、菜单行、对话框、底栏项……）只在它变了时重建，平时用存着的。

### 出错

宿主在每一环调扩展代码的地方 recover：

- 只把出错的那个扩展标成出错，Stop 它，依赖它的跟着关。注入方出错时，只拆掉它的注入，被注入的照常。
- `Guard`、无注入的快路径和注入计划的各调用边界，在检查是否开着的同一把读锁内捕获 Session；任务 panic 用任务自己的 Session。出错时若它已不是当前 Session，只写日志，不改状态、不进账、不提示。对外调用签名不变，工作线程仍可用 `PointReader.Capture`。
- `Host.Settle()` 在界面线程推进出错后待停的扩展；没有待收尾时靠原子标志直接返回，不拿锁。Set、Open 等宿主变更仍先走同一收尾逻辑。等待任务退出时不持 `h.mu`，任务不得等界面线程；同步收尾可能长时间等待。接口已有测试，desk 每帧接入留给下一批。
- 同一次调用不自动重做。原实现、Overwrite、Replace panic，或 Around 未调 next 就 panic，`Call` 返回零值和 `ErrCallFailed`；不回退到原实现。Before、After、next 成功后的 Around 出错只停注入方，保留内层结果和 nil。内层原有的 `ErrCallFailed` 必须保留；调过 next 不等于成功，Around 不调 next 自行返回仍是合法短路。
- next 只能在回调内同步调用、最多一次，不缓存、不转交其他 goroutine；这是约定，不加运行期机制。Before、After 只按值恢复，切片元素及指针内容不回滚，不能保证保存的是注入前参数。`ErrCallFailed` 不表示没有副作用，调用方不得自动重试写操作或补做默认操作；desk 调用点已核对。
- 提示条和详情页写明是哪个扩展、哪个回调、在谁的注入链里。
- 画界面的回调半路 panic 时，desk 把每个扩展画的东西包在一个 `ui.Box` 里，出错就把这个盒子藏起来：这一帧画了一半的不露出来，下一帧起不再调它。底栏的项例外：多包一层它们就没法照原型的规则收缩（英文下右栏开着时会被截断），所以直接排进底栏；出错照样记账、停掉，只是没有那层盒子来藏。
- 宿主调扩展有两种办法，都 recover、都记账、都只停出错的和必需依赖它的：
  - `ext.Call(h, hook, in, original)`：要让别的扩展能改的调用，经注入链。命令的 Run、各种 Draw、每帧的 Tick、离开作品前的回调都走它。
  - `ext.Guard(h, owner, target, fn)`：只是问一下、不该被注入的，不声明调用处、不进链。对话框开没开（`Overlay.Open`）、待处理的缓存键和那一组（`PendingSource`）、点了待处理的一项都走它。
  - 扩展没生效、或调用出错，都返回能用 `errors.Is(err, ext.ErrCallFailed)` 判断的错误。宿主自己（`ext.HostOwner`）的 panic 不吞。
- 提示条只写扩展名和错误原文：「{name}出错，已停下：{error}」（`locales.ExtFailed`）。出错的时间、阶段、调用处、在谁的注入里记在账的 `Errors` 里，详情页的「出错记录」列出来，新的在上。

### 账和详情页

宿主给每个扩展记一本账，详情页、「扩展」页、被注入那一处的链都只读这本账：

- 必需依赖、可选依赖（各自开没开），以及反过来谁依赖它、谁注入它；
- 开的点、放进去的贡献、提供和取用的服务；读了哪些点（`Ledger.Reads`，装配时查点在不在、类型对不对）；声明的调用（`Ledger.Calls`）；
- 注入：哪一处、哪一种、排第几、生效还是冲突、目标开没开；
- 设置、自定义设置页；
- 数据目录（`Host.DataPaths`；大小由宿主在后台量，不进账）；
- 状态、出错记录；
- 快捷键：每个键谁要、谁生效（`Ledger.Keys`）。

详情页的「功能入口」读点的声明：`ext.Declare` 写明这个点在界面上的什么地方（`ext.Place`，作者看不见的写 `ext.Hidden`），`ext.DeclareName` 写明怎么从值里取名字；被注入的那一处叫什么由开它的一方经 `ext.NameTarget` 声明。

扩展经 Registrar 登记的东西都记在它名下（cordis 的 `Fiber.effect` 和 `getEffects()`）。关掉时宿主照账拆，拆完这个扩展在生效表里一条不剩，有测试。`Persist` 的登记、Session 里一件件的任务和清理没有逐条进账：它们的寿命由 Session 管住，但账里看不到。

### 两层

- **Go 层**：仓库里的扩展用，就是上面这些。可以在帧里拿 `*ui.Context` 用 `kit`、`ui` 画，守 `mygo vet`。
- **数据层**：以后给外部插件用。规矩同一套，参数换成 id 和普通数据；界面交出描述，由宿主画；不交出 Go 指针或帧内对象。外部插件的清单另写宿主 API 版本（VS Code 的 `engines.vscode`）和权限。数据层等 mygo 的脚本方案出来再定细节。

### 例子

写例子是为了找出接口缺的东西。名字按现在的代码起，落地时再定。

**手机预览**：一个标签、一个命令、一个本机设置。

```go
var Module = ext.Module{ID: "phone", Kind: ext.Feature, Name: locales.PhoneName, Desc: locales.PhoneDesc, Setup: setup}

func setup(r *ext.Registrar) {
	p := &preview{}
	p.dark = ext.Setting(r, "dark", ext.Schema[bool]{Title: locales.PhoneDark, Scope: ext.Local})
	p.editing = ext.Use(r, hostapi.EditingService)
	ext.Contribute(r, ext.PaneTabs, "preview", ext.PaneTab{Title: locales.PhoneTab, Draw: p.draw})
	ext.Contribute(r, ext.Commands, "open", ext.Command{
		Label: locales.PhoneOpen, Menu: ext.MenuView, Run: p.open,
	})
	r.OnStop(p.reset)
}

func (p *preview) draw(c *ui.Context) ui.Element {
	id, ed, ok := p.editing.Get().Chapter()
	…
}
```

**导出 docx 被别的扩展换掉**：导出开了「导出格式」这个点，docx 是它自己放进去的一项；另一个扩展换掉它。

```go
// ext/export
ext.Declare(r, exportapi.Formats)
ext.Contribute(r, exportapi.Formats, "docx", exportapi.Format{Name: locales.ExportDocx, Ext: ".docx", Write: writeDocx})

// ext/fancydocx，Optional: []string{"export"}
ext.Replace(r, exportapi.Formats, "export.docx", exportapi.Format{Name: locales.FancyDocx, Ext: ".docx", Write: fancy})
```

关掉导出时 fancydocx 不跟着关，只是详情页写「导出没开，以下不生效」。再有一个扩展也 Replace `export.docx` 时，两个都不生效，导出对话框里 docx 那一项照原来的导，标出冲突。

**名字补全**：从 context 取设定的名字，放进核心的补全来源。现在 [desk/names.go](../../../desk/names.go) 的 `entities()` 直接读 `a.book.Setting()`。

```go
// ext/names，Requires: []string{"context"}
func setup(r *ext.Registrar) {
	lore := ext.Use(r, contextapi.Service)
	ext.Contribute(r, ext.Completions, "setting", ext.Completion{Names: func() []editor.Entity {
		return entities(lore.Get().Entries(contextapi.Names))
	}})
}
```

**注入核心的点**：一个把半角标点换成全角的扩展，在编辑器处理按键之前改输入。

```go
ext.Before(r, ext.EditorKey, func(in *ext.KeyIn) { in.Text = fullWidth(in.Text) })
```

写例子时发现的缺口，落地时一并定：

- 菜单位置要有名字：`ext.MenuView` 这类常量，加上「排在谁后面」。现在的标题栏菜单和 macOS 的系统菜单都从 [desk/commands.go](../../../desk/commands.go) 的表生成，命令的贡献要能进同一张表。
- 命令的快捷键要分系统写，和现在的 `macKey`、`macMods` 一样，冲突照快捷键表的规矩标出来。2026-11-05 做了：值实现 `ext.ShortcutSource` 的贡献都参加，宿主自己的键经 `Options.HostKeys` 参加且总是生效，结果记在账的 `Keys`，见[扩展模块](2026-11-02-extension-modules.md)「顺序」第 8 条。
- `Editing.Chapter` 交出 `*editor.Editor`、`Book` 交出 `Store`、`Content` 都只在 Go 层可以；数据层要换成章节 id 加读写正文的服务。
- `ext.EditorKey` 每按一下都跑整条链，链要预先建好，并有帧耗时的测试。
- 补全来源要说明多久取一次。现在 `namesKey` 按设定是否变了来缓存，点里要留一个「什么变了才重取」的键。

### 各点、服务在数据层怎么表达

下面「风险」里那条规矩要的表：Go 层每种点、服务在数据层怎么表达，表达不了的标「只限 Go 层」并写怎么适配。2026-11-05 审查后从代码里逐个列出，共 70 个。这只是给将来适配层的说明，数据层还没有实现。

| 包和键 | 现在的约定 | 数据层怎么表达 |
|---|---|---|
| ext/ai/aiapi: `ai.calls` (Calls) | `PointKey[func(Call)]` | 只限 Go 层：捕获的 Store 和 Session 身份。适配：书 ID 和这次开启 ID，加上不可变的用量 Record DTO，以及有类型的完成通知。 |
| ext/ai/aiapi: `ai.model-changes` (ModelChanges) | `PointKey[func()]` | 提供方/配置修订事件，没有原生负载；调用订阅者的动作 ID。 |
| ext/ai/aiapi: `ai.service` (Service) | `ServiceKey[Backend]` | 只限 Go 层：原生后端里的 Store 和补全函数；也给选好的模型句柄配写作的笔（`Assistant.Writer`，检查用）。远端使用固定的书 ID、模型/角色/上限 DTO、请求 ID、流式增量和取消。 |
| ext/ai/aiapi: `ai.tools` (Tools) | `PointKey[Tool]` | 只限 Go 层：ToolEnv.Store、context 和回调函数。适配：工具 ID/schema JSON，书 ID、这次开启 ID、会话 ID、轮次 ID、调用 ID，参数 JSON 和 ToolResult DTO。 |
| ext/ai/aiapi: `ai.usage` (Usage) | `PointKey[func(*ui.Context)]` | 只限 Go 层：绘制回调拿着帧内的 `*ui.Context`。适配：贡献 ID，宿主每帧要一份 UI 描述 DTO，交互回成动作 ID。 |
| ext/context/contextapi: `context.entries` (Entries) | `PointKey[EntryProvider]` | 提供方 ID + 书 ID → 条目 DTO 列表；空列表与不存在要区分开。 |
| ext/context/contextapi: `context.hooks` (Hooks) | `PointKey[HookProvider]` | 提供方 ID + 书 ID → 伏笔 DTO 列表；空列表与不存在要区分开。 |
| ext/context/contextapi: `context.outlines` (Outlines) | `PointKey[OutlineProvider]` | 章/卷 ID → 章纲文本和提供方是否存在的标记。 |
| ext/context/contextapi: `context.service` (Service) | `ServiceKey[Context]` | 书 ID、条目 ID、章 ID → Entries/Hooks/Outline DTO，以及明确的提供方是否存在标记。 |
| ext/find/findapi: `find.replacement` (ReplacementService) | `ServiceKey[Replacement]` | 章 ID、替换前后的字符串，以及替换/编辑操作 DTO；由宿主协调编辑器改动。 |
| ext/hostapi: `host.activities` (Activities) | `PointKey[Activity]` | 贡献 ID → 「在跑、不挡作者操作」「进度正显示着」两个标记，帧线程上查。 |
| ext/hostapi: `host.before-chapter-change` (BeforeChapterChange) | `PointKey[func()]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.before-editor` (BeforeEditor) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.before-leave` (BeforeLeave) | `PointKey[func()]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.before-prose` (BeforeProse) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.beside` (BesideItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.book` (BookService) | `ServiceKey[Book]` | 只限 Go 层：借用的 *book.Book 和受信任的 *workspace.Store；远端适配必须使用固定书 ID 和读取 DTO。 |
| ext/hostapi: `host.chapter-edits` (ChapterEditStates) | `PointKey[ChapterEdits]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.chapter-head` (ChapterHeadItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.chapter-open` (ChapterOpens) | `PointKey[func(ChapterOpened)]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.commands` (Commands) | `PointKey[Command]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.dialogs` (DialogsService) | `ServiceKey[Dialogs]` | 只限 Go 层：帧 Context 和原生回调。远端使用对话框请求 ID 和所选路径/结果数据。 |
| ext/hostapi: `host.editing` (EditingService) | `ServiceKey[Editing]` | 只限 Go 层：实时编辑器和 Snapshot/编辑协调。远端：书 ID、章 ID、编辑操作、快照 ID、撤销动作 ID 和状态 DTO。 |
| ext/hostapi: `host.error-texts` (ErrorTexts) | `PointKey[ErrorTranslator]` | 只限 Go 层：Go 错误身份和翻译回调。适配：结构化错误码/参数，以及按本地化时间生成的消息。 |
| ext/hostapi: `host.extensions` (ExtensionsService) | `ServiceKey[Extensions]` | 扩展 ID → 是否启用的布尔值；本地化名字 + 是否装着。 |
| ext/hostapi: `host.footer` (FooterItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.jobs` (JobsService) | `ServiceKey[Jobs]` | 只限 Go 层：工作函数、Context 和 Services。远端使用这次开启 ID/任务 ID、取消请求和结果 DTO。 |
| ext/hostapi: `host.leave-questions` (LeaveQuestions) | `PointKey[func(func()) bool]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.machine-paths` (MachinePathsService) | `ServiceKey[MachinePaths]` | 目录字符串和路径比较请求；在宿主权限下解析机器路径。 |
| ext/hostapi: `host.name-highlights` (NameHighlights) | `PointKey[NameProvider]` | 只限 Go 层：编辑器装饰数据和回调导航。适配：章 ID、范围/名称/样式 DTO 和动作 ID。 |
| ext/hostapi: `host.native-ui` (NativeUIService) | `ServiceKey[NativeUI]` | 只限 Go 层：借用的 Context/Element、菜单状态、动画和窗格绘制。宿主渲染 UI 描述；历史标签委托给 aiapi。 |
| ext/hostapi: `host.navigation` (NavigationService) | `ServiceKey[Navigation]` | 页面/设置/章/条目/扩展 ID、条目 DTO、焦点/打开/丢失/暂停标记；导航请求和状态回复。 |
| ext/hostapi: `host.notices` (NoticesService) | `ServiceKey[Notices]` | 只限 Go 层：*kit.Status 和原生撤销闭包；远端使用通知 DTO 和动作 ID。 |
| ext/hostapi: `host.overlays` (Overlays) | `PointKey[Overlay]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.pages` (Pages) | `PointKey[Page]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.palette` (PaletteService) | `ServiceKey[Palette]` | 只限 Go 层：原生调色板指针；远端适配会发布命名的颜色令牌数据。 |
| ext/hostapi: `host.pane` (PaneService) | `ServiceKey[Pane]` | pane/tab/page ID 与打开/选中标记，show/reveal/toggle 请求。 |
| ext/hostapi: `host.panes` (PaneTabs) | `PointKey[PaneTab]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.pending` (PendingGroups) | `PointKey[PendingSource]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.pending-state` (PendingService) | `ServiceKey[Pending]` | 待处理项目键 → 清除忽略请求。 |
| ext/hostapi: `host.predictions` (Predictions) | `PointKey[Prediction]` | 只限 Go 层：交出 `editor.Predictor`。适配：章 ID、光标前的正文 → 续写文字的请求和流式回复，接受/放弃回成动作 ID。 |
| ext/hostapi: `host.prose-marks` (ProseMarks) | `PointKey[ProseMarkProvider]` | 只限 Go 层：编辑器装饰数据和回调导航。适配：章 ID、范围/名称/样式 DTO 和动作 ID。 |
| ext/hostapi: `host.prose-overlays` (ProseOverlays) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.prose-ui` (ProseUIService) | `ServiceKey[ProseUI]` | 只限 Go 层：帧几何、编辑器和绘制/菜单回调。远端使用章 ID/文本、几何和 UI 描述消息。 |
| ext/hostapi: `host.save-edits` (SaveEdits) | `PointKey[func() error]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.search` (SearchService) | `ServiceKey[Search]` | 章 ID + 查询/引文 → 跳转或计数。 |
| ext/hostapi: `host.selection-buttons` (SelectionButtons) | `PointKey[SelectionButton]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.selection-menu` (SelectionMenus) | `PointKey[SelectionMenu]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/hostapi: `host.settings` (SettingsPages) | `PointKey[SettingsPage]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.settings-details` (SettingsDetails) | `PointKey[SettingsDetail]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.settings-sections` (SettingsSections) | `PointKey[SettingsSection]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.state` (State) | `PointKey[ViewState]` | 只限 Go 层：刷新/窗格回调；远端使用书 ID、模块 ID、窗格 ID 和修订通知。 |
| ext/hostapi: `host.ticks` (FrameTicks) | `PointKey[FrameTick]` | 只限 Go 层：每帧的回调拿着 `*ui.Context`。适配：宿主按帧或按修订号发事件（普通数据），扩展回动作 ID；不交出帧内对象。 |
| ext/hostapi: `host.title` (TitleItems) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.tool-hints` (ToolHints) | `PointKey[DrawItem]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.tools` (Tools) | `PointKey[Tool]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.window-activity` (WindowActivities) | `PointKey[func(WindowActivity)]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.word-popovers` (WordPopovers) | `PointKey[WordPopover]` | 只限 Go 层：绘制/动作回调或原生帧句柄。适配：贡献 ID、UI 描述/元数据 DTO、动作 ID 和可用性标记。 |
| ext/hostapi: `host.word-suffix` (WordSuffix) | `PointKey[CountSuffix]` | 按 ID 寻址的订阅/动作，事件字段为普通数据；在数据适配层，原生回调变为请求/动作 ID。 |
| ext/hostapi: `host.writing-keys` (WritingKeys) | `PointKey[WritingKey]` | 只限 Go 层：回调动作 / 借用的原生选择状态。适配：章 ID、范围 ID、项目 ID、动作描述和动作 ID。 |
| ext/lore/loreapi: `lore.active` (Active) | `ServiceKey[struct{}]` | 提供方存在标记，没有负载。 |
| ext/lore/loreapi: `lore.entry-navigation` (EntryNavigationService) | `ServiceKey[EntryNavigator]` | 条目 ID 和条目 DTO，用于打开/定位/跳转/编辑/回答；待处理列表的动作 ID。 |
| ext/lore/loreapi: `lore.revisions` (RevisionsService) | `ServiceKey[Revisions]` | 书 ID + 作者的 Revision/Op DTO → 修订后的记录或错误；撤销动作 ID；刷新请求。 |
| ext/lore/loreapi: `lore.selection` (SelectionService) | `ServiceKey[Selection]` | 当前章 ID/选择 ID + 引文/文本/类别草稿 DTO；打开/关闭表单的动作 ID 和打开状态。 |
| ext/marks/marksapi: `marks.margin-sources` (MarginSources) | `PointKey[MarginSource]` | 只限 Go 层：可比较的原生缓存键和回调动作。适配：提供方修订号 + 章 ID/文本 → 范围/批注 DTO 和动作 ID。 |
| ext/marks/marksapi: `marks.service` (Service) | `ServiceKey[Reader]` | 只限 Go 层：MarginItem 回调和编辑器 MarkKind。适配：章 ID/文本、批注/范围/样式 DTO、动作 ID 和修订令牌。 |
| ext/outline/outlineapi: `outline.active` (Active) | `ServiceKey[struct{}]` | 提供方存在标记，没有负载。 |
| ext/talk/talkapi: `talk.proposals` (Proposals) | `PointKey[Suggestions]` | 只限 Go 层的 ProposalFrame.Stop 回调。适配：讨论 ID、调用 ID、章 ID，不可变的建议更新/失败，停止动作的动作 ID，以及每次操作的请求/回复。 |
| ext/talk/talkapi: `talk.service` (Service) | `ServiceKey[ServiceAPI]` | 讨论 ID → 打开请求，或本地化标题 + 是否找到标记；开讨论的来源 Event DTO → 已开始标记，起草上下文、在跑、进度的 DTO。只限 Go 层的是开讨论时带的 `ui.Services`，适配成发起请求的 ID。 |
| ext/talk/talkapi: `talk.starters` (TalkStarters) | `PointKey[TalkStarter]` | 讨论自己开的点，在 `talk.service` 背后，不给别的扩展直接读。只限 Go 层的 Start.Services。适配：讨论来源 Event DTO，返回已开始标记；起草上下文/在跑/进度 DTO。 |

## Alternatives considered

- **每个子包一个窄 `Host` 接口（现在的做法）** — 最强的理由是每个包要什么一眼看得出，测试好替身。没选：只能取，不能往宿主里加东西；宿主不知道谁用了什么，账没法记。按需取服务以后，每个扩展用了什么照样列得出来，而且是宿主记的。
- **`ext.Use` 在 Setup 时直接返回实现** — 最强的理由是用起来少一层 `Get()`。没选：Setup 时提供方可能关着，关掉再开、换书以后拿到的会是旧的那份。
- **只有 Setup，开的时候跑、关的时候拆** — 最强的理由是只有一个入口，更简单。没选：Setup 读文件的话，扩展关着就列不出它加了什么；IO 只能放在 Start 里。
- **只有必需依赖，注入也得写进 Requires** — 最强的理由是只有一种关系。没选：作者 2026-11-03 要可选依赖；只有必需依赖的话，关掉目标会连带关掉注入方，也没法往可有可无的东西上注入（Fabric 的可选 mixin 就是为这个）。
- **可选依赖的服务也用 `Use`，`Get()` 没有时返回零值** — 最强的理由是少一个函数。没选：零值的接口一调就 panic，调用方容易忘了判断；返回 `(T, bool)` 让「没有」在类型上看得见。
- **改写机器码（gomonkey 这类）注入任意 Go 函数** — 最强的理由是什么都能改，不用宿主预先开口子。没选：内联后的函数改不到，macOS 有代码页写保护，`CGO_ENABLED=0` 的发行版上也不可靠；改了什么宿主也记不了账。
- **反射式依赖注入（uber/fx、dig）** — 最强的理由是不用手写装配。没选：依赖图在运行时才解，出错难读，也不好显示给作者。
- **字符串键、参数是 `any` 的事件总线** — 最强的理由是灵活，扩展之间不用共享类型。没选：编译期查不出类型错。带类型的键同样只是一个名字，而且查得出。
- **服务一出现就自动启用依赖方（cordis 的 `inject`）** — 最强的理由是不用作者动手。没选：作者要开关都看得见，见扩展模块篇。

## Acceptance criteria

每条规矩一个用假扩展的测试：

- 打开一个，它的依赖一起打开；关掉一个，依赖它的一起关；再开时各自回到原来的开关。
- 扩展关着时，账里也列得出它的贡献、注入、设置。
- Setup 里调 `Ref.Get` 会失败（读文件靠规矩，宿主拦不住）。
- 同一个扩展内 id 重名时，Setup 报错。
- 取用没声明依赖的服务、注入没声明的目标时，Setup 报错。
- 两个 overwrite 抢同一处：两个都不生效，原来的照跑，账里标冲突；作者选一个以后，被选的那个生效。
- 注入的目标关着时，注入方照常开着，注入不生效，账里写「X 没开，以下不生效」。
- 一个扩展的回调 panic：只有它和依赖它的变成出错或关着，别的照常。停下时的清理（`Session.OnStop`、`r.OnStop`）panic 只记账，停照常停完，不再连带别的扩展。
- 关掉以后，这个扩展在生效表里一条不剩：命令、菜单、标签、注入、服务都没了；它的后台任务已退出，排着的回调不再执行。可选依赖的工具若正在依赖方的请求里跑，关掉提供方不会等那个请求，靠交结果时比 Session 挡住。
- 换书、关了再开：上一本书的状态不会带到下一本；同一本书里关了再开，能接着用的照旧在（`desk/lifecycle_fix_test.go`）。
- 换书时开着的扩展 Stop、Start 一轮，总耗时有上限。
- 依赖边界测试：import 了谁的公开包，谁就在 Requires 或 Optional 里。
- 可选依赖：打开依赖方不会打开对方；关掉对方，依赖方照常开着，`UseOptional` 的 `Get()` 返回 false，往对方放的东西和注入都不生效；再打开对方，这些不重启依赖方就回来。

## Risks

- 例子只写了四个，伏笔、检查这类要碰正文标记的功能可能还要新的点。这等 marks 时再设计，先迁手机预览、导出、备份，验证这套接口。
- 「凡经宿主走的调用都能注入」会让每个贡献多一层间接调用。不经宿主的内部调用不受影响，每帧都跑的链要量。
- 画界面的回调出错后，这一帧能不能保持完整，取决于 mygo（待核实）。
- 换书时整轮 Stop、Start，扩展一多会变慢；到时可以改成只重启用了书的数据的那些扩展。
- 数据层没有实现，Go 层的接口可能长成数据层表达不了的样子。所以 Go 层每加一种点，都要说明它在数据层怎么表达；表达不了的，标成只限 Go 层。

## 相关笔记当场审计

- [2026-11-06 契约审查](2026-11-06-extension-contract-review.md)：记录已复现的启动回滚、Session 身份和回调边界缺口；作者 2026-11-06 确认了第四版修复方案，施工完要回到本篇改齐。
- [扩展模块](2026-11-02-extension-modules.md)：本篇的上篇，互链。那篇管模型和行为，本篇管接口；两篇说法不一样时以那篇为准，并当场改齐。
- [扩展的安全模式](../feature/2026-11-03-extension-safe-mode.md)：部分重叠。安全模式读的就是本篇的账和状态。
- [desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)、[理一遍代码](../../implemented/architecture/2026-10-21-code-reorganization.md)：部分重叠。它们定的窄 `Host` 会被服务取代，落地时改那两篇。
- [多语言方案](2026-11-02-i18n-on-mygo.md)：部分重叠。`Name`、`Label` 存函数，显示时才取，和那篇的规矩一致。
