// 扩展：propose。界面、动作和登记都在这里。
'use strict'

function askRewrite() {
  const pick = grab() || S.pick
  if (!pick) return
  set({ ctx: null, pick: null, bar: null, ai: { kind: 'ask', pick, want: '' } })
  unselect()
}
// 改写要写明要求：空着时按钮不可点，回车也不发。发出去就是一段新的讨论，第一句是选中的那段和要求；AI 在正文里提议怎么改。
function rewrite() {
  const { pick, want } = S.ai, cur = chapterOf(S.ch)
  if (!want.trim()) return
  set({ ai: null })
  startTalk({ origin: '改写', key: cur.key, title: L.sideRewriteTitle(clip(pick.text, 8)), text: want.trim(), cite: { key: cur.key, title: titleOf(cur), quote: pick.text } })
}
// 关掉改写要求：键盘回到正文，光标在选中的那段后面。
function dropAI() {
  const pick = S.ai && S.ai.pick
  set({ ai: null })
  if (pick) putCaret({ para: pick.para, off: pick.end }, true)
}

// ───────── 动作：AI 的提议 ─────────

// 一处提议写进正文的样子：接在章末、换掉全章，或者换掉一句。
function applied(paras, p) {
  if (p.at.kind === 'end') return [...paras, ...p.paras]
  if (p.at.kind === 'all') return [...p.paras]
  const i = paras.findIndex((x) => x.includes(p.at.quote))
  return i < 0 ? paras : paras.map((x, n) => (n === i ? x.replace(p.at.quote, p.paras.join('')) : x))
}
// 几处提议定了：从正文里拿掉，讨论里那一条照 why 写。
const ruled = (s, ids, why) => ({ props: s.props.filter((p) => !ids.includes(p.id)), fate: { ...s.fate, ...Object.fromEntries(ids.map((id) => [id, why])) } })
// 作者改了提议要换掉的那几句，提议作废；改别处不碍事，接在章末的不会作废。还在写的，写完再看。
function unfit(s, key, old, paras) {
  const gone = s.props.filter((p) => p.key === key && !p.writing && (p.at.kind === 'all' ? old.join('\n') !== paras.join('\n') : p.at.kind === 'swap' && !paras.some((x) => x.includes(p.at.quote))))
  if (!gone.length) return {}
  later(0, () => say(L.editorSuggestionsVoided(gone.length), L.editorVoidedWhy()))
  return ruled(s, gone.map((p) => p.id), '正文已改动，提议已作废')
}
// 一章里还没定的提议。
const propsOf = (key) => S.props.filter((p) => p.key === key)
// 采纳：先存快照，再写进正文，算一步，撤销一次就退回去。写进去的字带「AI 写的」记号。还在写的不能采纳。
function takeProps(key, ids) {
  const list = S.props.filter((p) => ids.includes(p.id) && !p.writing)
  if (!list.length) return
  // 快照里记下这些提议来自哪几段讨论（删了的记 null），显示时再写成字。
  const talks = [...new Set(list.map((p) => p.talk))].map((id) => S.talks.find((x) => x.id === id)), words = list.flatMap((p) => p.paras)
  set((s) => {
    const step = oneStep(s, key, (paras) => list.reduce(applied, paras))
    return { ...snap(s, key, 'AI 改动前', { talks: talks.map((x) => (x ? x.title : null)) }), ...step, ...ruled(s, list.map((p) => p.id), '已采纳'),
      chapters: step.chapters.map((c) => (c.key === key ? { ...c, ai: [...(c.ai || []), ...words] } : c)) }
  })
  say(L.editorAccepted(), L.editorAcceptedUndo(), 'ok')
  // 光标放在采纳的最后那段字后面：马上按 Ctrl+Z 就撤销。
  const paras = chapterOf(key).paras, last = words.at(-1), i = paras.findIndex((x) => x.includes(last))
  if (key === S.ch && i >= 0) putCaret({ para: i, off: paras[i].indexOf(last) + last.length }, true)
}
// 放弃：从正文里拿掉。还在写的那一处，放弃就让 AI 停下。
function dropProps(ids) {
  if (S.props.some((p) => ids.includes(p.id) && p.writing)) stopReply()
  set((s) => ruled(s, ids.filter((id) => s.props.some((p) => p.id === id)), '已放弃'))
  if (proseBox()) proseBox().focus({ preventScroll: true })
}

// 改稿建议：采纳了的 AI 的字带「AI 写的」记号，画一道下划线；作者改过的那几个字不再带。正文标记关着时不画，改稿建议照常能用。
contribute('MarginSources', 'propose', (cur) => (cur.ai || []).map((words) => ({ quote: words, cls: 'ai', id: '' })), 2)

// AI 提议的字是绿色的，不能在里面打字，读正文时跳过它（data-prop）。
const mine = (p) => ` data-prop="${p.id}" contenteditable="false"`
// 一处提议后面的「放弃」「采纳」。还在写的是「放弃」和后面的「正在写…」，放弃就让 AI 停下；写完以后「采纳」换掉「正在写…」，「放弃」不挪地方，写完那一刻点它不会落到「采纳」上。
const propActs = (p) => `<span class="prop-acts"${mine(p)}>` +
  `<span class="btn hit soft" title="${p.writing ? L.editorStopDrop() : L.editorDropOne()}"${onClick(() => dropProps([p.id]))}>${L.commonDiscard()}</span>` +
  (p.writing ? `<span class="prop-writing">${L.editorWriting()}</span>` : `<span class="btn hit primary" title="${L.editorAcceptOne()}"${onClick(() => takeProps(p.key, [p.id]))}>${L.editorAccept()}</span>`) + '</span>'

// 标题下面那一行：等作者写改写要求，或者本章有 AI 的提议。提议来自哪段讨论，点「查看讨论」在右栏打开它。
function aiBar(cur) {
  const a = S.ai
  if (a && a.kind === 'ask') return `<div class="ai-bar" data-k="ai-bar">${icon('pen', 'md', 'muted')}<div class="box line fit">${typed(a.want, (v) => set((s) => ({ ai: { ...s.ai, want: v } })), L.editorRewriteHint(), L.editorRewriteLabel(), keys(rewrite, dropAI) + ' autofocus')}</div>${button(L.editorRewrite(), a.want.trim() ? rewrite : null, a.want.trim() ? 'primary' : 'primary off')}<button class="icon-btn muted" aria-label="${L.editorCloseRewrite()}" title="${L.editorCloseRewrite()}"${onClick(dropAI)}>${icon('cross')}</button></div>`
  const list = propsOf(cur.key)
  // 起草、改写刚发出去，AI 还在想、在查，没开始写：也在这里说一声，能取消。
  const w = S.waiting, asked = w && w.write && !list.length && S.talks.find((x) => x.id === w.id)
  if (asked && asked.key === cur.key) {
    const shown = S.paneOpen && !S.focus && S.tab === '讨论' && S.tv === 'one' && S.tid === w.id
    return `<div class="ai-bar" data-k="ai-bar">${icon('spin', 'md', 'muted')}<div class="fit ell muted">${w.step === 'look' ? L.editorLooking() : L.toastThinking()}</div>` +
      (shown ? '' : `<div class="hit link"${onClick(() => openTalk(w.id))}>${L.editorSeeTalk()}</div>`) + button(L.commonCancel(), stopReply, 'soft') + '</div>'
  }
  if (!list.length) return ''
  const writing = list.some((p) => p.writing), ids = list.map((p) => p.id), talk = list.at(-1).talk
  const what = writing ? L.editorWritingSuggestions() : L.editorSuggestionsHere(list.length)
  const how = list.some((p) => p.at.kind !== 'end') ? L.editorHowReplace() : L.editorHowAppend()
  const shown = S.paneOpen && !S.focus && S.tab === '讨论' && S.tv === 'one' && S.tid === talk
  return `<div class="ai-bar" data-k="ai-bar">${icon('spark', 'md', writing ? 'muted' : 'green')}<div class="fit ell muted"${writing ? '' : ` title="${how}"`}>${t(what)}</div>` +
    (shown ? '' : `<div class="hit link"${onClick(() => openTalk(talk))}>${L.editorSeeTalk()}</div>`) +
    // 还在写时「全部采纳」是灰的，按钮都不挪地方。
    button(L.editorDropAll(), () => dropProps(ids), 'soft') + button(L.editorAcceptAll(), writing ? null : () => takeProps(cur.key, ids), writing ? 'primary off' : 'primary') + '</div>'
}

const whereWord = (s) => ({ 接在章末: L.chatWhereEnd(), 换掉全章: L.chatWhereAll(), 换掉一句: L.chatWhereSwap() })[s] || s

const FATE = () => ({ 已被新的提议替换: L.chatFateReplaced(), 已采纳: L.chatFateAdopted(), '正文已改动，提议已作废': L.chatFateChanged(), 已放弃: L.hooksDropped(), '关闭 AI 时已丢弃': L.chatFateAiOff(), '作品关闭时已丢弃': L.chatFateBookClosed() })

contribute('SelectionMenus', 'propose', (p) => menuRow(L.editorAIRewriteMore(), p ? askRewrite : null), 1)

contribute('State', 'propose', () => ({
  props: [],
  fate: {}
}))
