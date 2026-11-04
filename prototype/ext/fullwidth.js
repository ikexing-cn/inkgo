// 扩展：fullwidth。界面、动作和登记都在这里。
'use strict'

contribute('SettingsSections', 'fullwidth', () => `<div class="ext-prefs"><div class="ticks">${tickbox(S.quotes, L.modFullwidthQuotes(), () => set({ quotes: !S.quotes }))}</div></div>`, 0)

// 它在编辑器处理按键之前换掉半角标点：详情页「介入」里列出这一步。
contribute('Injections', 'fullwidth', { where: () => L.modInjectKeys(), kind: 'before' }, 0)

// 假装另装的「全角标点」自己的一项：引号也换成「」
contribute('State', 'fullwidth', () => ({
  quotes: false
}))
