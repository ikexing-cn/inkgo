// 设置的外壳、快捷键、作品和未保存的对话框。
'use strict'

// ───────── 设置 ─────────

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

// 设置左栏的一项：状态里记的是中文名，显示时换成 L 里的字。
const tabName = (name) => ({ 外观: L.viewLook(), 扩展: L.settingsTabExtensions(), 提醒: L.settingsTabRemind(), 备份: L.settingsTabBackup() })[name] ?? name

// 设置：左栏上面两项是宿主自己的（外观、扩展），下面是扩展登记的页（ext.js 的 prefPages）。
// 「扩展」里点一格进那个扩展自己的页（d.ext）；往下一层的页，标题写成路径「扩展 › 伏笔」，点前面那段回去。
function settingsDialog(d) {
  const navItem = (name) => `<div class="nav hit${d.tab === name ? ' on' : ''}"${onClick(() => set({ dialog: { kind: 'settings', tab: name, edit: '' }, ...SHUT }))}>${t(tabName(name))}</div>`
  const nav = `<div class="prefs-nav">${['外观', '扩展'].map(navItem).join('')}<div class="prefs-nav-gap"></div>${prefPages().map(([name]) => navItem(name)).join('')}</div>`
  const rec = contributions('SettingsDetails').find((x) => x.matches(d)), ext = d.tab === '扩展' && d.ext ? extOf(d.ext) : null
  const page = prefPage(d.tab)
  const lang = S.lang === 'system' ? 'system' : LANG
  const body = rec ? rec.draw(d) : ext ? extSection(ext.id, d) : page ? page[1].map((id) => extSection(id, d)).join('')
    : d.tab === '扩展' ? extOverview()
    : `<div class="prefs-row"><div class="prefs-label">${t(L.settingsColor())}</div>${seg(S.look, LOOKS(), (v) => set({ look: v }), L.settingsColor())}</div>` +
      `<div class="prefs-row"><div class="prefs-label">${t(L.settingsLanguage())}</div>${dots(lang, [['system', L.settingsLangSystem()], ['zh-Hans', '简体中文'], ['en', 'English']], (v) => { set({ lang: v }); switchLang(v === 'system' ? 'zh-Hans' : v) }, L.settingsLanguage())}</div>` +
      (OFFER_NO_GPU ? gpuSwitch() : '')
  const crumb = (label, change) => `<span class="hit prefs-crumb"${onClick(() => set((s) => ({ dialog: { ...s.dialog, ...change }, ...SHUT })))}>${t(label)}</span><span class="prefs-crumb-sep">›</span>`
  // 标题留在上面不动，只有内容滚动。右边那句只在页里所有的设置管的作品一样时写。
  const head = rec ? rec.head(crumb)
    : ext ? `<div class="prefs-title">${crumb(L.settingsTabExtensions(), { ext: '' })}${t(ext.name)}</div>`
    : `<div class="prefs-title">${t(tabName(d.tab))}</div>${d.tab === '外观' ? `<div class="note">${t(L.settingsLocalOnly())}</div>` : ''}`
  return dialog('prefs', nav + `<div class="prefs-main"><div class="prefs-head">${head}</div><div class="prefs-body scroll" data-k="prefs-${rec ? 'records' : ext ? 'ext-' + ext.id : d.tab}">${body}</div></div>` +
    `<button class="icon-btn muted prefs-close" aria-label="${t(L.settingsClose())}" title="${t(L.settingsClose())}"${onClick(shut)}>${icon('cross')}</button>`) + (d.ask ? askExt(d) : '')
}

// ───────── 正文那一页的几个对话框 ─────────

function unsavedDialog(d) {
  const drop = () => set((s) => (!s.saved && s.kept && s.kept.key === s.ch ? { ...retext(s, s.kept.paras), dirty: s.kept.dirty } : null))
  const then = (first) => () => { first(); set({ saved: true, kept: null, dialog: null }); shutTabs(d.gone, true) }
  const save = button(L.cmdSave(), then(() => {}), 'primary'), lose = button(L.dialogDontSave(), then(drop)), keep = button(L.commonCancel(), shut)
  return dialog('ask', dialogHead(L.dialogUnsavedTitle(titleOf(chapterOf(S.ch))), L.dialogUnsavedAbout()) +
    (OS === 'mac' ? `<div class="buttons asked-mac">${lose}<div class="grow"></div>${keep}${save}</div>` : `<div class="buttons asked">${save}${lose}${keep}</div>`)) // system order
}

// ───────── 侧栏点出来的几个对话框 ─────────

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

const DIALOGS = { leave: leaveDialog, settings: settingsDialog, unsaved: unsavedDialog, keys: keysDialog, newBook: newBookDialog, nameBook: nameBookDialog, importForm: importFormDialog, importPreview: importPreviewDialog }
const dialogs = () => {
  if (!S.dialog) return ''
  // 已打开的对话框仍照原来的状态画；开关只管入口，不在这里丢掉正在填的内容。
  const extra = registered('Dialogs').map((x) => x.value).find((x) => x.kind === S.dialog.kind)
  return (DIALOGS[S.dialog.kind] || extra?.draw)?.(S.dialog) || ''
}
