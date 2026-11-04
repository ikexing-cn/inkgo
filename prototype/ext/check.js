// 扩展：check。界面、动作和登记都在这里。
'use strict'

// ───────── 检查 ─────────

const KINDS = ['全部', '不一致', '名字有误', '新出现']
// 检查的分类：状态里记的是上面那几个字，显示时换成当前语言的字。
const kindWord = (k) => ({ 全部: L.checkKindAll(), 不一致: L.checkKindMismatch(), 名字有误: L.checkKindName(), 新出现: L.checkKindNew() })[k] ?? k

// 一条处理完了：从列表里拿掉，接着看下一条。
const settle = (id) => (s) => {
  const of = s.checks.filter((c) => s.kind === '全部' || c.kind === s.kind), at = of.findIndex((c) => c.id === id)
  const next = of[at + 1] || of[at - 1]
  return { checks: s.checks.filter((c) => c.id !== id), cid: next ? next.id : null }
}
// 一条检查结果能怎么处理，最后一样是主要的：检查页的按钮和页边批注上的字是同一份。不改的点错了，在提示条上撤销。
function checkActs(c) {
  const ch = chapterOf(c.key), e = S.entries.find((x) => x.id === c.entry)
  const leave = (word) => () => { const was = { checks: S.checks, cid: S.cid }; set(settle(c.id)); const id = say(word, '', 'ok', [[L.cmdUndo(), () => { unsay(id); set(was) }]]) }
  const add = () => { set((s) => ({ ...settle(c.id)(s), entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: c.cat, names: [c.title], text: c.as, src: { key: c.key, quote: c.quote } }], n: s.n + 1, log: did(s, L.checkLogAdded(c.cat, c.as)) })); say(L.checkAddedSetting(), '', 'ok') }
  if (c.kind === '新出现') return [[L.checkIgnore(), leave(L.checkIgnored())], [L.checkAddToSettings(), add]]
  return [[L.checkNoChange(), leave(L.checkMarkedNoChange())], ...(c.kind === '不一致' && e ? [[L.checkEditSetting(), () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: editing(e) })]] : []),
    [L.checkEditText(), ch ? goTo(ch.key, { key: ch.key, quote: c.quote }) : null]]
}

function checkPage() {
  const of = S.checks.filter((c) => S.kind === '全部' || c.kind === S.kind)
  const shown = of.find((c) => c.id === S.cid) || of[0]
  const count = (k) => (k === '全部' ? S.checks.length : S.checks.filter((c) => c.kind === k).length)
  const about = (c) => {
    const ch = chapterOf(c.key), at = chTitle(c.key)
    if (c.kind === '不一致') { const e = S.entries.find((x) => x.id === c.entry); return L.checkMismatchAbout(at, e ? e.cat + ' · ' + (e.names[0] || clip(e.text, 8)) : L.checkDeleted()) }
    return c.kind === '名字有误' ? L.checkNameAbout(at, c.right) : c.also ? L.checkFirstSeenMore(c.cat, at, c.also.length + 1) : L.checkFirstSeen(c.cat, at)
  }
  const row = (c) => `<div class="check-row row${shown && c.id === shown.id ? ' on' : ''}"${onClick(() => set({ cid: c.id }))}><div class="fit"><div class="ell">${t(c.title)}</div><div class="note ell">${t(about(c))}</div></div><div class="side">${t(kindWord(c.kind))}</div></div>`
  // 「检查全书」：先数出该查几章，问一句再开始。
  const owed = due()
  const again = () => {
    if (!S.entries.some((e) => e.place === 'body')) say(L.checkNoSettings())
    else if (!owed.length) say(L.checkAllDone())
    else set({ open: S.open === 'checkAll' ? '' : 'checkAll' })
  }
  const ask = () => `<div class="pop ask-pop" data-k="ask-pop"><div class="pop-title">${t(L.checkOwedTitle(owed.length))}</div><div class="note">${t(L.checkOwedNote(owed.length))}</div>` +
    `<div class="pop-buttons">${button(L.commonCancel(), () => set(SHUT))}${button(L.checkStart(), () => { set(SHUT); checkNow(...owed.map((c) => c.key)) }, 'primary')}</div></div>`
  const last = S.chapters.filter((c) => !S.never[c.key] && sizeOf(c) >= 200).at(-1)
  const running = chapterOf(S.checking), lost = S.chapters.find((c) => S.failed[c.key])
  const none = !S.entries.some((e) => e.place === 'body') ? [L.checkNoSettingsTitle(), L.checkNoSettingsAbout()]
    : S.checks.length ? [L.checkNoneInKind(), ''] : !last ? [L.checkNotYet(), L.checkNotYetAbout(L.checkAllBook())]
    : [L.checkNoneFound(), S.when === '手动检查' ? L.checkManualNote(L.checkModeManual(), L.checkRecheck()) : L.checkAutoNote()]
  const unfinished = lost && !S.checking // 失败的那章不当作没查出问题，上面那一行也不说「已检查至」，像是那以前都查过了
  if (unfinished) none.splice(0, 2, L.checkIncomplete(), L.checkFailedAt(titleOf(lost), S.failed[lost.key]))

  let detail = `<div class="check-detail">${blank(unfinished ? 'warn' : 'checked', S.checking ? L.checkRunningTitle() : none[0], S.checking ? L.checkRunningAbout() : none[1])}</div>`
  if (shown) {
    const ch = chapterOf(shown.key), e = S.entries.find((x) => x.id === shown.entry)
    const text = ch ? ch.paras[shown.para] || '' : ''
    const lit = (words, part) => words.split(part).map(t).join(`<span class="lit">${t(part)}</span>`)
    const seeText = ch ? goTo(ch.key, { key: ch.key, quote: shown.quote }) : null
    const seeEntry = () => set({ page: 'write', paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: e.id, form: null })
    const next = () => set({ cid: (of[of.indexOf(shown) + 1] || of[0]).id })
    const right = shown.kind === '不一致' && e ? `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkFromSetting(e.cat, e.names[0] || ''))}</div><div class="hit link"${onClick(seeEntry)}>${t(L.checkSeeSetting())}</div></div><div class="check-card">${lit(e.text, shown.says)}</div></div>`
      : shown.kind === '名字有误' ? `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkNameInSettings())}</div></div><div class="check-card"><span class="lit">${t(shown.right)}</span></div></div>`
      : `<div class="fit"><div class="check-from"><div class="fit ell">${t(L.checkCanAdd(shown.cat))}</div></div><div class="check-card"><span class="lit">${t(shown.title)}</span>${t(L.checkColon())}${t(shown.as)}</div></div>`
    const acts = checkActs(shown).map(([label, act], i, all) => button(label, act, i === all.length - 1 ? 'primary' : '')).join('')
    detail = '<div class="check-detail" data-k="check-detail">' +
      `<div class="check-title"><div class="fit ell">${t(shown.title)}</div><div class="side">${of.indexOf(shown) + 1} / ${of.length}</div><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>${t(L.checkNext())}</div></div>` +
      `<div class="check-sides"><div class="fit"><div class="check-from"><div class="fit ell">${t(ch ? L.checkTextPara(titleOf(ch), shown.para + 1) : L.checkTextParaNoChapter(shown.para + 1))}</div><div class="hit link"${onClick(seeText)}>${t(L.checkSeeText())}</div></div><div class="check-card">${lit(text, shown.quote)}</div></div>${right}</div>` +
      (shown.also ? `<div class="check-also">${t(shown.kind === '新出现' ? L.checkAlsoNew() : L.checkAlsoSimilar())}</div>` + shown.also.map((a) => `<div class="check-more"><div class="side">${t(a.at)}</div><div class="fit">${t(a.text)}</div><div class="side">${t(a.done ? L.checkMarkedNoChange() : L.checkAlsoOpen())}</div></div>`).join('') : '') +
      `<div class="grow"></div><div class="check-foot"><div class="hit link${of.length > 1 ? '' : ' off'}"${onClick(of.length > 1 ? next : null)}>${t(L.checkLater())}</div><div class="grow"></div>${acts}</div></div>`
  }
  return '<div class="check-list">' +
    // 不另写「检查」二字：标题栏正中已经说了在哪一页。
    '<div class="check-head">' +
    (running ? `<div class="muted">${t(S.queue.length ? L.checkRunningMore(titleOf(running), S.queue.length) : L.checkRunning(titleOf(running)))}</div><div class="grow"></div><div class="hit link"${onClick(() => set({ checking: false, queue: [] }))}>${t(L.checkStop())}</div>`
      : `<div class="muted">${t(unfinished ? L.checkProgress(S.chapters.filter((c) => !S.never[c.key] && sizeOf(c) >= 200 && !S.failed[c.key]).length, S.chapters.filter((c) => S.failed[c.key]).length) : last ? L.checkCheckedUpTo(titleOf(last)) : '')}</div><div class="grow"></div><div class="anchor"><div class="hit link"${onClick(again)}>${t(last ? L.checkRecheck() : L.checkAllBook())}</div>${S.open === 'checkAll' ? ask() : ''}</div>`) + '</div>' +
    seg(S.kind + ' ' + count(S.kind), KINDS.map((k) => [k + ' ' + count(k), '', kindWord(k) + ' ' + count(k)]), (v) => set({ kind: v.split(' ')[0], cid: null }), L.checkKindAria()) +
    `<div class="check-rows scroll">${of.map(row).join('')}</div></div>` + detail
}

const CHECK_KINDS = () => ({ 不一致: L.notesKindInconsistent(), 名字有误: L.notesKindName(), 新出现: L.notesKindNew() })

// 查一章，或者排着队一章一章查。查完的不再算没查过、改过、没查成。
function checkNow(...keys) {
  set({ checking: keys[0], queue: keys.slice(1) })
  const next = () => later(900, () => {
    const key = S.checking
    if (!key) return // 中途停下了
    const size = Math.round(sizeOf(chapterOf(key)) * 1.4)
    set((s) => ({ records: noted(s, '检查', chTitle(key), [['规则', 1400], ['设定', 1820], ['本章全文', size]], [3220 + size, 0, 1600]), checking: s.queue[0] || false, queue: s.queue.slice(1), dirty: { ...s.dirty, [key]: false }, never: { ...s.never, [key]: false }, failed: { ...s.failed, [key]: '' }, part: { ...s.part, [key]: { ...s.part[key], check: 0 } } }))
    if (S.checking) next()
  })
  next()
}
// 全书里该查的几章：够字数，而且没查过、改过或者上次没查成。
const due = () => S.chapters.filter((c) => sizeOf(c) >= 200 && (S.never[c.key] || S.dirty[c.key] || S.failed[c.key]))

// 检查：和设定对不上的那几个字。名字有误的只有标题：标题已经说了应为什么。
contribute('MarginSources', 'check', (cur) => S.checks.filter((c) => c.key === cur.key).map((c) => ({ quote: c.quote, para: c.para, cls: 'doubt', id: '',
  note: { id: c.id, warn: true, kind: CHECK_KINDS()[c.kind] || c.kind, title: c.title, text: c.says ? L.notesSays(c.says) : c.as || '', open: () => set({ page: 'check', kind: '全部', cid: c.id, ...SHUT }), acts: checkActs(c) } })), 1)

const PART = () => L.statusPartLong()

// 检查时机的三种：状态里记的值、下面那句说明、显示的字。正文底栏和设置里都用它。
const WHENS = () => [['写完一章后检查', L.statusWhenAfterAbout(), L.statusWhenAfter()], ['手动检查', L.statusWhenManualAbout(), L.statusWhenManual()], ['边写边检查', L.statusWhenWritingAbout(), L.statusWhenWriting()]]

contribute('SettingsSections', 'check', () => inBook() ? `<div class="ext-prefs"><div class="prefs-sub">${t(L.settingsCheckText())}</div>${radios(S.when, WHENS(), (v) => set({ when: v }))}</div>` : '', 0)

contribute('PendingGroups', 'check', () => {
  const list = []
  for (const c of S.checks) {
    const ch = chapterOf(c.key)
    list.push({ id: c.id, group: '检查', name: c.kind === '新出现' ? L.statusNotInLore(c.title) : c.title, about: ch ? titleOf(ch) : '', key: c.key, open: () => set({ page: 'check', kind: '全部', cid: c.id, ...SHUT }) })
  }
  return list
}, 0)

contribute('Pages', 'check', { id: 'check', label: () => L.pageCheck(), draw: checkPage, order: 3 }, 3)

// 检查：看哪一类、哪一条、正在查哪一章、哪几章查过以后又改了
// 检查：排着队等查的、尚未检查的、没查成的几章
// 太长、AI 只读了前面一段的几章：{ check } 是检查读了前多少字，再查一次就当它读全了
contribute('State', 'check', ({ of }) => ({
  checks: of('checks', []),
  kind: '全部',
  cid: null,
  checking: false,
  dirty: {},
  queue: [],
  never: {},
  failed: {},
  part: {},
  when: '写完一章后检查'
}))

function checkFoot(cur) {
  const whenName = (v) => (WHENS().find(([name]) => name === v) || [v, '', v])[2]
  if (!cur) return '' // 检查关着时没有检查的状态
  const n = S.checks.filter((c) => c.key === cur.key).length, dirty = S.dirty[cur.key]
  const part = (S.part[cur.key] || {}).check // 太长，只查了前面这么多字：不能说成全章查完了
  const seeAll = () => set({ page: 'check', kind: '全部', cid: S.checks.find((c) => c.key === cur.key).id })
  // 这一章查到哪了，和现在能不能点「立即检查」。
  let state = L.statusChecked(), can = false, tip = ''
  if (S.checking === cur.key) state = L.statusChecking()
  else if (S.queue.includes(cur.key)) state = L.statusQueued()
  else if (S.failed[cur.key]) { state = L.statusFailed(whyText(S.failed[cur.key])); can = true; tip = state } // 出错的说明长，放不下时悬停看全
  else if (!S.entries.some((e) => e.place === 'body')) state = L.statusNoEntries()
  else if (sizeOf(cur) < 200) state = L.statusShort()
  else if (S.never[cur.key]) { state = L.statusNever(); can = true }
  else if (n && dirty) { state = L.statusDirtyIssues(n); can = true }
  else if (n && part) state = L.statusPartIssues(part, n)
  else if (n) state = L.statusIssues(n)
  else if (dirty) { state = L.statusEdited(); can = true }
  else if (part) { state = L.statusPartOK(part); tip = PART() }
  const found = n && !S.checking && !S.failed[cur.key] && !S.never[cur.key] && sizeOf(cur) >= 200
  return '<div class="foot-check">' + (found ? `<div class="foot-state ell hit link" title="${L.statusGoCheck()}"${onClick(seeAll)}>${t(state)}</div>` : `<div class="foot-state ell"${tip ? ` title="${t(tip)}"` : ''}>${t(state)}</div>`) +
    (can ? `<div class="hit link" title="${L.statusCheckTip()}"${onClick(() => checkNow(cur.key))}>${L.statusCheckNow()}</div>` : '') + '<div class="faint">·</div>' +
    `<div class="anchor up left"><div class="hit" title="${L.statusWhenTitle()}"${onClick(() => set({ open: S.open === 'when' ? '' : 'when' }))}>${t(whenName(S.when))} ▴</div>${S.open === 'when' ? `<div class="pop when-pop" data-k="when-pop"><div class="pop-title">${L.statusWhenTitle()}</div>${radios(S.when, WHENS(), (v) => set({ when: v, ...SHUT }))}</div>` : ''}</div></div>`
}

contribute('FooterItems', 'check', (cur) => checkFoot(cur), 0)

contribute('SettingsPages', 'check', { name: '提醒', order: 20 }, 21)
