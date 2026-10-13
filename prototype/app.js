// 整个窗口怎么拼起来，和不管光标在哪都管用的那几个键。
'use strict'

const WINDOW_BUTTONS = '<div class="window-buttons">' + ['<path d="M0 5h10"></path>', '<rect x=".5" y=".5" width="9" height="9" rx="1"></rect>', '<path d="M.5.5l9 9M9.5.5l-9 9"></path>']
  .map((d) => `<div><svg class="icon-xs" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">${d}</svg></div>`).join('') + '</div>'

// 重新开始：点过、改过的都回到刚打开的样子，只留外观。
function restart() {
  stopAll()
  set({ ...fresh(), look: S.look })
}

// 右上角的提示，最多露三条，新的在下。
function toasts() {
  if (!S.toasts.length) return ''
  const shown = S.toasts.slice(-3), rest = S.toasts.length - shown.length
  const pic = { ok: icon('checked', 'md', 'green'), fail: icon('warn', 'md', 'amber'), busy: icon('spin', 'md', 'muted') }
  const one = (x) => `<div class="toast" data-k="${x.id}">${pic[x.kind] || ''}<div class="fit"><div class="toast-title">${t(x.title)}</div>${x.detail ? `<div class="note">${t(x.detail)}</div>` : ''}` +
    (x.acts.length ? `<div class="toast-acts">${x.acts.map(([label, act]) => `<div class="hit ink"${onClick(act)}>${t(label)}</div>`).join('')}</div>` : '') +
    `</div><button class="icon-btn muted" aria-label="关闭提示" title="关闭提示"${onClick(() => unsay(x.id))}>${icon('cross', 'sm')}</button></div>`
  return `<div id="toasts">${shown.map(one).join('')}${rest > 0 ? `<div class="toast-rest">还有 ${rest} 条</div>` : ''}</div>`
}

function view() {
  const focus = S.focus && S.page === 'write'
  const main = { write: () => editor(chapterOf(S.ch)), talk: talkPage, check: checkPage, cats: catsPage }
  const side = (S.page === 'write' && !focus) || S.page === 'talk' ? S.paneOpen : false
  return '<div class="stage">' +
    `<div class="stage-bar">${button('重新开始', restart)}<div class="anchor left"><div class="btn hit${S.open === 'rare' ? ' soft' : ''}" title="平时不易遇到的情况：出错、作品在软件外被改动"${onClick(() => set({ open: S.open === 'rare' ? '' : 'rare' }))}>少见的情况</div>` +
    (S.open === 'rare' ? `<div class="pop rare-pop" data-k="rare-pop">${rare().map(([label, act]) => menuRow(label, act)).join('')}</div>` : '') + `</div><div class="stage-note">内容为示例，正文不可输入，AI 仅回复固定内容。</div></div>` +
    `<div id="win" class="${dark() ? 'dark' : 'light'}">` + WINDOW_BUTTONS + veil() +
    (S.page === 'shelf' ? shelfPage() : (focus ? '' : sidebar()) + main[S.page]() + (side ? pane() : '')) +
    floats() + dialogs() + toasts() + '</div></div>'
}

// 按 Esc：从最上面的一层收起。
function backOut() {
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
  else if (!ctrl || !writing || !chapterOf(S.ch)) return
  else if (ev.key === 's') run(saveNow)
  else if (ev.key === 'f') run(() => set({ find: S.find || { q: '', to: '', all: false, at: 0 } }))
  else if (ev.key === 'j') run(continueText)
  else if (ev.key === 'l') run(cite)
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
