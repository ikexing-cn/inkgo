// 各种对话框，和侧栏里「已备份」点开的那一块。
'use strict'

// ───────── 备份 ─────────

// 备份为什么没成功：[短的, 标题, 怎么办]。why 空着的是找不到那块盘。
const backupFails = () => ({
  disk: [L.backupDiskShort(), L.backupDiskTitle(), L.backupDiskFix()],
  dir: [L.backupDirShort(), L.backupDirTitle(), L.backupDirFix()],
  deny: [L.backupDenyShort(), L.backupDenyTitle(), L.backupDenyFix()],
})
const backupWhy = () => backupFails()[S.backup.why || 'disk']

// 立即备份：还没选过地方，就先问放哪。
function backupNow() {
  if (S.backup.state === 'none') { set({ dialog: { kind: 'backup', place: PATHS().places[0][1] }, ...SHUT }); return }
  set((s) => ({ backup: { ...s.backup, state: 'busy' }, ...SHUT }))
  later(1200, () => set((s) => ({ backup: { ...s.backup, state: 'ok', why: '', at: stamp(), copies: s.backup.copies + 1 } })))
}
const backupSet = (change) => set((s) => ({ backup: { ...s.backup, ...change } }))
const backupNext = (b) => (b.close ? L.backupWhenClose() : b.hourly ? L.backupNextHour() : b.daily ? L.backupNextDay() : L.backupNextManual())
const backupTimes = (b, change) => tickbox(b.close, L.backupWhenClose(), () => change({ close: !b.close })) + tickbox(b.daily, L.backupTickDaily(), () => change({ daily: !b.daily })) + tickbox(b.hourly, L.backupTickHourly(), () => change({ hourly: !b.hourly }))

// 侧栏里那一行点开的：备份得怎么样，没成功的话怎么办。
function backupPop() {
  const b = S.backup
  if (b.state === 'none') return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + t(L.backupNone()) + '</div><div class="note">' + t(L.backupLocalOnly()) + '</div>' + `<div class="pop-buttons">${button(L.backupChoose(), backupNow, 'primary')}</div></div>`
  if (b.state === 'fail') {
    return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + t(backupWhy()[1]) + '</div>' + `<div class="note">${t(backupWhy()[2])}${b.at ? t(L.backupLastAt(timeShown(b.at))) : t(L.backupNoneYet())}</div>` +
      `<div class="pop-buttons"><div class="hit link"${onClick(() => set((s) => ({ backup: { ...s.backup, hush: true }, ...SHUT })))}>${t(L.backupHushToday())}</div>${button(L.backupChangePlace(), osAsk(L.dialogOsBackupFolder()))}${button(L.backupRetry(), backupNow, 'primary')}</div></div>`
  }
  return `<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">${t(L.backupDone(timeShown(b.at)))}</div><div class="note">${t(L.backupTo(b.dir))}<br>${t(L.backupNextAt(backupNext(b)))}</div>` +
    `<div class="pop-buttons">${button(L.backupSettings(), () => set({ dialog: { kind: 'settings', tab: '备份', edit: '' }, ...SHUT }))}${button(b.state === 'busy' ? L.backupBusy() : L.backupRunNow(), b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary')}</div></div>`
}

// 第一次备份：问放哪、备份时机。
function backupDialog(d) {
  const type = (change) => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const times = { close: d.close ?? true, daily: d.daily ?? true, hourly: d.hourly ?? false }
  const leave = () => set({ dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null, ...SHUT })
  const start = () => {
    set((s) => ({ backup: { ...s.backup, ...times, state: 'busy', dir: d.place, copies: 0 }, dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null }))
    later(1200, () => backupSet({ state: 'ok', at: stamp(), copies: 1 }))
  }
  return dialog('small', dialogHead(L.backupDialogTitle(), L.backupDialogAbout()) +
    `<div class="places">${PATHS().places.map(([label, dir]) => `<div class="place hit${d.place === dir ? ' on' : ''}"${onClick(() => type({ place: dir }))}><div class="ring${d.place === dir ? ' on' : ''}"></div><div class="fit"><div>${t(label)}</div><div class="note ell">${t(L.backupDetected(dir))}</div></div></div>`).join('')}` +
    `<div class="place hit"${onClick(osAsk(L.dialogOsBackupFolder()))}><div class="ring"></div><div class="fit"><div>${t(L.backupOtherFolder())}</div><div class="note">${t(L.backupOtherFolderNote())}</div></div></div></div>` +
    `<div class="field"><div class="label">${t(L.backupTiming())}</div><div class="ticks">${backupTimes(times, type)}</div></div>` +
    `<div class="buttons"><div class="hit link"${onClick(leave)}>${t(L.backupLater())}</div><div class="grow"></div>${button(L.backupStart(), d.place ? start : null, d.place ? 'primary' : 'primary off')}</div>`)
}

// ───────── 设置 ─────────

const useAt = (i, change) => set((s) => ({ uses: s.uses.map((u, n) => (n === i ? { ...u, ...change } : u)) }))
// AI 的几件事：状态里存的是中文（「讨论」「与你讨论故事」），显示时换成当前语言。
const jobWord = (v) => ({ 讨论: L.settingsJobChat(), 续写和检查: L.settingsJobCheck() })[v] ?? v
const aboutWord = (v) => ({ 与你讨论故事: L.settingsJobChatAbout(), '灰字续写，检查正文': L.settingsJobCheckAbout() })[v] ?? v
const PROTOCOLS = ['OpenAI Chat Completions（多数兼容服务）', 'OpenAI Responses', 'Anthropic Messages']

// 名称里的拉丁字母小写，从第一个字母起留数字、- 和 _，空格当 -：新服务没填 ID 时用它。
const idFrom = (name) => name.toLowerCase().replace(/ +/g, '-').replace(/[^a-z0-9_-]/g, '').replace(/^[^a-z]+/, '').replace(/[-_]+$/, '')

// 服务表单里哪一栏保存不了、为什么，写在那一栏下面；空着的那栏不算错，只让按钮等。
// 上下文上限和最大输出是正整数（可写 128K、1M），最大输出小于上下文上限；压缩阈值是整数，在一半到九成五之间。
function serviceProblems(d) {
  const f = d.svc, fresh = d.edit === 'new', id = f.id.trim() || (fresh ? idFrom(f.name) : '')
  const models = f.models.filter((m) => m.id.trim()).map((m) => ({ ...m, id: m.id.trim() }))
  const wrong = [
    [(m) => !count(m.max) || !count(m.out), (m) => L.settingsModelNumbers(m.id)],
    [(m) => !/^\d+$/.test(m.at.trim()), (m) => L.settingsModelAtInt(m.id)],
    [(m) => count(m.out) >= count(m.max), (m) => L.settingsModelOutMax(m.id)],
    [(m) => Number(m.at) < 50 || Number(m.at) > 95, (m) => L.settingsModelAtRange(m.id)],
    [(m) => models.filter((x) => x.id === m.id).length > 1, (m) => L.settingsModelDup(m.id)],
  ].map(([bad, why]) => [models.find(bad), why]).find(([m]) => m)
  const taken = fresh && S.services.find((x) => x.id === id)
  const p = {
    id, list: models,
    ID: !id ? '' : taken ? L.settingsIDTaken(taken.name) : /^[a-z][a-z0-9_-]*$/.test(id) ? '' : L.settingsIDRule(),
    url: !f.url.trim() ? '' : !/^https?:\/\//.test(f.url.trim()) ? L.settingsURLScheme() : !/^https?:\/\/[^/?#@\s]/.test(f.url.trim()) ? L.settingsURLNoHost()
      : /^https?:\/\/[^/?#@\s]+(\/[^?#\s]*)?$/.test(f.url.trim()) ? '' : L.settingsURLExtra(),
    env: !f.env.trim() || /^[A-Za-z_]\w*$/.test(f.env.trim()) ? '' : L.settingsEnvRule(),
    models: wrong ? wrong[1](wrong[0]) : '',
  }
  p.fits = !!(id && f.name.trim() && f.url.trim()) && !p.ID && !p.url && !p.env && !p.models
  return p
}

function saveService() {
  const f = S.dialog.svc, name = f.name.trim(), p = serviceProblems(S.dialog)
  if (!p.fits) return
  const made = { id: p.id, name, url: f.url.trim(), host: f.url.trim().replace(/^https?:\/\//, '').split('/')[0], proto: f.proto, env: f.env.trim(), models: p.list.map((m) => ({ ...m, max: size(count(m.max)), out: size(count(m.out)), at: m.at.trim() })) }
  set((s) => ({ services: s.dialog.edit === 'new' ? [...s.services, made] : s.services.map((x) => (x.id === made.id ? made : x)), dialog: { ...s.dialog, edit: '', svc: null } }))
  say(L.settingsSaved(), '', 'ok')
}
function fetchModels() {
  const id = say(L.settingsFetching(), '', 'busy', [[L.commonCancel(), () => { clearTimeout(timer); unsay(id) }]])
  const timer = aiLater(1200, () => { unsay(id); set((s) => (s.dialog && s.dialog.svc ? { dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: (BOOK.services.find((x) => x.id === s.dialog.svc.id) || BOOK.services[0]).models.map(limits) } } } : null)); say(L.settingsModelsFetched(), '', 'ok') })
}

// 这部作品用了多少：按功能各一行，请求数、token、按内置价格估的花费；表里没有价格的模型不估。
// 每次请求都在 AI 记录里有一行，点「查看全部记录」一行一行看；只能看，不能删。
const FEATURES = ['讨论', '起草', '改写', '续写', '检查']
const featureName = (f) => ({ 讨论: L.pageTalk(), 起草: L.settingsFeatureDraft(), 改写: L.settingsFeatureRewrite(), 续写: L.settingsFeatureContinue(), 检查: L.pageCheck() })[f] ?? f
const spentOn = (rows) => rows.reduce((n, r) => { const p = (BOOK.table[r[3]] || [])[2], [input, cached, output] = r[6]; return n + (p ? ((input - cached) * p[0] + cached * p[1] + output * p[2]) / 1e6 : 0) }, 0)
function spent() {
  const all = S.records
  const row = (feature) => {
    const rows = all.filter((r) => r[1] === feature), sum = (i) => rows.reduce((n, r) => n + r[6][i], 0)
    return `<div class="spent-row"><div>${t(featureName(feature))}</div><div class="fit ell">${t(L.settingsSpentRow(rows.length, tokens(sum(0)), Math.round((sum(1) / sum(0)) * 100), tokens(sum(2))))}</div><div>${t(L.settingsSpentCost(spentOn(rows)))}</div></div>`
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
    const [at, feature, where, model, effort, parts, [input, cached, output], end] = r, open = d.row === i, cost = price(model, [r[6]])
    return `<div class="record${open ? ' on' : ''}"><div class="record-row hit"${onClick(pick({ row: open ? -1 : i }))}><div class="record-at">${t(timeShown(at))}</div><div class="fit ell">${t(featureName(feature) + ' · ' + where)}</div>` +
      `<div class="record-n">${tokens(input + output)}</div><div class="record-cost">${t(cost || '—')}</div><div class="record-end ell${end === '完成' ? '' : end === '已取消' ? ' muted' : ' danger'}">${t(endName(end))}</div>${icon(open ? 'down' : 'right', 'xs')}</div>` +
      (open ? `<div class="record-more"><div>${t(model + ' · ' + effortWord(effort))}</div><div>${t(L.settingsRecordSent(parts.map(([name, x]) => partName(name) + ' ' + tokens(x)).join(' · ')))}</div>` +
        `<div>${cached ? t(L.settingsRecordActualCached(tokens(input), tokens(cached), tokens(output))) : t(L.settingsRecordActual(tokens(input), tokens(output)))}</div></div>` : '') + '</div>'
  }
  return `<div class="records-head"><div class="hit link"${onClick(pick({ records: false }))}>${t('‹ ' + L.settingsUsage())}</div><div class="grow"></div>${seg(d.only, [['全部', '', L.settingsRecordsAll()], ...FEATURES.map((f) => [f, '', featureName(f)])], (only) => set((s) => ({ dialog: { ...s.dialog, only, row: -1 } })), L.settingsFilterByFeature())}</div>` +
    `<div class="records">${shown.length ? shown.map((r) => line(r, S.records.indexOf(r))).join('') : `<div class="note">${t(L.settingsNoRecords())}</div>`}</div>`
}

function settingsModel(d) {
  const used = (svc) => S.page !== 'shelf' && S.uses.some((u) => u.service === svc.name) // 书架上没有打开的作品
  const use = (u, i) => {
    const same = u.service === '沿用讨论模型'
    const pickService = (name) => { const svc = S.services.find((x) => x.name === name); useAt(i, { service: name, model: svc && svc.models[0] ? svc.models[0].id : '' }) }
    return `<div class="use-row"><div class="use-name"><div>${t(jobWord(u.job))}</div><div class="note">${t(aboutWord(u.about))}</div></div>` +
      choose('service-' + i, u.service, [...(i ? [['沿用讨论模型', L.settingsSameAsTalk()]] : []), ...S.services.map((x) => x.name)], pickService, L.settingsServiceFor(jobWord(u.job))) +
      (same ? '<div class="fit"></div>' : `<div class="box line fit">${typed(u.model, (v) => useAt(i, { model: v }), L.settingsModelHint(), L.settingsModelFor(jobWord(u.job)))}</div>`) +
      choose('effort-' + i, u.effort, BOOK.efforts, (e) => useAt(i, { effort: e }), L.settingsEffortFor(jobWord(u.job))) + '</div>'
  }
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, svc: { ...s.dialog.svc, [key]: v } } }))
  // 改服务里的模型。ID 改成内置表里有的，三个数照表重填。
  const models = (change) => set((s) => ({ dialog: { ...s.dialog, svc: { ...s.dialog.svc, models: change(s.dialog.svc.models) } } }))
  const typeModel = (i, key) => (v) => models((list) => list.map((m, n) => (n !== i ? m : key === 'id' && BOOK.table[v.trim()] ? { ...limits(v.trim()), id: v } : { ...m, [key]: v })))
  const editing = (svc) => ({ edit: svc ? svc.id : 'new', svc: svc ? { id: svc.id, name: svc.name, url: svc.url, proto: svc.proto || PROTOCOLS[0], key: '', env: svc.env, models: svc.models.map((m) => ({ ...m })) } : { id: '', name: '', url: '', proto: PROTOCOLS[0], key: '', env: '', models: [] } })
  const limitRow = (m, i) => `<div class="limit-row"><div class="box line fit">${typed(m.id, typeModel(i, 'id'), '', L.settingsModelID())}</div>` +
    `<div class="box line num">${typed(m.max, typeModel(i, 'max'), '', L.settingsContextLimit())}</div><div class="box line num">${typed(m.out, typeModel(i, 'out'), '', L.settingsMaxOutput())}</div><div class="box line pct">${typed(m.at, typeModel(i, 'at'), '', L.settingsCompressAt())}</div>` +
    `<button class="icon-btn muted" aria-label="${t(L.settingsRemoveModel())}" title="${t(L.settingsRemoveModel())}"${onClick(() => models((list) => list.filter((_, n) => n !== i)))}>${icon('cross', 'sm')}</button></div>`
  const form = () => {
    const f = d.svc, p = serviceProblems(d), red = (why) => (why ? `<div class="note danger">${t(why)}</div>` : '')
    return '<div class="service-form">' +
      `<div class="pair id-first">${field(L.settingsIDLabel(), line(f.id, type('id'), d.edit === 'new' ? p.id : '', L.settingsProviderID(), d.edit === 'new' ? '' : ' disabled'))}${field(L.settingsDisplayName(), line(f.name, type('name'), '', L.settingsDisplayName()))}</div>` + red(p.ID) +
      field(L.settingsAPIURL(), line(f.url, type('url'), 'https://api.example.com/v1', L.settingsAPIURL())) +
      (!f.url.trim() ? '' : p.url ? red(p.url) : /^https?:\/\/(localhost|127\.0\.0\.1|\[::1\])([:/]|$)/.test(f.url.trim()) ? `<div class="note">${t(L.settingsLocalNote())}</div>` : `<div class="note amber">${t(L.settingsRemoteNote())}</div>`) +
      field(L.settingsProtocol(), choose('proto', f.proto, [[PROTOCOLS[0], L.settingsProtoChat()], PROTOCOLS[1], PROTOCOLS[2]], type('proto'), L.settingsProtocolAPI())) +
      `<div class="pair">${field('API key', line(f.key, type('key'), d.edit === 'new' ? L.settingsKeyEmptyNew() : L.settingsKeyEmptySaved(), 'API key', ' type="password"'))}${field(L.settingsEnvOr(), line(f.env, type('env'), L.settingsEnvExample(), L.settingsEnvName()) + red(p.env))}</div>` +
      `<div class="note">${t(L.settingsKeyNote())}${f.env.trim() && BOOK.services.some((x) => x.env === f.env.trim()) ? t(L.settingsEnvDetected(f.env.trim())) : ''}</div>` +
      `<div class="field"><div class="label">${t(L.settingsModels())}</div><div class="limits">` +
      (f.models.length ? `<div class="limit-row limit-head"><div class="fit">${t(L.settingsModelID())}</div><div class="num">${t(L.settingsContextLimit())}</div><div class="num">${t(L.settingsMaxOutput())}</div><div class="pct">${t(L.settingsCompressPct())}</div><div class="gap"></div></div>` + f.models.map(limitRow).join('') : `<div class="note">${t(L.settingsNoModels())}</div>`) + '</div>' + red(p.models) + '</div>' +
      `<div class="note">${t(L.settingsCompressNote())}</div>` +
      `<div class="buttons">${d.edit === 'new' ? button(L.commonCancel(), () => set((s) => ({ dialog: { ...s.dialog, edit: '', svc: null } }))) : `<div class="hit link danger"${onClick(() => say(L.settingsKeyCleared(), '', 'ok'))}>${t(L.settingsClearKey())}</div>`}${button(L.settingsAddModel(), () => models((list) => [...list, limits('')]))}${button(L.settingsFetchModels(), p.fits ? fetchModels : null, p.fits ? '' : 'off')}${button(L.settingsSaveProvider(), p.fits ? saveService : null, p.fits ? 'primary' : 'primary off')}</div></div>`
  }
  // 刚打开的表单连同它那张卡片滚进视野：新加的在列表最下面，常常在看得见的地方下面。
  const open = (change, id) => { set((s) => ({ dialog: { ...s.dialog, ...change(s) }, ...SHUT })); if (id) document.querySelector(`[data-reveal="${id}"]`).scrollIntoView({ block: 'nearest' }) }
  const card = (svc) => `<div class="service" data-reveal="${t(svc.id)}"><div class="service-head"><div class="fit"><div class="ell">${t(svc.name)}</div><div class="note ell">${t(L.settingsServiceMeta(svc.host, svc.models.length))}${used(svc) ? ' · ' + t(L.settingsInUse()) : ''}</div></div>` +
    button(d.edit === svc.id ? L.settingsCollapse() : L.settingsEdit(), () => (d.edit === svc.id ? open(() => ({ edit: '', svc: null })) : open(() => editing(svc), svc.id))) + '</div>' + (d.edit === svc.id ? form() : '') + '</div>'
  return (S.page === 'shelf' ? '' : `<div class="prefs-group"><div class="group-title">${t(L.settingsBookModels())}</div></div>` +
    `<div class="uses">${S.uses.map(use).join('')}</div>` + (extOn('usage') ? spent() : '') +
    `<div class="prefs-group"><div class="group-title">${t(L.settingsWriting())}</div></div>` +
    `<div class="ticks">${tickbox(S.auto, L.settingsAutoContinue(), () => set({ auto: !S.auto }))}<div class="note tick-note">${t(L.settingsAutoNote())}</div></div>`) +
    `<div class="prefs-group"><div class="group-title">${t(L.settingsServices())}</div><div class="note">${t(L.settingsServicesNote())}</div></div>` +
    `<div class="services">${S.services.map(card).join('')}` +
    (d.edit === 'new' ? `<div class="service" data-reveal="new">${form()}</div>` : `<div class="service add hit"${onClick(() => open(() => editing(null), 'new'))}>${ICON.plus}${t(L.settingsAddService())}</div>`) + '</div>' +
    `<div class="prefs-links"><div class="hit link"${onClick(osAsk(L.dialogOsOpenConfig()))}>${t(L.settingsOpenConfig())}</div><div class="hit link"${onClick(() => say(L.settingsReloaded(), '', 'ok'))}>${t(L.settingsReload())}</div></div>`
}

function settingsBackup() {
  const b = S.backup
  if (b.state === 'none') {
    return `<div class="backup-card"><div class="fit"><div class="group-title">${t(L.backupNone())}</div><div class="note">${t(L.backupLocalOnly())}</div></div>${button(L.backupChoose(), () => set({ dialog: { kind: 'backup', place: PATHS().places[0][1], back: true }, ...SHUT }), 'primary')}</div>`
  }
  const title = { ok: L.backupDone(timeShown(b.at)), busy: L.backupBusy(), fail: L.backupFailed() }[b.state]
  const row = (label, inner) => `<div class="prefs-row"><div class="prefs-label">${t(label)}</div>${inner}</div>`
  return `<div class="backup-card"><div class="fit"><div class="group-title">${t(title)}</div><div class="note">${b.state === 'fail' ? t(L.backupFailNote(backupWhy()[0], backupWhy()[2])) : t(L.backupSummary(wordsShown(S.shelf.find((x) => x.id === S.book).words), S.chapters.length, b.copies)) + (b.copies > 1 ? t(L.backupOldest()) : '')}</div></div>` +
    button(b.state === 'fail' ? L.backupRetry() : L.backupRunNow(), b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary') + '</div>' +
    row(L.backupDestLabel(), `<div class="fit ell">${t(b.dir)}</div>${button(L.backupChange(), osAsk(L.dialogOsBackupFolder()))}`) +
    row(L.backupTiming(), `<div class="ticks">${backupTimes(b, backupSet)}</div>`) +
    row(L.backupKeep(), `<div class="keep">${choose('keep', b.keep, [['最近 10 份', L.backupKeep10()], ['最近 30 份', L.backupKeep30()], ['最近 90 份', L.backupKeep90()]], (keep) => backupSet({ keep }), L.backupKeep())}</div><div class="note">${t(L.backupSize(b.copies * 1.1))}</div>`)
}

// 「AI」那一部分的最上面：一个开关，管本机的所有作品。点了不马上变，先问一句（ext.js 的 askExt）。
// 它和「功能」页里 AI 那一行是同一个开关。
const aiSwitch = () => `<div class="ticks">${tickbox(S.noAI, L.settingsCloseAI(), () => extFlip('ai'))}` +
  `<div class="note tick-note">${t(L.settingsAIOffNote())}</div></div>`

// 「外观」的最下面：禁用 GPU 加速，管本机。界面用什么画是打开软件时定的，改了要重新打开才生效：
// 和这次打开时不一样，说明下面多一句，跟着一个「立即重新打开」（软件里关掉窗口再打开，回到同一本书，设置不再开着）。
// mygo v0.6.4 以前关着：mygo 在 Windows 上不用 GPU 画，窗口多半是白的，勾上的人连这一项都看不见、勾不回去。改成 false 就藏起来。
const OFFER_NO_GPU = true
function gpuSwitch() {
  const pending = S.noGPU !== S.ranNoGPU
  return `<div class="ticks">${tickbox(S.noGPU, L.settingsDisableGPU(), () => set({ noGPU: !S.noGPU }))}` +
    (pending ? `<div class="where tick-note"><div>${t(L.settingsGPUNext())}</div><div class="hit link"${onClick(() => set((s) => ({ ranNoGPU: s.noGPU, dialog: null, ...SHUT })))}>${t(L.settingsRestartNow())}</div></div>` : '') + '</div>'
}

// 关闭 AI：正在做的 AI 任务停下。没采纳的提议丢掉；没回完的那句讨论不要了，作者的话还在，和点「取消请求」一样。
// 正开着的讨论、检查两页回到正文，右栏的讨论回到设定。讨论、检查结果、AI 记录都还在，重新开启后照旧：
// 关的时候停在讨论页、检查页的，开的时候还在正文页就回到那一页。
// 提示条一并收起：上面说的多半是 AI 的事，有的还能点「重试」。
function turnAI(on) {
  if (!on && S.waiting) stopReply()
  set((s) => (on ? { noAI: false, dialog: { ...s.dialog, ask: '' }, offFrom: '', page: s.offFrom && s.page === 'write' ? s.offFrom : s.page } : {
    noAI: true, dialog: { ...s.dialog, ask: '' }, toasts: [], ai: null, checking: false, queue: [], ...ruled(s, s.props.map((p) => p.id), '关闭 AI 时已丢弃'),
    page: s.page === 'talk' || s.page === 'check' ? 'write' : s.page, offFrom: s.page === 'talk' || s.page === 'check' ? s.page : '', sv: s.sv === 'ideas' ? 'list' : s.sv,
  }))
}

// 设置的几个分类：状态里记的是中文名，显示时换成 L 里的字。
const tabName = (name) => ({ 外观: L.viewLook(), 功能: L.settingsTabModules(), 提醒与检查: L.settingsTabRemind(), 提醒: L.settingsTabRemindOnly(), 备份: L.settingsTabBackup() })[name] ?? name

function settingsDialog(d) {
  // 书架上没有打开的作品，只有本机的两部分。AI 关着时，「提醒与检查」里只剩提醒。
  const remind = aiOn() ? '提醒与检查' : '提醒'
  const tabs = S.page === 'shelf' ? ['AI', '外观', '功能'] : ['AI', '外观', '功能', remind, ...(extOn('backup') ? ['备份'] : [])]
  // 每一类的标题和下面那句说明留在上面不动，只有内容滚动。
  const about = { AI: aiOn() ? '' : L.settingsLocalOnly(), 外观: L.settingsLocalOnly(), 功能: L.settingsLocalOnly(), [remind]: L.settingsBookOnly(), 备份: '' }
  const lateNow = S.marks.filter((m) => m.st === 'open' && BOOK.now - m.ch >= S.late).length
  const step =(by) => () => set((s) => ({ late: Math.max(5, s.late + by) }))
  const rec = d.tab === 'AI' && d.records && extOn('usage')
  const body = d.tab === 'AI' ? (rec ? records(d) : aiSwitch() + (aiOn() ? settingsModel(d) : '')) : d.tab === '备份' ? settingsBackup() : d.tab === '功能' ? extPage()
    : d.tab === '外观' ? `<div class="prefs-row"><div class="prefs-label">${t(L.settingsColor())}</div>${seg(S.look, LOOKS(), (v) => set({ look: v }), L.settingsColor())}</div>` +
      `<div class="prefs-row">${tickbox(S.names, L.settingsMarkNames(), () => set({ names: !S.names }))}</div>` + (OFFER_NO_GPU ? gpuSwitch() : '')
    : `<div class="prefs-group"><div class="group-title">${t(L.settingsForeshadow())}</div></div>` +
      `<div class="prefs-row"><div class="fit">${t(L.settingsLateAfter())}</div><div class="stepper"><div class="hit" title="${t(L.settingsLateLess())}"${onClick(step(-5))}>−</div><div class="stepper-n">${t(L.settingsLateChapters(S.late))}</div><div class="hit" title="${t(L.settingsLateMore())}"${onClick(step(5))}>+</div></div></div>` +
      `<div class="note">${t(L.settingsLateNow(lateNow))}</div>` +
      (aiOn() ? `<div class="prefs-group"><div class="group-title">${t(L.settingsCheckText())}</div></div>${radios(S.when, WHENS(), (v) => set({ when: v }))}` : '')
  return dialog('prefs', `<div class="prefs-nav"><div class="prefs-nav-title">${t(L.settingsTitle())}</div>${tabs.map((name) => `<div class="nav hit${d.tab === name ? ' on' : ''}"${onClick(() => set({ dialog: { kind: 'settings', tab: name, edit: '' }, ...SHUT }))}>${t(tabName(name))}</div>`).join('')}</div>` +
    `<div class="prefs-main"><div class="prefs-head"><div class="prefs-title">${t(rec ? L.settingsRecordsTitle() : tabName(d.tab))}</div>${rec || about[d.tab] ? `<div class="note">${t(rec ? L.settingsRecordsNote() : about[d.tab])}</div>` : ''}</div><div class="prefs-body scroll" data-k="prefs-${rec ? 'records' : d.tab}">${body}</div></div><button class="icon-btn muted prefs-close" aria-label="${t(L.settingsClose())}" title="${t(L.settingsClose())}"${onClick(shut)}>${icon('cross')}</button>`) + (d.ask ? askExt(d) : '')
}

// ───────── 正文那一页的几个对话框 ─────────

// 加入设定：选中的字先放进框里，可以改成自己的说法。
// 摘取原文：从一句话里拖选一段，照原话记成设定。
function sourceDialog(d) {
  const read = () => {
    const sel = getSelection(), box = document.querySelector('.source-text')
    const picked = sel && !sel.isCollapsed && box.contains(sel.anchorNode) && box.contains(sel.focusNode) ? sel.toString().trim() : ''
    if (picked !== d.picked) set((s) => ({ dialog: { ...s.dialog, picked } }))
  }
  const add = () => {
    if (!d.picked) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text: d.picked }], log: did(s, L.dialogLogEntry(d.cat, d.picked)) }))
    say(L.dialogSettingAdded(), '', 'ok')
  }
  return dialog('source', dialogHead(L.dialogSourceTitle(), L.dialogSourceAbout()) +
    `<div class="source-text scroll"${onUp(read)}>${t(d.text)}</div>` +
    `<div class="source-line">${choose('source-cat', d.cat, S.cats, (v) => set((s) => ({ dialog: { ...s.dialog, cat: v } })), L.dialogSourceCat())}<div class="${d.picked ? 'ink' : 'faint'}">${t(d.picked ? L.dialogPicked(d.picked.length) : L.dialogNotPicked())}</div><div class="grow"></div>` +
    `<div class="hit link"${onClick(shut)}>${t(L.dialogSourceCollapse())}</div>${button(L.dialogAddSetting(), add, d.picked ? 'primary' : 'primary off')}</div>`)
}

function takeDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const add = () => {
    const text = d.text.trim()
    if (!text) return
    set((s) => ({ dialog: null, ...SHUT, n: s.n + 1, entries: [...s.entries, { id: 'e' + s.n, place: 'body', cat: d.cat, names: [], text, src: d.src }], log: did(s, L.dialogLogEntry(d.cat, text)) }))
    say(L.dialogSettingAdded(), '', 'ok')
  }
  return dialog('small', dialogHead(L.dialogAddSetting(), L.dialogTakeAbout()) +
    field(L.dialogSettingText(), area(d.text, type('text'), '', L.dialogSettingText(), 4)) + field(L.dialogSettingCat(), choose('take-cat', d.cat, S.cats, type('cat'), L.dialogSettingCat())) +
    buttons(button(L.commonCancel(), shut), button(L.dialogAdd(), add, d.text.trim() ? 'primary' : 'primary off')))
}

// 起草在一段新的讨论里，用讨论模型。这里说清楚 AI 预先带上什么、大约多少 token；别的它要用时自己查。不预先报价。
function draftDialog(d) {
  const cur = chapterOf(S.ch), use = S.uses[0], prev = S.chapters[S.chapters.indexOf(cur) - 1]
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const cards = eventsOf(cur.key).length
  const sent = 1100 + S.entries.filter((e) => e.place === 'body').length * 260 + 600 + cards * 45 + (prev ? sizeOf(prev) : 0) // 规则、设定、章纲卷纲、本章的卡片、上一章全文
  const items = [cards ? L.draftItemCards(cards) : '', prev ? L.draftItemPrev() : '', cur.vol ? L.draftItemVolume() : L.draftItemBooks()].filter(Boolean).join(L.draftListSep())
  return dialog('small', dialogHead(L.cmdDraft(), L.draftAbout(cards ? L.draftAboutCards() : L.draftAboutOutline(), L.draftAboutFlow())) +
    `<div class="quote">${t(S.outlines[cur.key])}</div>` +
    field(L.draftGoal(), line(d.goal, type('goal'), L.draftGoalHint(), L.draftGoalLabel()) + (countOrNone(d.goal) ? '' : `<div class="note danger">${t(L.draftGoalInvalid())}</div>`)) +
    (cur.paras.length ? field(L.draftExisting(), dots(d.how, [['接在已有内容之后', L.draftAppend()], ['覆盖已有内容', L.draftOverwrite()]], type('how'), L.draftExistingHow())) : '') +
    `<div class="note">${t(L.draftSends(items, tokens(sent)))}</div>` +
    `<div class="note">${t(L.draftUsing(use.model, use.effort))}</div>` +
    buttons(button(L.commonCancel(), shut), countOrNone(d.goal) ? button(L.draftStart(), draftNow, 'primary') : button(L.draftStart(), null, 'primary off')))
}

// 两份稿子并起来看：old 里有、now 里没有的划掉，now 里新写的标绿。
function compare(old, now) {
  const out = []
  let i = 0, j = 0
  while (i < old.length || j < now.length) {
    if (i < old.length && old[i] === now[j]) { out.push(['', old[i]]); i++; j++ } else if (j < now.length && !old.includes(now[j])) out.push(['add', now[j++]])
    else if (i < old.length) out.push(['cut', old[i++]])
    else out.push(['add', now[j++]])
  }
  return out.map(([cls, text]) => `<div${cls ? ` class="${cls}"` : ''}>${t(text)}</div>`).join('')
}

// 「AI 改动前」的快照写着改动来自哪几段讨论，几个名字按界面语言连起来（「甲」和「乙」、“A” and “B”）。
const aiFrom = (talks) => L.snapAIFrom(new Intl.ListFormat(LANG.replace(/-XA$/, '')).format(talks.map((x) => (x === null ? L.snapAITalkGone() : L.snapAITalk(x)))))
// 快照是在什么之前存的。AI 关着时，「AI 改动前」只写「改动前」：快照还在，不提 AI。
const snapKind = (kind) => ({ 恢复前: L.snapKindRestore(), 'AI 改动前': L.snapKindAI(), 改动前: L.snapKindEdit(), 外部改动: L.snapKindOutside(), 全书替换前: L.snapKindSwap(), 写作中: L.snapKindWriting(), 打开时: L.snapKindOpening() })[kind === 'AI 改动前' && !aiOn() ? '改动前' : kind] ?? kind

function snapsDialog(d) {
  const cur = chapterOf(S.ch), list = S.snaps[cur.key] || []
  const shown = list.find((x) => x.id === d.id) || list[0]
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const restore = () => {
    set((s) => ({ dialog: null, ...snap(s, cur.key, '恢复前'), ...retype(s, cur.key, () => shown.paras), bar: null, pick: null })) // 选中的那几个字换掉了，浮条跟着收起
    say(L.snapRestored(timeShown(shown.at)), '', 'ok')
  }
  const same = shown && shown.paras.join('\n') === cur.paras.join('\n')
  return dialog('wide', dialogHead(L.snapTitle(titleOf(cur)), L.snapAbout(aiOn() ? L.snapAboutAI() : '')) +
    (shown
      ? `<div class="snaps"><div class="snap-list scroll">${list.map((x) => `<div class="snap-row row${x === shown ? ' on' : ''}"${onClick(pick({ id: x.id }))}><div>${t(timeShown(x.at))}</div><div class="note">${t(L.snapRowInfo(snapKind(x.kind), x.paras.join('').length))}</div></div>`).join('')}</div>` +
        `<div class="snap-side"><div class="snap-tools">${tickbox(d.diff, L.snapCompare(), pick({ diff: !d.diff }))}<div class="fit note ell">${same ? t(L.snapSame()) : d.diff ? t(L.snapDiffHint()) : ''}</div></div>` +
        (shown.ai && aiOn() ? `<div class="card"><div class="who">${t(aiFrom(shown.ai.talks))}</div>${shown.ai.saw ? `<div class="extra">${t(shown.ai.saw)}</div>` : ''}</div>` : '') +
        `<div class="snap-text scroll">${d.diff ? compare(shown.paras, cur.paras) : shown.paras.map((p) => `<div>${t(p)}</div>`).join('') || `<div class="faint">${t(L.snapEmpty())}</div>`}</div></div></div>`
      : `<div class="empty"><div>${t(L.snapNone())}</div></div>`) +
    buttons(button(L.commonClose(), shut), button(L.snapRestore(), shown && !same ? restore : null, shown && !same ? 'primary' : 'primary off')))
}

// 导出：正文、大纲、设定勾几样导几样，各成一个文件。勾的几样、格式和笔名都记着，范围每次从「全书」起。
const EXPORT_WHAT = () => ({ 正文: L.exportBody(), 大纲: L.pagePlan(), 设定: L.exportSettings() })
const RANGE_NAME = () => ({ 全书: L.exportRangeBook(), 本卷: L.exportRangeVolume(), 本章: L.exportRangeChapter(), 指定范围: L.exportRangeCustom() })
function exportDialog(d) {
  const cur = chapterOf(S.ch), book = S.shelf.find((b) => b.id === S.book), list = S.chapters
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const word = S.exportAs === 'Word', ticked = (what) => S.exportWhat.includes(what)
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
  const going = S.exportWhat.filter((w) => !skip[w])
  const part = range === '全书' ? '' : range === '本卷' ? S.volumes.find((v) => v[0] === cur.vol)[1] : range === '本章' ? titleOf(cur) : from === to ? titleOf(chapterOf(from)) : L.exportSpan(titleOf(chapterOf(from)), titleOf(chapterOf(to)))
  const nameOf = (what) => [book.name, what === '设定' ? '' : part, what === '正文' ? '' : EXPORT_WHAT()[what]].filter(Boolean).join('-').replace(/：/g, ' ') + (word ? '.docx' : '.txt')
  const out = () => {
    set((s) => ({ dialog: null, ...SHUT, ...(ticked('正文') && word ? { pen, shelf: s.shelf.map((b) => (b.id === s.book ? { ...b, pen } : b)) } : {}) }))
    say(going.length > 1 ? L.exportedMany(going.length, S.library) : L.exportedOne(S.library + sep() + nameOf(going[0])), '', 'ok')
  }
  const about = S.exportWhat.length > 1 ? L.exportAboutMany()
    : ticked('设定') ? L.exportAboutSettings() : ticked('大纲') ? L.exportAboutOutline()
    : word ? L.exportAboutWord() : L.exportAboutText()
  const pickOf = (id, key, options, pick, label) => field(label, choose(id, titleOf(chapterOf(key)), options.map(titleOf), pick, label))
  return dialog('small', dialogHead(L.exportTitle(), about) +
    field(L.exportContent(), `<div class="tick-row">${['正文', '大纲', '设定'].map((w) => tickbox(ticked(w), EXPORT_WHAT()[w], flip(w))).join('')}</div>`) +
    field(L.exportFormat(), dots(S.exportAs, [['Word', 'Word'], ['纯文本', L.exportPlain()]], (v) => set({ exportAs: v }), L.exportFormatAria())) +
    (ticked('正文') || ticked('大纲') ? field(L.exportRange(), dots(range, ranges.map((r) => [r, RANGE_NAME()[r]]), type('range'), L.exportRangeAria())) +
      (range === '指定范围' ? `<div class="pair">${pickOf('export-from', from, list, pickFrom, L.exportFrom())}${pickOf('export-to', to, list.slice(at(from)), (v) => type('to')(keyOf(v)), L.exportTo())}</div>` : '') : '') +
    (ticked('正文') && word ? field(L.exportPen(), line(pen, type('pen'), L.exportPenHint(), L.exportPen())) : '') +
    (ticked('正文') && !word ? tickbox(d.indent, L.exportIndent(), () => type('indent')(!d.indent)) : '') +
    S.exportWhat.filter((w) => skip[w]).map((w) => `<div class="note">${t(skip[w])}</div>`).join('') +
    buttons(button(L.commonCancel(), shut), button(L.exportTitle(), going.length ? out : null, going.length ? 'primary' : 'primary off')))
}

// 全书替换前再问一句。
function swapDialog(d) {
  return dialog('ask', dialogHead(L.dialogSwapTitle(), L.dialogSwapAbout(d.chapters, d.total, d.q, d.to ? L.dialogSwapTo(d.to) : L.dialogSwapNone())) +
    buttons(button(L.commonCancel(), shut), button(L.dialogSwapConfirm(), swapBook, 'primary')))
}

// 关标签页时那一章还有没存的字：存了再关、不存就关（回到存着的那一份）、或者不关。
// 三个按钮照系统问「要不要保存」的顺序排：Windows 是保存、不保存、取消；macOS 是「不保存」单独在最左，取消、保存靠右。
function unsavedDialog(d) {
  const drop = () => set((s) => (!s.saved && s.kept && s.kept.key === s.ch ? { ...retext(s, s.kept.paras), dirty: s.kept.dirty } : null))
  const then = (first) => () => { first(); set({ saved: true, kept: null, dialog: null }); shutTabs(d.gone, true) }
  const save = button(L.cmdSave(), then(() => {}), 'primary'), lose = button(L.dialogDontSave(), then(drop)), keep = button(L.commonCancel(), shut)
  return dialog('ask', dialogHead(L.dialogUnsavedTitle(titleOf(chapterOf(S.ch))), L.dialogUnsavedAbout()) +
    (OS === 'mac' ? `<div class="buttons asked-mac">${lose}<div class="grow"></div>${keep}${save}</div>` : `<div class="buttons asked">${save}${lose}${keep}</div>`)) // system order
}

// ───────── 侧栏点出来的几个对话框 ─────────

function binDialog(d) {
  const back = (b) => () => {
    set((s) => {
      const vol = b.volume, home = (c) => (vol || s.volumes.some((v) => v[0] === c.vol) ? c.vol : '')
      return { bin: s.bin.filter((x) => x !== b), volumes: vol ? [...s.volumes, vol] : s.volumes, chapters: [...s.chapters, ...b.chapters.map((c) => ({ ...c, vol: home(c) }))] }
    })
    say(L.trashRestored(b.title), '', 'ok')
  }
  const row = (b) => `<div class="bin-row"><div class="fit"><div class="ell">${t(b.title)}</div><div class="note">${t(L.trashDeletedAt(timeShown(b.at)))}${b.volume ? ' · ' + t(L.trashWithChapters(b.chapters.length)) : ''}</div></div>` +
    (d.sure === b.id ? button(L.commonCancel(), () => set({ dialog: { kind: 'bin', sure: '' } })) + button(L.trashConfirm(), () => set((s) => ({ bin: s.bin.filter((x) => x !== b), dialog: { kind: 'bin', sure: '' } })), 'danger')
      : `<div class="hit link danger"${onClick(() => set({ dialog: { kind: 'bin', sure: b.id } }))}>${t(L.trashDelete())}</div>` + button(L.trashRestore(), back(b))) + '</div>'
  return dialog('small', dialogHead(L.trashTitle(), L.trashAbout()) +
    `<div class="bin-rows scroll">${S.bin.length ? S.bin.map(row).join('') : `<div class="empty"><div>${t(L.trashEmpty())}</div></div>`}</div>` + buttons(button(L.commonClose(), shut)))
}

// 第三项写着 ai 的，是 AI 的那几组、几行：AI 关着时不列。
// 快捷键的几组和几行；第三项写着 ai 的，是 AI 的那几组、几行（AI 关着时不列）。键的写法（Ctrl+S）不翻，说明和组名跟着语言走。
function KEYS() {
  return [
    [L.helpGroupWrite(), [['Ctrl+S', L.helpSave()], ['Ctrl+F', L.helpFind()], ['Ctrl+N', L.helpNewChapter()], ['F2', L.helpRename()], ['Ctrl+PgDn / Ctrl+PgUp', L.helpChapterNav()], ['Ctrl+Tab / Ctrl+Shift+Tab', L.helpTabNav()], ['Ctrl+W', L.helpCloseTab()], ['F11', L.helpFocus()], ['Ctrl+J', L.helpContinue(), 'ai'], ['Tab / Esc', L.helpAcceptDrop(), 'ai'], ['Ctrl+L', L.helpCite(), 'ai'], ['Ctrl+Z / Ctrl+Shift+Z', L.helpUndoRedo()], ['Ctrl+Backspace / Ctrl+Delete', L.helpDeleteWord()], ['Ctrl+' + L.helpWheel(), L.helpZoomText()]]],
    [L.helpGroupChat(), [['Ctrl+Enter', L.helpSend()]], 'ai'],
    [L.helpGroupGlobal(), [['Ctrl+= / Ctrl+- / Ctrl+0', L.helpZoomUI()], ['Ctrl+/', L.helpOpenKeys()]]],
  ]
}
function keysDialog() {
  const shown = (list) => list.filter(([, , ai]) => !ai || aiOn())
  return dialog('keys', dialogHead(L.keysTitle()) + `<div class="keys scroll">${shown(KEYS()).map(([group, of]) => `<div class="head">${t(group)}</div>` + shown(of).filter(([key]) => keyLabel(key)).map(([key, does]) => `<div class="key-row"><div class="key">${t(keyLabel(key))}</div><div class="fit">${t(does)}</div></div>`).join('')).join('')}</div>` + buttons(button(L.commonClose(), shut)))
}

// ───────── 书架上的两个对话框 ─────────

function newBookDialog(d) {
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const name = d.name.trim() || L.dialogNewBookUntitled(), at = (d.at ?? S.library + sep() + name).trim()
  // 位置下面说那里是什么；书架上的作品都在作品库里，同名的文件夹里已有文件。
  const taken = S.shelf.some((b) => S.library + sep() + b.name === at), fits = at && !taken
  // 路径长了框里只露出开头，要核对的文件夹名在说明里写出来。
  const folder = at.split(/[\\/]/).filter(Boolean).at(-1) || at
  const where = !at ? `<div class="note">${t(L.dialogNewBookNoPlace())}</div>` : taken ? `<div class="note danger">${t(L.dialogNewBookTaken(folder))}</div>` : `<div class="note">${t(L.dialogNewBookWillCreate(folder))}</div>`
  const create = () => {
    if (!fits) return
    const id = 'book' + S.n
    set((s) => ({ shelf: [{ id, name, kind: d.type, words: 0, at: stamp() }, ...s.shelf], n: s.n + 1, dialog: null }))
    openBook(id)
  }
  return dialog('small', dialogHead(L.dialogNewBookTitle()) + field(L.dialogNewBookName(), line(d.name, type('name'), L.dialogNewBookNameHint(), L.dialogNewBookName(), keys(create) + ' autofocus')) + field(L.dialogNewBookKind(), choose('book-kind', d.type, BOOK.kinds.map((k) => [k, genreWord(k)]), type('type'), L.dialogNewBookKind())) +
    `<div class="field"><div class="label">${t(L.dialogNewBookPlace())}</div><div class="pick-line"><div class="box line fit">${typed(d.at ?? S.library + sep() + name, type('at'), '', L.dialogNewBookPlaceLabel(), keys(create))}</div>${button(L.dialogNewBookChange(), osAsk(L.dialogOsNewBookFolder()))}</div>${where}</div>` +
    buttons(button(L.commonCancel(), shut), button(L.dialogNewBookCreate(), fits ? create : null, fits ? 'primary' : 'primary off')))
}

function nameBookDialog(d) {
  const name = d.name.trim()
  const save = () => { if (name) set((s) => ({ shelf: s.shelf.map((b) => (b.id === d.id ? { ...b, name } : b)), dialog: null })) }
  return dialog('small', dialogHead(L.dialogNameBookTitle()) + line(d.name, (v) => set((s) => ({ dialog: { ...s.dialog, name: v } })), '', L.dialogNameBookNew(), keys(save) + ' autofocus') + `<div class="note">${t(L.dialogNameBookNote())}</div>` + buttons(button(L.commonCancel(), shut), button(L.cmdSave(), name ? save : null, name ? 'primary' : 'primary off')))
}

// 关掉作品时还有没定的 AI 提议：先问一句。提议不存盘，关了就没了；它写的字在讨论里还看得到，要的话让 AI 再提一次。
function leaveDialog(d) {
  const n = S.props.length, close = () => { set((s) => ({ dialog: null, ...ruled(s, s.props.map((p) => p.id), '作品关闭时已丢弃') })); d.then() }
  return dialog('ask', dialogHead(L.dialogLeaveTitle(n), L.dialogLeaveAbout()) + buttons(button(L.commonCancel(), shut), button(L.dialogLeaveDiscard(), close, 'primary')))
}
// 要关掉这部作品（换一部、回到书架）：有没定的 AI 提议，先问。
const leave = (then) => () => (S.props.length ? set({ dialog: { kind: 'leave', then }, ...SHUT }) : then())

const DIALOGS = { leave: leaveDialog, backup: backupDialog, settings: settingsDialog, take: takeDialog, source: sourceDialog, draft: draftDialog, snaps: snapsDialog, export: exportDialog, swap: swapDialog, unsaved: unsavedDialog, bin: binDialog, keys: keysDialog, newBook: newBookDialog, nameBook: nameBookDialog, card: cardDialog, importForm: importFormDialog, importPreview: importPreviewDialog }
const dialogs = () => (S.dialog ? DIALOGS[S.dialog.kind](S.dialog) : '')
