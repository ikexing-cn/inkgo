// 扩展：trash。界面、动作和登记都在这里。
'use strict'

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

contribute('Dialogs', 'trash', { kind: 'bin', draw: binDialog }, 0)

contribute('BookActions', 'trash', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdTrash(), inBook ? ask({ kind: 'bin', sure: '' }) : null), 1)

contribute('State', 'trash', ({ of }) => ({
  bin: of('bin', [])
}))
