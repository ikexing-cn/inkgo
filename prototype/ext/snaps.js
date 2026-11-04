// 扩展：snaps。界面、动作和登记都在这里。
'use strict'

function compare(old, now) {
  const out = []
  let i = 0, j = 0
  while (i < old.length || j < now.length) {
    if (i < old.length && old[i] === now[j]) { out.push(['', old[i]]); i++; j++ } else if (j < now.length && !old.includes(now[j])) out.push(['add', now[j++]])
    else if (i < old.length) out.push(['cut', old[i++]])
    else out.push(['add', now[j++]])
  }
  return out.map(([cls, text]) => `<div${cls ? ` class="${cls}"` : ''}>${t(text)}</div>`).join('')
}

// 「AI 改动前」的快照写着改动来自哪几段讨论，几个名字按界面语言连起来（「甲」和「乙」、“A” and “B”）。
const aiFrom = (talks) => L.snapAIFrom(new Intl.ListFormat(LANG.replace(/-XA$/, '')).format(talks.map((x) => (x === null ? L.snapAITalkGone() : L.snapAITalk(x)))))
// 快照是在什么之前存的。AI 关着时，「AI 改动前」只写「改动前」：快照还在，不提 AI。
const snapKind = (kind) => ({ 恢复前: L.snapKindRestore(), 'AI 改动前': L.snapKindAI(), 改动前: L.snapKindEdit(), 外部改动: L.snapKindOutside(), 全书替换前: L.snapKindSwap(), 写作中: L.snapKindWriting(), 打开时: L.snapKindOpening() })[kind === 'AI 改动前' && !aiOn() ? '改动前' : kind] ?? kind

function snapsDialog(d) {
  const cur = chapterOf(S.ch), list = S.snaps[cur.key] || []
  const shown = list.find((x) => x.id === d.id) || list[0]
  const pick = (change) => () => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const restore = () => {
    set((s) => ({ dialog: null, ...snap(s, cur.key, '恢复前'), ...retype(s, cur.key, () => shown.paras), bar: null, pick: null })) // 选中的那几个字换掉了，浮条跟着收起
    say(L.snapRestored(timeShown(shown.at)), '', 'ok')
  }
  const same = shown && shown.paras.join('\n') === cur.paras.join('\n')
  return dialog('wide', dialogHead(L.snapTitle(titleOf(cur)), L.snapAbout(aiOn() ? L.snapAboutAI() : '')) +
    (shown
      ? `<div class="snaps"><div class="snap-list scroll">${list.map((x) => `<div class="snap-row row${x === shown ? ' on' : ''}"${onClick(pick({ id: x.id }))}><div>${t(timeShown(x.at))}</div><div class="note">${t(L.snapRowInfo(snapKind(x.kind), x.paras.join('').length, 'character'))}</div></div>`).join('')}</div>` +
        `<div class="snap-side"><div class="snap-tools">${tickbox(d.diff, L.snapCompare(), pick({ diff: !d.diff }))}<div class="fit note ell">${same ? t(L.snapSame()) : d.diff ? t(L.snapDiffHint()) : ''}</div></div>` +
        (shown.ai && aiOn() ? `<div class="card"><div class="who">${t(aiFrom(shown.ai.talks))}</div>${shown.ai.saw ? `<div class="extra">${t(shown.ai.saw)}</div>` : ''}</div>` : '') +
        `<div class="snap-text scroll">${d.diff ? compare(shown.paras, cur.paras) : shown.paras.map((p) => `<div>${t(p)}</div>`).join('') || `<div class="faint">${t(L.snapEmpty())}</div>`}</div></div></div>`
      : `<div class="empty"><div>${t(L.snapNone())}</div></div>`) +
    buttons(button(L.commonClose(), shut), button(L.snapRestore(), shown && !same ? restore : null, shown && !same ? 'primary' : 'primary off')))
}

// 导出：正文、大纲、设定勾几样导几样，各成一个文件。勾的几样、格式和笔名都记着，范围每次从「全书」起。

const snap = (s, key, kind, ai) => ({ snaps: { ...s.snaps, [key]: [{ id: 's' + s.n, at: stamp(), kind, ai, paras: chapterOf(key).paras }, ...(s.snaps[key] || [])] }, n: s.n + 1 })

contribute('Dialogs', 'snaps', { kind: 'snaps', draw: snapsDialog }, 0)

contribute('FileActions', 'snaps', (cur, inBook, writing, then, ask, outline) => menuRow(L.cmdSnapshots(), cur ? ask({ kind: 'snaps', id: '', diff: false }) : null), 0)

contribute('State', 'snaps', ({ last, snaps }) => ({
  snaps: last ? { [last.key]: snaps } : {}
}))
