// ───────── 扩展模块：宿主的那部分 ─────────
// 有哪些扩展、各自需要谁，开着没有，拨开关时连带开、关哪些，设置里的「功能」页。
// 做法见 .agents/notes/proposed/architecture/2026-11-02-extension-modules.md。

// 一行一个扩展，照笔记里那张表写；两边对不上，有一边错了。
// lib：前置扩展，给别的扩展用，自己没有界面。req：必需依赖，连带开关。opt：可选依赖，开着就用，不连带。
// off：关掉以后会怎样，多说的一句。
const EXTS = () => [
  { id: 'context', lib: true, name: L.modContext(), where: L.modContextWhere() },
  { id: 'marks', lib: true, name: L.modMarks(), where: L.modMarksWhere() },
  { id: 'ai', lib: true, name: L.modAI(), where: L.modAIWhere() },
  { id: 'lore', name: L.modLore(), where: L.modLoreWhere(), req: ['context'] },
  { id: 'outline', name: L.modOutline(), where: L.modOutlineWhere(), req: ['context'] },
  { id: 'usage', name: L.modUsage(), where: L.modUsageWhere(), req: ['ai'] },
  { id: 'talk', name: L.modTalk(), where: L.modTalkWhere(), req: ['ai', 'context'] },
  { id: 'continue', name: L.modContinue(), where: L.modContinueWhere(), req: ['ai', 'context'] },
  { id: 'draft', name: L.modDraft(), where: L.modDraftWhere(), req: ['ai', 'context'] },
  { id: 'propose', name: L.modPropose(), where: L.modProposeWhere(), req: ['ai'] },
  { id: 'check', name: L.modCheck(), where: L.modCheckWhere(), req: ['ai', 'context', 'marks'] },
  { id: 'hooks', name: L.modHooks(), where: L.modHooksWhere(), req: ['context', 'marks'] },
  { id: 'notes', name: L.modNotes(), where: L.modNotesWhere(), req: ['marks'] },
  { id: 'names', name: L.modNames(), where: L.modNamesWhere(), req: ['context'] },
  { id: 'phone', name: L.modPhone(), where: L.modPhoneWhere() },
  { id: 'export', name: L.modExport(), where: L.modExportWhere(), opt: ['outline', 'lore'] },
  { id: 'backup', name: L.modBackup(), where: L.modBackupWhere(), off: L.modBackupOff() },
  { id: 'goal', name: L.modGoal(), where: L.modGoalWhere() },
  { id: 'focus', name: L.modFocus(), where: L.modFocusWhere() },
  { id: 'cats', name: L.modCats(), where: L.modCatsWhere() },
  { id: 'snaps', name: L.modSnaps(), where: L.modSnapsWhere() },
  { id: 'trash', name: L.modTrash(), where: L.modTrashWhere() },
  { id: 'find', name: L.modFind(), where: L.modFindWhere() },
]
const extOf = (id) => EXTS().find((e) => e.id === id)
const extNames = (list) => list.map((e) => e.name).join(L.modListSep())

// 它自己的开关，本机的：AI 的就是「关闭 AI」那一个（S.noAI），别的关掉的记在 S.extOff。
const extOwn = (id) => (id === 'ai' ? !S.noAI : !S.extOff.includes(id))
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

// 拨「功能」页的开关。会连带别的扩展的先问一句；AI 开、关都问（DESIGN.md「AI 可以整个关掉」）。别的点了马上生效：什么都不丢。
function extFlip(id) {
  const on = !extOwn(id)
  if (id === 'ai' || (on ? extStarts(id) : extStops(id)).length) return set((s) => ({ dialog: { ...s.dialog, ask: on ? 'ext-on' : 'ext-off', ext: id }, ...SHUT }))
  extSet(id, on)
}
// 标题栏正中的几页各归哪个扩展；正文那一页归核心。
const PAGE_EXT = { talk: 'talk', plan: 'outline', check: 'check' }

// AI 的开关经 turnAI：停下正在做的 AI 任务，离开讨论、检查两页。
// 别的关掉以后，停在它（或连带停用的）那一页的回到正文。
function extSet(id, on) {
  if (id === 'ai') return turnAI(on)
  set((s) => ({ extOff: on ? s.extOff.filter((x) => x !== id) : s.extOff.includes(id) ? s.extOff : [...s.extOff, id] }))
  if (PAGE_EXT[S.page] && !extOn(PAGE_EXT[S.page])) set({ page: 'write' })
}

// 先问的那一句，浮在设置上面，遮罩只盖一层。
function askExt(d) {
  const e = extOf(d.ext), on = d.ask === 'ext-on', back = () => set((s) => ({ dialog: { ...s.dialog, ask: '' } }))
  const also = on ? extStarts(d.ext) : extStops(d.ext)
  const said = also.length ? (on ? L.modAskOnAbout(extNames(also)) : L.modAskOffAbout(extNames(also))) : ''
  const ai = d.ext === 'ai'
  const title = ai ? (on ? L.settingsAskOnTitle() : L.settingsAskOffTitle()) : on ? L.modAskOnTitle(e.name) : L.modAskOffTitle(e.name)
  const about = ai ? (on ? L.settingsAskOnAbout() : L.settingsAskOffAbout()) : ''
  const go = () => {
    back()
    if (on) also.forEach((x) => extSet(x.id, true))
    extSet(d.ext, on)
  }
  return `<div class="scrim top"${onClick(back)}></div><div class="dialog-wrap top"><div class="dialog ask" data-k="dialog-ask">` +
    dialogHead(title, about) + (said ? `<div class="note">${t(said)}</div>` : '') +
    buttons(button(L.commonCancel(), back), button(ai ? (on ? L.settingsTurnOn() : L.settingsCloseAI()) : on ? L.modTurnOn() : L.modTurnOff(), go, 'primary')) + '</div></div>'
}

// 「功能」页的一行：开关和名字，下面灰字说它在哪儿出现、需要谁、可选用谁、被谁需要。
// 自己开着、必需的关着：勾还在，淡下去，下面一句说是谁关着。
function extRow(e) {
  const own = extOwn(e.id), waits = own ? (e.req || []).filter((r) => !extOn(r)).map(extOf) : []
  const by = extNeededBy(e.id)
  const ties = [
    e.req ? L.modNeeds(extNames(e.req.map(extOf))) : '',
    e.opt ? L.modOptional(extNames(e.opt.map(extOf))) : '',
    by.length ? L.modNeededBy(extNames(by)) : e.lib ? L.modNobodyNeeds() : '',
  ].filter(Boolean).join(' · ')
  return `<div class="ext-row${waits.length ? ' ext-wait' : ''}" data-ext="${e.id}">${tickbox(own, e.name, () => extFlip(e.id))}` +
    `<div class="note tick-note">${t(e.where)}${e.off ? '<br>' + t(e.off) : ''}</div>` +
    (ties ? `<div class="note tick-note faint">${t(ties)}</div>` : '') +
    (waits.length ? `<div class="note tick-note warn">${t(L.modWaiting(extNames(waits)))}</div>` : '') + '</div>'
}

// 设置里的「功能」：先是有界面的，后是前置。
const extPage = () => {
  const all = EXTS()
  return `<div class="ticks">${all.filter((e) => !e.lib).map(extRow).join('')}</div>` +
    `<div class="prefs-group"><div class="group-title">${t(L.modGroupLibraries())}</div><div class="note">${t(L.modLibrariesAbout())}</div></div>` +
    `<div class="ticks">${all.filter((e) => e.lib).map(extRow).join('')}</div>`
}
