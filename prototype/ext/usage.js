// 扩展：usage。界面、动作和登记都在这里。
'use strict'

function logged(s, talk, use) {
  const feature = talk.origin || '讨论', where = talk.origin ? chTitle(talk.key) : talk.title
  const own = { 起草: [['章纲和卷纲', 260], ['本卷前面各章章纲', 310], ['上一章全文', 5060]], 改写: [['章纲和卷纲', 260], ['本章全文', 4300]] }[feature] || []
  const parts = (input) => { const fixed = [['规则', 1100], ['设定', 1820], ...own]; return [...fixed, ['之前的对话和查阅', Math.max(input - fixed.reduce((n, [, x]) => n + x, 0), 0)]] }
  return [...use.map((u) => [stamp(), feature, where, s.uses[0].model, s.uses[0].effort, parts(u[0]), u, '完成']).reverse(), ...s.records]
}

// 续写、检查也各记一行，用「续写和检查」那个模型。
function noted(s, feature, where, parts, use, end = '完成') {
  const u = s.uses[1].model ? s.uses[1] : s.uses[0]
  return [[stamp(), feature, where, u.model, s.uses[1].effort, parts, use, end], ...s.records]
}

// AI 回这段讨论一句：先想，再查（搜全书、读正文、查设定），再写；写完之前可以取消。
// 要记进设定的，AI 在这一轮里当场记下，回复上方列着记了什么，修改记录里也有一条。

const FEATURES = ['讨论', '起草', '改写', '续写', '检查']
const featureName = (f) => ({ 讨论: L.pageTalk(), 起草: L.settingsFeatureDraft(), 改写: L.settingsFeatureRewrite(), 续写: L.settingsFeatureContinue(), 检查: L.pageCheck() })[f] ?? f
function spent() {
  const all = S.records
  const row = (feature) => {
    const rows = all.filter((r) => r[1] === feature), sum = (i) => rows.reduce((n, r) => n + r[6][i], 0)
    return `<div class="spent-row"><div>${t(featureName(feature))}</div><div class="fit ell">${t(L.settingsSpentRow(rows.length, tokens(sum(0)), Math.round((sum(1) / sum(0)) * 100), tokens(sum(2))))}</div></div>`
  }
  const used = FEATURES.filter((f) => all.some((r) => r[1] === f))
  return `<div class="prefs-group"><div class="group-title">${t(L.settingsUsage())}</div></div>` +
    (used.length ? `<div class="spent">${used.map(row).join('')}</div><div class="prefs-links"><div class="hit link"${onClick(() => set((s) => ({ dialog: { ...s.dialog, records: true, only: '全部', row: -1 }, ...SHUT })))}>${t(L.settingsViewAll(all.length))}</div></div>` : `<div class="note">${t(L.settingsNoUsage())}</div>`)
}
// AI 记录：每次请求一行，新的在上，失败的、取消的也在。点一行看这次发了哪几部分、服务商算的实际用量。不存发出去的字。
// AI 记录里的字：状态里存的是中文（「规则」「设定 7 条」「不思考」「失败：…」），显示时换成当前语言；「设定 7 条」的数照原样填进去。
const endName = (end) => ({ 完成: L.settingsRecordDone(), 已取消: L.settingsRecordCancelled(), '失败：无法连接模型服务': L.recordsFailNoModel() })[end] ?? end
const PART_WORDS = () => ({ 规则: L.recordsRules(), 设定: L.recordsLore(), 章纲和卷纲: L.recordsOutlines(), 章纲: L.recordsChapterOutline(), 本卷前面各章章纲: L.recordsVolumeOutlines(), 上一章全文: L.recordsPrevChapter(), 本章全文: L.recordsWholeChapter(), 本章正文: L.recordsChapterText(), 本章前文: L.recordsChapterSoFar(), 光标前的正文: L.recordsBeforeCursor(), 讨论摘要: L.recordsChatSummary(), 之前的对话: L.recordsEarlierChat(), 之前的对话和查阅: L.recordsEarlierChatLookups(), 本次发言: L.recordsThisMessage(), 查阅结果: L.recordsLookups() })
const partName = (name) => { const n = /^设定 (\d+) 条$/.exec(name); return n ? L.recordsLoreCount(Number(n[1])) : PART_WORDS()[name] ?? name }
const effortWord = (e) => ({ 不思考: L.recordsEffortNone() })[e] ?? e
function records(d) {
  const shown = S.records.filter((r) => d.only === '全部' || r[1] === d.only)
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const line = (r, i) => {
    const [at, feature, where, model, effort, parts, [input, cached, output], end] = r, open = d.row === i
    return `<div class="record${open ? ' on' : ''}"><div class="record-row hit"${onClick(pick({ row: open ? -1 : i }))}><div class="record-at">${t(timeShown(at))}</div><div class="fit ell">${t(featureName(feature) + ' · ' + where)}</div>` +
      `<div class="record-n">${tokens(input + output)}</div><div class="record-end ell${end === '完成' ? '' : end === '已取消' ? ' muted' : ' danger'}">${t(endName(end))}</div>${icon(open ? 'down' : 'right', 'xs')}</div>` +
      (open ? `<div class="record-more"><div>${t(model + ' · ' + effortWord(effort))}</div><div>${t(L.settingsRecordSent(parts.map(([name, x]) => partName(name) + ' ' + tokens(x)).join(' · ')))}</div>` +
        `<div>${cached ? t(L.settingsRecordActualCached(tokens(input), tokens(cached), tokens(output))) : t(L.settingsRecordActual(tokens(input), tokens(output)))}</div></div>` : '') + '</div>'
  }
  return `<div class="records-head">${seg(d.only, [['全部', '', L.settingsRecordsAll()], ...FEATURES.map((f) => [f, '', featureName(f)])], (only) => set((s) => ({ dialog: { ...s.dialog, only, row: -1 } })), L.settingsFilterByFeature())}</div>` +
    `<div class="records">${shown.length ? shown.map((r) => line(r, S.records.indexOf(r))).join('') : `<div class="note">${t(L.settingsNoRecords())}</div>`}</div>`
}

contribute('AIUsage', 'usage', () => spent(), 0)

contribute('State', 'usage', ({ of }) => ({
  records: of('records', [])
}))

contribute('SettingsDetails', 'usage', { matches: (d) => d.tab === 'AI' && d.records, draw: records, head: (crumb) => `<div class="prefs-title">${crumb(L.modAI(), { records: false })}${t(L.settingsRecordsTitle())}</div><div class="note">${t(L.settingsRecordsNote())}</div>` }, 0)
