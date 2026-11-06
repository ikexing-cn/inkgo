// 扩展：ai。界面、动作和登记都在这里。
'use strict'

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

// 这部作品用了多少：按功能各一行，请求数和 token。软件不估花费。
// 每次请求都在 AI 记录里有一行，点「查看全部记录」一行一行看；只能看，不能删。

// 模型资料：新模型的上下文上限和最大输出从 models.dev 取，所有作品共用。开着自动同步时，启动后在后台请求一次，之后一天最多一次；
// 关着时只用软件自带的和手动同步的那份。开关和「立即同步」都写明会向 models.dev 发请求。
function syncModels() {
  const id = say(L.settingsModelsSyncing(), '', 'busy')
  later(1200, () => { unsay(id); set({ modelsSynced: stamp(), modelsFailed: '' }); say(L.settingsModelsSynced(), '', 'ok') })
}
function modelData() {
  const status = S.modelsFailed ? `<div class="note danger">${t(L.settingsModelsFailed(S.modelsFailed))}</div>` : `<div>${t(S.modelsSynced ? L.settingsModelsLast(timeShown(S.modelsSynced)) : L.settingsModelsNever())}</div>`
  return `<div class="prefs-group"><div class="group-title">${t(L.settingsModelData())}</div><div class="note">${t(L.settingsModelDataNote())}</div></div>` +
    `<div class="ticks">${tickbox(S.modelsAuto, L.settingsModelsAuto(), () => set({ modelsAuto: !S.modelsAuto }))}<div class="note tick-note">${t(L.settingsModelsAutoNote())}</div></div>` +
    `<div class="sync-row"><div class="fit">${status}<div class="note">${t(L.settingsModelsSyncNote())}</div></div>${button(L.settingsModelsSyncNow(), syncModels)}</div>`
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
    `<div class="uses">${S.uses.map(use).join('')}</div>` + drawPoint('AIUsage') +
    `<div class="prefs-group"><div class="group-title">${t(L.settingsWriting())}</div></div>` +
    `<div class="ticks">${tickbox(S.auto, L.settingsAutoContinue(), () => set({ auto: !S.auto }))}<div class="note tick-note">${t(L.settingsAutoNote())}</div></div>`) +
    `<div class="prefs-group"><div class="group-title">${t(L.settingsServices())}</div><div class="note">${t(L.settingsServicesNote())}</div></div>` +
    `<div class="services">${S.services.map(card).join('')}` +
    (d.edit === 'new' ? `<div class="service" data-reveal="new">${form()}</div>` : `<div class="service add hit"${onClick(() => open(() => editing(null), 'new'))}>${ICON.plus}${t(L.settingsAddService())}</div>`) + '</div>' +
    modelData() +
    `<div class="prefs-links"><div class="hit link"${onClick(osAsk(L.dialogOsOpenConfig()))}>${t(L.settingsOpenConfig())}</div><div class="hit link"${onClick(() => say(L.settingsReloaded(), '', 'ok'))}>${t(L.settingsReload())}</div></div>`
}

const writer = () => { const u = S.uses[1].model ? S.uses[1] : S.uses[0]; return [u.service + '/' + u.model, S.uses[1].effort] }

contribute('SettingsSections', 'ai', (d) => `<div class="ext-prefs">${settingsModel(d)}</div>`, 0)

// AI 正在正文里做的事：续写出的一句，或者等作者写改写要求
contribute('State', 'ai', () => ({
  ai: null,
  uses: structuredClone(BOOK.uses),
  services: BOOK.services.map((x) => ({ ...x, models: x.models.map(limits) })),
  auto: false,
  modelsAuto: true, modelsSynced: '今天 09:12', modelsFailed: '', // 模型资料：自动同步开着没有、上次同步、上次没成功的原因
  noAI: false,
  offFrom: ''
}))

contribute('SettingsPages', 'ai', { name: 'AI', order: 10 }, 10)
