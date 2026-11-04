// 扩展：cats。界面、动作和登记都在这里。
'use strict'

// ───────── 作品分类 ─────────

function saveCat() {
  const name = S.catForm.name.trim(), land = S.catForm.land.trim()
  if (!name) { say(L.catsNameEmpty()); return }
  if (!land) { say(L.catsLandEmpty()); return }
  set((s) => ({ cats: s.cats.includes(name) ? s.cats : [...s.cats.filter((c) => c !== '其他'), name, '其他'], lands: { ...s.lands, [name]: land }, catForm: { name: '', land: '' } }))
  say(L.catsSaved(name), '', 'ok')
}
function dropCat(name) {
  set((s) => ({ cats: s.cats.filter((c) => c !== name), entries: s.entries.map((e) => (e.cat === name ? { ...e, cat: '其他' } : e)), catDrop: '' }))
  say(L.catsDeleted(name), '', 'ok')
}

// 删除分类：有设定的先问一句（设定要移到「其他」），空的直接删。
function catsPage() {
  const count = (c) => S.entries.filter((e) => e.cat === c && e.place !== 'dropped').length
  const type = (key) => (v) => set((s) => ({ catForm: { ...s.catForm, [key]: v } }))
  // 同名的是改那一条，不是加：按保存以前就说是哪条、现在落在哪，也能「取消」不改了。
  const name = S.catForm.name.trim()
  const what = !name ? '' : S.cats.includes(name) ? L.catsWillEdit(name, S.lands[name] || L.catsLandDefault()) : L.catsWillAdd()
  const row = (c) => (S.catDrop === c
    ? `<div class="cat-row"><div class="fit">${t(L.catsMoveTo(count(c), '其他'))}</div>${button(L.commonCancel(), () => set({ catDrop: '' }))}${button(L.catsConfirmDelete(), () => dropCat(c), 'danger')}</div>`
    : `<div class="cat-row hover-acts"><div class="cat-name">${t(c)}</div><div class="fit muted">${t(S.lands[c] || L.catsLandDefault())}</div><div class="side">${t(L.catsCount(count(c)))}</div><div class="hit link"${onClick(() => { set({ catForm: { name: c, land: S.lands[c] || L.catsLandDefault() } }); document.querySelector('.cat-form').scrollIntoView({ block: 'nearest' }) })}>${t(L.catsEdit())}</div>` +
      (c === '其他' ? `<div class="off">${t(L.commonDelete())}</div>` : `<div class="hit link"${onClick(() => (count(c) ? set({ catDrop: c }) : dropCat(c)))}>${t(L.commonDelete())}</div>`) + '</div>')
  return '<div class="editor"><div class="paper scroll"><div class="column narrow">' +
    `<div class="page-title">${t(L.cmdCategories())}</div><div class="note">${t(L.catsNote())}</div>` +
    `<div class="cat-rows"><div class="cat-row cols"><div class="cat-name">${t(L.catsName())}</div><div class="fit">${t(L.catsDest())}</div></div>${S.cats.map(row).join('')}</div>` +
    `<div class="group-title">${t(L.catsFormTitle())}</div>` +
    `<div class="cat-form">${field(L.catsName(), line(S.catForm.name, type('name'), '', L.catsNameLabel()))}${field(L.catsDest(), line(S.catForm.land, type('land'), L.catsLandHint(L.catsLandDefault()), L.catsDest(), keys(saveCat)))}${S.cats.includes(name) ? button(L.commonCancel(), () => set({ catForm: { name: '', land: '' } })) : ''}${S.catForm.name.trim() && S.catForm.land.trim() ? button(L.catsSave(), saveCat, 'primary') : button(L.catsSave(), null, 'primary off')}</div>` +
    (what ? `<div class="note">${t(what)}</div>` : '') +
    '</div></div></div>'
}

contribute('Pages', 'cats', { id: 'cats', label: null, draw: catsPage, order: 4 }, 4)

contribute('BookActions', 'cats', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdCategories(), inBook ? toPage('cats') : null), 0)

// 作品分类：正在填的、要删的
contribute('State', 'cats', () => ({
  cats: [...BOOK.cats],
  lands: { ...BOOK.lands },
  catForm: { name: '', land: '' },
  catDrop: ''
}))
