# Agent Note: 扩展契约失败路径修复

Status: implemented

## Problem

启动失败曾覆盖未恢复的偏好，旧调用异常可能误伤新 Session，注入失败可能重做副作用，帧回调和失败清理缺少完整边界。

## Decision

保留可信 Go 模块的分层与同步停止，修复现有可达路径；不承诺稳定第三方插件 API。现行规则见[API](2026-11-03-extension-api.md)、[模块](2026-11-02-extension-modules.md)及[多语言](2026-11-02-i18n-on-mygo.md)。

失败按调用开始时的 Session 归属，启动未成功不保存状态，注入不自动重做。每帧 Settle 推进清理，失活 Request 按 epoch 结清 busy，旧结果不能清掉新请求。App.Close 在关闭扩展以后结算扩展请求，Store 照常关闭。

备份 Run 工作函数用 defer 关闭 done，读取、压缩及提交前检查取消；提交成功按完成报告。StartJob 接收 ctx。原生保存对话框由宿主管理，使用 SaveInto/Reveal。UseOptional 绑定 Session，删除 Pref.OnChange，帮助行使用稳定 ID。AI 工具在执行前检查快照 Session，执行后如实返回结果。

### 调用与取消的限制

检查开着没有和捕获 Session 在同一读锁中完成，覆盖 Guard、无注入快路径及注入计划；任务 panic 属于自己的 Session，旧实例只写日志。工具快照在原调用闭包里确认旧激活，避免检查后换实例的窗口；执行成功后不再事后改报失败。点拥有者的启停同样影响调用资格。

Request ctx 经 context.AfterFunc 随 Session 取消，busy 不靠不断累积 OnStop 回调清理；finished 在 apply 前清归属，App.work/resetView 同样清理，Close 在 leaveWork 后、epoch 前结算。取消回调的 stop 不等待正在执行的回调退出，资源同步仍由拥有资源的任务负责。备份取消只是合作检查，不能打断阻塞中的单次 IO；Prune 不是可回滚事务，提交成功后按完成报告。

### 审查历史与依据

2026-11-06 记录的推进授权为「确认」及「第一第二同步推进」；其对应的是契约修复与盘点中的工作并行，不是扩大防御范围。

几轮复审曾提出两段式停止、每项带身份和重新设计请求结算；第四版撤回这些建议。现有同步拓扑停止已经保证依赖资源的寿命，额外协议没有当前可信模块的可达路径支撑。保留同步等待的代价，不用超时掩盖仍在写入的任务。最初的临时复现程序已移除，回归测试承接验证。

### 验证

`ext` 的契约测试覆盖启动回滚、旧 Session、注入失败与可选服务绑定；`desk/contract_batch2_test.go` 覆盖帧回调、请求结算、退出和语言切换；`ext/backup/worker_panic_test.go` 覆盖工作函数 panic 后 done 关闭及离开不阻塞。未实施范围见[后续范围](../../proposed/architecture/2026-11-06-extension-followups.md)。

## Alternatives considered

- **只改三篇文案，不审实现。** 成本最低，也能消除过期示例；但已复现的跨 Session 故障、偏好覆盖和回调边界缺失不能靠措辞解决。
- **改成另一套通用插件框架或立即上脚本隔离。** 可以统一生命周期和权限，但会扩大迁移，不能自动解决作者现有的书级状态、后台请求与正文操作语义。当前优先修现有宿主。
- **保持框架，针对失败路径补契约和验证。** 修改范围最可控，保留已有扩展和 UI；代价是必须逐个清点所有回调入口，不能只补一个总 recover。已采用：保留现有框架，逐入口补错误边界和回归测试。


## Consequences

不重做失败调用避免重复写入；旧 Session 不再误伤新实例，启动失败不覆盖偏好，每帧收尾及退出结算消除长期 busy。新增边界局限于扩展回调，jobs.poll 和宿主结算自身的 panic 不被总 recover 吞掉。

保留同步依赖停止，避免 AI 后端在讨论任务退出前关闭；代价是帧可能停在 Settle，任务必须响应取消且不得等帧。只按值恢复 Before/After，不能回滚指针副作用；ErrCallFailed 不保证未写入，调用方不能补做默认操作。没有引入通用身份协议、深拷贝或隔离，外部插件仍须另定契约。

### 验证记录的范围

最初审查的全套验证并未全绿：Windows 符号链接权限使 LocalExtDir 测试失败，EnglishFooterFitsPrototypeCounts 单跑仍失败，重启计时在并发负载下报 1.483 秒、单跑通过，原型完整走查超过 18 分钟后停止。保留这些为当时的验证缺口，不据此宣称当前全部仍失败，也不能用本次笔记检查补算原型验收。

后续三项测试补齐时 go build、go vet、go test ./desk ./ext/... 已通过；本次返工只改笔记，没有重跑产品或原型验收。
