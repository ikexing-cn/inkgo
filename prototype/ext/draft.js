// 扩展：draft。界面、动作和登记都在这里。
'use strict'

function draftDialog(d) {
  const cur = chapterOf(S.ch), use = S.uses[0], prev = S.chapters[S.chapters.indexOf(cur) - 1]
  const type = (key) => (v) => set((s) => ({ dialog: { ...s.dialog, [key]: v } }))
  const cards = eventsOf(cur.key).length
  const sent = 1100 + S.entries.filter((e) => e.place === 'body').length * 260 + 600 + cards * 45 + (prev ? sizeOf(prev) : 0) // 规则、设定、章纲卷纲、本章的卡片、上一章全文
  const items = [cards ? L.draftItemCards(cards) : '', prev ? L.draftItemPrev() : '', cur.vol ? L.draftItemVolume() : L.draftItemBooks()].filter(Boolean).join(L.draftListSep())
  return dialog('small', dialogHead(L.cmdDraft(), L.draftAbout(cards ? L.draftAboutCards() : L.draftAboutOutline(), L.draftAboutFlow())) +
    `<div class="quote">${t(S.outlines[cur.key])}</div>` +
    field(L.draftGoal('character'), line(d.goal, type('goal'), L.draftGoalHint(), L.draftGoalLabel('character')) + (countOrNone(d.goal) ? '' : `<div class="note danger">${t(L.draftGoalInvalid('character'))}</div>`)) +
    (cur.paras.length ? field(L.draftExisting(), dots(d.how, [['接在已有内容之后', L.draftAppend()], ['覆盖已有内容', L.draftOverwrite()]], type('how'), L.draftExistingHow())) : '') +
    `<div class="note">${t(L.draftSends(items, tokens(sent)))}</div>` +
    `<div class="note">${t(L.draftUsing(use.model, use.effort))}</div>` +
    buttons(button(L.commonCancel(), shut), countOrNone(d.goal) ? button(L.draftStart(), draftNow, 'primary') : button(L.draftStart(), null, 'primary off')))
}

// 两份稿子并起来看：old 里有、now 里没有的划掉，now 里新写的标绿。

// 按章纲起草：照对话框里填的，在一段新的讨论里替作者说出要求。AI 写好的在正文里提议，作者在讨论里接着说还能改。
function draftNow() {
  const d = S.dialog, cur = chapterOf(S.ch)
  if (!countOrNone(d.goal)) return
  set({ dialog: null, ...SHUT, bar: null })
  const over = cur.paras.length > 0 && d.how === '覆盖已有内容'
  startTalk({ origin: '起草', key: cur.key, over, title: L.sideDraftTitle(titleOf(cur)), text: '按章纲起草本章' + (d.goal.trim() ? '，约 ' + d.goal.trim() + ' 字' : '') + (cur.paras.length ? '，' + d.how : '') })
}

contribute('Dialogs', 'draft', { kind: 'draft', draw: draftDialog }, 0)

contribute('WriteActions', 'draft', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdDraft(), outline ? ask({ kind: 'draft', goal: '', how: '接在已有内容之后' }) : null, '', cur && !outline ? L.cmdDraftNeedsOutline() : ''), 1)
