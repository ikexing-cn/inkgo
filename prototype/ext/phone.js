// 扩展：phone。界面、动作和登记都在这里。
'use strict'

// ───────── 画：手机预览 ─────────

// 这一章在读者手机上大概的样子。尺寸、边距、字号都是手机上的点（393 宽是常见的 iPhone）；
// 手机里一律写 em，1em 就是手机上的一个字，整个手机再按右栏的大小缩放。行距借用正文的三级行高。
const PHONES = { 窄屏: [360, 780], 标准: [393, 852], 宽屏: [430, 932] }
const SPACING = { 紧凑: 'loose', 适中: 'read', 宽松: 'paper' }
// 选项的字：状态里是中文，显示时换成当前语言；seg 的选项是 [状态值, 图标, 显示的字]。
const wordOpts = (keys, words) => keys.map((k) => [k, null, words()[k]])
const SIZE_WORDS = () => ({ 窄屏: L.previewNarrow(), 标准: L.previewStandard(), 宽屏: L.previewWide() })
const SPACING_WORDS = () => ({ 紧凑: L.previewTight(), 适中: L.previewMedium(), 宽松: L.previewLoose() })
const THEME_WORDS = () => ({ 白天: L.previewDay(), 夜间: L.previewNight(), 护眼: L.previewEyeCare() })
const MODE_WORDS = () => ({ 翻页: L.previewPaged(), 滚动: L.previewScroll() })
const LONG = 5 // 一段在手机上超过几行算长
let phoneAt = 0, phoneKey = '', phoneCaret = '', phoneScreens = 1, phoneLongAt = [] // 看到第几屏（从 0 起）、哪一章、光标上次在哪一屏、共几屏、长段各在第几屏
const phoneSet = (change) => set((s) => ({ phone: { ...s.phone, ...change } }))

// 翻一屏，只在这一章里翻。
function phoneFlip(d) {
  const to = phoneAt + d
  if (to >= 0 && to < phoneScreens) { phoneAt = to; draw() }
}
// 在预览里点的是那一段的第几个字后面：正文的光标放到同一处。
function phoneOff(ev) {
  const r = document.caretRangeFromPoint(ev.clientX, ev.clientY)
  return r && r.startContainer.parentElement === ev.target.closest('[data-phone]') ? r.startOffset : 0
}
// 跳到下一处长段所在的那一屏。
function phoneNextLong() {
  if (!phoneLongAt.length) return
  phoneAt = phoneLongAt.find((n) => n > phoneAt) ?? phoneLongAt[0]
  draw()
}

function phoneView() {
  const c = chapterOf(S.ch)
  if (S.vol || !c) return `<div class="phone-view" data-k="phone-view">${empty(L.previewOpenChapter())}</div>`
  const P = S.phone, [W, H] = PHONES[P.size], e = (px) => +(px / P.fs).toFixed(3) + 'em', flip = P.mode === '翻页'
  const on = S.open === 'phone-look'
  const step = (label, tip, ok, d) => `<div class="hit${ok ? '' : ' off'}" title="${tip}"${onClick(() => ok && phoneSet({ fs: P.fs + d }))}>${label}</div>`
  const look = '<div class="pop phone-pop" data-k="phone-pop">' +
    `<div class="pop-label">${t(L.previewScreen())}</div><div class="pop-seg">${seg(P.size, wordOpts(Object.keys(PHONES), SIZE_WORDS), (v) => phoneSet({ size: v }), L.previewScreen())}</div>` +
    `<div class="pop-label">${t(L.previewFont())}</div><div class="pop-seg"><div class="seg" aria-label="${t(L.previewFont())}">${step('A−', L.previewFontDown(), P.fs > 16, -1)}<div class="phone-fs">${P.fs}</div>${step('A+', L.previewFontUp(), P.fs < 24, 1)}</div></div>` +
    `<div class="pop-label">${t(L.previewLineSpacing())}</div><div class="pop-seg">${seg(P.lh, wordOpts(Object.keys(SPACING), SPACING_WORDS), (v) => phoneSet({ lh: v }), L.previewLineSpacing())}</div>` +
    `<div class="pop-label">${t(L.previewBackground())}</div><div class="pop-seg">${seg(P.theme, wordOpts(['白天', '夜间', '护眼'], THEME_WORDS), (v) => phoneSet({ theme: v }), L.previewBackground())}</div>` + HAIR +
    `<div class="phone-ticks">${tickbox(P.frame, L.previewFrame(), () => phoneSet({ frame: !P.frame }))}${tickbox(P.long, L.previewLongTick(LONG), () => phoneSet({ long: !P.long }))}</div>` +
    `<div class="pop-buttons"><div class="hit link"${onClick(() => set({ phone: { ...PHONE, mode: P.mode } }))}>${t(L.previewReset())}</div></div></div>`
  const tool = `<div class="tool phone-tool" data-k="tool">${seg(P.mode, wordOpts(['翻页', '滚动'], MODE_WORDS), (v) => phoneSet({ mode: v }), L.previewViewMode())}<div class="grow"></div>` +
    `<div class="more hit phone-aa${on ? ' on' : ''}" title="${t(L.previewLayout())}"${onClick(() => set({ open: on ? '' : 'phone-look' }))}>Aa</div>${on ? look : ''}</div>`
  const title = titleOf(c).replace('：', ' ')
  const paras = c.paras.map((p, i) => `<p data-phone="${i}"${onClick((ev) => putCaret({ para: i, off: phoneOff(ev) }, true))}>${t(p)}</p>`).join('')
  const screen = `<div class="phone-screen ${{ 白天: 'light', 夜间: 'dark', 护眼: 'light eye' }[P.theme]}" style="width:${e(W)};height:${e(H)}">` +
    `<div class="phone-status" style="height:${e(44)};padding:0 ${e(28)}"><span>${BOOK.clock}</span><span class="phone-battery"></span></div>` +
    `<div class="phone-page${flip ? '' : ' rolled'}" style="height:${e(H - 86)};margin:${e(8)} ${e(20)} 0">` +
    `<div class="phone-text lh-${SPACING[P.lh]}"${flip ? ` style="column-width:${e(W - 40)};column-gap:${e(40)}"` : ''}><div class="phone-title">${t(title)}</div>${paras}<i class="phone-caret" hidden></i></div></div>` +
    `<div class="phone-foot" style="height:${e(34)};padding:0 ${e(20)}"><span class="ell">${t(title)}</span><span class="phone-at"></span></div></div>`
  return '<div class="phone-view" data-k="phone-view">' + tool +
    `<div class="phone-stage"><div class="phone dark${P.frame ? ' framed' : ''}">${screen}</div></div>` +
    (flip ? `<div class="phone-nav"><div class="hit link phone-back"${onClick(() => phoneFlip(-1))}>‹ ${t(L.previewPrev())}</div><div class="phone-count"></div><div class="hit link phone-next"${onClick(() => phoneFlip(1))}>${t(L.previewNext())} ›</div></div>`
      : '<div class="phone-nav"><div class="grow"></div><div class="phone-count"></div><div class="grow"></div></div>') +
    `<div class="phone-long"><div class="hit link"${onClick(phoneNextLong)}></div></div></div>`
}

// 画完以后量：手机缩放到放得下，正文分成几屏，每段几行，正文里选中的字和光标落在预览的哪儿。只改量出来的那几处，不重画。
function fitPhone() {
  const view = document.querySelector('.phone-view .phone')
  if (!view) return
  const root = view.closest('.phone-view'), stage = root.querySelector('.phone-stage'), page = root.querySelector('.phone-page'), text = root.querySelector('.phone-text')
  const flip = S.phone.mode === '翻页'
  view.style.transform = ''
  const k = Math.min(1, stage.clientWidth / view.offsetWidth, stage.clientHeight / view.offsetHeight)
  view.style.transform = `scale(${k})`
  text.style.transform = ''
  const step = flip ? page.clientWidth + parseFloat(getComputedStyle(text).columnGap) : page.clientHeight
  const base = text.getBoundingClientRect(), lh = parseFloat(getComputedStyle(text.querySelector('p') || text).lineHeight), kz = k * winZoom() // 量出来的长度除以 kz 是手机上的长度
  const at = (r) => Math.floor(((flip ? r.left - base.left : r.top - base.top) / kz + 1) / step) // 一处字落在第几屏
  const paras = [...text.querySelectorAll('p')]
  const all = [...text.children].filter((el) => !el.matches('.phone-caret')).flatMap((el) => [...el.getClientRects()])
  phoneScreens = Math.max(1, all.length ? Math.max(...all.map(at)) + 1 : 1)
  // 正文里选中的字在预览里也选中；没选字时，正文有光标，预览也画一道。
  const span = spanNow(), spot = ({ para, off }) => { const p = paras[para], n = p && p.firstChild; return n ? [n, Math.min(off, n.length)] : p ? [p, 0] : null }
  const from = span && spot(span[0]), to = span && spot(span[1]), pick = new Range(), caret = root.querySelector('.phone-caret')
  if (from && to) { pick.setStart(...from); pick.setEnd(...to) }
  CSS.highlights.set('phone-pick', new Highlight(...(from && to && !pick.collapsed ? [pick] : [])))
  const end = to && new Range()
  if (end) { end.setStart(...to); end.collapse(true) }
  const mark = end && (end.getClientRects()[0] || end.startContainer.getClientRects?.()[0])
  const where = mark ? String(at(mark)) : ''
  caret.hidden = !(mark && pick.collapsed && document.activeElement === proseBox())
  if (mark) Object.assign(caret.style, { left: (mark.left - base.left) / kz + 'px', top: (mark.top - base.top) / kz + 'px', height: mark.height / kz + 'px' })
  // 换了一章：从第一屏看起，光标在哪不算挪过。
  if (S.ch !== phoneKey) { phoneKey = S.ch; phoneCaret = where; phoneAt = 0 }
  // 翻页看时，光标到了另一屏，预览跟过去；作者在预览里往回翻着看时，光标还在同一屏里打字，不把它拉回来。滚动看时跟着正文滚。
  if (flip && mark && where !== phoneCaret) { phoneCaret = where; phoneAt = at(mark) }
  phoneAt = Math.min(Math.max(phoneAt, 0), phoneScreens - 1)
  if (flip) text.style.transform = `translateX(${-phoneAt * step}px)`; else syncPhone()
  phoneLongAt = []
  paras.forEach((p) => {
    const rects = [...p.getClientRects()], lines = Math.round(rects.reduce((h, r) => h + r.height, 0) / kz / lh)
    const long = S.phone.long && lines > LONG
    p.classList.toggle('long', long)
    if (long) { p.title = L.previewLinesTip(lines); if (!phoneLongAt.includes(at(rects[0]))) phoneLongAt.push(at(rects[0])) } else p.removeAttribute('title')
  })
  const word = (el, s) => { if (el) el.textContent = s }
  word(root.querySelector('.phone-at'), flip ? `${phoneAt + 1}/${phoneScreens}` : '')
  word(root.querySelector('.phone-count'), flip ? L.previewScreenOf(phoneAt + 1, phoneScreens) : L.previewAbout(phoneScreens))
  root.querySelector('.phone-back')?.classList.toggle('off', phoneAt === 0)
  root.querySelector('.phone-next')?.classList.toggle('off', phoneAt === phoneScreens - 1)
  const longs = paras.filter((p) => p.classList.contains('long')).length
  word(root.querySelector('.phone-long .link'), longs ? (flip ? L.previewLongNext(longs, LONG) : L.previewLong(longs, LONG)) : '')
}

// 滚动看时，预览跟着正文滚：正文滚到几成，就取窗口里几成高的那一处，在预览里放到同样几成高的地方。滚到顶、到底，两边一起到顶、到底。
function syncPhone() {
  const page = document.querySelector('.phone-page.rolled'), paper = document.querySelector('.editor .paper.scroll'), box = proseBox()
  if (!page || !paper || !box) return
  const max = paper.scrollHeight - paper.clientHeight, f = max > 0 ? paper.scrollTop / max : 0
  const pr = paper.getBoundingClientRect(), y = pr.top + f * pr.height, all = blocks(box), mine = page.querySelectorAll('p')
  const n = all.findIndex((el) => el.getBoundingClientRect().bottom >= y)
  if (n < 0 || !mine[n]) { page.scrollTop = page.scrollHeight; return }
  const r = all[n].getBoundingClientRect(), into = Math.min(Math.max((y - r.top) / (r.height || 1), 0), 1)
  page.scrollTop = n === 0 && y < r.top ? 0 : mine[n].offsetTop + into * mine[n].offsetHeight - f * page.clientHeight
}
document.addEventListener('scroll', (ev) => { if (ev.target.matches?.('.editor .paper')) syncPhone() }, true)
document.addEventListener('selectionchange', () => { if (!drawing) fitPhone() })

contribute('PaneTabs', 'phone', { name: '预览', label: () => L.paneTabPreview(), draw: phoneView, badge: () => 0, beside: false }, 4)

contribute('AfterDraw', 'phone', () => fitPhone(), 0) // 手机预览排好以后才知道分成几屏

contribute('ViewActions', 'phone', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdPhonePreview(), writing ? () => set({ paneOpen: true, tab: '预览', ...SHUT }) : null), 2)

// 手机预览的排版：翻页还是滚动、哪种屏幕、字号、行距、背景、要不要手机外框、标不标长段
contribute('State', 'phone', () => ({
  phone: { ...PHONE }
}))

// 手机预览的排版，刚装好时的样子：翻页、393 宽的手机、20 号字。
const PHONE = { mode: '翻页', size: '标准', fs: 20, lh: '适中', theme: '白天', frame: true, long: true }
