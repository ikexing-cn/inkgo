// 扩展：fullwidth。界面、动作和登记都在这里。
'use strict'

contribute('SettingsSections', 'fullwidth', () => `<div class="ext-prefs"><div class="ticks">${tickbox(S.quotes, L.modFullwidthQuotes(), () => set({ quotes: !S.quotes }))}</div></div>`, 0)

// 假装另装的「全角标点」自己的一项：引号也换成「」
contribute('State', 'fullwidth', () => ({
  quotes: false
}))
