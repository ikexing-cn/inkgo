// 原型的底子：一份状态，改了就整个重画，只动变了的地方。别的几份文件都用这里的东西。
'use strict'

// ───────── 状态 ─────────

// 卷里的章，标题是号加名字；作者自己把标题整个改写过的（own），写了什么就是什么。
const numbered = (c) => c.vol && !c.own
const titleOf = (c) => (numbered(c) ? '第' + c.no + '章' + (c.t ? '：' + c.t : '') : c.t)
const chapterOf = (key) => S.chapters.find((c) => c.key === key)
const clip = (text, n) => (text.length > n ? text.slice(0, n) + '…' : text)
// 一章有多少字。书里带来的章写着（假的）字数，新写的照实数。
const sizeOf = (c) => c.c || String(c.paras.join('').length)

// 刚打开一本书的样子。书架上只有第一本有内容，别的是空书。
function fresh(book = BOOK.shelf[0].id) {
  const full = book === BOOK.shelf[0].id
  const of = (key, none) => structuredClone(full ? BOOK[key] : none)
  const kept = (BOOK.shelf.find((b) => b.id === book) || {}).backup || 'none' // 这本书备份得怎么样
  const chapters = of('chapters', [])
  const last = chapters.find((c) => c.no === BOOK.now)
  const snaps = of('snaps', []).map((s) => ({ ...s, paras: last.paras.filter((_, i) => !s.less.includes(i)) }))
  return {
    book, page: 'write', // 在哪一页：write 正文、talk 讨论、check 检查、cats 作品分类、shelf 书架
    volumes: of('volumes', []), chapters, bin: of('bin', []), outlines: of('outlines', {}), summaries: of('summaries', {}),
    entries: of('entries', []), log: of('log', []), marks: of('marks', []), talks: of('talks', []), checks: of('checks', []),
    snaps: last ? { [last.key]: snaps } : {}, cats: [...BOOK.cats], lands: { ...BOOK.lands },
    ch: last ? last.key : '', trail: last ? [last.key] : [], step: 0, // 开着哪一章；trail 是打开过的章，step 是现在在第几个
    tabs: last ? [last.key] : [], // 标签页：开着的几章（和点了「编辑卷纲」的卷），从左到右；ch、vol 是其中正在看的那个
    folds: Object.fromEntries(BOOK.volumes.map(([key]) => [key, !last || key !== last.vol])),
    paneOpen: true, sideOpen: true, tab: '设定', focus: false, // 右栏、侧栏开着吗，右栏在哪一页；focus 是专注写作
    open: '', // 开着的菜单或下拉的名字，一次只开一个
    dialog: null, // 开着的对话框：{ kind, … }
    n: 100, // 新东西的编号，用一个加一个
    sv: 'list', sid: null, sq: '', form: null, // 设定：在哪一页、看哪一条、搜什么、正在填的表
    fv: 'list', fid: null, fq: '', renote: null, // 伏笔：同上，renote 是正在改的备注
    tv: 'all', tid: null, tq: '', draft: '', attach: true, cite: null, // 讨论；cite 是从正文拿来的一段
    waiting: null, // AI 正在回哪段讨论、到哪一步了：{ id, step }，step 是 think 在想、reply 在写
    thoughts: {}, // 哪几条回复的「思考过程」展开了
    shelved: {}, shelvedShown: false, // 侧栏里收起来的几段讨论，和要不要把它们列出来
    told: {}, // 讨论页里哪条回复展开了「AI 参考内容」
    ctx: null, pick: null, bury: null, pay: null, // 正文上浮着的：右键菜单、选中的字、记伏笔、选回收哪条
    bar: null, // 选中一段字以后浮出来的那两个按钮
    flash: null, // 「查看原文」跳过去后要标出来的那句
    find: null, // 查找替换：{ q, to, all, at }
    ai: null, // AI 正在正文里做的事：续写出的一句、改写、起草
    box: '', // 标题下面展开的是「章纲」还是「摘要」
    vol: '', // 开着的是哪一卷；开着一章时空着
    named: null, // 设定里改了名字，正文里还留着旧名：{ was, now, n }
    tidying: false, // AI 正在后台把刚聊的整理进设定
    saved: true, // 正文存了没有
    kept: null, // 没存的时候，存着的那一份：{ key, paras, dirty, summaries }。关标签页选「不保存」就回到它
    undo: {}, // 每章打字前的样子：{ back, fwd }，各是几份 paras，撤销和重做用
    outside: '', // 软件外面动了开着的这一章：changed 改过、lost 找不到了
    rename: null, // 侧栏里正在改名字的：{ key, t, all }。all 是一开始把字全选上，打字就整个换掉
    tree: null, // 右键点出来的菜单：{ key, x, y }；talk 是侧栏里的一段讨论，tab 是一个标签页，都不是就是章节树里的一行
    todoAll: true, more: {}, ignored: [], // 待处理：看全部还是本章、哪几组展开了、忽略掉的
    kind: '全部', cid: null, checking: false, dirty: {}, // 检查：看哪一类、哪一条、正在查哪一章、哪几章查过以后又改了
    queue: [], never: {}, failed: {}, // 检查：排着队等查的、尚未检查的、没查成的几章
    part: {}, // 太长、AI 只读了前面一段的几章：{ check, sum } 是检查和摘要各读了前多少字，再查、再生成一次就当它读全了
    catForm: { name: '', land: '' }, catDrop: '', // 作品分类：正在填的、要删的
    toasts: [],
    backup: { state: kept, at: kept === 'ok' ? '今天 10:32' : kept === 'fail' ? '昨天 21:40' : '', dir: kept === 'none' ? '' : 'E:\\写作备份', copies: kept === 'none' ? 0 : 12, close: true, daily: true, hourly: false, keep: '最近 30 份', hush: false },
    // 换一本书也不变的：外观、书架、设置里的东西。
    look: '跟随系统', shelf: structuredClone(BOOK.shelf), cards: true, library: BOOK.library,
    uses: structuredClone(BOOK.uses), services: structuredClone(BOOK.services), budget: '16000', auto: false, recap: true,
    when: '写完一章后检查', late: BOOK.late, goal: '', paneW: 0, // paneW 是右栏拖出来的宽度，没拖过是 0
    noAI: false, // 关闭了 AI：本机的所有作品都不显示 AI 功能，也不发送任何内容
  }
}
const KEPT = ['look', 'shelf', 'cards', 'library', 'uses', 'services', 'budget', 'auto', 'recap', 'when', 'late', 'goal', 'paneW', 'noAI']

let S = fresh()
let sent = 0 // 作者发过几条消息，AI 按顺序回下一句
const timers = new Set()
let saving = 0 // 停笔后自己存的那一下；又打了字就从头等
let composing = false, owed = false // 拼音正在选字：这时不重画，免得把没选完的字冲掉；owed 是欠着的一遍

// 停笔片刻自己存；又打了字就从头等。正问着「存不存」的时候不存，接着等。
function autosave() {
  clearTimeout(saving); timers.delete(saving)
  saving = later(1500, () => (S.dialog && S.dialog.kind === 'unsaved' ? autosave() : set((s) => (s.saved ? null : { saved: true }))))
}

// change 是要改的那几项，或者一个看着现在的状态算出它们的函数；返回空就什么都不变。
function set(change) {
  const next = typeof change === 'function' ? change(S) : change
  if (!next) return
  const was = S
  S = { ...S, ...next }
  if (next.saved === false && was.saved) { const c = was.chapters.find((c) => c.key === S.ch); S.kept = c ? { key: c.key, paras: c.paras, dirty: was.dirty, summaries: was.summaries } : null } // 存着的那一份留一下
  if (next.saved === false) autosave()
  draw()
}

// 过一会儿再做。换书、重新开始时，还没到点的都作废。
function later(ms, act) {
  const id = setTimeout(() => { timers.delete(id); act() }, ms)
  timers.add(id)
  return id
}
// AI 做的事过一会儿才有结果。这时 AI 已经关了，结果就不要了。
const aiLater = (ms, act) => later(ms, () => { if (!S.noAI) act() })
// 开着 AI 吗。关着时，用到 AI 的地方整个不显示，不是变灰。
const aiOn = () => !S.noAI
function stopAll() {
  for (const id of timers) clearTimeout(id)
  timers.clear()
  sent = 0
}

// 右上角的提示。kind：'' 一句话、ok 成了、fail 没成、busy 正在做；没成的和正在做的不会自己消失。
// acts 是提示里能点的几个：每个是 [字, 点了做什么]。
function say(title, detail = '', kind = '', acts = []) {
  const id = 'toast' + Math.random().toString(36).slice(2, 8)
  // 只给了一句话的：第一个逗号、分号前面是标题，后面是小字，提示条就不用拉得很宽。
  const cut = detail ? -1 : title.search(/[，；]/)
  if (cut > 0) [title, detail] = [title.slice(0, cut), title.slice(cut + 1)]
  if (kind === 'fail') acts = [...acts, ['复制详情', () => { if (navigator.clipboard) navigator.clipboard.writeText(title + (detail ? '：' + detail : '')).catch(() => {}) }]]
  set((s) => ({ toasts: [...s.toasts.filter((x) => x.title !== title), { id, title, detail, kind, acts }] }))
  if (kind !== 'fail' && kind !== 'busy') later(3000, () => unsay(id))
  return id
}
const unsay = (id) => set((s) => ({ toasts: s.toasts.filter((x) => x.id !== id) }))

const dark = () => S.look === '深色' || (S.look === '跟随系统' && matchMedia('(prefers-color-scheme: dark)').matches)
const SHUT = { open: '' }

// ───────── 画：小零件 ─────────

// 每画一遍，把这一遍用到的动作记在这里，元素上只留它的号码。
let H = []
const hook = (kind) => (fn) => (fn ? ` data-${kind}="${H.push(fn) - 1}"` : '')
const onClick = hook('click'), onInput = hook('input'), onKey = hook('keydown'), onMenu = hook('menu'), onUp = hook('up'), onOver = hook('over'), onMid = hook('mid'), onPaste = hook('paste'), onBlur = hook('blur')

// 书里的字、作者打的字，放进页面前都过一遍。
const t = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))

const svg = (cls, inner, width = 1.8) => `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${width}" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
const path = (d) => `<path d="${d}"></path>`
const PANEL = '<rect x="4" y="5" width="16" height="14" rx="2"></rect>'
const SHAPE = {
  chat: path('M5 5h14v10H10l-4 4v-4H5z'),
  book: path('M6 4h11a1 1 0 011 1v15H7a2 2 0 01-2-2V5a1 1 0 011-1zM5 17a2 2 0 012-2h11'),
  checked: '<circle cx="12" cy="12" r="8"></circle>' + path('M8.5 12.2l2.5 2.5 4.5-5'),
  tag: path('M4 12V4h8l8 8-8 8z'),
  spark: path('M10 4l1.6 4.4L16 10l-4.4 1.6L10 16l-1.6-4.4L4 10l4.4-1.6zM18 15l.8 2.2L21 18l-2.2.8L18 21l-.8-2.2L15 18l2.2-.8z'),
  page: path('M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z') + path('M14 3v5h5'),
  clock: path('M4 12a8 8 0 1 0 2.5-5.8M4 5v4h4M12 8v4l3 2'),
  save: path('M12 4v11M7 11l5 5 5-5M5 20h14'),
  screen: '<rect x="4" y="5" width="16" height="11" rx="1.5"></rect>' + path('M9 20h6M12 16v4'),
  sun: '<circle cx="12" cy="12" r="3.5"></circle>' + path('M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4L7 17M17 7l1.4-1.4'),
  moon: path('M19 14.5A7.5 7.5 0 0 1 9.5 5a7.5 7.5 0 1 0 9.5 9.5z'),
  pen: path('M4 20l1-4L16 5l3 3L8 19zM14 7l3 3'),
  keep: path('M7 4h10v16l-5-3.5L7 20zM10 9h4'),
  cross: path('M6 6l12 12M18 6L6 18'),
  warn: path('M12 4l9 16H3zM12 10v4M12 17v.5'),
  spin: path('M12 4v3M12 17v3M4 12h3M17 12h3M6.3 6.3l2.2 2.2M15.5 15.5l2.2 2.2M6.3 17.7l2.2-2.2M15.5 8.5l2.2-2.2'),
  list: path('M9 7h11M9 12h11M9 17h11M5 7h.5M5 12h.5M5 17h.5'),
  grid: '<rect x="5" y="5" width="5.5" height="5.5" rx="1"></rect><rect x="13.5" y="5" width="5.5" height="5.5" rx="1"></rect><rect x="5" y="13.5" width="5.5" height="5.5" rx="1"></rect><rect x="13.5" y="13.5" width="5.5" height="5.5" rx="1"></rect>',
  lines: path('M5 7h14M5 12h9M5 17h11'),
  copy: '<rect x="9" y="9" width="10" height="10" rx="2"></rect>' + path('M15 9V7a2 2 0 0 0-2-2H7a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2'),
  eye: path('M3 12s3.5-6 9-6 9 6 9 6-3.5 6-9 6-9-6-9-6z') + '<circle cx="12" cy="12" r="2.5"></circle>',
  leaf: path('M5 19c0-8 5-13 14-14 0 9-5 14-14 14zM5 19l7-7'),
  left: path('M15 6l-6 6 6 6'), right: path('M9 6l6 6-6 6'), down: path('M6 9l6 6 6-6'), up: path('M6 15l6-6 6 6'),
  find: '<circle cx="11" cy="11" r="6.5"></circle>' + path('M20 20l-4-4'),
  plus: path('M12 5v14M5 12h14'), tick: path('M5 12l5 5 9-10'), send: path('M12 19V5M6 11l6-6 6 6'),
  flag: path('M6 20V4h10l-2 4 2 4H6'),
  link: path('M10 14l4-4M8.5 11.5l-2 2a3 3 0 004 4l2-2M15.5 12.5l2-2a3 3 0 00-4-4l-2 2'),
  folder: path('M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z'),
  bin: path('M5 7h14M10 7V5h4v2M7 7l1 12h8l1-12'),
  gear: path('M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z') + '<circle cx="12" cy="12" r="3"></circle>',
  focus: path('M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7'),
  dots: path('M5 12h.01M12 12h.01M19 12h.01'),
}
// 一个图标：name 是上面的名字，size 是 xs、sm、md、lg，cls 是颜色之类的。
const icon = (name, size = 'md', cls = '') => svg(`icon-${size}${cls ? ' ' + cls : ''}`, SHAPE[name], name === 'dots' ? 2.6 : 1.8)
const ICON = {
  left: icon('left', 'sm', 'muted'), right: icon('right', 'sm', 'faint'), down: icon('down', 'sm', 'muted'),
  find: icon('find', 'sm', 'faint'), plus: icon('plus', 'sm'), dots: icon('dots'), send: icon('send', 'sm'), check: icon('tick', 'sm'),
}
const DOT = '<div class="dot"></div>'
const HAIR = '<div class="hair"></div>'
// 菜单开着时铺在下面的一层，点它就收起来。
const veil = () => (S.open ? `<div class="veil" data-k="veil"${onClick(() => set(SHUT))}></div>` : '')

// 输入框。change 拿到的是打完以后的字。
const typed = (value, change, hint, label, more = '') => `<input class="in" value="${t(value)}" placeholder="${t(hint)}" aria-label="${t(label)}"${onInput(change)}${more}>`
const line = (value, change, hint, label, more = '') => `<div class="box line">${typed(value, change, hint, label, more)}</div>`
const area = (value, change, hint, label, rows = 4, more = '') => `<div class="box area"><textarea class="in" rows="${rows}" placeholder="${t(hint)}" aria-label="${t(label)}"${onInput(change)}${more}>${t(value)}</textarea></div>`
const field = (label, box) => `<div class="field"><div class="label">${t(label)}</div>${box}</div>`
const button = (label, act, cls = '') => `<div class="btn hit ${cls}"${onClick(act)}>${t(label)}</div>`
const buttons = (...bs) => `<div class="buttons">${bs.join('')}</div>`
const where = (text, find, see = '查看原文') => `<div class="where"><div class="fit ell">${t(text)}</div><div class="see hit link"${onClick(find)}>${see}</div></div>`
// 按回车时做 ok，按 Esc 时做 no。拼音还没选完字的回车不算。
const keys = (ok, no) => onKey((ev) => { if (ev.key === 'Enter' && !ev.isComposing) ok(); else if (ev.key === 'Escape' && no) { ev.stopPropagation(); no() } })
// 打勾的方框，带一句话。
const tickbox = (on, label, flip, tip = '') => `<div class="attach hit"${tip ? ` title="${t(tip)}"` : ''}${onClick(flip)}><div class="check${on ? ' on' : ''}">${on ? ICON.check : ''}</div>${t(label)}</div>`
// 几个里选一个的一排：options 每项是字，或者 [字, 图标]。
const seg = (value, options, pick, label) => `<div class="seg" aria-label="${t(label)}">${options.map((o) => { const [name, pic] = [].concat(o); return `<div class="hit${name === value ? ' on' : ''}"${onClick(() => pick(name))}>${pic ? icon(pic, 'sm') : ''}${t(name)}</div>` }).join('')}</div>`
// 圆点单选的几行：options 每项是 [字, 小字]。
const radios = (value, options, pick) => `<div class="radios">${options.map(([name, about]) => `<div class="radio hit"${onClick(() => pick(name))}><div class="ring${name === value ? ' on' : ''}"></div><div><div>${t(name)}</div><div class="note">${t(about)}</div></div></div>`).join('')}</div>`
// 下拉：id 是它的名字，开着时列出 options。
function choose(id, value, options, pick, label) {
  const on = S.open === id
  return `<div class="choose" data-k="${t(id)}"><div class="box line hit" aria-label="${t(label)}"${onClick(() => set({ open: on ? '' : id }))}><div class="fit ell">${t(value)}</div>${ICON.down}</div>` +
    (on ? `<div class="choose-menu pop scroll">${options.map((o) => `<div class="pop-row row${o === value ? ' on' : ''}"${onClick(() => { set(SHUT); pick(o) })}>${t(o)}</div>`).join('')}</div>` : '') + '</div>'
}
// 菜单里的一行：左边图标，右边快捷键或小字；act 空着就是灰的。
const menuRow = (label, act, pic, side = '') => `<div class="pop-row row${act ? '' : ' off'}"${onClick(act)}>${pic ? icon(pic, 'md', 'muted') : ''}<div class="grow">${t(label)}</div><div class="side">${t(side)}</div></div>`
// 对话框：盖住整个窗口，点外面或按 Esc 关掉。
const shut = () => set({ dialog: null, ...SHUT })
const dialog = (cls, inner) => `<div class="scrim"${onClick(shut)}></div><div class="dialog-wrap">${veil()}<div class="dialog ${cls}" data-k="dialog">${inner}</div></div>`
const dialogHead = (title, about) => `<div class="dialog-title">${t(title)}</div>${about ? `<div class="note">${t(about)}</div>` : ''}`

// 一段字，搜的词标出来。
function marked(text, find) {
  if (!find) return t(text)
  return text.split(find).map(t).join(`<span class="found">${t(find)}</span>`)
}
// 从搜的词前面一点开始，免得一行截断后看不到它。
function near(text, find) {
  const i = find ? text.indexOf(find) : -1
  return i > 8 ? '…' + text.slice(i - 6) : text
}

// 列表里的几种行。
const rows = (list) => `<div class="rows scroll" data-k="rows">${list.join('')}</div>`
const head = (label) => `<div class="head">${t(label)}</div>`
const empty = (words, how) => `<div class="empty"><div>${t(words)}</div>${how ? `<div class="note">${t(how)}</div>` : ''}</div>`
const todo = (label, n, open) => `<div class="todo row"${onClick(open)}>${DOT}<div class="grow">${t(label)}</div><div class="side">${n}</div>${ICON.right}</div>`
const item = ({ name, about = '', find = '', dot, side, open }) => `<div class="item row"${onClick(open)}><div class="name ell">${t(name)}</div><div class="about ell">${marked(about, find)}</div>${dot ? DOT : ''}${side ? `<div class="side">${t(side)}</div>` : ''}${ICON.right}</div>`
// 一页正中的一段话：图标、标题、说明，下面可以放按钮。
const blank = (pic, title, about, more = '') => `<div class="blank"><div class="blank-icon">${icon(pic, 'lg')}</div><div class="blank-title">${t(title)}</div><div class="blank-about">${t(about)}</div>${more}</div>`

// ───────── 把画好的换到页面上 ─────────

const root = document.getElementById('app')
let born = null // 这一遍新放进页面的、要把光标放进去的输入框
let drawing = false // 正在换页面：这时输入框被拿掉而丢了光标，不算作者点了别处

// 有 id 或 data-k 的元素认名字，换了位置也还是它；别的按顺序对。
const keyOf = (n) => (n.nodeType === 1 ? n.id || n.getAttribute('data-k') : null)

// 让 old 变成 next 的样子：只改不一样的属性和字，输入框里正打着的字、正文里选中的字都不受打扰。
function patch(old, next) {
  if (old.nodeType !== 1) { if (old.data !== next.data) old.data = next.data; return }
  for (const { name } of [...old.attributes]) if (!next.hasAttribute(name)) old.removeAttribute(name)
  for (const { name, value } of [...next.attributes]) if (old.getAttribute(name) !== value) old.setAttribute(name, value)
  if (old.nodeName === 'INPUT' || old.nodeName === 'TEXTAREA') { if (old.value !== next.value) old.value = next.value; return }
  children(old, next)
}

function children(parent, want) {
  const named = new Map()
  for (const c of parent.children) if (keyOf(c)) named.set(keyOf(c), c)
  const wanted = new Set([...want.childNodes].map(keyOf))
  let at = parent.firstChild
  for (const w of [...want.childNodes]) {
    // 有名字、这次不要了的先拿掉：后面的留在原处，不用挪到它前面去（挪一下，里面的光标就丢了）。
    while (at && keyOf(at) && !wanted.has(keyOf(at))) { const gone = at; at = at.nextSibling; gone.remove() }
    const key = keyOf(w)
    let have = null
    if (key) { have = named.get(key) ?? null; named.delete(key) } else if (at && !keyOf(at) && at.nodeName === w.nodeName) have = at
    if (have && have.nodeName !== w.nodeName) have = null
    if (!have) {
      parent.insertBefore(w, at)
      if (w.nodeType === 1) born = (w.matches('[autofocus]') ? w : w.querySelector('[autofocus]')) || born
    } else {
      if (have === at) at = at.nextSibling; else parent.insertBefore(have, at)
      patch(have, w)
    }
  }
  while (at) { const gone = at; at = at.nextSibling; gone.remove() }
}

// view() 在 app.js 里：整个页面现在该是什么样。
function draw() {
  if (composing) { owed = true; return }
  H = []
  born = null
  drawing = true
  const want = document.createElement('template')
  want.innerHTML = view()
  children(root, want.content)
  if (born) { born.focus(); if (born.hasAttribute('data-all')) born.select(); else born.setSelectionRange(born.value.length, born.value.length) }
  drawing = false
  for (const talk of document.querySelectorAll('.talk')) talk.scrollTop = talk.scrollHeight // 讨论总是停在最新一句
  const tab = document.querySelector('.sheet.on') // 标签多到排不下时，正在看的那个总在看得见的地方
  if (tab) { const strip = tab.parentElement; strip.scrollLeft = Math.max(tab.offsetLeft + tab.offsetWidth - strip.clientWidth, Math.min(strip.scrollLeft, tab.offsetLeft)) }
}

// 页面上只挂这几个听的：事情出在哪个元素上，就照它身上的号码去找动作。
for (const [event, kind] of [['click', 'click'], ['keydown', 'keydown'], ['contextmenu', 'menu'], ['mouseup', 'up'], ['mouseover', 'over'], ['auxclick', 'mid'], ['paste', 'paste']]) {
  root.addEventListener(event, (ev) => { const el = ev.target.closest(`[data-${kind}]`); if (el) H[el.dataset[kind]](ev) })
}
root.addEventListener('focusout', (ev) => { const el = ev.target.closest('[data-blur]'); if (el && !drawing) H[el.dataset.blur](ev) }) // 光标离开了输入框
root.addEventListener('input', (ev) => { const el = ev.target.closest('[data-input]'); if (el) H[el.dataset.input](el.value) })
// 在正文里用拼音：选完字才把页面上的读回来。
root.addEventListener('compositionstart', (ev) => { if (ev.target.closest('.prose')) composing = true })
root.addEventListener('compositionend', () => { if (!composing) return; composing = false; wrote(); if (owed) { owed = false; draw() } })
