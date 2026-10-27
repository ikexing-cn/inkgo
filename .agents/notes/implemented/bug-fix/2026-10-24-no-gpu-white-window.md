# Agent Note: 「禁用 GPU 加速」先不给：mygo 不用 GPU 画时，Windows 上的窗口多半是白的

Status: implemented

> 2026-10-27 起这一项又给了：mygo v0.6.4 修了白窗口，`offerNoGPU` 和 `OFFER_NO_GPU` 改回 `true`（见文末「代价与已知上限」）。下面说「先不给」的那几条是当时的做法，开关和 `TestNoGPUNotOffered` 还在，改回 `false` 就又藏起来。

## Problem

[禁用 GPU 加速](../feature/2026-10-24-no-gpu.md)加上的当天，作者勾上以后：「我发现了，禁用了就开始白屏了，百分百复现。」窗口整块是白的，设置也看不见，勾不回去，只能手改 `works.json`。

在作者的机器上复现了。1280×820 的窗口，`MYGO_GPU=0`，`MYGO_FRAME_STATS=all` 显示每一帧都画了、也交了（「drawn in memory」），屏幕上和 `PrintWindow` 截到的都是白的。只画一块蓝底的最小 mygo 窗口有时正常；把 Inkgo 的界面换成同样的蓝底，照样白。所以不是界面的内容，也不是窗口的选项（标题栏、记住位置、最小尺寸），一个一个去掉试过，好坏只是碰巧。

## Decision

- **这一项先不给**：`desk/settings.go` 的 `offerNoGPU` 是 `false`，「外观」里不显示这一项，`NoGPU()` 也总是 `false`，`cmd/inkgo` 不再因为书架里存着 `noGPU` 而设 `MYGO_GPU=0`。已经勾上的机器下次打开就回到用 GPU 画，不用手改文件。
- **书架照旧认得 `noGPU`**：`shelf` 用 strictjson 读，去掉这个字段，已经存了 `"noGPU": true` 的 `works.json` 就打不开了。字段留着，只是不读它。
- 代码、测试、原型都留着，只关着：`TestNoGPUWaitsForTheNextStart` 临时打开 `offerNoGPU` 照常跑；原型 `prototype/dialogs.js` 的 `OFFER_NO_GPU` 是 `false`，`walk.ts` 里那几步跟着它，关着时只查这一项没有出现。新加 `TestNoGPUNotOffered`：书架里存着勾上，设置里也不显示，也不按它画。
- **什么时候放出来**：mygo 在 Windows 上用一整块系统内存放在内存里画的帧（或者交给 GDI 之前拷进 GDI 自己的位图）、升级到那个版本以后，把 `offerNoGPU` 和 `OFFER_NO_GPU` 改回 `true`，在真窗口里开关几次看画面。

## 原因

mygo 不用 GPU 时，在内存里画好一帧（`internal/raster` 的 `Renderer.Image`），再用 GDI 的 `SetDIBitsToDevice` 贴到窗口上（`internal/windows/surface.go` 的 `PresentPixels`），不看返回值。v0.5.5 和 v0.6.3 这两处一样。

给 mygo 的一份副本加了日志：

- `SetDIBitsToDevice` 返回 0（一行也没贴上），`GetLastError` 没有错误；进程的 GDI 对象 50 个，不是用完了。
- 同一块内存只贴前 16 行能贴上；分成每段 64 行贴，812 行里固定有一段贴不上，那一段每次都横跨一个 4 MB 整数倍的地址（`0x108f6bbe2000..0x108f6bc31000` 跨过 `0x108f6bc00000`）。
- Go 1.27 在 64 位 Windows 上按 4 MB 一块向系统要堆（`runtime/malloc.go` 的 `heapArenaBytes`），每块是单独一次 `VirtualAlloc`。Windows 上的 `allocPixels` 用 `make` 从 Go 的堆里要这一帧（Mac、Linux 用 `mmap` 单独要）。GDI 碰到横跨两次 `VirtualAlloc` 的内存就不贴。
- 1264×812 的一帧 4,105,472 字节，比 4 MB 只少 88,832 字节，Go 按 8 KB 一页放大对象，落在一块之内的机会大约 2%，所以几乎每次都白。窗口越小越不容易跨，那个 900×600 的最小窗口一半上下能显示，作者头一回用 `MYGO_GPU=0` 觉得顺手，也是碰巧。头一帧贴上了、后面的没贴上时，画面停在头一帧。
- 把副本里 Windows 的 `allocPixels` 改成一次 `VirtualAlloc`，连开 4 次，30 次交帧没有一次失败，画面正常。

和这台机器无关：任何 Windows 电脑在 mygo 不用 GPU 画的时候都会这样，包括 mygo 建不起 Direct3D、退回去在内存里画的电脑。

## Alternatives considered

- **整个删掉，以后要再加** — 最强的理由：不留关着的代码，原型、走查、测试里没有一块平时不跑的分支。不用：作者选了先藏起来；上游修好以后只要改两个开关，设置、立即重新打开、原型和走查都是现成的，而且关着的时候测试照样跑着它。
- **仓库里带一份补过的 mygo，`go.mod` 用 `replace` 指过去** — 最强的理由：这一项马上就能用，配置差的电脑现在就能关掉 GPU。不用：多带 20 MB 的第三方代码，以后每次升 mygo 都要重新打补丁；而且升到 v0.6.3 以后，用 GPU 时一帧在主线程上只要 2–3 ms，原来卡在交帧上的 28 ms 没有了（见[依赖升级那篇](../process/2026-10-13-mygo-0.4.md)），关掉 GPU 不再那么必要。

## Consequences

- **收益**：勾上这一项的作者不会再看到白窗口，存着 `noGPU` 的机器打开就是正常的。
- **代价与已知上限**：
  - 配置差的电脑暂时没有办法在设置里关掉 GPU；在终端里设 `MYGO_GPU=0` 的，照样会碰上白窗口。
  - 关着的代码要靠 `TestNoGPUWaitsForTheNextStart` 和原型的开关守着；改设置页、改 `walk.ts` 的时候要记得它们在。
  - 原因是在 mygo 副本里加日志查出来的，没有改 mygo 本身。作者看过以后报给了 mygo，附了上面那个改法：[egoist/mygo#690](https://github.com/egoist/mygo/issues/690)。
  - mygo v0.6.4 照这个改法修了（`internal/raster/pixels_windows.go` 一帧单独一次 `VirtualAlloc`），v0.7.0 又让不用 GPU 的窗口按屏幕刷新的快慢画、不再占满一个核。2026-10-27 升到 v0.7.1（见[依赖升级那篇](../process/2026-10-13-mygo-0.4.md)），`MYGO_GPU` 还是画第一帧时看一次。同一天作者在设置里找这一项，照上面「什么时候放出来」把 `offerNoGPU` 和 `OFFER_NO_GPU` 改回了 `true`；`TestNoGPUNotOffered` 改成自己关掉开关再查，开关还能用。作者在真窗口里看过：「确认可行了」。哪台机器勾上以后要是又白了，删掉 `%APPDATA%\writer\works.json` 里的 `"noGPU": true` 那一行就回到用 GPU 画。

## Verification

- `go test ./desk -run 'TestNoGPU|TestSettingsSayNothing'`。
- `SHOT=<目录> go test ./desk -run Shot` 的 `settings-look-dark.png`：「外观」只有颜色和标出设定的名字。
- `npm run verify-prototype`：关着时「外观」里没有这一项。
- 作者机器上的 `works.json` 删掉了 `"noGPU": true` 一行（作者同意的，删之前留了备份）。
