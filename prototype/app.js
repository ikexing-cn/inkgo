// 整个窗口怎么拼起来，和不管光标在哪都管用的那几个键。
'use strict'

S = fresh()

const WINDOW_BUTTONS = '<div class="window-buttons">' + ['<path d="M0 5h10"></path>', '<rect x=".5" y=".5" width="9" height="9" rx="1"></rect>', '<path d="M.5.5l9 9M9.5.5l-9 9"></path>']
  .map((d) => `<div><svg class="icon-xs" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">${d}</svg></div>`).join('') + '</div>'
// 窗口的按钮是系统画的：Windows 在最右，是上面那三个；macOS 在最左，是红黄绿三个点，标题栏从它们右边排起。
const LIGHTS = '<div class="lights"><i></i><i></i><i></i></div>'
const chromeLeft = () => (OS === 'mac' ? LIGHTS : ''), chromeRight = () => (OS === 'mac' ? '' : WINDOW_BUTTONS)

// 菜单是命令的总表：哪一页用不上的那几行是灰的；AI 关着时，AI 的那几行不显示。开着一个的时候，指针移到旁边的名字上就换过去。
// Windows 上菜单在标题栏里，软件自己画；macOS 上在屏幕顶上的系统菜单栏里（作者 2026-11-02 定），多出系统的「Inkgo」「窗口」两个：
// 设置挪进「Inkgo」，外观是打勾的三行，系统的菜单里放不下分段控件。原型里两处用同一种样子画，真的系统菜单是系统画的。
function menus() {
  const writing = S.page === 'write', cur = writing && chapterOf(S.ch), mac = OS === 'mac'
  const inBook = S.page !== 'shelf', [hasPane, paneOn, paneTo] = paneHere()
  const outline = cur && (S.outlines[cur.key] || '').trim(), steps = (cur && !S.ai && S.undo[cur.key]) || { back: [], fwd: [] }
  const then = (act) => () => { set(SHUT); act() }
  const ask = (dialog) => () => set({ dialog, ...SHUT })
  const settings = ask({ kind: 'settings', tab: 'AI', edit: '' })
  const looks = LOOKS(), shut = () => set(SHUT)
  return [
    ...(mac ? [['Inkgo', () => menuRow(L.appAbout(), shut) + HAIR + menuRow(L.appSettings(), settings, '', 'Ctrl+,') + HAIR +
      menuRow(L.appHide(), shut, '', 'Ctrl+H') + menuRow(L.appHideOthers(), shut, '', 'Ctrl+Alt+H') + HAIR + menuRow(L.appQuit(), shut, '', 'Ctrl+Q')]] : []),
    [L.menuFile(), () => menuRow(L.cmdNewChapter(), writing ? then(() => newChapter()) : null, '', 'Ctrl+N') + menuRow(L.cmdNewFolder(), writing ? then(newFolder) : null) + HAIR +
      menuRow(L.cmdSave(), cur ? then(saveNow) : null, '', 'Ctrl+S') + drawPoint('FileActions', cur, inBook, writing, then, ask, outline) + HAIR +
      drawPoint('BookActions', cur, inBook, writing, then, ask, outline) + (mac ? '' : HAIR + menuRow(L.appSettings(), settings))],
    [L.menuEdit(), () => menuRow(L.cmdUndo(), steps.back.length ? then(() => unwrite(-1)) : null, '', 'Ctrl+Z') + menuRow(L.cmdRedo(), steps.fwd.length ? then(() => unwrite(1)) : null, '', 'Ctrl+Shift+Z') + HAIR +
      (drawPoint('EditActions', cur, inBook, writing, then, ask, outline)) +
      menuRow(L.cmdRename(), cur && !S.rename ? then(renameOpen) : null, '', 'F2') + (contributions('WriteActions').length ? HAIR : '') +
      drawPoint('WriteActions', cur, inBook, writing, then, ask, outline)],
    [L.menuView(), () => menuRow(S.sideOpen ? L.cmdHideSidebar() : L.cmdShowSidebar(), inBook ? () => moving(() => set({ sideOpen: !S.sideOpen, ...SHUT }), 'fold') : null) +
      menuRow(paneOn ? L.cmdHidePane() : L.cmdShowPane(), hasPane ? () => moving(() => set({ ...paneTo, ...SHUT }), 'fold') : null) +
      drawPoint('ViewActions', cur, inBook, writing, then, ask, outline) + HAIR +
      `<div class="pop-label">${t(L.viewZoom(Math.round(S.zoom * 100)))}</div>` +
      menuRow(L.cmdZoomIn(), zoomNext(1) ? () => zoomBy(1) : null, '', 'Ctrl+=') + menuRow(L.cmdZoomOut(), zoomNext(-1) ? () => zoomBy(-1) : null, '', 'Ctrl+-') +
      menuRow(L.cmdZoomReset(), S.zoom !== 1 ? () => zoomBy(0) : null, '', 'Ctrl+0') + HAIR + `<div class="pop-label">${t(L.viewLook())}</div>` +
      (mac ? looks.map(([v, , shown]) => menuRow(shown, () => set({ look: v, ...SHUT }), S.look === v ? 'tick' : 'blank')).join('')
        : `<div class="pop-seg">${seg(S.look, looks, (v) => set({ look: v }), L.viewLook())}</div>`)],
    ...(mac ? [[L.menuWindow(), () => menuRow(L.winMinimize(), shut, '', 'Ctrl+M') + menuRow(L.winZoom(), shut) + HAIR + menuRow(L.winBringAll(), shut)]] : []),
    [L.menuHelp(), () => menuRow(L.cmdShortcuts(), ask({ kind: 'keys' }), '', 'Ctrl+/')],
  ]
}

// 外观的三种：状态里记的值、图标、显示的字。菜单和设置里的外观都用它。
const LOOKS = () => [['跟随系统', 'screen', L.lookSystem()], ['浅色', 'sun', L.lookLight()], ['深色', 'moon', L.lookDark()]]

// 一排菜单的名字，点开一个在它下面。
function menuTitles(list) {
  const inMenus = S.open.startsWith('menu-')
  return list.map(([name, rows]) => {
    const id = 'menu-' + name, on = S.open === id
    return `<div class="anchor left"><div class="menu-title hit${on ? ' on' : ''}"${onClick(() => set({ open: on ? '' : id }))}${onOver(() => { if (inMenus && !on) set({ open: id }) })}>${name}</div>` +
      (on ? `<div class="pop menu-pop" data-k="${id}">${rows()}</div>` : '') + '</div>'
  }).join('')
}

// 标题栏：自己占一行。最左是侧栏的开关，接着是菜单（macOS 上菜单在系统菜单栏，这里没有）；写作页右边是待处理、查找、专注这几样，
// 再右是右栏的开关，最右是窗口的三个按钮。
// 专注写作时这一行只剩正中的「章名 · 字数」、右边那几样和窗口的按钮。
function titlebar() {
  if (S.focus && S.page === 'write') {
    const cur = chapterOf(S.ch)
    return `<div class="titlebar bare" data-k="titlebar">${chromeLeft()}<div class="bar-title ell">${cur ? t(titleOf(cur)) : ''}</div><div class="grow"></div>${tools(cur)}${paneSwitch()}${chromeRight()}</div>`
  }
  const writing = S.page === 'write', cur = writing && chapterOf(S.ch), inBook = S.page !== 'shelf'
  const sideWord = S.sideOpen ? L.cmdHideSidebar() : L.cmdShowSidebar()
  return '<div class="titlebar" data-k="titlebar">' + chromeLeft() +
    `<button class="icon-btn muted${inBook ? '' : ' off'}" aria-label="${sideWord}" title="${sideWord}"${onClick(inBook ? () => moving(() => set({ sideOpen: !S.sideOpen, ...SHUT }), 'fold') : null)}>${svg('icon-md', PANEL + path('M9 5v14'))}</button>` +
    (OS === 'mac' ? '' : menuTitles(menus())) + (inBook ? pageSwitch() : '') + '<div class="grow"></div>' + (writing ? tools(cur) : '') + paneSwitch() + chromeRight() + '</div>'
}

// 去哪一页。讨论页回到上次看的那一段，没看过就是最新的一段。
const toPage = (page) => () => set({ page, ...(page === 'talk' && S.tv === 'all' ? { tid: S.talks[0] ? S.talks[0].id : null, tv: S.talks[0] ? 'one' : 'new' } : {}), ...SHUT })

// 标题栏正中换页：讨论、正文、大纲、检查，正在的那一页是墨色。讨论、大纲、检查各归一个扩展，关着的那一页不在（ext.js 的 Pages）。
// 作品分类不常去，在「文件」菜单里。
function pageSwitch() {
  const pages = [...contributions('Pages').filter((x) => x.label).map((x) => [x.label(), x.id, x.order]), [L.pageWrite(), 'write', 1]].sort((a, b) => a[2] - b[2])
  return `<div class="pages">${pages.map(([name, page]) => `<div class="hit${S.page === page ? ' on' : ''}"${onClick(toPage(page))}>${name}${page === 'check' && S.checks.length ? `<span class="badge">${S.checks.length}</span>` : ''}</div>`).join('')}</div>`
}

// 这一页有没有右栏、开着没有、按一下怎么变。大纲页的右栏单记着开没开。
function paneHere() {
  const plan = S.page === 'plan', has = (S.page === 'write' && !S.focus && paneTabs().length) || ((S.page === 'talk' || plan) && contributions('BesideItems').length), on = has && (plan ? S.planPane : S.paneOpen)
  return [has, on, plan ? { planPane: !on } : { paneOpen: !on }]
}

// 右栏的开关：和最左的侧栏开关对着，收起、展开是同一个按钮，一直在这里不挪地方。
// 这一页没有右栏时它灰着；专注写作时只留出它的位置，「退出专注」才和「专注写作」在同一处。
function paneSwitch() {
  if (S.focus && S.page === 'write') return '<div class="pane-switch"></div>'
  const [has, on, to] = paneHere(), word = on ? L.cmdHidePane() : L.cmdShowPane()
  return `<button class="icon-btn muted pane-switch${has ? '' : ' off'}" aria-label="${word}" title="${word}"${onClick(has ? () => moving(() => set({ ...to, ...SHUT }), 'fold') : null)}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`
}

// 重新开始：点过、改过的都回到刚打开的样子，只留外观和缩放。
function restart() {
  stopAll()
  set({ ...fresh(), look: S.look, zoom: S.zoom })
}

// 换平台：像换了一台电脑，也从头开始。作品库、备份文件夹这些路径在打开时就写进了状态，不从头会两种写法混着。
function switchOS(os) {
  if (os === OS) return
  OS = os
  restart()
}

// macOS 的菜单栏在屏幕最上面，不在窗口里，也不跟着窗口缩放。
const macBar = (w) => `<div class="mac-bar" data-k="mac-bar" style="width:${w}px">${menuTitles(menus())}</div>`

// 右下角的提示，最多露三条，新的在下；没露的是旧的，「还有几条」写在它们上面。提示里能点的几个靠右排，最要紧的（给的第一个）在最右。
// 讨论在回复、那段讨论又不在眼前（别的讨论、讨论列表、别的页、右栏收起或在别的标签、专注写作）时，
// 右下角说它在想，能取消；回到那段讨论就收起，那里有它自己的进度。
function toasts() {
  const w = S.waiting, here = w && aiOn() && S.tv === 'one' && S.tid === w.id &&
    (S.page === 'talk' || (S.page === 'write' && !S.focus && S.paneOpen && S.tab === '讨论'))
  const list = w && !here ? [...S.toasts, { id: 'busy-talk', kind: 'busy', title: L.toastThinking(), acts: [[L.toastCancel(), stopReply]] }] : S.toasts
  if (!list.length) return ''
  const shown = list.slice(-3), rest = list.length - shown.length
  const pic = { ok: icon('checked', 'md', 'green'), fail: icon('warn', 'md', 'danger'), busy: icon('spin', 'md', 'muted') }
  const one = (x) => `<div class="toast" data-k="${x.id}">${pic[x.kind] || ''}<div class="fit"><div class="toast-title">${t(x.title)}</div>${x.detail ? `<div class="note">${t(x.detail)}</div>` : ''}` +
    (x.acts.length ? `<div class="toast-acts" data-keep>${[...x.acts].reverse().map(([label, act]) => `<div class="hit link"${onClick(act)}>${t(label)}</div>`).join('')}</div>` : '') +
    // 正在做的不能关：它做完自己收起，不要了点「取消」。
    `</div>${x.kind === 'busy' ? '' : `<button class="icon-btn muted" aria-label="${L.toastClose()}" title="${L.toastClose()}"${onClick(() => unsay(x.id))}>${icon('cross', 'sm')}</button>`}</div>`
  return `<div id="toasts">${rest > 0 ? `<div class="toast-rest">${t(L.toastMore(rest))}</div>` : ''}${shown.map(one).join('')}</div>`
}

// 标题栏和它下面那一块各有名字（data-k）：前面多出一层遮罩（开了菜单、对话框）时它们还是原来那两个，
// 正文不重建，光标留在原处。
function view() {
  const focus = S.focus && S.page === 'write'
  // 换页入口看开关；已经在那一页、或从舞台摆出的少见情况，仍用登记过的画法。
  const main = { write: () => editor(chapterOf(S.ch)), ...Object.fromEntries(registered('Pages').map(({ value: x }) => [x.id, x.draw])) }
  // 讨论页的右栏是设定放进来的：设定关着时没有右栏，和开关灰着一致。
  const side = S.page === 'write' && !focus ? S.paneOpen : S.page === 'talk' ? S.paneOpen && !!paneHere()[0] : S.page === 'plan' ? S.planPane : false
  const bare = focus || !S.sideOpen
  // 缩放：窗口里按 1/z 的大小排，再整个放大 z 倍；排出来窄过 1180 就是窄窗口。
  const [w, h] = winSize(), z = S.zoom, narrow = w / z < 1180
  return '<div class="stage">' +
    `<div class="stage-bar">${button('重新开始', restart)}<div class="anchor left"><div class="btn hit${S.open === 'rare' ? ' soft' : ''}" title="平时不易遇到的情况：出错、作品在软件外被改动"${onClick(() => set({ open: S.open === 'rare' ? '' : 'rare' }))}>少见的情况</div>` +
    (S.open === 'rare' ? `<div class="pop rare-pop" data-k="rare-pop">${rare().map(([label, act]) => menuRow(label, act)).join('')}</div>` : '') + '</div>' +
    seg(OS === 'mac' ? 'macOS' : 'Windows', ['Windows', 'macOS'], (v) => switchOS(v === 'macOS' ? 'mac' : 'win'), '平台') +
    seg(LANG, [['zh-Hans', '', '中文'], ['en', '', 'English'], ['en-XA', '', '伪英文']], switchLang, '语言') +
    `<div class="stage-note">内容为示例。正文可输入，刷新后复原；AI 仅回复固定内容。</div></div>` + (OS === 'mac' ? macBar(w) : '') +
    `<div id="win" lang="${LANG}" class="${dark() ? 'dark' : 'light'}${narrow ? ' narrow' : ''}${OS === 'mac' ? ' mac' : ''}" style="zoom:${z};width:${w / z}px;height:${h / z}px">` + veil() + titlebar() +
    '<div class="win-body" data-k="win-body">' + (S.page === 'shelf' ? shelfPage() : (bare ? '' : sidebar()) + main[S.page]() + (side ? pane() : '')) + '</div>' +
    floats() + dialogs() + toasts() + '</div></div>'
}

// 按 Esc：从最上面的一层收起。
function backOut() {
  if (S.imp && S.imp.menu) return set((s) => ({ imp: { ...s.imp, menu: null } })) // 预览里右键点出来的菜单
  if (S.dialog && S.dialog.ask) return set((s) => ({ dialog: { ...s.dialog, ask: '' } })) // 浮在设置上面的那一问
  if (S.dialog) return shut()
  if (S.open) return set(SHUT)
  if (S.ctx || S.bury || S.pay || S.tree) return closeFloat()
  if (S.page === 'plan' && (S.planOutline || S.planLine)) return set({ planOutline: null, planLine: null })
  if (S.rename) return set({ rename: null })
  if (S.ai) return set({ ai: null })
  if (S.find) return set({ find: null })
  if (S.bar) { unselect(); return set({ bar: null }) }
}

document.addEventListener('keydown', (ev) => {
  const writing = S.page === 'write' && !S.dialog, ctrl = ev.ctrlKey || ev.metaKey
  const run = (act) => { ev.preventDefault(); act() }
  if (ev.key === 'Escape') backOut()
  else if (ev.key === 'Tab' && S.ai && S.ai.kind === 'ghost') run(() => ghost(true))
  // macOS 的 ⌃⌘F；F11、F2 在 macOS 的样子下也认，原型多在 Windows 键盘上点。
  else if (contributions('FocusKeys').some((act) => act(ev, writing, run))) return
  else if (ctrl && ev.key === '/') run(() => set({ dialog: { kind: 'keys' }, ...SHUT }))
  else if (ev.key === 'F2' && writing && chapterOf(S.ch) && !S.rename) run(renameOpen)
  else if (ctrl && (ev.key === '=' || ev.key === '+')) run(() => zoomBy(1))
  else if (ctrl && ev.key === '-') run(() => zoomBy(-1))
  else if (ctrl && ev.key === '0') run(() => zoomBy(0))
  else if (!ctrl || !writing || !chapterOf(S.ch)) return
  else if (ev.key === 's') run(saveNow)
  else if (ev.key === 'f') run(() => contributions('WritingKeys').filter((x) => x.key === 'f').forEach((x) => x.act()))
  else if (ev.key === 'j') run(() => contributions('WritingKeys').filter((x) => x.key === 'j').forEach((x) => x.act()))
  else if (ev.key === 'l') run(() => { if (aiOn()) cite() })
})
// 标签排不下时，滚轮在标签条上左右挪。
document.addEventListener('wheel', (ev) => { const strip = ev.target.closest && ev.target.closest('.sheets'); if (strip) strip.scrollLeft += ev.deltaY }, { passive: true })
// 章节树里拖一行：拖到谁上面，谁就亮一下；松手时照 dropOn 挪过去。
let dragged = ''
const nodeAt = (ev) => { const row = ev.target.closest('[data-node]'); return row && dragged && row.dataset.node !== dragged ? row : null }
root.addEventListener('dragstart', (ev) => { const row = ev.target.closest('[data-node]'); if (row) { dragged = row.dataset.node; ev.dataTransfer.effectAllowed = 'move' } })
root.addEventListener('dragover', (ev) => { const row = nodeAt(ev); if (row) { ev.preventDefault(); row.classList.add('over') } })
root.addEventListener('dragleave', (ev) => { const row = ev.target.closest('[data-node]'); if (row) row.classList.remove('over') })
root.addEventListener('drop', (ev) => { const row = nodeAt(ev); if (row) { ev.preventDefault(); dropOn(dragged, row.dataset.node) } })
root.addEventListener('dragend', () => { dragged = ''; draw() })

// 按标题栏和打开对话框的按钮（data-keep）不拿走正文的光标：开了菜单又收起、点了侧栏的开关、对话框关掉，接着打的字还进正文。
root.addEventListener('mousedown', (ev) => { if (ev.target.closest('.titlebar, .mac-bar, [data-keep]')) ev.preventDefault() })

// 拖右栏的左边改它的宽度：不窄于 300，不宽过半个窗口。
root.addEventListener('mousedown', (ev) => {
  if (!ev.target.closest('.pane-edge')) return
  ev.preventDefault()
  const win = document.getElementById('win'), right = win.getBoundingClientRect().right
  const z = winZoom(), held = (right - ev.clientX) / z - document.getElementById('pane').offsetWidth // 按下的地方离右栏的边有多远
  const move = (e) => set({ paneW: Math.round(Math.min(Math.max((right - e.clientX) / z - held, 300), win.offsetWidth / 2)) })
  const up = () => { document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up) }
  document.addEventListener('mousemove', move)
  document.addEventListener('mouseup', up)
})

// 「跟随系统」时，系统换了深浅就跟着换。
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', draw)

draw()
