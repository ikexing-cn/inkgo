// ───────── 扩展模块：宿主的那部分 ─────────
// 有哪些扩展、各自需要谁，开着没有，拨开关时连带开、关哪些；设置里的「扩展」总览、每个扩展的页头和它自己的页。
// 做法见 .agents/notes/proposed/architecture/2026-11-02-extension-modules.md。

// 一行一个扩展，照笔记里那张表写；两边对不上，有一边错了。顺序就是「扩展」总览里的顺序。
// g：在总览的哪一组；lib 那一组是基础扩展，给别的扩展用，自己没有界面。
// req：必需依赖，连带开关。opt：可选依赖，开着就用，不连带。off：关着时多说的一句。
// page：扩展自己登记进设置左栏的一页 [页名, 顺序]；几个扩展登记同一个页名就合用一页。不登记的，设置只在「扩展」里它自己的页。
const EXTS = () => [
  { id: 'focus', g: 'write', name: L.modFocus(), about: L.modFocusAbout() },
  { id: 'goal', g: 'write', name: L.modGoal(), about: L.modGoalAbout() },
  { id: 'names', g: 'write', name: L.modNames(), about: L.modNamesAbout(), req: ['lore'] },
  { id: 'find', g: 'write', name: L.modFind(), about: L.modFindAbout() },
  { id: 'notes', g: 'write', name: L.modNotes(), about: L.modNotesAbout(), req: ['marks'] },
  { id: 'lore', g: 'lore', name: L.modLore(), about: L.modLoreAbout(), req: ['context'], opt: ['marks'] },
  { id: 'outline', g: 'lore', name: L.modOutline(), about: L.modOutlineAbout(), req: ['context'] },
  { id: 'hooks', g: 'lore', name: L.modHooks(), about: L.modHooksAbout(), req: ['context', 'marks'], page: ['提醒', 20] },
  { id: 'talk', g: 'ai', name: L.modTalk(), about: L.modTalkAbout(), req: ['ai', 'context'] },
  { id: 'continue', g: 'ai', name: L.modContinue(), about: L.modContinueAbout(), req: ['ai', 'context'] },
  { id: 'draft', g: 'ai', name: L.modDraft(), about: L.modDraftAbout(), req: ['ai', 'outline'] },
  { id: 'propose', g: 'ai', name: L.modPropose(), about: L.modProposeAbout(), req: ['ai'], opt: ['marks'] },
  { id: 'check', g: 'ai', name: L.modCheck(), about: L.modCheckAbout(), req: ['ai', 'lore', 'marks'], page: ['提醒', 20] },
  { id: 'usage', g: 'ai', name: L.modUsage(), about: L.modUsageAbout(), req: ['ai'] },
  { id: 'snaps', g: 'files', name: L.modSnaps(), about: L.modSnapsAbout() },
  { id: 'trash', g: 'files', name: L.modTrash(), about: L.modTrashAbout() },
  { id: 'cats', g: 'files', name: L.modCats(), about: L.modCatsAbout() },
  { id: 'backup', g: 'files', name: L.modBackup(), about: L.modBackupAbout(), off: L.modBackupOff(), page: ['备份', 30] },
  { id: 'export', g: 'files', name: L.modExport(), about: L.modExportAbout(), opt: ['outline', 'lore'] },
  { id: 'phone', g: 'files', name: L.modPhone(), about: L.modPhoneAbout() },
  // 原型里假装另装的一个，看另装的扩展在设置里怎么出现；它什么也不做。
  { id: 'fullwidth', g: 'more', name: L.modFullwidth(), about: L.modFullwidthAbout() },
  { id: 'context', g: 'lib', name: L.modContext(), about: L.modContextAbout() },
  { id: 'marks', g: 'lib', name: L.modMarks(), about: L.modMarksAbout() },
  { id: 'ai', g: 'lib', name: L.modAI(), about: L.modAIAbout(), page: ['AI', 10] },
]
const EXT_GROUPS = () => [['write', L.modGroupWrite()], ['lore', L.modGroupLore()], ['ai', L.modGroupAI()], ['files', L.modGroupFiles()], ['more', L.modGroupInstalled()], ['lib', L.modGroupLibraries()]]
const extOf = (id) => EXTS().find((e) => e.id === id)
const extNames = (list) => list.map((e) => e.name).join(L.modListSep())
const isLib = (e) => e.g === 'lib'

// 开关先看这部作品单独设的（S.extBook，键是「作品 id:扩展 id」），没有就看本机的：
// AI 的是「关闭 AI」那一个（S.noAI），别的关掉的记在 S.extOff。
// 基础扩展不能单独设；AI 也是基础扩展，它管本机的所有作品（DESIGN.md「AI 可以整个关掉」）。
const inBook = () => S.page !== 'shelf' && !!S.book
const bookKey = (id) => S.book + ':' + id
const extMine = (id) => inBook() && !isLib(extOf(id)) && bookKey(id) in S.extBook
const extOwn = (id) => (extMine(id) ? S.extBook[bookKey(id)] : id === 'ai' ? !S.noAI : !S.extOff.includes(id))
// 能用：自己开着，必需的也都能用。自己的开关和能不能用分开记，必需的那个再打开时，它回到自己原来的开关。
const extOn = (id) => extOwn(id) && (extOf(id).req || []).every(extOn)
const aiOn = () => extOn('ai')

// 直接必需 id 的扩展。
const extNeededBy = (id) => EXTS().filter((e) => (e.req || []).includes(id))
// 关掉 id 时一起停用的：现在能用、直接或隔几层必需它的。
function extStops(id) {
  const out = []
  const walk = (x) => extNeededBy(x).forEach((e) => { if (!out.includes(e) && extOn(e.id)) { out.push(e); walk(e.id) } })
  walk(id)
  return out
}
// 打开 id 时一起打开的：它直接或隔几层必需、自己的开关关着的。
function extStarts(id) {
  const out = []
  const walk = (x) => (extOf(x).req || []).forEach((r) => { if (!extOwn(r) && !out.includes(extOf(r))) out.push(extOf(r)); walk(r) })
  walk(id)
  return out
}

// 拨页头的开关。会连带别的扩展的先问一句；AI 开、关都问（DESIGN.md「AI 可以整个关掉」）。别的点了马上生效：什么都不丢。
function extFlip(id) {
  const on = !extOwn(id)
  if (id === 'ai' || (on ? extStarts(id) : extStops(id)).length) return set((s) => ({ dialog: { ...s.dialog, ask: on ? 'ext-on' : 'ext-off', ext: s.dialog.ext, flip: id }, ...SHUT }))
  extSet(id, on)
}
// 标题栏正中的几页各归哪个扩展；正文那一页归核心。
const PAGE_EXT = { talk: 'talk', plan: 'outline', check: 'check' }
// 停在已经关掉的扩展那一页的，回到正文。
const leaveOffPage = () => { if (PAGE_EXT[S.page] && !extOn(PAGE_EXT[S.page])) set({ page: 'write' }) }

// AI 的开关经 turnAI：停下正在做的 AI 任务，离开讨论、检查两页。
// 别的：这部作品单独设了的改它自己的，没有的改本机的。
function extSet(id, on) {
  if (id === 'ai') return turnAI(on)
  if (extMine(id)) set((s) => ({ extBook: { ...s.extBook, [bookKey(id)]: on } }))
  else set((s) => ({ extOff: on ? s.extOff.filter((x) => x !== id) : s.extOff.includes(id) ? s.extOff : [...s.extOff, id] }))
  leaveOffPage()
}
// 这部作品单独记一份开关（照本机的抄），或丢掉这一份、回到本机的开关；「恢复默认」不问。
function extMineSet(id, mine) {
  set((s) => {
    const book = { ...s.extBook }
    if (mine) book[bookKey(id)] = extOwn(id)
    else delete book[bookKey(id)]
    return { extBook: book }
  })
  leaveOffPage()
}

// 页头「⋯」里的「只在本作品关闭 / 打开」（VS Code 的 Disable (Workspace)）：这部作品单独记一份，和本机的相反。
// 连带的照样先问，d.book 记着问完只改这部作品。
function extFlipBook(id) {
  const on = !extOwn(id)
  if ((on ? extStarts(id) : extStops(id)).length) return set((s) => ({ dialog: { ...s.dialog, ask: on ? 'ext-on' : 'ext-off', flip: id, book: true }, ...SHUT }))
  set(SHUT); extMineSet(id, true); extSet(id, on)
}

// 先问的那一句，浮在设置上面，遮罩只盖一层。
function askExt(d) {
  const e = extOf(d.flip), on = d.ask === 'ext-on', back = () => set((s) => ({ dialog: { ...s.dialog, ask: '', book: false } }))
  const also = on ? extStarts(d.flip) : extStops(d.flip)
  const said = also.length ? (on ? L.modAskOnAbout(extNames(also)) : L.modAskOffAbout(extNames(also))) : ''
  // 只在本作品打开时，一起打开的基础扩展没有按作品的开关，开的是本机的：说一句。
  const libs = on && d.book ? also.filter(isLib) : []
  const ai = d.flip === 'ai'
  const title = ai ? (on ? L.settingsAskOnTitle() : L.settingsAskOffTitle()) : on ? L.modAskOnTitle(e.name) : L.modAskOffTitle(e.name)
  const about = ai ? (on ? L.settingsAskOnAbout() : L.settingsAskOffAbout()) : ''
  const go = () => {
    back()
    const one = (id) => { if (d.book) extMineSet(id, true); extSet(id, on) }
    if (on) also.forEach((x) => one(x.id))
    one(d.flip)
  }
  return `<div class="scrim top"${onClick(back)}></div><div class="dialog-wrap top"><div class="dialog ask" data-k="dialog-ask">` +
    dialogHead(title, about) + (said ? `<div class="note">${t(said)}</div>` : '') + (libs.length ? `<div class="note">${t(L.modAskLibraryLocal(extNames(libs)))}</div>` : '') +
    buttons(button(L.commonCancel(), back), button(ai ? (on ? L.settingsTurnOn() : L.settingsCloseAI()) : on ? L.modTurnOn() : L.modTurnOff(), go, 'primary')) + '</div></div>'
}

// 扩展的页头，每个扩展自己的页、设置左栏里扩展带来的页，每段顶上都是它：
// 开关和名字；一句说它做什么；灰的一行是需要谁、谁要用它。
// 开关管本机的所有作品，平时不写范围；这部作品单独设了，灰的那行才多一句。单独设在右上角「⋯」里，书架上和基础扩展没有。
// 自己开着、必需的关着：勾还在，淡下去，下面琥珀色一句说是谁关着。
function extHead(e) {
  const own = extOwn(e.id), waits = own ? (e.req || []).filter((r) => !extOn(r)).map(extOf) : [], by = extNeededBy(e.id), mine = extMine(e.id)
  const meta = [
    e.req ? L.modNeeds(extNames(e.req.map(extOf))) : '',
    e.opt ? L.modOptional(extNames(e.opt.map(extOf))) : '',
    by.length ? L.modNeededBy(extNames(by)) : isLib(e) ? L.modNobodyNeeds() : '',
    mine ? (own ? L.modBookOn() : L.modBookOff()) : '',
  ].filter(Boolean).map(t)
  const key = 'ext-more-' + e.id, open = S.open === key
  const more = inBook() && !isLib(e) ? `<div class="anchor ext-more"><button class="icon-btn muted" aria-label="${t(L.commonMoreActions())}" title="${t(L.commonMoreActions())}"${onClick(() => set({ open: open ? '' : key }))}>${ICON.dots}</button>` +
    (open ? '<div class="pop">' + (mine ? menuRow(L.modFollowLocal(), () => { set(SHUT); extMineSet(e.id, false) }) : menuRow(own ? L.modBookOnlyOff() : L.modBookOnlyOn(), () => extFlipBook(e.id))) + '</div>' : '') + '</div>' : ''
  return `<div class="ext-head${waits.length ? ' ext-wait' : ''}" data-ext="${e.id}">${tickbox(own, e.name, () => extFlip(e.id))}${more}` +
    `<div class="note tick-note">${t(e.about)}${e.off && !own ? '<br>' + t(e.off) : ''}</div>` +
    (meta.length ? `<div class="note tick-note faint">${meta.join(' · ')}</div>` : '') +
    (waits.length ? `<div class="note tick-note warn">${t(L.modWaiting(extNames(waits)))}</div>` : '') + '</div>'
}

// 扩展自己的设置，接在页头下面；扩展关着时不在。要有打开的作品才有的，书架上不在。
const EXT_PREFS = {
  ai: (d) => settingsModel(d),
  hooks: () => lateRows(),
  check: () => `<div class="prefs-sub">${t(L.settingsCheckText())}</div>${radios(S.when, WHENS(), (v) => set({ when: v }))}`,
  names: () => `<div class="ticks">${tickbox(S.names, L.settingsMarkNames(), () => set({ names: !S.names }))}</div>`,
  backup: () => settingsBackup(),
  fullwidth: () => `<div class="ticks">${tickbox(S.quotes, L.modFullwidthQuotes(), () => set({ quotes: !S.quotes }))}</div>`,
}
const BOOK_PREFS = ['hooks', 'check', 'backup']
const extPrefs = (id, d) => (EXT_PREFS[id] && extOn(id) && (inBook() || !BOOK_PREFS.includes(id)) ? `<div class="ext-prefs">${EXT_PREFS[id](d)}</div>` : '')
const extSection = (id, d) => `<div class="ext-sec">${extHead(extOf(id))}${extPrefs(id, d)}</div>`

// 设置左栏外观、扩展下面，是扩展登记的页（EXTS 的 page），照登记的顺序排：[页名, 哪几个扩展各一段]。
// 登记了就一直在，扩展关着也在：页头的开关能把它再打开。
const prefPages = () => {
  const out = []
  EXTS().filter((e) => e.page).sort((a, b) => a.page[1] - b.page[1]).forEach((e) => {
    const p = out.find(([n]) => n === e.page[0])
    if (p) p[1].push(e.id)
    else out.push([e.page[0], [e.id]])
  })
  return out
}
const prefPage = (name) => prefPages().find(([n]) => n === name)

// 「扩展」总览：每组两列，一格一个，点一格进它自己的页。关着的、必需的关着而用不了的，右边写「已关闭」。
const extCell = (e) => `<div class="ext-cell hit${extOn(e.id) ? '' : ' off'}" data-ext="${e.id}"${onClick(() => set((s) => ({ dialog: { ...s.dialog, ext: e.id }, ...SHUT })))}>` +
  `<div class="fit ell">${t(e.name)}</div>${extOn(e.id) ? '' : `<div class="note faint">${t(L.modOff())}</div>`}${icon('right', 'xs')}</div>`
const extOverview = () => EXT_GROUPS().map(([g, title]) => {
  const list = EXTS().filter((e) => e.g === g)
  return `<div class="prefs-group"><div class="group-title">${t(title)}</div>${g === 'lib' ? `<div class="note">${t(L.modLibrariesAbout())}</div>` : ''}</div>` +
    `<div class="ext-grid">${list.map(extCell).join('')}${list.length % 2 ? '<div class="ext-cell blank"></div>' : ''}</div>`
}).join('')
