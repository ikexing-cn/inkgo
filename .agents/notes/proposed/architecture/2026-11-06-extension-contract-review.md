# Agent Note: 扩展模块、API 和多语言的契约审查

Status: proposed

## Problem

作者要求对照已有实现，审核[扩展模块](2026-11-02-extension-modules.md)、[扩展 API](2026-11-03-extension-api.md)、[多语言](2026-11-02-i18n-on-mygo.md)：职责是否清晰、是否过度设计、生命周期与执行顺序是否可靠。本文记录 2026-11-06 的发现和建议，不表示建议已经采纳。本次不修改产品代码。

## Proposal

保留现有分层，先修失败和停用的契约，再整理文档；暂不扩大扩展点或实施外部运行时。当前方案足以继续承载仓库内可信 Go 模块，但不宜据此承诺稳定的第三方插件 API。

### 总体判断

| 笔记 | 判断 | 应保留 | 应收紧 |
|---|---|---|---|
| 扩展模块 | 责任划分基本清楚；将功能拆出不等于过度设计 | 显式装配；必需依赖；全局/按书开关；声明账与生效表分离 | Optional 是可关闭还是可缺席；多提供方语义；可信 Go 能力和外部权限的边界 |
| 扩展 API | 类型、登记和正常启停已有基础，失败路径仍有明显缺口 | 带类型的键；Setup/Start 分离；Session；服务细分；不可变调用计划 | 调用身份、失败清理、启动失败回滚、取消等待、可选服务绑定 |
| 多语言 | 复用 mygo、区分书语言和界面语言合理，无须换库 | 显示时取词；结构化错误；生成代码入库；单一源表 | 原型平铺不能成为扩展全局命名约束；严格齐全检查和运行时回退须分清 |

### P1：启动失败会保存尚未恢复的偏好

- 证据：[ext/host.go](../../../../ext/host.go) 524–537 行先执行 OnStart，成功后才 LoadState；[ext/failure.go](../../../../ext/failure.go) 39–46 行在启动失败时直接 saveState。
- 最小内存案例：已有偏好 42，OnStart 把内存复位为 0 后失败；LoadState 次数为 0，SaveState 次数为 1，原值变成 0。
- 不是只有假扩展才可能遇到：[讨论启动](../../../../ext/talk/module.go) 86–124 行先清 Shelved、Attach 等状态，再读会话文件，读取或刷新可以失败；[saveState](../../../../ext/talk/panel.go) 196–199 行会保存这些默认值。
- 建议：区分“构造/恢复中”和“已成功激活”。只有完成恢复并成功激活的 Session 才能保存；启动失败只拆已取得的资源，不覆盖原偏好。明确 Provide → OnStart → LoadState → 发布服务/贡献的阶段及回滚规则。

### P1：旧调用的异常会停掉新 Session

- 证据：[Guard](../../../../ext/guard.go) 14–22 行只检查 owner 当前是否生效；[failCall](../../../../ext/failure.go) 70–91 行按扩展 ID 取当前 entry，直接标失败并取消当前 Session，没有比对发起调用的 Session。Call 的原调用及注入也走这条失败路径。
- 最小并发案例：旧 Guard 回调进入后暂停；主线程关闭并重新打开该扩展；释放旧回调并让它 panic，新 Session 从 On 变为 Failed。
- 这与 API 篇允许“可选工具在依赖方请求中继续执行，提供方关闭时不等待”叠加，交结果时检查 Active 太晚：recover 已经误伤新实例。
- 建议：Call/Guard/注入计划捕获发起时的激活身份，失败归属使用该身份；旧实例的错误可以留日志，但不能改变新实例。仅检查 module ID 或当前 On 不足以守住生命周期。

### P1：排回界面的扩展回调没有错误边界

- 证据：[sessionJobs.Post](../../../../desk/session_jobs.go) 20–24 行仅查 Active 后直接 fn；[jobs.poll](../../../../desk/jobs.go) 42–49 行也直接调用。StartJob 的 apply 和 Request 的完成处理都借这条路。
- 临时 desk 测试的实测：Post 回调 panic 从 poll 逃出，扩展仍为 On，账中错误数为 0。原生选目录的 picked 回调也在 [session_dialogs.go](../../../../desk/session_dialogs.go) 25–35 行直接运行。
- 建议：所有返回帧线程的扩展回调统一经过带激活身份的错误边界，保留原始 owner；完成/取消时的忙碌状态结算必须独立执行。只保护工作线程和 Draw/Run 不足以实现“一个扩展出错只停它自己”。

### P1：失败后的清理没有保证会到来的执行时机

- 证据：[failure.go](../../../../ext/failure.go) 62–65 行明确把清理推迟到下一次 Set/Open/SetOrder/Choose/Close；drainFailures 只有这些修改操作调用。正常的 [extensionFrame](../../../../desk/extensions.go) 47–59 行只处理通知和刷新派生表，没有推进失败清理。
- 最小案例：Guard 报错后 State 已为 Failed，但 OnStop 次数为 0；调用 Close 后才变成 1。现有 [注入测试](../../../../ext/injection_test.go) 374 行特意调用 SetOrder 才断言清理，不能证明失败后自行收尾。
- 后果：继续留在当前作品正常写作，不改扩展开关时，资源清理可一直不发生；[sessionJobs.Request](../../../../desk/session_jobs.go) 的 busy 结算也挂在 Session.OnStop 上。
- 建议：错误立即使激活失效并排队，宿主下一帧在调用栈外推进收尾，不依赖用户执行另一个修改动作。不能在出错的工作线程里同步等自己退出。

### P1：有副作用的 overwrite 失败后会再次执行默认操作

- 证据：[injection.go](../../../../ext/injection.go) 541–552 行在替换函数 panic 后无条件 callOriginal。
- 最小案例：替换函数先增加一次写操作计数再 panic，默认函数又执行一次；最终副作用次数为 2，Call 返回 nil error。当前测试主要验证纯数值函数回退。
- 对写文件、启动模型请求、修改正文的操作，recover 不能撤销先前副作用。“故障扩展被禁用”和“当前操作可安全重试”不是同一件事。
- 建议：默认返回明确失败；恢复默认实现用于后续调用。当前调用自动回退只能用于显式声明可重试的纯操作。Around 的 next 调用次数和 Before/After 对切片、指针的浅拷贝恢复也须写清，不承诺事务回滚。

### P2：停止等待无上限，部分任务 API 根本不给取消信号

- 证据：[Session.stop](../../../../ext/session.go) 78–89 行先 cancel、再无期限 wg.Wait，最后才清理资源；[Jobs.StartJob](../../../../ext/hostapi/api.go) 280–282 行回调没有 context；[实现](../../../../desk/session_jobs.go) 27–35 行主动丢掉了 Session context。
- 实际调用包括 [备份](../../../../ext/backup/view.go) 137–144 行的磁盘归档，以及 [导出](../../../../ext/export/export.go) 290–303 行等待原生保存对话框。关闭扩展或换书在帧线程 join 时，可能长时间无响应。原生选目录已在 session_dialogs.go 明确按宿主拥有、只丢弃过期结果处理，保存对话框却未统一。
- 现有 30 模块启停耗时测试明确排除了 IO 和任务，无法验证这条性质。
- 建议：能取消的任务必须收到 context；不可取消的系统操作归宿主，只把结果绑定 Session；必须完成的写入需要明确的离开等待语义。需要关闭资源才能退出的任务，不能把关闭动作只放在 Wait 后。不要简单超时返回后就让新旧写任务并行。

### P2：UseOptional 跳过了消费者 Session 绑定

- 证据：[registrar.go](../../../../ext/registrar.go) 157–161 行只有 Ref.Get 调 SessionBinder；OptRef 没有 owner，179 行直接 serviceGet。
- 最小案例：同一宿主服务经 Use 得到的对象绑定 Session，经 UseOptional 得到的对象未绑定，两者都报告可用。
- Jobs/Dialogs 都是 SessionBinder；可选使用这些服务会失去预期的失效/回调保护，Jobs.Run 还会落到拒绝未绑定调用的实现。未发现随包扩展目前用这种组合；这是可复现的 API 缺口。
- 建议：必需、可选只区别“缺席时怎么办”，共用解析与激活绑定；Lookup 保留宿主专用语义。

### P2：Pref.OnChange 是未说明归属的长寿命回调

- 证据：[pref.go](../../../../ext/pref.go) 120–133 行只追加监听器，不提供取消，不查扩展状态；Open 也给全部 Pref 通知。
- 最小案例：关闭扩展后 Set，其 OnChange 仍执行。若在每次 OnStart 登记，关开多次会累积监听器；回调若读已拆掉的状态，又可能把关闭的扩展标成 Failed。当前非测试模块尚未使用 OnChange，因此应在外部使用前纠正契约。
- 建议：选择并写明一种规则：Setup 期间登记且仅在激活期派发，或订阅绑定 Session 并自动撤销。声明元数据可在停用时读取，不代表运行期订阅也应执行。

### P2：Optional 同时承担“可选能力”和“强装配/排序关系”

- 证据：[host.go](../../../../ext/host.go) 86–114、126 行把 Requires/Optional 一并要求存在、一起查环、一起拓扑排序；177–180 行连可选服务也要求提供者注册。测试明确要求缺少 Optional 模块时报装配错误。
- 当前实现可支持“已经安装但被关闭”，不能支持“没安装”。A、B 互为可选也使整套装配失败，即使两个都关闭。
- 这不是建议立即支持任意环：当前仓库固定清单可以维持强约束，但笔记必须写明只支持这一种 Optional。对外之前另定缺席时的校验、在场时的顺序和互操作规则，不把内置清单的假设发布成插件契约。

### P2：多语言的全局检查抵消了扩展命名空间和缺译回退

- 证据：[extlocales.load](../../../../internal/extlocales/main.go) 242–243 行要求每个扩展的语言集合与宿主完全相同；268–269 行拒绝不同扩展相同 key；311–313 行拒绝缺译。
- 多语言笔记前面承诺按模块命名空间、缺译回退，后面又为原型平铺引入全局唯一与完整性要求。这些可以作为随软件发布的质量标准，但不能同时解释为所有扩展的运行时契约。加一个宿主语言会迫使全部扩展同时补表；两个独立扩展无法各自用 commonSave。
- 建议：原型生成时按模块保留命名空间；发布检查可以要求内置翻译齐全，插件加载仍按各自 source 回退，不因缺译拒载。自己的生成器继续只做 mygo 未覆盖的库目录与类型函数，不复制运行时消息解释器。

### 执行顺序与扩展性需要写成精确规则

以下是设计合同缺口，不混作已发生的运行故障：

1. **四种顺序分开写。** 装配顺序管展示，拓扑顺序管 Setup/Start，贡献自己的 Order/Before 管具体界面，注入顺序另算。当前注入执行固定为所有 Before → Around 嵌套 → 原调用/替换 → 所有 After；SetOrder 不能让 After 跑到 Before 前。After 当前同序，Around 当前先登记的在外层。页面所谓“调顺序”要说明可调哪一层。
2. **服务构造发生在 OnStart 前。** Provide 不是 OnStart 之后的懒工厂；依赖可用不等于本模块状态已经初始化。书架也会启动 Session（Book 为空），并非只在“这本书开着”时存在。失败回滚中哪些 OnStop 可能在初始化未完成时执行，须统一说明。
3. **context 的多提供方语义未显式定下。** [context/module.go](../../../../ext/context/module.go) 的 Entries/Outline/Hooks 找到第一个成功回调就返回；不是汇总。第一个返回空，也会挡住后面的非空来源。如果只允许一个，应检查并报告冲突；若要接入多个资料来源，应定义合并与 ID 冲突规则，不能依赖装配顺序静默遮蔽。
4. **排序锚点不要用翻译后的字。** [hostapi.Command](../../../../ext/hostapi/api.go) 的 HelpAfter/HelpBefore 是文案函数，[desk/extensions.go](../../../../desk/extensions.go) 258–270 行比较翻译结果。不同语言里两条文案相同就会歧义；应改稳定行 ID，显示函数只负责文字。
5. **可信 Go 边界不是隔离边界。** Store、ui 指针和直接服务接口依然依赖约定，无法阻止绕开注入或记账；这是当前内置层可接受的取舍。将来数据层仍须单独设计权限、请求取消、修订校验与动作身份，70 项“改成 DTO”的表不能当作已经有了数据协议。

### 过度设计与文档清晰度

- 不建议推倒注册表、取消可选依赖、撤回 marks/context，或另换 i18n。它们已经承载真实功能和开关需求。
- 建议收窄“所有经宿主调用都能注入”。查询、事件、命令、绘制的结果/错误/线程语义不同；先修现有使用路径，不再为每个读操作补命名 hook。高自定义是作者的要求，但不等于所有操作共用同一种失败恢复。
- Go/脚本双层的方向合理；目前逐项维护 70 条 DTO 想象、重复描述原型和宿主行为的成本偏高。先写统一的数据边界和少量真正不同的适配类别，等运行时确定再定协议。
- 三篇均仍为 proposed，正文却混合 11 月 2 日基线、已实施规则、后续撤销、待实施计划。modules 第 6/7 条宣布覆盖前面的多处内容；API 例子仍使用 ext.PaneTabs、ext.Commands、旧 Declare/Before 签名；i18n 同时出现“传 key”和“存函数”、旧目录和新目录。不能要求扩展作者读完历史再自行推断现行规则。
- 建议下一批就地整理：先写当前生效的职责/生命周期/类型合同；已落地部分按技能移入 implemented 并同步入站链接；尚未落地的外部运行时留 proposed。真实取舍保留，施工流水不再承担规范。模块篇“关掉后账上一条不剩”应改为“生效表一条不剩”，声明账仍保留。

### 复核（Claude，2026-11-06）

逐条对过代码：

- 前四条 P1 成立，其中三条比上面写的重：
  - 第 1 条被覆盖的是书级状态（`BookState`），偏好在 Set 时就写了，不走这条路。OnStart 先于 LoadState 的顺序要留着：讨论的 OnStart 先清空 `Shelved`、`Attach`，LoadState 再填回来，倒过来会清掉刚读进的状态。
  - 第 2 条在随包扩展里就有路径：讨论调设定的工具，在界面线程外经 `CallReadPoint` 跑（[扩展模块](2026-11-02-extension-modules.md)里写的经 `ext.Guard` 已经过时）。没有注入时走快路径，只看现在开没开，出错按 ID 记。讨论进行中把设定关了再开，旧工具 panic 会停掉新 Session。后台任务没有这个问题：`Session.stop` 等它们退出以后才会重开。
  - 第 3 条更重：desk 不 recover；mygo v0.7.4 的 `ui` 包里唯一的 recover 在 `keyHash`，另两处在 `ipc.go`（网页调 Go 的服务方法）和 `protocol.go`（自定义协议），都不包帧回调。Post 回调 panic 就是整个程序崩溃。
  - 第 4 条有个直接后果：`Session.Active` 看 ctx，出错当下就为假。正在跑 Request 的扩展出错后，结果回来被跳过，`settle` 只挂在 `Session.OnStop` 上，`a.busy` 一直不清，之后所有扩展的 Request 都在 busy 检查处直接返回，直到作者动一次开关。
- 第 5 条（overwrite 重做副作用）契约上成立，降为 P2：测试以外没有 `Overwrite`、`Replace`、`Around` 的用法。其中一处现在就错：`callBase` 和 Around 的 recover 丢掉了 `callFailure` 返回的错误，扩展出了错 `Call` 仍返回 nil。Around 在调 next 之前 panic 也会重跑 next，是同一类问题。
- 多语言一条：全局不重名、语言集合一致，是对仓库内扩展的生成期检查，原因写在[多语言](2026-11-02-i18n-on-mygo.md)：原型把这些表平铺在一起用。它不是运行时加载规则，mygo 的缺译回退照样在。同意按模块加命名空间，留到外部扩展之前做。
- context 的三个点现在各只有一个提供方（设定、伏笔、章纲），遮蔽还不会发生。帮助行锚点确实比的是翻译后的字（`desk/extensions.go` 260、268 行）。

### 修复方案（第四版，Claude 与 Codex 一致，作者已确认）

作者先说「先写方案」，看过两人的讨论后说「先写笔记吧」；2026-11-06 看过推进计划后说「确认」，并定「第一第二同步推进」：本篇和[盘点](2026-11-06-reinvented-wheels.md)里定下的几项同时施工。第一到第三版经过下面两节复审逐版加重；作者提醒复审可能过度防御，于是 Claude 和 Codex（astra）按下面的标准重新讨论了两轮，下面各条双方都同意。下面两节复审里与本版冲突的建议，已经在讨论中撤回：两段式停止及其连带机制、给每一项带身份、重新设计请求结算。

**标准**

- 现在只有仓库里的可信 Go 模块，第三方插件 API 还没承诺，也没有兼容负担。只为随包代码里真能走到的故障建机制。
- 一项防御要留下，至少满足一条：随包代码里有能走到的路径，说得出具体后果；或者代价很小，不新增 API 概念、不改调用签名。
- 已有的结构能保证的不再加协议；没人用的 API 删掉；边界写进文档，不为边界造机制。
- 只有「将来外部插件可能这样」这一类理由的，记为对外之前要补，不放进这一批。

**1. 停下仍然同步**

- 保留现在的做法：装配按拓扑序，停的时候倒着停，等 Session 的任务退出、跑完清理才返回。讨论的任务等完、讨论收尾以后，才轮到 AI 关后端，依赖顺序不用另外管。
- 停下可能长时间等待，当前接受这个代价，笔记和 API 篇如实写明，不承诺不卡。
- 能缩短等待的几处照做：
  - `StartJob` 的工作函数收 ctx。
  - 备份归档在读取、压缩、提交之前各查一次取消；提交成功以后按「已完成」报。
  - 备份 `Run` 里的 `close(done)` 改成 defer。现在工作函数 panic 会跳过它，`backupNow` 和备份的停下回调都会一直等着。
  - 原生保存对话框照 `PickInto` 的做法归宿主管，不再算 Session 的任务。`Dialogs()` 返回两个裸函数的写法删掉；「在文件夹里显示」（章纲在用）改成 `Dialogs` 上的普通方法。

**2. 出错只算在当时那次 Session 上**

- `Guard`、无注入的快路径、注入计划，在检查「开着没有」的同一把读锁里记下当时的 Session，出错只记到它头上。它已经不是当前实例时，不改状态、不进账、不弹提示，只写日志。
- 不改调用签名，不给每一项带身份类型。工作线程上的 `PointReader.Capture` 照旧能用。
- 任务的 `onPanic` 按任务自己的 Session 记。

已施工（第一批前半）。

**3. 出错后的收尾每帧推进**

- `Host.Settle()` 推进待收尾的扩展；没有待收尾的就直接返回，不碰锁。Set、Open 仍先调它。
- 等任务时不持 `h.mu`，照现在 `stop` 的锁分工。任务不能等帧线程。
- 每帧的顺序：装好本帧的唤醒函数 → `Settle` → 检查请求所属的 Session 并结算 → 处理扩展的错误通知 → `jobs.poll`。
- 任务迟迟不退出时，帧会停在 `Settle`，这就是第 1 条接受的代价。

接口和测试已施工（第一批前半）；desk 帧循环接入留给下一批。

**4. 回到帧线程的回调和 busy**

- 扩展回到帧线程的回调包上出错边界：Post、StartJob 的 apply、`PickInto` 的 picked、保存对话框的结果。宿主自己的结算和 panic 留在边界外，`jobs.poll` 本身不 recover。
- busy 是 App 的状态。Request 开始时在 App 上记下这次请求所属的 Session，和 epoch 放在一起；每帧看到这个 Session 已经不 Active，就按 epoch 结算。出错、关掉、换书、工作函数 panic 都在下一帧结清，旧任务的结果因为 epoch 不对被丢掉。
- 删掉每次 Request 往 `Session.OnStop` 追加的那一条。请求的 ctx 本来就经 `context.AfterFunc` 挂在 Session 上，Session 停了请求会自动取消。
- 新增的归属字段要在这几处一起清：
  - 正常结束：`finished` 清 busy 时一起清，在跑 apply 之前；业务错误的重试照旧。
  - 宿主自己发请求（`App.work`）时置空；`resetView` 清 busy 时一起清。
  - 退出程序：`App.Close` 在 `leaveWork` 返回后、推进 epoch 之前手动结算一次，否则 busy 还在，会跳过关 Store。
- 结算不要用「已取消」的提示盖掉扩展出错的通知。

**5. 书级状态只在成功开过之后保存**

- 保留 Provide → OnStart → LoadState → 开着的顺序。只有到过「开着」的 Session 才保存；启动、LoadState 失败只拆资源，不写回。
- 用着用着出错停下时照旧保存：那是作者用到出错为止的状态。

已施工（第一批前半）。

**6. 注入出错不重做**

- 同一次调用不自动重做，next 至多调一次。删掉「可改跑原来的」：mygo 在画界面时处理交互，Draw 也有副作用。
- 原实现、Overwrite、Replace 出错，或 Around 没调 next 就出错：`Call` 返回零值和 `ErrCallFailed`。现在 `callBase`、Around、Before、After 丢掉 `callFailure` 返回值的几处一并改。
- 内层成功、只是外围注入方出错（Before、After、next 成功以后的 Around）：注入方记账停下，`Call` 返回内层结果和 nil。
- 写进 API 篇的三个限定，不加机制：
  - 调过 next 不等于 next 成功，内层的 `ErrCallFailed` 照样返回，不被外层改成 nil；Around 不调 next 自己返回是合法的短路。
  - Before、After 只按值恢复，改过的切片元素、指针指向的内容不回滚。能保证「保存照常做一次」，不能保证「保存的是注入前的参数」。
  - `ErrCallFailed` 不表示没有副作用，调用方不能据此自动重试写操作，也不补做默认操作；施工时逐处核对 desk 的调用点。
- next 只能在回调里同步调用、最多一次，写成约定，不为跨线程或缓存 next 加机制。

**7. 收掉的几处 API**

- `UseOptional` 和 `Use` 共用解析和 Session 绑定，只差服务没有时返回 false。
- 删掉 `Pref.OnChange`：测试以外没人用。
- 帮助行的 `HelpAfter`、`HelpBefore` 改用稳定的行 ID，显示函数只管文字。
- context 的三个点只能有一个提供方，不加 API，用一条测试守：随包的扩展清单里每个点最多一个。
- Optional 只指「装了但关着」，写进模块篇。

**8. AI 工具的结果如实返回**

- 讨论调设定的工具时，快照里的检查和调用边界记下 Session 之间，设定可能被关了又开，工具在新实例上真的写了设定，事后检查却回报失败。
- 改法：
  - 在传给 `CallReadPoint`、`Call` 的原调用闭包里，查快照记下的那个旧实例还开着没有，失效就返回失败、不执行工具。
  - 删掉 `ext/ai/aiapi/snapshot.go` 30、91 行的事后检查：工具做完了，就如实返回它的结果。
  - `propose_edit` 的 `v.tool()`（`ext/propose/module.go` 183–191 行）已经会判断建议收没收下，结果也按这个判断返回：没收下就返回失败、不带建议。否则旧实例的建议会经讨论的事件绕进新实例，模型也会以为建议已经显示了。

**这一批不做**

- 多语言的命名空间，以及三篇笔记的就地整理。模块篇里「工具经 `ext.Guard` 跑」已经过时，整理时一起改。
- 外部运行时、安全模式、通用的身份协议、异步隔离、深拷贝。

**验证**

审查里的案例都用假扩展写成测试：

- 启动失败、LoadState 失败都不写回书级状态。
- 设定或 AI 关了再开期间，旧调用 panic 不碰新 Session；工具在旧实例失效后不执行。
- Post、StartJob、Request、`PickInto` 的回调 panic 记在对的扩展名下，下一帧接着画。
- Request 的工作函数正常返回、返回 error、panic、Session 被停，都在下一帧结清 busy，旧请求清不掉新请求的 busy；退出程序时 busy 结清，Store 照常关。
- 出错以后不动开关，下一帧也会停掉它并跑完清理。
- 备份工作函数 panic，`backupNow` 不会卡住。
- Overwrite panic 时副作用只有一次，`Call` 报错；原实现 panic、外层 Around 再 panic 时仍报原来的失败；next 成功以后 Around panic 返回结果和 nil。
- propose 关了再开时，旧建议不进新实例，工具返回失败。
- `UseOptional` 拿到的服务绑定了 Session；context 的点各只有一个提供方；换界面语言后帮助行的位置不变。

另外：重启耗时的三个测试不能变慢；box 上 `npm run verify` 全绿。

**施工**

分两批交 Codex，每批在 box 上跑通 `npm run verify` 再交下一批：

1. ext：第 2、5、6、7 条，第 8 条的工具检查，`Settle` 接口（先不接进帧循环）。
2. desk 和各扩展：每帧调 `Settle`、第 4 条、第 1 条的备份和对话框、帮助行 ID、propose 的返回。`Settle` 接进帧循环和备份的 `defer close(done)` 必须在这一批同时落地。

### 第二版复审（Codex，2026-11-06）

第二版已经解决第一版中 Request 漏结算、Draw 默认回退、只绑贡献方身份、把 Settle 提前接入的主要方向问题。F 的取舍可以接受，撤回第一版复审中“next 成功后外围 Around panic 必须结果加错误”的一刀切建议。不过下面三组条件仍须补入实施合同，不表示产品已修复。

**F：区分调用结果和外围注入故障可以，但不是 exactly-once 承诺**

- next 的内层调用正常完成、仅外围 Around/After 出错时，保留该内层结果并返回 nil，故障通过扩展账和提示报告，是合理的降级语义。若 Out 自身带业务 error 或 Failed 标记，原样保留；这里的 nil 只表示扩展调用边界没有失败，不等于业务操作成功。
- “调过 next”不等于“next 成功”。原实现 panic、内层 Around 在调用前失败、或目标失效产生的 ErrCallFailed 必须保留，不能被外层 Around 的故障处理改成 nil。Around 正常不调 next 而自行返回，仍属于已有的合法短路，不算操作失败。
- Before 的参数恢复只有值拷贝。临时测试传入含切片的参数，Before 修改元素后 panic，原调用读到 modified、原输入也变成 modified、Call 返回 nil。故不能承诺“保存的是注入前参数”；对不能接受这种影响的输入，需逐点提供独立副本或禁止直接修改引用指向的数据。After 同理，不引入通用深拷贝框架。
- ErrCallFailed 也绝不表示“没有任何副作用，可以重试”：原实现或 Overwrite 仍可能先写入再 panic。契约应写“同一次调用不自动重做、next 至多一次”，而不是推导出业务恰好执行一次。调用方遇错不得据此盲目重试写操作。
- 本次测试确认：现有 Around 成功 next 后 panic 保留结果的测试通过；新增“原实现 panic + 外层 Around 再 panic”测试保留原失败。实现第二版时这两个分支都要守住。

**A：目标身份和注入方身份不能用一个“任意失效就整次拒绝”的规则**

- 第二版 A 写身份里任一 Session 失效则整次不执行；F 又要求 Before 出错停掉它后，原操作照常做一次。若把已停注入方也算作原操作必需的身份检查，两条互相矛盾。
- 目标贡献方与扩展点拥有者是原调用的必需身份；它们失效，拒绝开始原调用。某个 Before/After/Around 注入方失效，应跳过它而保留仍有效的下游；已进入失败的 Overwrite/Replace 不在同次回退。若注入方失败连带使目标也失效，则仍须停止，不能为了“继续保存”调用已拆状态。
- 捕获身份应是“同一个快照里取贡献值、目标身份和计划”，禁止拆开按 ID 重取。帧线程获取不必反复查表，但保留工作线程的原子 Capture：[ai/accounting.go](../../../../ext/ai/accounting.go) 21 行在每次模型请求开始取 Calls，现有 [PointReader.Capture](../../../../ext/reader.go) 明确允许工作线程调用。不能为了“只在帧线程取身份”退回创建模型句柄时取定，否则用量扩展关开后缓存的模型句柄会再次丢记录。

**D/施工：两段式停止还要守住依赖资源、旧书和清理完成的顺序**

- 不能各模块只等自己的任务完成就独立 OnStop：消费者任务可能仍在使用提供者资源，提供者却没有自己的 Session 任务。比如 [AI 的 OnStop](../../../../ext/ai/module.go) 109 行关闭后端，讨论请求在讨论的 Session 中。停止先取消这组模块，再按原依赖逆序完成资源释放；至少保证提供者不早于使用它的旧任务和消费者清理结束就释放资源。同一模块不得提前重启。
- [desk.attach](../../../../desk/app.go) 164–174 行现在 leaveWork 后立刻关闭旧 Store 并切换 a.Store/a.book。若 leaveWork 变成异步，不能原样保留：[workspace.Store.Close](../../../../workspace/store.go) 47 行会等 Store 的锁，仍可卡住；后续旧任务的写入也会收到 closed。必须把旧 Store 释放纳入旧 Session 的收尾，明确切书请求和收尾的协调方式。
- 延迟 OnStop 不能再通过“当前书”服务操作新书。[lore.OnStop](../../../../ext/lore/module.go) 44 行会 SaveEdits，[propose.OnStop](../../../../ext/propose/module.go) 133 行会清编辑器标记，而 [extensionHost.Store/Content](../../../../desk/extensions.go) 482–484 行每次读当前 App。保持旧书及旧编辑器的明确归属，或把需要当前界面的收尾放在切书前的停用阶段；只阻止同模块 Start 不够。
- 第二批接入 Settle 时，备份 close(done) 的 defer 也必须提前到第二批。否则后台备份 panic 后，Session 任务已退出，Settle 就进入 [backup.OnStop](../../../../ext/backup/module.go) 69–72 行等待永远不会关闭的 Done，重新卡死帧循环。完整归档取消、对话框迁移、帮助行和 context 约束仍可留第三批。
- 验收“下一帧跑完 OnStop”应改成“下一帧开始推进；任务未退出时保持响应，退出后再收尾”，否则与阻塞两秒任务测试矛盾。两段式停止消除的是帧线程 join 任务，不自动消除其他同步存盘或 Store 锁等待。

本次只核对方案和运行针对性测试，未实施修复。四项相关测试通过；临时测试随后删除，笔记检查另跑。第一版复审保留在下文作历史依据，其中已经由第二版处理的建议不再算未解决项。

### 第一版修复方案验证（Codex，2026-11-06）

结论：A–G 的主要方向成立，尚不能原样施工。以下核对的是提案的充分性，不表示修复已经完成。保留上面的复核和方案，下面列出需要补进方案的条件。

**截图中三项事实的核实**

- 帧回调的 panic 没有 mygo 兜底：本机 go.mod 锁定的 mygo v0.7.4 中，ui/runtime.go:597 直接执行 view；ui/context.go 的 recover 用于 keyHash，ipc.go:297、protocol.go:134 的 recover 分别保护网页服务调用和协议处理，不保护这个栈。其他子系统也有 recover，不能表述为整个库只存在这几处，但“未隔离的 Post panic 会终止程序”的判断成立。
- 备份确实先写临时文件、提交后再清旧备份：[archive.Make](../../../../ext/backup/archive/backup.go) 160–180 行使用 Create → write → Commit → Prune；[atomicfile.Commit](../../../../internal/atomicfile/atomicfile.go) 48–59 行是 Sync → Close → Rename。但“取消时旧备份一份不少”必须限定为提交前取消；当前函数尚无 ctx，不能说现有实现已支持取消。
- “在文件夹里显示”确实仍由章纲使用：[plan.go](../../../../ext/outline/plan.go) 285–287 行。宿主 [RevealFolder](../../../../desk/welcome.go) 281–288 行只启动系统文件管理器、不等待其退出。保留为有明确线程约定的普通方法可行。

**1. D 仍不能兑现“停止等待不卡界面”，必须修正承诺或实现两阶段停止**

给任务传 ctx 是合作取消，不会打断已经阻塞的文件读取、写入、Sync 或资源清理。[archive.Read](../../../../ext/backup/archive/backup.go) 114 行先整文件读取，[readWork](../../../../ext/backup/view.go) 115–120 行还在 Store.Hold 内；只给 Make 在文件间查 ctx，覆盖不到这些时间。Session.stop 保留帧线程 wg.Wait，仍可能长时间卡住。

若要保证帧线程不等任务结束：停止先撤销调用资格、发取消并保留待收尾记录；任务完成后请求一帧，再在帧线程做 OnStop。旧 Session 未完成收尾前，不可启动同模块的新 Session，也不可把共享状态重置给新书。不能只是把 Wait 移到任意 goroutine 后立即重开。若这一批保留同步等待，标题和验收必须降为“合作取消，仍有 IO 阻塞风险”，不能继续承诺不卡界面。

备份还需明确提交点：读取、压缩、提交前检查取消；提交成功以后按“已完成备份”处理，不回报“已取消且未改任何文件”。Prune 不是可回滚事务。只有文件间检查，不能保证大文件途中及时退出。所有结束信号须用 defer 保证关闭；现有 Run 的 close(done) 在正常路径末尾，工作函数 panic 会跳过它，OnStop/backupNow 再等 Done 就会卡住。

**2. B 的 busy 结算必须涵盖工作函数自身 panic**

“结果回来先结算”只处理正常返回。临时测试 TestReviewPlanRequestPanicHasNoCompletion 实测：Request 工作函数 panic 后，busy 仍为 busy，完成队列为空，扩展为 Failed；执行旧 OnStop 后 busy 才清空。原因是 Session.Go 外层 recover，Request 函数内排结果的代码已被跳过。

删掉每次请求累积的 OnStop 注册是对的，但必须先有独立的请求终结机制：正常、error、panic、取消/停用都能恰好一次按 epoch 结算，不依赖 Active 的结果过滤。旧请求不得清掉新请求的 busy/retry。若工作函数不退出，仍须由宿主决定是否允许别的请求，不能把“任务已取消”和“任务已完成”混成一个状态。

**3. F 不能把“各种 Draw”统一列入可回退的纯操作**

mygo 是在构建界面时处理交互的；[phone.go](../../../../ext/phone/phone.go) 317–326 行的绘制直接改偏好并处理点击，其他 Draw 也可能保存、导航、发请求，已经生成的节点也是副作用。发生 panic 后当前帧再调默认 Draw，不自动具备回滚条件。

建议本批所有可变操作和 Draw 都不做同次自动回退；只给逐个核实为纯查询的点开放显式标记，或暂不加该标记。Before/After 的 panic 也要定义是否中断、是否返回 ErrCallFailed；现有实现同样会丢掉 callFailure 的错误，仅补 Overwrite/Replace/Around 不够。Around 的 next 规定同步、最多一次，不准缓存到回调退出后；已执行 next 后报错应返回保留的结果加错误，不能再被 run 的统一零值分支抹掉。

**4. A 需保留点拥有者的激活检查，并覆盖真实工具路径**

当前 [targetActive](../../../../ext/injection.go) 254–260 行既查贡献方，又查点的拥有者。比如 lore 的工具贡献给 ai.tools：lore 的 Session 还在，AI 却可以关开，旧调用不应被当成新 AI 实例的调用。缓存的计划须保留调用相关的贡献方、注入方、点拥有者身份，宿主点另作常驻处理，不能把 nil Session 当作宿主已停。

截图引用的“工具经 Guard”已经过时：当前 [SnapshotReadTools](../../../../ext/ai/aiapi/snapshot.go) 87 行走 CallReadPoint。Guard 捕获身份的测试不够，还要测真实 CallReadPoint/Call、无注入快路径和旧工具快照。快照应携带预期 Session 进入调用边界，不能只在边界前检查 Active 再按 ID 获取当前计划，留下检查与使用之间换新的窗口。

**5. C、D 的交付顺序要一致，E/G 可按当前收窄方向做**

方案正文要求 C 和 D 一起，施工却把 C 放第一批、D 放第二批。第一批可以增加尚未接入帧循环的 Settle 接口；desk 真正开始每帧调用它的那一批，必须同时完成停止策略和请求结算。context 的单提供方只需给这三个点加基数约束，不应重新引入已删掉的整套候选选择机制。

E 保留 OnStart → LoadState 的现有顺序、以“曾成功激活”限制保存是正确的；这里是 BookState（书级界面状态），不要与 Pref.Set 的设置持久化混淆。G 的统一引用绑定、删除尚无产品调用方的 OnChange、稳定帮助行 ID、明确 Optional 仅指已安装模块的开关均可采用。

context.AfterFunc 的 close 在独立 goroutine 执行，须保证资源可并发关闭；返回的 stop 不等待 close 完成，需要的同步由拥有资源的任务负责，也应守住扩展错误边界。

**验证补充**

- go test ./ext ./ext/backup/archive ./internal/atomicfile -count=1 通过。
- 临时 Request-panic 案例通过并确认缺口，测试随后删除；产品代码未修改。
- 本次未重新运行全套 UI/原型走查；上面的全套失败记录仍有效，不能写成“修复验证通过”。后续落地须补工作函数 panic、提交边界取消、旧工具快照、点拥有者关开和慢任务不阻塞帧循环的测试。

## Alternatives considered

- **只改三篇文案，不审实现。** 成本最低，也能消除过期示例；但已复现的跨 Session 故障、偏好覆盖和回调边界缺失不能靠措辞解决。
- **改成另一套通用插件框架或立即上脚本隔离。** 可以统一生命周期和权限，但会扩大迁移，不能自动解决作者现有的书级状态、后台请求与正文操作语义。当前优先修现有宿主。
- **保持框架，针对失败路径补契约和验证。** 修改范围最可控，保留已有扩展和 UI；代价是必须逐个清点所有回调入口，不能只补一个总 recover。建议采用。

## Acceptance criteria

- 启动或 LoadState 失败不覆盖上次有效偏好；成功激活后才允许保存。
- 旧调用在提供方关开后返回或 panic，不更改新 Session 的状态，不往新实例提交结果。
- Post、Request、StartJob 和原生对话框结果回调 panic，能定位正确扩展；下一帧继续，其他回调有明确执行规则。
- 不再执行额外 Set/Open，失败扩展也会完成收尾；任务不等待自己，busy 能结清。
- 带副作用的替换失败不在同次调用自动重做原操作。
- 取消有真实的阻塞任务测试；不可取消的原生操作不让帧线程无期限等待。
- 必需/可选引用统一绑定；停用和重启不会继续触发或叠加旧监听器。
- Optional 缺席与环、context 多提供方、注入各阶段顺序都有明确文档和对应验证。
- 内置翻译完整性和插件缺译回退分开验证；排序不依赖翻译文案。
- 修复落地前运行 npm run verify，报告真实结果，不用现有测试通过替代上述反例。

## Risks

- 修改失败回退是行为变化，必须同步注入测试和笔记，不能只悄悄换实现。
- 仅加 shutdown 超时会留下旧任务继续写旧/新书的风险，必须连同激活和写入所有权设计。
- 当前可信模块没有使用的 API 组合也有缺口；它们阻碍外部契约稳定，但不等于用户已经遇到对应故障。
- 此次评审没有实施修复；临时复现程序和测试运行后移除。

## Verification

已用独立的内存程序复现：失败清理延后、启动失败写回未加载状态、旧 Guard 误伤新 Session、UseOptional 未绑定、停用 OnChange 仍触发、overwrite 副作用重做。另用临时 desk 测试确认 Post 的 panic 逃出且不记扩展错误。上述程序不触碰用户作品，运行后均已移除。

验证结果：

- npm run verify-notes 通过，90 篇笔记的树、格式和相对链接有效；git diff --check 通过。
- npm run verify 首次在沙箱内因 Go 缓存权限和浏览器调试连接失败；获准在沙箱外重跑。vet/i18n、notes、独立 frame timing、ext 及各扩展包测试通过。
- 全套重跑未全绿：workspace 的 TestLocalExtDirIsPerBookAndRejectsInvalidIDsAndSymlinks 因当前 Windows 客户端缺少创建符号链接的权限失败。
- TestEnglishFooterFitsPrototypeCounts 在 footer_api_test.go:42 失败，单独重跑仍失败；这与多语言验收要求有关，但本次未进一步修改或定位布局。
- TestContextAndOutlineBookRestartIsCheapAndClearsSessionState 在并行全套验证中报 1.483 秒；单独重跑通过，记录为负载敏感的计时失败，不据此认定启停逻辑错误。
- 原型坐标点击测试通过，后续完整走查超过 18 分钟未退出；已获准停止本次 verify-prototype 子进程。原型验收未完成，不能标为通过。

## 相关笔记当场审计

- [扩展模块](2026-11-02-extension-modules.md)、[扩展 API](2026-11-03-extension-api.md)、[多语言](2026-11-02-i18n-on-mygo.md)：部分重叠，保留原决定，互链。本篇是待采纳的纠错建议。
- [扩展安全模式](../feature/2026-11-03-extension-safe-mode.md)：相关；仍待实施，不能用它代替本次普通运行期失败收尾。
- [项目架构审查](2026-10-27-maintainable-project-review.md)：部分重叠；本文聚焦扩展与多语言的运行时合同，不重复 App 整理建议。
- [desk 状态](../../implemented/architecture/2026-10-22-desk-state.md)、[文件存储](../../implemented/architecture/2026-10-08-m1-storage-without-index.md)：保留其线程和存储约束，没有被本次建议替代。
- rejected 中的讨论创意池方案与本次无关；无应删除或归档的已有笔记。
