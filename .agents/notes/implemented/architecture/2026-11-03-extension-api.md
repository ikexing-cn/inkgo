# Agent Note: 宿主给扩展的 API

Status: implemented

## Problem

扩展需要类型明确的登记、启停和错误边界，避免旧任务污染新的书和激活实例。

## Decision

`Module.Setup(*Registrar)` 每次装配只运行一次，只登记，不取服务、不启动任务。`ext.Use` / `ext.UseOptional` 返回引用，两者共用 Session 绑定；Optional 只在服务不可用时返回 false。服务按 Provide → OnStart → LoadState → 发布的顺序激活，停止按拓扑逆序同步取消、等待任务并清理。只有成功激活过的 Session 才保存书级状态；启动失败不覆盖原状态。

`Session.Go` 接收 ctx，工作任务不能等待帧线程；同步停止允许长时间等待。`Jobs.StartJob(ui.Services, func(context.Context) func())` 的返回函数在帧线程执行。Post、StartJob 的 apply、Request 和原生对话框结果回调都有所属 Session 的错误边界。原生对话框由宿主管理，不属于 Session 的等待任务；使用 `PickInto`、`SaveInto` 和 `Reveal`，没有返回裸函数的 `Dialogs()`。

`Pref` 按需读写持久值，没有 `Pref.OnChange`。`HelpAfter` / `HelpBefore` 按稳定行 ID 定位，显示函数只负责文案。每帧由宿主调用 `Host.Settle` 收尾失败的 Session，再结算失活请求、处理错误通知和轮询 jobs。退出在 leaveWork 后、epoch 推进前结算请求，保证 busy 不阻止 Store 关闭。

贡献经 `ext.Contribute` 登记；调用点经 `ext.DeclareCalls` 声明，注入使用 Before、After、Around、Overwrite 与 Replace。调用计划和 Guard 捕获检查时的 Session，旧调用失败不影响新实例。查询型非注入回调可用 Guard；工具使用 Call/CallReadPoint。

同一次调用不自动重做。原实现、替换或未调用 next 的 Around panic 返回零值和 `ErrCallFailed`；外围注入失败但内层成功时保留内层结果。next 只可同步调用一次，不能缓存或转交线程；内层失败不能被外层抹成成功。Before/After 只按值恢复，指针和切片内容不回滚，调用方不能把 ErrCallFailed 当作无副作用。

### 状态、登记与调用计划的补充约束

Session 的三类状态不能混用：本次开启的任务、信箱和临时 UI 在 Stop 后作废；同书可接续的输入与未定提议按捕获的 Store 区分，换书清空；长期界面偏好经 Persist 保存。宿主不再依赖 ViewState.Reset。`.writer/desk.json` 各扩展一段原样 JSON，关闭扩展的段保留；读失败先使用默认值并提示，首次写入前保留 `desk.json.unreadable`，保留失败则不写，避免某扩展坏状态毁掉别的扩展偏好。

Stop 在取消、等待后逐条跑 Session.OnStop 和 Registrar.OnStop，单条 panic 记账但不跳过后续清理。任务不得等帧线程；任务持有启动时的 Store，不在后台再问当前是哪本书。Jobs 按消费方 Session 绑定，贡献执行时同样检查当前激活。宿主提供的服务一直在，不必写 Requires，线程要求见 `ext/hostapi` 各方法注释。

贡献与服务 ID 由宿主加模块前缀，同名在 Setup 报错。公开 `<id>api` 只放类型、键和无状态适配，不存开启状态。具体源码许可及 desk 的有限例外以 `ext/boundary_test.go` 为准。Declare 必须给出 Place（不可见用 Hidden），名字提取函数只做本地化；读点同样入账并校验存在及类型。

链在登记、开关和排序变化时构建为不可变计划，不逐调用拼闭包。Before/After/Around 可叠加，Overwrite（包括整项 Replace）单处只允许一方；多方未选时用默认，顺序和选择持久化。`Host.Changes()` 无分配计数供 desk 缓存失效，语言修订号单独参与缓存判断。宿主内部没有注入键。

设置按 JSON 原子写入本机或书根 ext.json；Local、BookScope、Both 区分覆盖范围。读坏文件先按默认并提示，真正保存前不覆盖。自动设置表单能力和自定义 SettingsSections / SettingsPages 见模块篇；没有 Pref 监听器，也没有未实现的具名设置分组 API。

### Go 层与未来数据层

当前是可信 Go 接口，不把 *ui.Context、ui.Element、Store 或 Go 回调当作跨运行时数据。原来的“两层”和逐点数据适配表由[后续范围](../../proposed/architecture/2026-11-06-extension-followups.md)承接；新增点须说明普通数据表达方式，不能表达的标只限 Go。现行接口和签名以 `ext/hostapi`、各 `<id>api` 及 `ext/*_test.go` 为入口；旧四个代码示例已经过时，不再复制另一套签名。

菜单位置使用 hostapi 的具名菜单与前后锚点，命令提供普通及 macOS 快捷键字段；ShortcutSource 和 Options.HostKeys 统一参加冲突裁定。补全来源提供变化键供缓存，而非每帧重取设定；具体例子见 `ext/phone/module.go`、`ext/export/module.go`、`ext/names/module.go` 和 `ext/classic/module.go`。

### Testing

`ext` 下生命周期、注入、服务及账的测试守住 Setup 限制、冲突、Optional、旧 Session 与清理；`desk/contract_batch2_test.go` 检查帧结算和回调。context 随包提供方唯一性由测试约束，不增加通用单值点。换书开销及正文帧耗时保留现有计时测试。

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


## Consequences

类型键、声明账和 Session 绑定使错接服务、重复 ID、旧任务提交能在装配或调用边界发现；不可变调用计划及 Changes 缓存避免正常帧反复分配。代价是增加间接调用和缓存失效责任，调用 next、指针数据及后台线程仍有需要扩展遵守的约定。

同步取消并等待保护资源寿命，但慢 IO 或不响应取消的任务仍阻塞停止和 Settle；不以超时假装已经释放。恢复成功才保存避免覆盖旧偏好，读坏文件的保留机制增加恢复分支。panic 边界保证归属与后续清理，不承诺正在绘制的那一帧完整；mygo 中途绘制失败的帧完整性仍是未核实边界。将来数据适配层未实现，当前 Go 指针能力不能直接暴露给不可信插件。
