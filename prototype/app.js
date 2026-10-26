// 整个窗口怎么拼起来，和不管光标在哪都管用的那几个键。
'use strict'

const WINDOW_BUTTONS = '<div class="window-buttons">' + ['<path d="M0 5h10"></path>', '<rect x=".5" y=".5" width="9" height="9" rx="1"></rect>', '<path d="M.5.5l9 9M9.5.5l-9 9"></path>']
  .map((d) => `<div><svg class="icon-xs" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">${d}</svg></div>`).join('') + '</div>'

// 标题栏：自己占一行。最左是侧栏的开关，接着是菜单；写作页右边是待处理、查找、专注这几样，再右是右栏的开关，最右是窗口的三个按钮。
// 菜单是命令的总表：哪一页用不上的那几行是灰的；AI 关着时，AI 的那几行不显示。开着一个的时候，指针移到旁边的名字上就换过去。
// 专注写作时这一行只剩正中的「章名 · 字数」、右边那几样和窗口的按钮。
function titlebar() {
  if (S.focus && S.page === 'write') {
    const cur = chapterOf(S.ch)
    return `<div class="titlebar bare" data-k="titlebar"><div class="bar-title ell">${cur ? t(titleOf(cur)) : ''}</div><div class="grow"></div>${tools(cur)}${paneSwitch()}${WINDOW_BUTTONS}</div>`
  }
  const writing = S.page === 'write', cur = writing && chapterOf(S.ch)
  const inBook = S.page !== 'shelf', [hasPane, paneOn, paneTo] = paneHere()
  const outline = cur && (S.outlines[cur.key] || '').trim(), steps = (cur && !S.ai && S.undo[cur.key]) || { back: [], fwd: [] }
  const then = (act) => () => { set(SHUT); act() }
  const ask = (dialog) => () => set({ dialog, ...SHUT })
  const menus = [
    ['文件', () => menuRow('新建章节', writing ? then(() => newChapter()) : null, '', 'Ctrl+N') + menuRow('新建文件夹', writing ? then(newFolder) : null) + HAIR +
      menuRow('保存', cur ? then(saveNow) : null, '', 'Ctrl+S') + menuRow('快照', cur ? ask({ kind: 'snaps', id: '', diff: false }) : null) +
      menuRow('导出…', inBook ? ask({ kind: 'export', range: '全书', indent: true }) : null) + HAIR +
      menuRow('作品分类', inBook ? toPage('cats') : null) + menuRow('回收站…', inBook ? ask({ kind: 'bin', sure: '' }) : null) + HAIR +
      menuRow('设置…', ask({ kind: 'settings', tab: 'AI', edit: '' }))],
    ['编辑', () => menuRow('撤销', steps.back.length ? then(() => unwrite(-1)) : null, '', 'Ctrl+Z') + menuRow('重做', steps.fwd.length ? then(() => unwrite(1)) : null, '', 'Ctrl+Y') + HAIR +
      menuRow('查找替换', cur ? openFind : null, '', 'Ctrl+F') +
      menuRow('重命名', cur && !S.rename ? then(renameOpen) : null, '', 'F2') + (aiOn() ? HAIR +
      menuRow('续写一句', cur ? then(continueText) : null, '', 'Ctrl+J') +
      menuRow('起草本章', outline ? ask({ kind: 'draft', goal: '', how: '接在已有内容之后' }) : null, '', cur && !outline ? '请先填写章纲' : '') : '')],
    ['视图', () => menuRow(S.sideOpen ? '收起侧栏' : '展开侧栏', inBook ? () => moving(() => set({ sideOpen: !S.sideOpen, ...SHUT }), 'fold') : null) +
      menuRow(paneOn ? '收起右栏' : '展开右栏', hasPane ? () => moving(() => set({ ...paneTo, ...SHUT }), 'fold') : null) +
      menuRow('专注写作', writing ? () => set({ focus: true, ...SHUT }) : null, '', 'F11') +
      menuRow('手机预览', writing ? () => set({ paneOpen: true, tab: '预览', ...SHUT }) : null) + HAIR +
      `<div class="pop-label">外观</div><div class="pop-seg">${seg(S.look, [['跟随系统', 'screen'], ['浅色', 'sun'], ['深色', 'moon']], (v) => set({ look: v }), '外观')}</div>`],
    ['帮助', () => menuRow('快捷键…', ask({ kind: 'keys' }), '', 'Ctrl+/')],
  ]
  const inMenus = S.open.startsWith('menu-')
  const menu = ([name, rows]) => {
    const id = 'menu-' + name, on = S.open === id
    return `<div class="anchor left"><div class="menu-title hit${on ? ' on' : ''}"${onClick(() => set({ open: on ? '' : id }))}${onOver(() => { if (inMenus && !on) set({ open: id }) })}>${name}</div>` +
      (on ? `<div class="pop menu-pop" data-k="${id}">${rows()}</div>` : '') + '</div>'
  }
  const sideWord = S.sideOpen ? '收起侧栏' : '展开侧栏'
  return '<div class="titlebar" data-k="titlebar">' +
    `<button class="icon-btn muted${inBook ? '' : ' off'}" aria-label="${sideWord}" title="${sideWord}"${onClick(inBook ? () => moving(() => set({ sideOpen: !S.sideOpen, ...SHUT }), 'fold') : null)}>${svg('icon-md', PANEL + path('M9 5v14'))}</button>` +
    menus.map(menu).join('') + (inBook ? pageSwitch() : '') + '<div class="grow"></div>' + (writing ? tools(cur) : '') + paneSwitch() + WINDOW_BUTTONS + '</div>'
}

// 去哪一页。讨论页回到上次看的那一段，没看过就是最新的一段。
const toPage = (page) => () => set({ page, ...(page === 'talk' && S.tv === 'all' ? { tid: S.talks[0] ? S.talks[0].id : null, tv: S.talks[0] ? 'one' : 'new' } : {}), ...SHUT })

// 标题栏正中换页：讨论、正文、大纲、检查，正在的那一页是墨色。讨论和检查要用 AI，AI 关着时只有正文、大纲。
// 作品分类不常去，在「文件」菜单里。
function pageSwitch() {
  const pages = [['讨论', 'talk'], ['正文', 'write'], ['大纲', 'plan'], ['检查', 'check']].filter(([, page]) => aiOn() || (page !== 'talk' && page !== 'check'))
  return `<div class="pages">${pages.map(([name, page]) => `<div class="hit${S.page === page ? ' on' : ''}"${onClick(toPage(page))}>${name}${page === 'check' && S.checks.length ? `<span class="badge">${S.checks.length}</span>` : ''}</div>`).join('')}</div>`
}

// 这一页有没有右栏、开着没有、按一下怎么变。大纲页的右栏单记着开没开。
function paneHere() {
  const plan = S.page === 'plan', has = (S.page === 'write' && !S.focus) || S.page === 'talk' || plan, on = has && (plan ? S.planPane : S.paneOpen)
  return [has, on, plan ? { planPane: !on } : { paneOpen: !on }]
}

// 右栏的开关：和最左的侧栏开关对着，收起、展开是同一个按钮，一直在这里不挪地方。
// 这一页没有右栏时它灰着；专注写作时只留出它的位置，「退出专注」才和「专注写作」在同一处。
function paneSwitch() {
  if (S.focus && S.page === 'write') return '<div class="pane-switch"></div>'
  const [has, on, to] = paneHere(), word = on ? '收起右栏' : '展开右栏'
  return `<button class="icon-btn muted pane-switch${has ? '' : ' off'}" aria-label="${word}" title="${word}"${onClick(has ? () => moving(() => set({ ...to, ...SHUT }), 'fold') : null)}>${svg('icon-md', PANEL + path('M15 5v14'))}</button>`
}

// 重新开始：点过、改过的都回到刚打开的样子，只留外观。
function restart() {
  stopAll()
  set({ ...fresh(), look: S.look })
}

// 右下角的提示，最多露三条，新的在下；没露的是旧的，「还有几条」写在它们上面。提示里能点的几个靠右排，最要紧的（给的第一个）在最右。
// 讨论在回复、那段讨论又不在眼前（别的讨论、讨论列表、别的页、右栏收起或在别的标签、专注写作）时，
// 右下角说它在想，能取消；回到那段讨论就收起，那里有它自己的进度。
function toasts() {
  const w = S.waiting, here = w && aiOn() && S.tv === 'one' && S.tid === w.id &&
    (S.page === 'talk' || (S.page === 'write' && !S.focus && S.paneOpen && S.tab === '讨论'))
  const list = w && !here ? [...S.toasts, { id: 'busy-talk', kind: 'busy', title: 'AI 正在思考…', acts: [['取消', stopReply]] }] : S.toasts
  if (!list.length) return ''
  const shown = list.slice(-3), rest = list.length - shown.length
  const pic = { ok: icon('checked', 'md', 'green'), fail: icon('warn', 'md', 'danger'), busy: icon('spin', 'md', 'muted') }
  const one = (x) => `<div class="toast" data-k="${x.id}">${pic[x.kind] || ''}<div class="fit"><div class="toast-title">${t(x.title)}</div>${x.detail ? `<div class="note">${t(x.detail)}</div>` : ''}` +
    (x.acts.length ? `<div class="toast-acts" data-keep>${[...x.acts].reverse().map(([label, act]) => `<div class="hit link"${onClick(act)}>${t(label)}</div>`).join('')}</div>` : '') +
    // 正在做的不能关：它做完自己收起，不要了点「取消」。
    `</div>${x.kind === 'busy' ? '' : `<button class="icon-btn muted" aria-label="关闭提示" title="关闭提示"${onClick(() => unsay(x.id))}>${icon('cross', 'sm')}</button>`}</div>`
  return `<div id="toasts">${rest > 0 ? `<div class="toast-rest">还有 ${rest} 条</div>` : ''}${shown.map(one).join('')}</div>`
}

// 标题栏和它下面那一块各有名字（data-k）：前面多出一层遮罩（开了菜单、对话框）时它们还是原来那两个，
// 正文不重建，光标留在原处。
function view() {
  const focus = S.focus && S.page === 'write'
  const main = { write: () => editor(chapterOf(S.ch)), plan: planPage, talk: talkPage, check: checkPage, cats: catsPage }
  const side = (S.page === 'write' && !focus) || S.page === 'talk' ? S.paneOpen : S.page === 'plan' ? S.planPane : false
  const bare = focus || !S.sideOpen
  return '<div class="stage">' +
    `<div class="stage-bar">${button('重新开始', restart)}<div class="anchor left"><div class="btn hit${S.open === 'rare' ? ' soft' : ''}" title="平时不易遇到的情况：出错、作品在软件外被改动"${onClick(() => set({ open: S.open === 'rare' ? '' : 'rare' }))}>少见的情况</div>` +
    (S.open === 'rare' ? `<div class="pop rare-pop" data-k="rare-pop">${rare().map(([label, act]) => menuRow(label, act)).join('')}</div>` : '') + `</div><div class="stage-note">内容为示例。正文可输入，刷新后复原；AI 仅回复固定内容。</div></div>` +
    `<div id="win" class="${dark() ? 'dark' : 'light'}${S.narrow ? ' narrow' : ''}">` + veil() + titlebar() +
    '<div class="win-body" data-k="win-body">' + (S.page === 'shelf' ? shelfPage() : (bare ? '' : sidebar()) + main[S.page]() + (side ? pane() : '')) + '</div>' +
    floats() + dialogs() + toasts() + '</div></div>'
}

// 按 Esc：从最上面的一层收起。
function backOut() {
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
  else if (ev.key === 'F11' && writing) run(() => set({ focus: !S.focus, ...SHUT }))
  else if (ctrl && ev.key === '/') run(() => set({ dialog: { kind: 'keys' }, ...SHUT }))
  else if (ev.key === 'F2' && writing && chapterOf(S.ch) && !S.rename) run(renameOpen)
  else if (!ctrl || !writing || !chapterOf(S.ch)) return
  else if (ev.key === 's') run(saveNow)
  else if (ev.key === 'f') run(openFind)
  else if (ev.key === 'j') run(() => { if (aiOn()) continueText() }) // AI 关着时按了不起作用
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
// 软件每两秒重读一次改坏的大纲文件；原型里切回窗口，就当作已经改好了。
window.addEventListener('focus', () => { if (S.planBroken) set({ planBroken: false }) })
// 大纲页里拖一张卡片：按章节拖进一格，就换到那一章、那条情节线；时间线上拖进一格，就换到那个时间、那条情节线。
let draggedCard = ''
const cardAt = (ev) => { const at = ev.target.closest('[data-slot], [data-cell]'); return at && draggedCard ? at : null }
root.addEventListener('dragstart', (ev) => { const card = ev.target.closest('[data-card]'); if (card) { draggedCard = card.dataset.card; ev.dataTransfer.effectAllowed = 'move' } })
root.addEventListener('dragover', (ev) => { const at = cardAt(ev); if (at) { ev.preventDefault(); at.classList.add('over') } })
root.addEventListener('dragleave', (ev) => { const at = ev.target.closest('[data-slot], [data-cell]'); if (at) at.classList.remove('over') })
root.addEventListener('drop', (ev) => {
  const at = cardAt(ev)
  if (!at) return
  ev.preventDefault()
  if (at.dataset.slot) { const [col, line] = at.dataset.slot.split('|'); return dropAt(draggedCard, col, line) }
  const [key, line] = at.dataset.cell.split('|')
  placeCard(draggedCard, key, key ? line : null) // 拖进「未放进章节」的，情节线不变
})
root.addEventListener('dragend', () => { if (draggedCard) { draggedCard = ''; draw() } })

// 按标题栏和打开对话框的按钮（data-keep）不拿走正文的光标：开了菜单又收起、点了侧栏的开关、对话框关掉，接着打的字还进正文。
root.addEventListener('mousedown', (ev) => { if (ev.target.closest('.titlebar, [data-keep]')) ev.preventDefault() })

// 拖右栏的左边改它的宽度：不窄于 300，不宽过半个窗口。
root.addEventListener('mousedown', (ev) => {
  if (!ev.target.closest('.pane-edge')) return
  ev.preventDefault()
  const win = document.getElementById('win'), right = win.getBoundingClientRect().right
  const held = right - ev.clientX - document.getElementById('pane').offsetWidth // 按下的地方离右栏的边有多远
  const move = (e) => set({ paneW: Math.round(Math.min(Math.max(right - e.clientX - held, 300), win.offsetWidth / 2)) })
  const up = () => { document.removeEventListener('mousemove', move); document.removeEventListener('mouseup', up) }
  document.addEventListener('mousemove', move)
  document.addEventListener('mouseup', up)
})

// 「跟随系统」时，系统换了深浅就跟着换。
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', draw)

draw()
