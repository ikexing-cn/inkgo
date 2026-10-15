// 整个窗口怎么拼起来，和不管光标在哪都管用的那几个键。
'use strict'

const WINDOW_BUTTONS = '<div class="window-buttons">' + ['<path d="M0 5h10"></path>', '<rect x=".5" y=".5" width="9" height="9" rx="1"></rect>', '<path d="M.5.5l9 9M9.5.5l-9 9"></path>']
  .map((d) => `<div><svg class="icon-xs" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">${d}</svg></div>`).join('') + '</div>'

// 标题栏：自己占一行。左边是侧栏的开关和菜单，右边是窗口的三个按钮；下面各栏不用给它们让位。
// 菜单是命令的总表：哪一页用不上的那几行是灰的；AI 关着时，AI 的那几行不显示。开着一个的时候，指针移到旁边的名字上就换过去。
// 专注写作时没有这一行：窗口的三个按钮回到工具栏那一行的右端。
function titlebar() {
  const writing = S.page === 'write', cur = writing && chapterOf(S.ch)
  const inBook = S.page !== 'shelf', hasPane = writing || S.page === 'talk'
  const outline = cur && (S.outlines[cur.key] || '').trim(), steps = (cur && !S.ai && S.undo[cur.key]) || { back: [], fwd: [] }
  const then = (act) => () => { set(SHUT); act() }
  const ask = (dialog) => () => set({ dialog, ...SHUT })
  const menus = [
    ['文件', () => menuRow('新建章节', writing ? then(() => newChapter()) : null, '', 'Ctrl+N') + menuRow('新建文件夹', writing ? then(newFolder) : null) + HAIR +
      menuRow('保存', cur ? then(saveNow) : null, '', 'Ctrl+S') + menuRow('快照', cur ? ask({ kind: 'snaps', id: '', diff: false }) : null) +
      menuRow('导出…', inBook ? ask({ kind: 'export', range: cur ? '本章' : '全书', indent: true }) : null) + HAIR +
      menuRow('设置…', ask({ kind: 'settings', tab: 'AI', edit: '' }))],
    ['编辑', () => menuRow('撤销', steps.back.length ? then(() => unwrite(-1)) : null, '', 'Ctrl+Z') + menuRow('重做', steps.fwd.length ? then(() => unwrite(1)) : null, '', 'Ctrl+Y') + HAIR +
      menuRow('查找替换', cur ? () => set({ find: S.find || { q: '', to: '', all: false, at: 0 }, ...SHUT }) : null, '', 'Ctrl+F') +
      menuRow('重命名', cur && !S.rename ? then(renameOpen) : null, '', 'F2') + (aiOn() ? HAIR +
      menuRow('续写一句', cur ? then(continueText) : null, '', 'Ctrl+J') +
      menuRow('起草本章', outline ? ask({ kind: 'draft', goal: '', how: '接在已有内容之后' }) : null, '', cur && !outline ? '请先填写章纲' : '') : '')],
    ['视图', () => menuRow(S.sideOpen ? '收起侧栏' : '展开侧栏', inBook ? () => set({ sideOpen: !S.sideOpen, ...SHUT }) : null) +
      menuRow(S.paneOpen ? '收起右栏' : '展开右栏', hasPane ? () => set({ paneOpen: !S.paneOpen, ...SHUT }) : null) +
      menuRow('专注写作', writing ? () => set({ focus: true, ...SHUT }) : null, '', 'F11') + HAIR +
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
    `<button class="icon-btn muted${inBook ? '' : ' off'}" aria-label="${sideWord}" title="${sideWord}"${onClick(inBook ? () => set({ sideOpen: !S.sideOpen, ...SHUT }) : null)}>${svg('icon-md', PANEL + path('M9 5v14'))}</button>` +
    menus.map(menu).join('') + WINDOW_BUTTONS + '</div>'
}

// 重新开始：点过、改过的都回到刚打开的样子，只留外观。
function restart() {
  stopAll()
  set({ ...fresh(), look: S.look })
}

// 右上角的提示，最多露三条，新的在下。提示里能点的几个靠右排，最要紧的（给的第一个）在最右。
function toasts() {
  if (!S.toasts.length) return ''
  const shown = S.toasts.slice(-3), rest = S.toasts.length - shown.length
  const pic = { ok: icon('checked', 'md', 'green'), fail: icon('warn', 'md', 'amber'), busy: icon('spin', 'md', 'muted') }
  const one = (x) => `<div class="toast" data-k="${x.id}">${pic[x.kind] || ''}<div class="fit"><div class="toast-title">${t(x.title)}</div>${x.detail ? `<div class="note">${t(x.detail)}</div>` : ''}` +
    (x.acts.length ? `<div class="toast-acts" data-keep>${[...x.acts].reverse().map(([label, act]) => `<div class="hit link"${onClick(act)}>${t(label)}</div>`).join('')}</div>` : '') +
    `</div><button class="icon-btn muted" aria-label="关闭提示" title="关闭提示"${onClick(() => unsay(x.id))}>${icon('cross', 'sm')}</button></div>`
  return `<div id="toasts">${shown.map(one).join('')}${rest > 0 ? `<div class="toast-rest">还有 ${rest} 条</div>` : ''}</div>`
}

// 标题栏和它下面那一块各有名字（data-k）：前面多出一层遮罩（开了菜单、对话框）时它们还是原来那两个，
// 正文不重建，光标留在原处。
function view() {
  const focus = S.focus && S.page === 'write'
  const main = { write: () => editor(chapterOf(S.ch)), talk: talkPage, check: checkPage, cats: catsPage }
  const side = (S.page === 'write' && !focus) || S.page === 'talk' ? S.paneOpen : false
  const bare = focus || !S.sideOpen
  return '<div class="stage">' +
    `<div class="stage-bar">${button('重新开始', restart)}<div class="anchor left"><div class="btn hit${S.open === 'rare' ? ' soft' : ''}" title="平时不易遇到的情况：出错、作品在软件外被改动"${onClick(() => set({ open: S.open === 'rare' ? '' : 'rare' }))}>少见的情况</div>` +
    (S.open === 'rare' ? `<div class="pop rare-pop" data-k="rare-pop">${rare().map(([label, act]) => menuRow(label, act)).join('')}</div>` : '') + `</div><div class="stage-note">内容为示例。正文可输入，刷新后复原；AI 仅回复固定内容。</div></div>` +
    `<div id="win" class="${dark() ? 'dark' : 'light'}">` + veil() + (focus ? WINDOW_BUTTONS : titlebar()) +
    '<div class="win-body" data-k="win-body">' + (S.page === 'shelf' ? shelfPage() : (bare ? '' : sidebar()) + main[S.page]() + (side ? pane() : '')) + '</div>' +
    floats() + dialogs() + toasts() + '</div></div>'
}

// 按 Esc：从最上面的一层收起。
function backOut() {
  if (S.dialog && S.dialog.ask) return set((s) => ({ dialog: { ...s.dialog, ask: '' } })) // 浮在设置上面的那一问
  if (S.dialog) return shut()
  if (S.open) return set(SHUT)
  if (S.ctx || S.bury || S.pay || S.tree) return closeFloat()
  if (S.rename) return set({ rename: null })
  if (S.ai && S.ai.kind !== 'drafting') return set({ ai: null })
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
  else if (ev.key === 'f') run(() => set({ find: S.find || { q: '', to: '', all: false, at: 0 } }))
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
