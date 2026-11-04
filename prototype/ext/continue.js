// 扩展：continue。界面、动作和登记都在这里。
'use strict'

function continueText() {
  if (!chapterOf(S.ch) || S.ai) return
  set({ ctx: null, pick: null, bar: null, ...SHUT, ai: { kind: 'ghosting' } })
  later(700, () => set((s) => (s.ai && s.ai.kind === 'ghosting' ? { ai: { kind: 'ghost', text: BOOK.ghost }, records: noted(s, '续写', chTitle(s.ch), [['规则', 300], ['章纲', 120], ['光标前的正文', 2600], ['设定', 1820]], [5600, 5200, 24]) } : null)))
}
// 续写出的那一句：采纳就接在最后一段后面。
function ghost(keep) {
  set((s) => {
    if (!s.ai || s.ai.kind !== 'ghost') return null
    return { ai: null, ...(keep ? oneStep(s, s.ch, (paras) => (paras.length ? [...paras.slice(0, -1), paras.at(-1) + s.ai.text] : [s.ai.text])) : {}) }
  })
}

contribute('WriteActions', 'continue', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdContinue(), cur ? then(continueText) : null, '', 'Ctrl+J'), 0)

contribute('SelectionMenus', 'continue', (p) => menuRow(L.cmdContinue(), continueText, '', 'Ctrl+J'), 0)

contribute('WritingKeys', 'continue', { key: 'j', act: () => continueText() }, 0) // 续写关着时按了不起作用
