// 扩展：export。界面、动作和登记都在这里。
'use strict'

const EXPORT_WHAT = () => ({ 正文: L.exportBody(), 大纲: L.pagePlan(), 设定: L.exportSettings() })
const RANGE_NAME = () => ({ 全书: L.exportRangeBook(), 本卷: L.exportRangeVolume(), 本章: L.exportRangeChapter(), 指定范围: L.exportRangeCustom() })
function exportDialog(d) {
  const cur = chapterOf(S.ch), book = S.shelf.find((b) => b.id === S.book), list = S.chapters
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  // 大纲、设定关着时导出里没有那一项；勾着的也不算。
  const offered = ['正文', ...contributions('ExportContents')]
  const word = S.exportAs === 'Word', ticked = (what) => offered.includes(what) && S.exportWhat.includes(what)
  const flip = (what) => () => set((s) => ({ exportWhat: ['正文', '大纲', '设定'].filter((w) => (w === what) !== s.exportWhat.includes(w)) }))
  const ranges = ['全书', ...(cur && cur.vol ? ['本卷'] : []), ...(cur ? ['本章'] : []), ...(list.length > 1 ? ['指定范围'] : [])]
  const range = ranges.includes(d.range) ? d.range : '全书'
  // 指定范围：从第一章起，到第三章止；「到」只列「从」那一章和它后面的。
  const at = (key) => list.findIndex((c) => c.key === key)
  const from = d.from ?? (list[0] || {}).key, to = d.to && at(d.to) >= at(from) ? d.to : (list[Math.min(at(from) + 2, list.length - 1)] || {}).key
  const keyOf = (title) => list.find((c) => titleOf(c) === title).key
  const pickFrom = (title) => { const k = keyOf(title); set((s) => ({ dialog: { ...s.dialog, from: k, to: at(to) < at(k) ? k : to } })) }
  const within = range === '全书' ? list : range === '本卷' ? list.filter((c) => c.vol === cur.vol) : range === '本章' ? [cur] : list.slice(at(from), at(to) + 1)
  const pen = d.pen ?? book.pen ?? S.pen
  const written = (key) => (S.outlines[key] || '').trim()
  // 勾了却导不出的，说一句为什么，导出时跳过它。
  const skip = {
    正文: !within.length && L.exportSkipBody(),
    大纲: !within.length ? L.exportSkipOutlineNone() : !within.some((c) => written(c.key) || written(c.vol)) && L.exportSkipOutlineEmpty(),
    设定: !S.entries.some((e) => e.place === 'body') && L.exportSkipSettings(),
  }
  const going = S.exportWhat.filter((w) => ticked(w) && !skip[w])
  const part = range === '全书' ? '' : range === '本卷' ? S.volumes.find((v) => v[0] === cur.vol)[1] : range === '本章' ? titleOf(cur) : from === to ? titleOf(chapterOf(from)) : L.exportSpan(titleOf(chapterOf(from)), titleOf(chapterOf(to)))
  const nameOf = (what) => [book.name, what === '设定' ? '' : part, what === '正文' ? '' : EXPORT_WHAT()[what]].filter(Boolean).join('-').replace(/：/g, ' ') + (word ? '.docx' : '.txt')
  const out = () => {
    set((s) => ({ dialog: null, ...SHUT, ...(ticked('正文') && word ? { pen, shelf: s.shelf.map((b) => (b.id === s.book ? { ...b, pen } : b)) } : {}) }))
    say(going.length > 1 ? L.exportedMany(going.length, S.library) : L.exportedOne(S.library + sep() + nameOf(going[0])), '', 'ok')
  }
  const about = offered.filter(ticked).length > 1 ? L.exportAboutMany()
    : ticked('设定') ? L.exportAboutSettings() : ticked('大纲') ? L.exportAboutOutline()
    : word ? L.exportAboutWord() : L.exportAboutText()
  const pickOf = (id, key, options, pick, label) => field(label, choose(id, titleOf(chapterOf(key)), options.map(titleOf), pick, label))
  return dialog('small', dialogHead(L.exportTitle(), about) +
    field(L.exportContent(), `<div class="tick-row">${offered.map((w) => tickbox(ticked(w), EXPORT_WHAT()[w], flip(w))).join('')}</div>`) +
    field(L.exportFormat(), dots(S.exportAs, [['Word', 'Word'], ['纯文本', L.exportPlain()]], (v) => set({ exportAs: v }), L.exportFormatAria())) +
    (ticked('正文') || ticked('大纲') ? field(L.exportRange(), dots(range, ranges.map((r) => [r, RANGE_NAME()[r]]), type('range'), L.exportRangeAria())) +
      (range === '指定范围' ? `<div class="pair">${pickOf('export-from', from, list, pickFrom, L.exportFrom())}${pickOf('export-to', to, list.slice(at(from)), (v) => type('to')(keyOf(v)), L.exportTo())}</div>` : '') : '') +
    (ticked('正文') && word ? field(L.exportPen(), line(pen, type('pen'), L.exportPenHint(), L.exportPen())) : '') +
    (ticked('正文') && !word ? tickbox(d.indent, L.exportIndent(), () => type('indent')(!d.indent)) : '') +
    offered.filter((w) => ticked(w) && skip[w]).map((w) => `<div class="note">${t(skip[w])}</div>`).join('') +
    buttons(button(L.commonCancel(), shut), button(L.exportTitle(), going.length ? out : null, going.length ? 'primary' : 'primary off')))
}

// 全书替换前再问一句。

contribute('Dialogs', 'export', { kind: 'export', draw: exportDialog }, 0)

contribute('FileActions', 'export', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdExport(), inBook ? ask({ kind: 'export', range: '全书', indent: true }) : null), 1)

// 导出勾了哪几样、成什么格式，上次写在首页的笔名（新作品沿用它）
contribute('State', 'export', () => ({
  exportWhat: ['正文'],
  exportAs: 'Word',
  pen: ''
}))
