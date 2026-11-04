// 扩展：names。界面、动作和登记都在这里。
'use strict'

// 点正文里的名字：右栏打开那一条设定，光标留在正文里。
const seeName = (id) => () => set({ paneOpen: true, focus: false, tab: '设定', sv: 'list', sid: id, form: null })

contribute('SettingsSections', 'names', () => `<div class="ext-prefs"><div class="ticks">${tickbox(S.names, L.settingsMarkNames(), () => set({ names: !S.names }))}</div></div>`, 0)

// 正文里标出设定的名字
contribute('State', 'names', () => ({
  names: true
}))

contribute('NameHighlights', 'names', (text, cuts) => {
  if (S.names) for (const e of S.entries) if (e.place === 'body') for (const n of e.names) for (let at = text.indexOf(n); at >= 0; at = text.indexOf(n, at + n.length)) cuts.push({ a: at, b: at + n.length, cls: 'nm', id: '', entry: e.id })
}, 0)
