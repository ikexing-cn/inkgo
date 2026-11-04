// 书架和导入书籍；扩展带来的页面在各自的文件里。
'use strict'

// ───────── 书架 ─────────

// 原型开不了系统的窗口，只说一声本来会开什么。
const osAsk = (what) => () => say(L.shelfOSTitle(), L.shelfOSStep(what))

// 添加已有作品：原型里当作每次选的都是同一个文件夹。头一回加上，再点就说已经在了。
function addBook() {
  if (S.shelf.some((b) => b.id === 'found')) { say(L.shelfAlready()); return }
  set((s) => ({ shelf: [...s.shelf, { id: 'found', name: BOOK.found, kind: '出版小说', words: 21000, at: null }] }))
  say(L.shelfAdded(), BOOK.found, 'ok')
}

// 换一本书：外观、书架、设置里的东西带过去，别的照那本书刚打开的样子。
function openBook(id) {
  stopAll()
  set({ ...fresh(id), ...Object.fromEntries(KEPT.map((k) => [k, S[k]])) })
}

function shelfPage() {
  const all = S.shelf
  const open = (b) => onClick(b.gone ? null : () => openBook(b.id))
  const about = (b) => genreWord(b.kind) + ' · ' + wordsShown(b.words)
  const more = (b) => `<div class="anchor"><button class="icon-btn muted" aria-label="${t(L.commonMoreActions())}" title="${t(L.commonMoreActions())}"${onClick(() => set({ open: S.open === 'book-' + b.id ? '' : 'book-' + b.id }))}>${ICON.dots}</button>` +
    (S.open === 'book-' + b.id ? '<div class="pop shelf-pop">' + menuRow(L.commonOpenFolder(), () => { set(SHUT); osAsk(L.shelfShowFolder())() }) + menuRow(L.shelfRenameBook(), () => set({ dialog: { kind: 'nameBook', id: b.id, name: b.name }, ...SHUT })) + menuRow(L.shelfRemove(), () => removeBook(b)) + '</div>' : '') + '</div>'
  const removeBook = (b) => { set((s) => ({ shelf: s.shelf.filter((x) => x.id !== b.id), ...SHUT })); say(L.shelfRemoved(b.name), '', 'ok') }
  const card = (b) => `<div class="shelf-card${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-card-top"><div class="shelf-logo">${t(b.name[0])}</div><div class="grow"></div>${b.gone ? '' : more(b)}</div>` +
    `<div class="shelf-name ell">${t(b.name)}</div><div class="note">${t(about(b))}</div><div class="grow"></div>` +
    (b.gone ? `<div class="shelf-gone"><div class="fit danger">${t(L.shelfFolderMissing())}</div><div class="hit link"${onClick(osAsk(L.shelfPickBookFolder()))}>${t(L.shelfLocate())}</div><div class="hit link"${onClick(() => removeBook(b))}>${t(L.shelfRemoveShort())}</div></div>` : `<div class="faint">${t(b.at === null ? L.shelfNotOpened() : timeShown(b.at))}</div>`) + '</div>'
  const row = (b) => `<div class="shelf-row${b.gone ? ' gone' : ' hit'}"${open(b)}><div class="shelf-logo">${t(b.name[0])}</div><div class="fit"><div class="shelf-row-name"><span class="shelf-name">${t(b.name)}</span><span class="note">${t(about(b))}</span></div><div class="faint ell">${t(S.library + sep() +b.name)}</div></div>` +
    (b.gone ? `<div class="danger">${t(L.shelfFolderMissing())}</div><div class="hit link"${onClick(osAsk(L.shelfPickBookFolder()))}>${t(L.shelfRelocate())}</div><div class="hit link"${onClick(() => removeBook(b))}>${t(L.shelfRemove())}</div>` : `<div class="faint">${t(b.at === null ? L.shelfNotOpened() : timeShown(b.at))}</div>${more(b)}`) + '</div>'
  const bring = button(L.shelfImportMore(), startImport)
  const create = button(L.shelfNewBook(), () => set({ dialog: { kind: 'newBook', name: '', type: BOOK.kinds[0] } }), 'primary')
  if (!all.length) {
    // AI 关着时，这一页不讲 AI。
    return '<div class="shelf">' + (aiOn() ? blank('book', L.shelfStartTitle(), L.shelfStartAbout(),
      `<div class="shelf-facts"><div>${t(L.shelfFactFiles())}</div><div>${t(L.shelfFactAI())}</div><div>${t(L.shelfFactKey())}</div></div>` +
      buttons(button(L.shelfAddExisting(), addBook), bring, create))
      : blank('book', L.shelfStartTitle(), L.shelfEmptyAbout(),
        `<div class="shelf-facts"><div>${t(L.shelfFactFilesNoAI())}</div></div>` + buttons(button(L.shelfAddExisting(), addBook), bring, create))) + '</div>'
  }
  return '<div class="shelf scroll"><div class="shelf-column">' +
    `<div class="shelf-head"><div class="fit"><div class="shelf-title">${t(L.shelfTitle())}</div><div class="shelf-sub"><span class="ell">${t(L.shelfCountLibrary(all.length, S.library))}</span><span class="hit link"${onClick(osAsk(L.shelfPickLibrary()))}>${t(L.shelfChange())}</span></div></div>` +
    `<div class="seg icons" aria-label="${t(L.shelfView())}">${[[L.shelfList(), 'list', false], [L.shelfCards(), 'grid', true]].map(([label, pic, cards]) => `<div class="hit${S.cards === cards ? ' on' : ''}" title="${t(label)}"${onClick(() => set({ cards }))}>${icon(pic)}</div>`).join('')}</div>` +
    button(L.shelfAddExisting(), addBook) + bring + create + '</div>' +
    (S.cards ? `<div class="shelf-cards">${all.map(card).join('')}</div>` : `<div class="shelf-rows">${all.map(row).join('')}</div>`) +
    '</div></div>'
}

// ───────── 导入书籍 ─────────

// 导入书籍是书架上的两个对话框：先是一张小表单（作品名称、体裁、位置），「下一步」再弹出预览，左边是切出来的卷和章，右边是选中的那一章，
// 标题和正文都能改。左边的树和正文页的侧栏是同一套操作：重命名、新建章节、新建文件夹、上移下移、移到哪一卷、拖动，右键点出同样的菜单；
// 只有「移到回收站」换成「删除」（作品还没建，没有回收站），点了马上生效，提示条上能撤销。
// 切错了还可以在标题右边补救：并入上一章，或者从光标处另起一章。预览里的「确定」才新建作品，原文件不动。
// S.imp 和正文页一样把卷和章分开记：volumes 是 [id, 标题]，chapters 每章有 id、title、paras、vol（所在的卷，没有就是空）；
// sel 是正看着的那一章，folds 是折起的卷，rename 是正在改名的一行，menu 是右键点出来的菜单，
// at 是改过的位置（没改过就跟着名字走），can 是光标正停在能另起一章的地方。
const impWords = (c) => c.paras.join('').replace(/\s/g, '').length
const impBox = () => document.querySelector('.import-prose')
const impCur = () => S.imp.chapters.find((c) => c.id === S.imp.sel)
const impOpen = () => S.dialog && S.dialog.kind === 'importPreview' && !!S.imp
const impChapters = () => S.imp.chapters
const impPatch = (patch) => set((s) => ({ imp: { ...s.imp, ...patch } }))
const impShut = () => impPatch({ menu: null })

function startImport() {
  let n = 0
  const volumes = [], chapters = []
  let vol = ''
  for (const r of BOOK.source.rows) {
    if (r.vol) { vol = 'v' + n++; volumes.push([vol, r.vol]) } else { const k = n++; chapters.push({ id: 'i' + k, title: r.title, vol, paras: Array.from({ length: r.paras }, (_, p) => BOOK.source.lines[(k + p) % BOOK.source.lines.length]) }) }
  }
  set({ imp: { name: BOOK.source.file.replace(/\.[^.]+$/, ''), type: BOOK.kinds[0], at: null, volumes, chapters, sel: chapters[0].id, folds: {}, can: false, n, rename: null, menu: null }, dialog: { kind: 'importForm' }, ...SHUT })
}

// 作品叫什么、放在哪。名字空着，或者那个文件夹里已有文件，都不能导入。
function impWhere() {
  const imp = S.imp, name = imp.name.trim(), at = (imp.at ?? S.library + sep() +(name || L.shelfUntitledBook())).trim()
  const taken = !!at && S.shelf.some((b) => S.library + sep() +b.name === at)
  return { name, at, taken, ok: !!name && !!at && !taken }
}

// 换到一章：正文回到最上面。
function impShow(id) {
  impPatch({ sel: id, can: false })
  const box = document.querySelector('.imp-text')
  if (box) box.scrollTop = 0
}

// 改一处切分：先把改之前的存下，提示条上点「撤销」就整个退回去。
// 这个撤销只管到下一处改动：之后又改了别的（打字、改名、挪动），提示条就收起，免得撤销把后来的改动一起退掉。
let impUndo = ''
const impRetire = () => { if (impUndo) { unsay(impUndo); impUndo = '' } }
function importChange(then, words) {
  impRetire()
  const before = { ...S.imp, rename: null, menu: null }
  set((s) => ({ imp: { ...then(s.imp), rename: null, menu: null } }))
  const box = document.querySelector('.imp-text')
  if (box) box.scrollTop = 0
  impUndo = say(words, '', 'ok', [[L.cmdUndo(), () => { impRetire(); if (S.imp) set({ imp: before }) }]])
}

// 一章前面的那一章：只在同一卷里找，卷里的第一章没有上一章。
const impPrev = (c) => { const peers = S.imp.chapters.filter((x) => x.vol === c.vol); return peers[peers.indexOf(c) - 1] }
const impMergeable = () => { const c = impCur(); return !!c && !!impPrev(c) }

// 并入上一章：这一章的标题当作一段，和正文一起接到上一章后面。
function importMerge() {
  if (!impMergeable()) return
  const gone = impCur(), prev = impPrev(gone)
  importChange((imp) => ({ ...imp, sel: prev.id, can: false, chapters: imp.chapters.filter((c) => c.id !== gone.id).map((c) => (c.id === prev.id ? { ...c, paras: [...c.paras, gone.title, ...gone.paras] } : c)) }), L.shelfMergedInto(gone.title))
}

// 光标在正文第几段第几个字后面；不在正文里时没有。
const impCaret = () => { const box = impBox(), s = box && spanNow(box); return s && s[1] }
// 光标两边一边是本章留下的字，一边是新的一章；哪边没有字都不能分。
function impParts() {
  const at = impCaret(), c = S.imp && impCur()
  if (!at || !c || !c.paras.length) return null
  const head = [...c.paras.slice(0, at.para), c.paras[at.para].slice(0, at.off)].filter((p) => p.trim())
  const rest = [c.paras[at.para].slice(at.off), ...c.paras.slice(at.para + 1)].filter((p) => p.trim())
  return head.length && rest.length ? [head, rest] : null
}

// 从光标处另起一章：光标后面的字成为新的一章，排在这一章后面。它的第一行短、不以句号结尾，就当标题，否则叫「未命名」。
function importSplit() {
  const parts = impParts()
  if (!parts) return
  const [head, rest] = parts, c = impCur(), first = rest[0].trim()
  const titled = first.length <= 20 && !/[。！？…”」]$/.test(first), title = titled ? first : L.commonUntitled(), nid = 'i' + S.imp.n
  importChange((imp) => ({ ...imp, n: imp.n + 1, sel: nid, can: false, chapters: imp.chapters.flatMap((x) => (x.id === c.id ? [{ ...x, paras: head }, { id: nid, title, vol: c.vol, paras: titled ? rest.slice(1) : rest }] : [x])) }), L.shelfSplitDone(title))
  putCaret({ para: 0, off: 0 }, false, impBox())
}

// 光标挪到哪儿，「从光标处另起一章」跟着能点或灰着。
document.addEventListener('selectionchange', () => {
  if (!impOpen() || composing) return
  const can = !!impParts()
  if (can !== S.imp.can) impPatch({ can })
})

function impParas(id, paras) {
  impRetire()
  set((s) => ({ imp: { ...s.imp, chapters: s.imp.chapters.map((c) => (c.id === id ? { ...c, paras } : c)) } }))
}
// 浏览器替作者改了正文上的字：读回来。拼音还没选完字时先不读。
function importWrote() {
  const box = impBox()
  if (!box || composing) return
  const at = spanNow(box), read = blocks(box).map((e) => textOf(e).replace(/\n/g, ''))
  const paras = read.length ? read : [box.textContent]
  impParas(S.imp.sel, paras.length === 1 && !paras[0] ? [] : paras)
  if (at) putCaret(at[1], false, box)
}
// 把选中的换成 text，没选中就放在光标那里；text 里的换行另起一段。
function impPut(text) {
  const box = impBox(), span = box && spanNow(box), c = impCur()
  if (!span) return
  const [a, b] = span, old = c.paras.length ? c.paras : [''], lines = text.split('\n')
  const paras = [...old.slice(0, a.para), ...((old[a.para] || '').slice(0, a.off) + text + (old[b.para] || '').slice(b.off)).split('\n'), ...old.slice(b.para + 1)]
  impParas(c.id, paras.length === 1 && !paras[0] ? [] : paras)
  putCaret({ para: a.para + lines.length - 1, off: (lines.length > 1 ? 0 : a.off) + lines.at(-1).length }, true, box)
}
const importKey = (ev) => { if (ev.key === 'Enter' && !ev.isComposing) { ev.preventDefault(); impPut('\n') } }
const importPaste = (ev) => { ev.preventDefault(); impPut(plain(ev.clipboardData.getData('text/plain'))) }

// ───────── 导入书籍：树里的操作，和正文页侧栏一样 ─────────

const impIsVol = (key) => S.imp.volumes.some((v) => v[0] === key)
const impTitleOf = (key) => { const v = S.imp.volumes.find((x) => x[0] === key); return v ? v[1] : S.imp.chapters.find((c) => c.id === key).title }

// 新建章节：开着一章就建在它后面；在一卷上右键建的（into 是那一卷），就建在这一卷的末尾。名字全选上等着换。
function impNewChapter(into) {
  impRetire()
  set((s) => {
    const imp = s.imp, cur = imp.chapters.find((c) => c.id === imp.sel), home = typeof into === 'string' ? into : cur ? cur.vol : ''
    const anchor = typeof into === 'string' ? imp.chapters.filter((c) => c.vol === home).at(-1) : cur, id = 'i' + imp.n
    const at = anchor ? imp.chapters.indexOf(anchor) + 1 : imp.chapters.length
    const name = L.shelfNewChapterName()
    const chapters = [...imp.chapters.slice(0, at), { id, title: name, vol: home, paras: [] }, ...imp.chapters.slice(at)]
    return { imp: { ...imp, chapters, n: imp.n + 1, sel: id, can: false, menu: null, folds: home ? { ...imp.folds, [home]: false } : imp.folds, rename: { key: id, t: name } } }
  })
}
function impNewFolder() {
  impRetire()
  const name = L.shelfNewFolderName()
  set((s) => ({ imp: { ...s.imp, volumes: [...s.imp.volumes, ['v' + s.imp.n, name]], n: s.imp.n + 1, menu: null, rename: { key: 'v' + s.imp.n, t: name } } }))
}
// 名字定下来：按了回车，或者光标离开了输入框。空着就还是原来的。
function impRenameNow() {
  const r = S.imp && S.imp.rename
  if (r && r.t.trim() && r.t.trim() !== impTitleOf(r.key)) impRetire()
  set((s) => {
    if (!s.imp || !s.imp.rename) return null
    const { key, t: title } = s.imp.rename, name = title.trim()
    if (!name) return { imp: { ...s.imp, rename: null } }
    return { imp: { ...s.imp, rename: null, chapters: s.imp.chapters.map((c) => (c.id === key ? { ...c, title: name } : c)), volumes: s.imp.volumes.map((v) => (v[0] === key ? [key, name] : v)) } }
  })
}
// 上移、下移：和同一层里前一个、后一个换位置。
function impShift(key, d) {
  impRetire()
  set((s) => {
    const imp = s.imp
    const swap = (list, same, at) => {
      const peers = list.filter(same), other = peers[peers.indexOf(list[at]) + d]
      if (!other) return list
      const out = [...list], j = list.indexOf(other)
      out[at] = other; out[j] = list[at]
      return out
    }
    const v = imp.volumes.findIndex((x) => x[0] === key), c = imp.chapters.findIndex((x) => x.id === key)
    return { imp: { ...imp, menu: null, volumes: v >= 0 ? swap(imp.volumes, () => true, v) : imp.volumes, chapters: c >= 0 ? swap(imp.chapters, (x) => x.vol === imp.chapters[c].vol, c) : imp.chapters } }
  })
}
function impMoveInto(key, vol) {
  impRetire()
  set((s) => {
    const c = s.imp.chapters.find((x) => x.id === key)
    return { imp: { ...s.imp, menu: null, chapters: [...s.imp.chapters.filter((x) => x !== c), { ...c, vol }], folds: vol ? { ...s.imp.folds, [vol]: false } : s.imp.folds } }
  })
}
// 拖一行：放到一卷上就进这一卷的末尾，放到一章上就排到它前面。卷不能放进卷里。
function impDrop(key, target) {
  impRetire()
  set((s) => {
    const imp = s.imp, c = imp.chapters.find((x) => x.id === key), into = imp.volumes.some((v) => v[0] === target), before = imp.chapters.find((x) => x.id === target)
    if (!c || c === before) return null
    const rest = imp.chapters.filter((x) => x !== c)
    if (into) return { imp: { ...imp, chapters: [...rest, { ...c, vol: target }], folds: { ...imp.folds, [target]: false } } }
    if (!before) return null
    const at = rest.indexOf(before)
    return { imp: { ...imp, chapters: [...rest.slice(0, at), { ...c, vol: before.vol }, ...rest.slice(at)] } }
  })
}
// 删除：一卷连同里面的章一起走。正看着的那一章没了，就换到挨着它的一章。
function impDelete(key) {
  const imp = S.imp, vol = imp.volumes.find((v) => v[0] === key), gone = imp.chapters.filter((c) => (vol ? c.vol === key : c.id === key)), title = impTitleOf(key)
  importChange((i) => {
    const chapters = i.chapters.filter((c) => !gone.includes(c))
    const sel = chapters.some((c) => c.id === i.sel) ? i.sel : (chapters[Math.min(i.chapters.indexOf(gone[0]), chapters.length - 1)] || { id: '' }).id
    return { ...i, chapters, sel, can: false, volumes: i.volumes.filter((v) => v !== vol) }
  }, L.shelfDeleted(title))
}

let impDragged = ''
const impNodeAt = (ev) => { const row = ev.target.closest('[data-imp-node]'); return row && impDragged && row.dataset.impNode !== impDragged ? row : null }
root.addEventListener('dragstart', (ev) => { const row = ev.target.closest('[data-imp-node]'); if (row) { impDragged = row.dataset.impNode; ev.dataTransfer.effectAllowed = 'move' } })
root.addEventListener('dragover', (ev) => { const row = impNodeAt(ev); if (row) { ev.preventDefault(); row.classList.add('over') } })
root.addEventListener('dragleave', (ev) => { const row = ev.target.closest('[data-imp-node]'); if (row) row.classList.remove('over') })
root.addEventListener('drop', (ev) => { const row = impNodeAt(ev); if (row) { ev.preventDefault(); impDrop(impDragged, row.dataset.impNode) } })
root.addEventListener('dragend', () => { impDragged = '' })

// 右键点出来的菜单，和侧栏里的一样（「移到」也是下一级）；浮在预览上面。
function importMenu() {
  const m = S.imp.menu, key = m.key, vol = impIsVol(key), c = S.imp.chapters.find((x) => x.id === key), win = document.getElementById('win')
  const spots = vol ? [] : [[L.shelfTopLevel(), ''], ...S.imp.volumes.map((v) => [v[1], v[0]])].filter(([, to]) => to !== c.vol)
  const rows = [[L.commonRename(), () => impPatch({ menu: null, rename: { key, t: impTitleOf(key) } })], [L.cmdNewChapter(), () => impNewChapter(vol ? key : undefined)], [L.cmdNewFolder(), impNewFolder]]
  return `<div class="veil float-veil top"${onClick(impShut)}${onMenu((ev) => { ev.preventDefault(); impShut() })}></div>` +
    `<div class="ctx pop top${m.x > win.offsetWidth - 208 - 208 ? ' flip' : ''}" data-k="imp-menu"${subOver(!!m.into, (on) => impPatch({ menu: { ...S.imp.menu, into: on } }))} style="left:${Math.min(m.x, win.offsetWidth - 208)}px;top:${Math.min(m.y, win.offsetHeight - Math.max(250, 148 + Math.min(spots.length, 10) * 30))}px">` +
    rows.map(([label, act]) => menuRow(label, act)).join('') + HAIR +
    menuRow(L.commonMoveUp(), () => impShift(key, -1)) + menuRow(L.commonMoveDown(), () => impShift(key, 1)) +
    (spots.length ? subRow(L.shelfMoveTo(), m.into, () => impPatch({ menu: { ...S.imp.menu, into: true } }), 'imp-into', spots.map(([label, to]) => menuRow(label, () => impMoveInto(key, to))).join('')) : '') + HAIR +
    menuRow(L.commonDelete(), () => impDelete(key)) + '</div>'
}

// 第一个对话框：作品叫什么、什么体裁、放在哪，和「新建作品」同样的字。名字空着、文件夹里已有文件时「下一步」是灰的。
function importFormDialog() {
  const imp = S.imp, w = impWhere(), type = (key) => (v) => set((s) => ({ imp: { ...s.imp, [key]: v } }))
  const folder = w.at.split(/[\\/]/).filter(Boolean).at(-1) || w.at
  const where = !w.at ? `<div class="note">${t(L.shelfLocationNeeded())}</div>` : w.taken ? `<div class="note danger">${t(L.shelfFolderTaken(folder))}</div>` : `<div class="note">${t(L.shelfWillCreate(folder))}</div>`
  const next = () => { if (w.ok) set({ dialog: { kind: 'importPreview' }, ...SHUT }) }
  return dialog('small', dialogHead(L.shelfImportBook(), L.shelfImportAbout()) +
    `<div class="field"><div class="label">${t(L.shelfFile())}</div><div class="pick-line"><div class="box line fit"><div class="fit ell">${t(BOOK.source.file)}</div></div>${button(L.shelfChangeFile(), osAsk(L.shelfPickImport()))}</div>` +
    `<div class="note">${t(L.shelfDetected(imp.volumes.length, imp.chapters.length))}</div></div>` +
    field(L.shelfBookName(), line(imp.name, type('name'), '', L.shelfBookName(), keys(next) + ' autofocus')) + (w.name ? '' : `<div class="note">${t(L.shelfNameNeeded())}</div>`) +
    field(L.shelfGenre(), choose('imp-kind', imp.type, BOOK.kinds.map((k) => [k, genreWord(k)]), type('type'), L.shelfGenre())) +
    `<div class="field"><div class="label">${t(L.shelfLocation())}</div><div class="pick-line"><div class="box line fit">${typed(w.at, type('at'), '', L.shelfBookLocation(), keys(next))}</div>${button(L.shelfChangePlace(), osAsk(L.shelfPickNewLocation()))}</div>${where}</div>` +
    buttons(button(L.commonCancel(), () => set({ dialog: null, imp: null, ...SHUT })), button(L.commonNext(), w.ok ? next : null, w.ok ? 'primary' : 'primary off')))
}

// 第二个对话框：预览。左边是切出来的卷和章，和正文页的侧栏一样：卷能折起，字数指针移上去才出来，右键、拖动、改名都和那里一样；
// 右边是选中的那一章，标题和正文都能改，标题右边是补救切分的两样；按下它们不拿走正文的光标（data-keep）。「确定」新建作品并打开它。
function importPreviewDialog() {
  const imp = S.imp, w = impWhere(), cur = impCur(), chapters = imp.chapters
  const total = chapters.reduce((n, c) => n + impWords(c), 0), merge = impMergeable()
  const menu = (key) => onMenu((ev) => { ev.preventDefault(); const [x, y] = spot(ev.clientX, ev.clientY); impPatch({ menu: { key, x, y } }) })
  // 改名字的那一行还是那一行：图标、缩进都不动，只是名字换成输入框。回车和光标离开都算定了，Esc 不改。
  const naming = (key, pic, deep) => `<div class="tree-row naming${pic ? '' : ' leaf'}${deep ? ' deep' : ''}${key === imp.sel ? ' on' : ''}" data-k="imp-${key}">${pic}` +
    `${typed(imp.rename.t, (v) => set((s) => (s.imp && s.imp.rename ? { imp: { ...s.imp, rename: { ...s.imp.rename, t: v } } } : null)), '', L.commonRename(), keys(impRenameNow, () => impPatch({ rename: null })) + onBlur(impRenameNow) + ' autofocus data-all')}</div>`
  const leaf = (c) => (imp.rename && imp.rename.key === c.id ? naming(c.id, '', c.vol)
    : `<div class="tree-row leaf hit${c.vol ? ' deep' : ''}${c.id === imp.sel ? ' on' : ''}" draggable="true" data-k="imp-${c.id}" data-imp-node="${c.id}"${onClick(() => impShow(c.id))}${menu(c.id)}><div class="fit ell">${t(c.title.trim() || L.commonUntitled())}</div><div class="count">${t(L.commonNumber(impWords(c)))}</div></div>`)
  const volume = ([key, title]) => {
    const folded = !!imp.folds[key], pic = icon(folded ? 'right' : 'down', 'xs'), mine = chapters.filter((c) => c.vol === key)
    return (imp.rename && imp.rename.key === key ? naming(key, pic)
      : `<div class="tree-row hit" data-k="imp-${key}" data-imp-node="${key}"${onClick(() => impPatch({ folds: { ...imp.folds, [key]: !folded } }))}${menu(key)}>${pic}<div class="fit ell">${t(title)}</div><div class="count">${t(L.commonNumber(mine.reduce((n, c) => n + impWords(c), 0)))}</div></div>`) +
      (folded ? '' : mine.map(leaf).join(''))
  }
  const tree = imp.volumes.map(volume).join('') + chapters.filter((c) => !c.vol).map(leaf).join('')
  const add = `<div class="import-tree-head"><div class="faint fit">${t(L.shelfVolumesChapters())}</div><button class="icon-btn muted" aria-label="${t(L.cmdNewChapter())}" title="${t(L.cmdNewChapter())}"${onClick(() => impNewChapter())}>${ICON.plus}</button></div>`
  const acts = '<div class="import-acts" data-keep>' +
    `<div class="hit link${merge ? '' : ' off'}" title="${t(merge ? L.shelfMergeHint() : L.shelfMergeNone())}"${onClick(merge ? importMerge : null)}>${t(L.shelfMerge())}</div>` +
    `<div class="hit link${imp.can ? '' : ' off'}" title="${t(imp.can ? L.shelfSplitHint() : L.shelfSplitNeedCursor())}"${onClick(imp.can ? importSplit : null)}>${t(L.shelfSplit())}</div></div>`
  const toProse = () => putCaret({ para: 0, off: 0 }, false, impBox())
  const side = !cur ? '<div class="snap-side">' + empty(L.shelfNoChapters(), L.shelfNoChaptersAbout()) + '</div>' : '<div class="snap-side">' +
    `<div class="import-head"><input class="in chapter-title import-title" value="${t(cur.title)}" placeholder="${t(L.commonUntitled())}" aria-label="${t(L.shelfChapterTitle())}" spellcheck="false"${onInput((v) => { impRetire(); set((s) => ({ imp: { ...s.imp, chapters: s.imp.chapters.map((c) => (c.id === cur.id ? { ...c, title: v } : c)) } })) })}${keys(toProse)}>${acts}</div><div class="note">${t(L.shelfSplitNote())}</div>` +
    `<div class="snap-text imp-text scroll"><div class="import-prose" contenteditable="plaintext-only" spellcheck="false" aria-label="${t(L.shelfTextLabel())}"${onInput(importWrote)}${onKey(importKey)}${onPaste(importPaste)}>${(cur.paras.length ? cur.paras : ['']).map((p, i) => `<div data-para="${i}">${p ? t(p) : '<br>'}</div>`).join('')}</div></div>` +
    `<div class="faint">${t(L.shelfChapterWords(impWords(cur), 'character'))}</div></div>`
  const ok = !!chapters.length
  const go = () => {
    const id = 'book' + S.n, words = total
    set((s) => ({ shelf: [{ id, name: w.name, kind: imp.type, words, at: stamp() }, ...s.shelf], n: s.n + 1, dialog: null, imp: null }))
    openBook(id)
    say(L.shelfImported(chapters.length), w.name, 'ok')
  }
  return dialog('preview', dialogHead(L.shelfPreviewTitle(BOOK.source.file), L.shelfPreviewAbout(imp.volumes.length, imp.chapters.length, total, 'character')) +
    '<div class="snaps">' +
    `<div class="import-tree">${add}<div class="tree scroll">${tree}</div></div>` + side + '</div>' +
    buttons(button(L.commonCancel(), () => set({ dialog: null, imp: null, ...SHUT })), button(L.commonBack(), () => set({ dialog: { kind: 'importForm' }, ...SHUT })), button(L.commonOK(), ok ? go : null, ok ? 'primary' : 'primary off'))) +
    (imp.menu ? importMenu() : '')
}
