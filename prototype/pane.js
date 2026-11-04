// 右栏的外壳和几页共用的小零件；各页由扩展登记。
'use strict'

// ───────── 动作 ─────────

// ───────── 画：三页都用的 ─────────

// 列表顶上：搜索、旁边的按钮（add）、「···」里的几项（more）。
function tool(hint, key, add, more) {
  const on = S.open === 'tool'
  return '<div class="tool" data-k="tool">' +
    `<div class="find box">${ICON.find}${typed(S[key], (v) => set({ [key]: v }), hint, hint)}</div>` +
    (add ? `<div class="add hit"${onClick(add[1])}>${ICON.plus}${t(add[0])}</div>` : '') +
    (more ? `<div class="more hit${on ? ' on' : ''}" title="${t(L.commonMore())}"${onClick(() => set({ open: on ? '' : 'tool' }))}>${ICON.dots}</div>` : '') +
    (more && on ? `<div class="tool-menu pop">${more.map(([label, n, pick]) => `<div class="pop-row row"${onClick(pick)}><div class="grow">${t(label)}</div><div class="side">${n || ''}</div></div>`).join('')}</div>` : '') +
    '</div>'
}
// 详情页顶上：返回哪里，右边还能做什么。
function back(label, goBack, actLabel, act) {
  return `<div class="back" data-k="back"><div class="back-to hit"${onClick(goBack)}>${ICON.left}<div class="ell">${t(label)}</div></div><div class="grow"></div>` +
    (act ? `<div class="hit link"${onClick(act)}>${t(actLabel)}</div>` : '') + '</div>'
}

// 右栏的几页，各归一个扩展；关着的不在。正文以外的页只有设定。
const paneItems = () => contributions('PaneTabs').filter((x) => S.page === 'write' || x.beside)
const paneTabs = () => paneItems().map((x) => x.name)

function pane() {
  // 标签上的数是那一页里等作者处理的几条：待确认的设定；原句已删、久未回收的伏笔（和待处理里的「伏笔」同一个数）。
  // 停在的那一页所属的扩展关了，这里显示设定（设定也关了就是第一页）；停在哪一页记着，重新打开后回到那一页。
  const tabs = paneTabs(), at = tabs.includes(S.tab) ? S.tab : tabs.includes('设定') ? '设定' : tabs[0]
  const tab = (name, n) => `<div class="tab hit${name === at ? ' on' : ''}"${onClick(() => set({ tab: name, ...SHUT }))}>${t(paneItems().find((x) => x.name === name).label())}${n ? `<span class="badge">${n}</span>` : ''}</div>`
  const edge = `<div class="pane-edge" title="${t(L.paneResize())}"></div>`, wide = S.paneW ? ` style="width:${S.paneW}px"` : ''
  // 正文那一页有四页（AI 关着时没有讨论）；别的页只留设定。
  // 别的页已经展开的右栏沿用原来的画法；开关是否可点由 paneHere 管。
  const beside = registered('PaneTabs').map((x) => x.value).find((x) => x.beside)
  if (S.page !== 'write') return `<div id="pane"${wide}>${edge}<div class="tabs"><div class="tab on">${t(beside.label())}${beside.badge() ? `<span class="badge">${beside.badge()}</span>` : ''}</div></div>${registered('BesideItems').map((x) => x.value()).join('')}</div>`
  // 批注不带数：几条就在页边，一眼看得到。
  return `<div id="pane" class="${at === '批注' ? 'margin-pane' : ''}"${wide}>${edge}` + `<div class="tabs">${tabs.map((name) => tab(name, paneItems().find((x) => x.name === name).badge())).join('')}</div>` +
    (paneItems().find((x) => x.name === at)?.draw() || '') + '</div>'
}
