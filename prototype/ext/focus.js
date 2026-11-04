// 扩展：focus。界面、动作和登记都在这里。
'use strict'

contribute('ViewActions', 'focus', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdFocus(), writing ? () => set({ focus: true, ...SHUT }) : null, '', 'F11'), 1)

contribute('Tools', 'focus', (cur, wide, here, tool, flip) => tool(S.focus ? L.tabExitFocus() : L.cmdFocus(), S.focus ? L.tabExitFocusTip(keyLabel('F11')) : L.tabFocusTip(keyLabel('F11')), 'focus', () => set({ focus: !S.focus, ...SHUT }), S.focus), 1)

contribute('State', 'focus', () => ({
  focus: false
}))

contribute('FocusKeys', 'focus', (ev, writing, run) => {
  if ((ev.key === 'F11' || (ev.ctrlKey && ev.metaKey && ev.key.toLowerCase() === 'f')) && writing) { run(() => set({ focus: !S.focus, ...SHUT })); return true }
}, 0)
