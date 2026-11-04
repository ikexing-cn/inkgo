// 扩展：outline。界面、动作和登记都在这里。
'use strict'

// ───────── 大纲 ─────────

// 大纲是一叠卡片，一件事一张。S.events 的先后就是故事里发生的先后，作者自己排；
// key 是写在哪一章、line 是哪条情节线：两样都有的落在「按章节」那张表里的一格，没有章的放在下面「未放进章节」。
const eventsOf = (key) => S.events.filter((e) => e.key === key)
// 一章在故事里是什么时候：它的卡片里第一个写了时间的到最后一个。
const whenOf = (evs) => { const w = [...new Set(evs.map((e) => e.when).filter(Boolean))]; return w.length > 1 ? w[0] + ' — ' + w.at(-1) : w[0] || '' }
// 一章里有比前面各章更早的事（回忆、倒叙）：它最早的那张，排在前面各章最晚的那张之前。
function goesBack(c) {
  const at = (e) => S.events.indexOf(e), before = S.chapters.slice(0, S.chapters.indexOf(c)).flatMap((x) => eventsOf(x.key)).map(at)
  const mine = eventsOf(c.key).map(at)
  return mine.length > 0 && before.length > 0 && Math.min(...mine) < Math.max(...before)
}
const gistOf = (c) => (S.outlines[c.key] || '').trim()
const lineOf = (id) => S.lines.find((l) => l.id === id)
const found = (e, q) => !q || [e.title, e.about, e.when, e.key ? chTitle(e.key) : ''].some((x) => x.includes(q))

// 新的一张排在时间线上哪里：给了 after 就排它后面；放进一章的，排在这一章和前面各章的卡片后面；都没有就排在最后。
function placeAt(events, chapters, key, after) {
  if (after) return events.findIndex((e) => e.id === after) + 1
  if (!key) return events.length
  const no = (k) => chapters.findIndex((c) => c.key === k), mine = no(key)
  return events.reduce((at, e, i) => (e.key && no(e.key) <= mine ? i + 1 : at), 0)
}
// 拖到另一格：换章、换情节线。从「未放进章节」拖出来的，在时间线上也排到那一章的位置。
function placeCard(id, key, line) {
  set((s) => {
    const e = s.events.find((x) => x.id === id)
    if (!e || (e.key === key && (line === null || e.line === line))) return null
    const moved = { ...e, key, line: line === null ? e.line : line }, rest = s.events.filter((x) => x !== e)
    const at = !e.key && key ? placeAt(rest, s.chapters, key, null) : s.events.indexOf(e)
    return { events: [...rest.slice(0, at), moved, ...rest.slice(at)] }
  })
}
// 一张卡片在弹窗里改：点一张卡片改它，点格子里的「＋」新记一张。弹窗里改的是一份草稿，保存了才算。
// 新记的一张带着它那一格的章、情节线、时间；after 是在时间线上排到哪一张后面。
const formOf = (e) => ({ title: e.title, when: e.when, about: e.about, key: e.key, line: e.line })
function openCard(id) {
  const e = S.events.find((x) => x.id === id)
  if (e) set({ dialog: { kind: 'card', id, form: formOf(e), n: 0 }, planOutline: null, planLine: null, ...SHUT })
}
const newCard = (key, line, after = null, when = '') => set({ dialog: { kind: 'card', id: null, form: { title: '', when, about: '', key, line }, after, n: 0 }, planOutline: null, planLine: null, ...SHUT })
// 存下弹窗里的这一张。more 是「继续添加」：存下以后接着记下一张，章、情节线、时间照旧，排在刚记的这张后面。
function saveCard(more) {
  set((s) => {
    const d = s.dialog, f = { ...d.form, title: d.form.title.trim(), when: d.form.when.trim() }
    if (d.id) {
      const e = s.events.find((x) => x.id === d.id), rest = s.events.filter((x) => x !== e)
      const at = !e.key && f.key ? placeAt(rest, s.chapters, f.key, null) : s.events.indexOf(e) // 刚放进一章的，在时间线上排到那一章的位置
      return { events: [...rest.slice(0, at), { ...e, ...f }, ...rest.slice(at)], dialog: null }
    }
    const id = 'ev' + s.n, at = placeAt(s.events, s.chapters, f.key, d.after && s.events.some((e) => e.id === d.after) ? d.after : null)
    return { events: [...s.events.slice(0, at), { id, ...f }, ...s.events.slice(at)], n: s.n + 1, planSee: id,
      dialog: more ? { ...d, form: { ...f, title: '', about: '' }, after: id, n: d.n + 1 } : null }
  })
  if (S.planSee) seeCard(S.planSee)
}
// 把一张卡片挪到看得见的地方，标出来一会儿。
function seeCard(id) {
  const card = document.querySelector(`[data-card="${id}"]`)
  if (card) card.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  later(1600, () => set((s) => (s.planSee === id ? { planSee: null } : null)))
}
function cardDialog(d) {
  const f = d.form, ok = f.title.trim() !== ''
  const type = (k) => (v) => set((s) => ({ dialog: { ...s.dialog, form: { ...s.dialog.form, [k]: v } } }))
  const save = () => (ok ? saveCard(false) : null)
  const drop = () => { set((s) => ({ events: s.events.filter((x) => x.id !== d.id), dialog: null })); say(L.planCardDeleted(f.title), '', 'ok') }
  const chs = [L.planNoChapter(), ...S.chapters.map(titleOf)], lines = [L.planNoLine(), ...S.lines.map((l) => l.name)]
  const pickCh = (v) => type('key')(v === chs[0] ? '' : S.chapters.find((c) => titleOf(c) === v).key)
  const pickLine = (v) => type('line')(v === lines[0] ? '' : S.lines.find((l) => l.name === v).id)
  return dialog('small', dialogHead(d.id ? L.planCardEdit() : L.planCardNew(), d.id ? '' : d.n ? L.planCardAdded(S.events.find((e) => e.id === d.after).title) : L.planCardIntro()) +
    // 「继续添加」以后换一个新的输入框，光标回到它里面。
    `<div class="field" data-k="card-title-${d.n}"><div class="label">${t(L.planCardWhat())}</div>${line(f.title, type('title'), L.planCardWhatHint(), L.planCardEvent(), keys(save) + ' autofocus')}</div>` +
    `<div class="pair">${field(L.planCardChapter(), choose('card-ch', f.key ? titleOf(chapterOf(f.key)) : chs[0], chs, pickCh, L.planCardChapter()))}${field(L.planLine(), choose('card-line', lineOf(f.line) ? lineOf(f.line).name : lines[0], lines, pickLine, L.planLine()))}</div>` +
    field(L.planCardWhen(), line(f.when, type('when'), L.planCardWhenHint(), L.planCardWhenLabel(), keys(save))) +
    field(L.planCardAbout(), area(f.about, type('about'), '', L.planCardNotes(), 4)) +
    '<div class="buttons">' + (d.id ? `<div class="hit link danger"${onClick(drop)}>${t(L.planCardDelete())}</div><div class="grow"></div>` : '') + button(L.commonCancel(), shut) +
    (d.id ? '' : button(L.planCardAddMore(), ok ? () => saveCard(true) : null, ok ? '' : 'off')) + button(d.id ? L.cmdSave() : L.planAddCard(), ok ? save : null, ok ? 'primary' : 'primary off') + '</div>')
}
// 一张卡片：停上去浮起来一点，点开改。
const cardView = (e, q, also = '') => `<div class="card hit${e.key ? '' : ' later'}${S.planSee === e.id ? ' seen' : ''}${found(e, q) ? '' : ' dim'}" draggable="true" data-k="card-${e.id}" data-card="${e.id}" title="${t(L.planCardHint())}"${onClick(() => openCard(e.id))}>` +
  (e.when && also !== 'ch' ? `<div class="card-when ell">${marked(e.when, q)}${also === 'line' && lineOf(e.line) ? `<span class="faint"> · ${t(lineOf(e.line).name)}</span>` : ''}</div>` : '') +
  `<div class="card-title clamp${e.title ? '' : ' none'}">${e.title ? marked(e.title, q) : t(L.commonUntitled())}</div>` +
  (also === 'ch' ? `<div class="card-ch ell">${e.key ? marked(chTitle(e.key), q) : t(L.planNoChapter())}</div>` : '') + '</div>'

// 格子里的「添加」：停在格子上才显出来，和卡片一样宽，点了弹出新卡片。
const slotAdd = (label, act) => `<div class="slot-add hit" title="${t(L.planAddCardTitle())}" aria-label="${t(label)}"${onClick(act)}>${icon('plus', 'sm')}${t(L.planAddCard())}</div>`
// 一条情节线的名字：点名字改名，「…」里改名、挪位置、删除。按章节在表头，时间线在左边一列，两处一样。
function lineTitle(l, where) {
  if (S.planLine && S.planLine.id === l.id) {
    return `<div class="box line fit" data-k="lh-${where}-${l.id}-edit">${typed(S.planLine.name, (v) => set((s) => ({ planLine: { ...s.planLine, name: v } })), L.planLineNameLabel(), L.planLineNameLabel(), keys(saveLine, () => set({ planLine: null })) + onBlur(saveLine) + ' autofocus data-all')}</div>`
  }
  const ls = S.lines, n = S.events.filter((e) => e.line === l.id).length, menu = 'line-' + where + '-' + l.id
  return `<div class="fit ell hit head-name" data-k="lh-${where}-${l.id}" title="${t(L.planLineRenameHint())}"${onClick(() => set({ planLine: { id: l.id, name: l.name }, ...SHUT }))}>${t(l.name)}</div><div class="faint">${n}</div>` +
    `<div class="anchor"><button class="icon-btn muted" aria-label="${t(L.planLineActions(l.name))}" title="${t(L.commonMoreActions())}"${onClick(() => set({ open: S.open === menu ? '' : menu }))}>${ICON.dots}</button>` +
    (S.open === menu ? '<div class="pop line-pop">' + menuRow(L.planLineRename(), () => set({ planLine: { id: l.id, name: l.name }, ...SHUT })) + menuRow(where === 'grid' ? L.planLineMoveLeft() : L.commonMoveUp(), ls[0] === l ? null : () => shiftLine(l.id, -1)) + menuRow(where === 'grid' ? L.planLineMoveRight() : L.commonMoveDown(), ls.at(-1) === l ? null : () => shiftLine(l.id, 1)) + '<div class="hair"></div>' + menuRow(L.planLineDelete(), () => dropLine(l.id)) + '</div>' : '') + '</div>'
}
const addLineButton = () => `<div class="plan-add hit" title="${t(L.planLineAddHint())}"${onClick(() => addLine())}>${icon('plus', 'sm')}${t(L.planLineAdd())}</div>`

// 按章节：一章一行，一条情节线一列，卡片落在交叉的格子里；哪条线断了几章，一眼看得出。
// 左边的章节一列和上面的表头滚动时不动，「新增情节线」总在右上角。
function planGrid(q) {
  const ls = S.lines
  const outlineBox = (key, hint, rows) => `<div class="box area" data-k="outline-${key}"><textarea class="in" rows="${rows}" placeholder="${t(hint)}" aria-label="${t(chapterOf(key) ? L.planChapterOutline() : L.planVolumeOutline())}" autofocus` +
    `${onInput((v) => set((s) => ({ outlines: { ...s.outlines, [key]: v } })))}${onBlur(() => set({ planOutline: null }))}${onKey((ev) => { if (ev.key === 'Escape') { ev.stopPropagation(); set({ planOutline: null }) } })}>${t(S.outlines[key] || '')}</textarea></div>`
  const gist = (key, text, none, cls) => (S.planOutline === key ? outlineBox(key, none, cls === 'vol-gist' ? 3 : 4)
    : `<div class="${cls} hit${text ? '' : ' none'}" title="${t(L.planClickEdit())}"${onClick(() => set({ planOutline: key, planLine: null }))}>${text ? marked(text, q) : t(none)}</div>`)
  const cell = (c, l) => `<div class="board-cell" data-cell="${c.key}|${l.id}">${S.events.filter((e) => e.key === c.key && e.line === l.id).map((e) => cardView(e, q)).join('')}` +
    slotAdd(L.planSlotInChapter(titleOf(c), l.name), () => newCard(c.key, l.id)) + '</div>'
  const chRow = (c) => {
    const when = whenOf(eventsOf(c.key)), stray = S.events.filter((e) => e.key === c.key && !lineOf(e.line))
    return `<div class="board-side grid-ch" data-k="row-${c.key}"><div class="ch-name"><div class="hit ell" title="${t(L.planOpenBody())}"${onClick(goTo(c.key))}>${marked(titleOf(c), q)}</div>${goesBack(c) ? `<div class="tag" title="${t(L.planBacktrackHint())}">${t(L.planBacktrack())}</div>` : ''}</div>` +
      (when ? `<div class="note ell">${t(L.planWhenNote(when))}</div>` : '') + gist(c.key, gistOf(c), L.planWriteChapterOutline(), 'ch-gist clamp') +
      (stray.length ? `<div class="faint">${t(L.planStray(stray.length))}</div>` : '') + '</div>' + ls.map((l) => cell(c, l)).join('') + '<div class="board-fill"></div>'
  }
  const hits = (c) => !q || [titleOf(c), gistOf(c)].some((x) => x.includes(q)) || eventsOf(c.key).some((e) => found(e, q))
  const block = (key, title, inside) => {
    const shown = inside.filter(hits)
    if (q && !shown.length && ![title, S.outlines[key] || ''].some((x) => x.includes(q))) return '' // 卷名、卷纲搜到了，这一卷的标题留着
    const when = whenOf(inside.flatMap((c) => eventsOf(c.key)))
    return `<div class="grid-vol" data-k="vol-${key || 'loose'}"><div class="vol-in"><div class="vol-name"><div class="group-title">${marked(title, q)}</div><div class="note">${t(L.planChapterCount(inside.length))}${when ? ' · ' + t(when) : ''}</div></div>` +
      (key ? gist(key, (S.outlines[key] || '').trim(), L.planWriteVolumeOutline(), 'vol-gist') : '') + '</div></div>' + shown.map(chRow).join('')
  }
  const loneCh = S.chapters.filter((c) => !c.vol)
  const body = S.volumes.map(([key, title]) => block(key, title, S.chapters.filter((c) => c.vol === key))).join('') + (loneCh.length ? block('', L.planNoVolume(), loneCh) : '')
  return `<div class="board grid scroll" data-k="grid"><div class="board-table" style="grid-template-columns: 248px repeat(${ls.length}, 200px) minmax(160px, 1fr)">` +
    `<div class="board-head board-corner">${t(L.planChapters())}</div>` + ls.map((l) => `<div class="board-head">${lineTitle(l, 'grid')}</div>`).join('') + `<div class="board-head board-end">${addLineButton()}</div>` +
    // 章节里一处也没搜到，搜到的只有下面托盘里的卡片。
    (body || `<div class="grid-none">${empty(L.planNoneInChapters(q), L.planFoundCardsBelow(L.planNoChapter()))}</div>`) + '</div></div>' + tray(q)
}
const saveLine = () => set((s) => (s.planLine ? { lines: s.lines.map((l) => (l.id === s.planLine.id ? { ...l, name: s.planLine.name.trim() || l.name } : l)), planLine: null } : null))
// 新的一条线放在最后，名字先选中，打字就换掉。
const addLine = () => set((s) => { const id = 'l' + s.n, name = L.planLineNew(); return { lines: [...s.lines, { id, name }], n: s.n + 1, planLine: { id, name }, planOutline: null, ...SHUT } })
const shiftLine = (id, d) => set((s) => { const i = s.lines.findIndex((l) => l.id === id), list = [...s.lines];[list[i], list[i + d]] = [list[i + d], list[i]]; return { lines: list, ...SHUT } })
// 删掉一条线：它的卡片不删，放到「未放进章节」里，写在哪一章的在卡片上记着。
function dropLine(id) {
  const l = lineOf(id), n = S.events.filter((e) => e.line === id).length
  set((s) => ({ lines: s.lines.filter((x) => x.id !== id), events: s.events.map((e) => (e.line === id ? { ...e, line: '' } : e)), ...SHUT }))
  say(L.planLineDeleted(l.name), n ? L.planLineDeletedCards(n) : '', 'ok')
}

// 下面一条：想到了、还没定写在哪的卡片。拖进上面的格子，就放进那一章。
function tray(q) {
  const cards = S.events.filter((e) => !e.key), all = cards.length
  return `<div class="tray" data-k="tray" data-cell="|"><div class="tray-head"><div class="group-title">${t(L.planNoChapter())}</div><div class="note">${t(all ? L.planTrayCount(all) : L.planTrayEmpty())}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(() => newCard('', ''))}>${t(L.planTrayAdd())}</div></div>` +
    (cards.length ? `<div class="tray-cards">${cards.map((e) => cardView(e, q, 'line')).join('')}</div>` : '') + '</div>'
}

// 时间线上的一列：挨着的、故事时间写得一样的几张并成一列；没写时间的自成一列。
function timeCols(events) {
  const cols = []
  for (const e of events) { const last = cols.at(-1); if (last && e.when && last.when === e.when) last.cards.push(e); else cols.push({ when: e.when, cards: [e] }) }
  return cols
}
// 拖到时间线上的一格：排进那一列的末尾，时间跟着那一列，换到那一行的情节线。col 是 end 的，排到最后。
function dropAt(id, col, line) {
  set((s) => {
    const e = s.events.find((x) => x.id === id), rest = s.events.filter((x) => x !== e)
    const to = col === 'end' ? null : timeCols(s.events)[+col]
    const stay = to && to.cards.length === 1 && to.cards[0] === e
    const last = to && !stay ? to.cards.filter((x) => x !== e).at(-1) : null
    const at = stay ? s.events.indexOf(e) : last ? rest.indexOf(last) + 1 : rest.length
    const moved = { ...e, line, when: to ? to.when : e.when }
    return { events: [...rest.slice(0, at), moved, ...rest.slice(at)] }
  })
}

// 时间线：同一叠卡片按故事里发生的先后从左往右排开，一条情节线一行，和「按章节」的列对得上。
// 实线框的已经放进章节，虚线框的还没有。点一列的时间，整列一起改；左边一列和上面的时间滚动时不动。
function planTimeline(q) {
  const cols = timeCols(S.events)
  const lanes = [...S.lines, ...(S.events.some((e) => !lineOf(e.line)) ? [{ id: '', name: L.planNoLine() }] : [])]
  const inLane = (e, l) => (l.id ? e.line === l.id : !lineOf(e.line))
  const add = (l, when, after) => slotAdd(L.planSlotAtTime(when || L.planLastSlot(), l.name), () => newCard('', l.id, after, when))
  const timePop = (c) => '<div class="pop time-pop" data-k="time-pop">' +
    `<div class="pop-label">${t(L.planTimeColumn(c.cards.length))}</div>` +
    `<div class="box line">${typed(S.planTime.when, (v) => set((s) => ({ planTime: { ...s.planTime, when: v } })), L.planTimeHint(), L.planTimeAria(), keys(saveTime) + ' autofocus data-all')}</div>` +
    `<div class="pop-buttons">${button(L.commonCancel(), () => set(SHUT))}${button(L.cmdSave(), saveTime, 'primary')}</div></div>`
  const time = (c, i) => `<div class="board-head tl-time anchor left" data-k="time-${i}">` +
    `<div class="tl-when hit ell${c.when ? '' : ' none'}" title="${t(L.planClickEditTime())}"${onClick(() => set({ open: 'time', planTime: { i, when: c.when } }))}>${c.when ? marked(c.when, q) : t(L.planNoTime())}</div>` +
    `<div class="tl-axis"><i class="ev-dot${c.cards.some((e) => e.key) ? '' : ' later'}"></i></div>${S.open === 'time' && S.planTime.i === i ? timePop(c) : ''}</div>`
  const lane = (l) => `<div class="board-side tl-lane">${l.id ? lineTitle(l, 'tl') : `<div class="fit ell head-name">${t(l.name)}</div>`}</div>` +
    cols.map((c, i) => `<div class="board-cell" data-slot="${i}|${l.id}">${c.cards.filter((e) => inLane(e, l)).map((e) => cardView(e, q, 'ch')).join('')}${add(l, c.when, c.cards.at(-1).id)}</div>`).join('') +
    `<div class="board-cell" data-slot="end|${l.id}">${add(l, '', S.events.length ? S.events.at(-1).id : null)}</div>`
  const off = S.chapters.filter((c) => !eventsOf(c.key).length)
  return `<div class="tl-note"><span><i class="tl-key"></i>${t(L.planInChapter())}</span><span><i class="tl-key later"></i>${t(L.planNoChapter())}</span><span class="faint">${t(L.planTimelineHint())}</span><span class="grow"></span>` +
    (off.length ? `<span>${t(off.length === 1 ? L.planOneNoCards(titleOf(off[0])) : L.planChaptersNoCards(off.length))}</span>` : '') + '</div>' +
    `<div class="board tl scroll" data-k="tl"><div class="board-table" style="grid-template-columns: 168px repeat(${cols.length}, 200px) 200px">` +
    `<div class="board-head tl-time board-corner"><div class="tl-when none">${t(L.planLine())}</div><div class="tl-axis"></div></div>` + cols.map(time).join('') +
    `<div class="board-head tl-time"><div class="tl-when none">${t(L.planTimeEnd())}</div><div class="tl-axis end"></div></div>` +
    lanes.map(lane).join('') + `<div class="board-side tl-lane">${addLineButton()}</div><div class="board-fill" style="grid-column: 2 / -1"></div></div></div>`
}
// 改一列的时间：这一列的几张一起改。改成和旁边一列一样的，就并到一起。
const saveTime = () => set((s) => {
  if (!s.planTime) return null
  const col = timeCols(s.events)[s.planTime.i], when = s.planTime.when.trim()
  return { events: s.events.map((e) => (col.cards.includes(e) ? { ...e, when } : e)), planTime: null, ...SHUT }
})

// 大纲文件在软件外被改坏了：整页一句话，正文和设定照常。不用「重试」：软件每两秒重读一次，改好了自己回来。
function planBroken() {
  return `<div class="editor"><div class="toolbar"><div class="crumb here">${t(L.pagePlan())}</div></div><div class="plan-none">` +
    blank('warn', L.planBrokenTitle(), L.planBrokenAbout(),
      `<div class="note faint">${t(L.planBrokenLine(12))}</div>` + buttons(button(L.commonOpenFolder(), osAsk(L.shelfShowFolder())))) + '</div></div>'
}

function planPage() {
  if (S.planBroken) return planBroken()
  const timeline = S.plan === '时间线', q = S.planQ.trim()
  const loose = S.events.filter((e) => !e.key).length
  // 搜着的时候，上面那一句换成找到了多少；一个也没找到，整页换成一句话和「清除搜索」，表头和下面的托盘都不留。
  const cards = q ? S.events.filter((e) => found(e, q)).length : 0
  const chs = q && !timeline ? S.chapters.filter((c) => [titleOf(c), gistOf(c)].some((x) => x.includes(q))).length + S.volumes.filter(([key, title]) => [title, S.outlines[key] || ''].some((x) => x.includes(q))).length : 0
  const sum = !q ? (loose ? L.planSummaryLoose(S.chapters.length, S.lines.length, S.events.length, loose) : L.planSummary(S.chapters.length, S.lines.length, S.events.length))
    : cards || chs ? (chs && cards ? L.planFoundBoth(chs, cards) : chs ? L.planFoundChapters(chs) : L.planFoundCards(cards)) : L.planNoResults()
  const clear = () => set({ planQ: '' })
  const none = q && !cards && !chs
  return `<div class="editor"><div class="toolbar"><div class="crumb here">${t(L.pagePlan())}</div><div class="note plan-sum">` + t(sum) + '</div><div class="grow"></div>' +
    `<div class="box line plan-find">${ICON.find}${typed(S.planQ, (v) => set({ planQ: v }), timeline ? L.planSearchCards() : L.planSearchCardsAndOutlines(), L.planSearch(), keys(() => {}, clear))}` +
    (q ? `<button class="icon-btn muted" aria-label="${t(L.planClearSearch())}" title="${t(L.planClearSearchHint())}"${onClick(clear)}>${icon('cross', 'sm')}</button>` : '') + '</div>' +
    seg(S.plan, [['按章节', '', L.planByChapter()], ['时间线', '', L.planTimelineView()]], (v) => set({ plan: v, planSee: null, planOutline: null, planLine: null, ...SHUT }), L.planViewAria()) + '</div>' +
    (none ? '<div class="plan-none">' + blank('find', L.planNotFound(q), timeline ? L.planSearchTimeline() : L.planSearchChapters(), buttons(button(L.planClearSearch(), clear))) + '</div>'
      : timeline ? planTimeline(q) : planGrid(q)) + '</div>'
}

// 标题下面的「章纲」，点开是一个能写字的框。
function chapterMore(cur) {
  const outline = S.outlines[cur.key] || '', on = S.box === '章纲'
  const box = on ? area(outline, (v) => set((s) => ({ outlines: { ...s.outlines, [cur.key]: v } })), L.editorOutlineHint(), L.editorOutline(), 3) : ''
  const evs = eventsOf(cur.key), when = whenOf(evs)
  // 到时间线上，这一章的第一张卡片标出来、挪到看得见的地方。
  const seeWhen = () => { const id = evs.find((e) => e.when).id; set({ page: 'plan', plan: '时间线', planSee: id, planOutline: null, planLine: null, planQ: '', ...SHUT }); seeCard(id) }
  return `<div class="chapter-more"><div class="hit${on ? ' on' : ''}"${onClick(() => set({ box: on ? '' : '章纲' }))}>${L.editorOutline()}${outline.trim() ? '' : L.editorOutlineEmpty()}${on ? ' ▴' : ' ▾'}</div>` +
    (when ? `<div class="hit" title="${L.editorSeeOnTimeline()}"${onClick(seeWhen)}>${t(L.editorInStory(when))}</div>` : '') + '</div>' +
    (box ? `<div class="chapter-box" data-k="chapter-box">${box}</div>` : '')
}

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

contribute('Dialogs', 'outline', { kind: 'card', draw: cardDialog }, 0)

contribute('ExportContents', 'outline', '大纲', 0)

contribute('ChapterHeadItems', 'outline', (cur) => chapterMore(cur), 0)

contribute('Pages', 'outline', { id: 'plan', label: () => L.pagePlan(), draw: planPage, order: 2 }, 2)

// 大纲的卡片和情节线
// 大纲页：看哪一种、标出来一会儿的那张卡片、正在改的章纲或卷纲、情节线的名字（{ id, name }）、时间线上一列的时间（{ i, when }），搜什么，右栏开着吗，大纲文件读不出来
contribute('State', 'outline', ({ of }) => ({
  outlines: of('outlines', {}),
  events: of('events', []),
  lines: of('lines', []),
  plan: '按章节',
  planSee: null,
  planOutline: null,
  planLine: null,
  planTime: null,
  planQ: '',
  planPane: false,
  planBroken: false
}))
