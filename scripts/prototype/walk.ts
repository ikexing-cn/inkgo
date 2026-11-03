/**
 * Clicks through everything the prototype can do, the way its author would,
 * and throws where it stops behaving. Each line of steps ends by saying what
 * should now be on the page; change the prototype, change its steps here.
 */
import type { Page } from './browser.ts'

export async function walk({ js, shot, click: press, rclick, drag, type, text, key, clear, sleep }: Page, said: (step: string) => void): Promise<void> {
  const flat = (t: string) => t.replace(/\s+/g, '')
  const reads = (where: string) => async (words: string, want = true) => {
    const text = await js(`(document.querySelector('${where}') || { innerText: '' }).innerText`)
    if (flat(text).includes(flat(words)) !== want) throw new Error((want ? 'missing in ' : 'unexpected in ') + where + ': ' + words)
  }
  const pane = reads('#pane'), win = reads('#win')
  const margined = async (n: number) => { const got = await js(`document.querySelectorAll('.mnote').length`); if (got !== n) throw new Error('the margin holds ' + got + ' notes, not ' + n) }
  const must = (ok: boolean, words: string) => { if (!ok) throw new Error(words) }
  // The settings, the categories and the trash open from the file menu.
  const settings = async () => { await click('文件'); await click('设置…') }
  const cats = async () => { await click('文件'); await click('作品分类') }
  const bin = async () => { await click('文件'); await click('回收站…') }
  const go = (page: string) => (page === '作品分类' ? cats() : click(page))
  // Where the first element sel picks stands in the window, to see that a switch did not move.
  const spot = (sel: string): Promise<string> => js(`(() => { const e = document.querySelector(${JSON.stringify(sel)}); if (!e) return 'absent'; const r = e.getBoundingClientRect(); return Math.round(r.left) + ',' + Math.round(r.top) })()`)
  // The mark a toast wears: the class of its icon, 'none', or 'gone' when no toast says words.
  const toastMark = (words: string): Promise<string> => js(`(() => { const t = [...document.querySelectorAll('.toast')].find((x) => x.textContent.includes(${JSON.stringify(words)})); const m = t && t.querySelector(':scope > svg'); return t ? (m ? m.getAttribute('class') : 'none') : 'gone' })()`)
  // Buttons stand at the right of their row, the one that goes ahead last: DESIGN.md has it as a rule. The question
  // of saving is the one row in the system's order: on Windows (.asked) the one that goes ahead first; on macOS
  // (.asked-mac) 不保存 stands apart at the left and the rest keep the rule.
  // After every click, each row that holds a button is looked at; what is found is told at the end, all at once.
  const loose = new Set<string>()
  const LOOSE = `(() => {
    const out = [], seen = new Set(), shown = (e) => e.getClientRects().length > 0, isButton = (e) => e.matches('.btn')
    for (const b of document.querySelectorAll('#win .btn, #win .link')) {
      const row = b.parentElement
      if (seen.has(row) || !shown(b)) continue
      seen.add(row)
      const kids = [...row.children].filter(shown), last = kids.at(-1), box = row.getBoundingClientRect(), css = getComputedStyle(row)
      // Words to press count as buttons where the row holds nothing else; one after a sentence goes with the sentence.
      if (!kids.some(isButton) && !kids.every((e) => e.matches('.hit'))) continue
      const gap = box.right - parseFloat(css.paddingRight) - parseFloat(css.borderRightWidth) - last.getBoundingClientRect().right
      const main = kids.find((e) => e.matches('.btn.primary, .btn.danger'))
      // From the first button on they stand one against the next: none is left of a stretch of nothing.
      const from = kids.findIndex((e, i) => isButton(e) && (!row.matches('.asked-mac') || i > 0)), apart = parseFloat(css.columnGap) || 0
      const parted = from >= 0 && kids.slice(from + 1).some((e, i) => e.matches('.grow') || e.getBoundingClientRect().left - kids[from + i].getBoundingClientRect().right > apart + 1.5)
      if (gap > 1.5) out.push(row.className + ' is not at the right: ' + row.innerText.replace(/\\s+/g, ' ').slice(0, 40))
      else if (parted) out.push(row.className + ' has a button left of a gap: ' + row.innerText.replace(/\\s+/g, ' ').slice(0, 40))
      else if (row.matches('.asked') ? kids[0] !== main : main && kids.slice(kids.indexOf(main) + 1).some(isButton)) out.push(row.className + ' has a button after the main one: ' + row.innerText.replace(/\\s+/g, ' ').slice(0, 40))
    }
    return out
  })()`
  // After every click, whatever can be clicked is looked at too: the pointer over it has to show, by a rule of the
  // stylesheet for :hover that fits it, and the pointer has to turn into a hand. The one already chosen among a
  // set of choices is passed over: it is where the author is.
  const dead = new Set<string>()
  const DEAD = `(() => {
    const hover = window.__hover || (window.__hover = [])
    if (!hover.length) for (const sheet of document.styleSheets) {
      let rules = []
      try { rules = [...sheet.cssRules] } catch { continue }
      for (const rule of rules) for (const s of (rule.selectorText || '').split(',')) {
        const at = s.indexOf(':hover')
        if (at >= 0 && !s.slice(0, at).endsWith(':not(')) hover.push(s.slice(0, at).trim())
      }
    }
    const out = []
    for (const e of document.querySelectorAll('#win [data-click]')) {
      if (!e.getClientRects().length || e.closest('.off') || e.matches('.scrim, .veil, .nav.on, .tab.on, .seg > .on')) continue
      const name = (e.className || e.tagName).trim().replace(/\\s+/g, ' ')
      if (!hover.some((s) => e.matches(s))) out.push(name + ' does not answer the pointer: ' + e.innerText.replace(/\\s+/g, ' ').slice(0, 24))
      else if (getComputedStyle(e).cursor !== 'pointer') out.push(name + ' has no hand for a pointer')
    }
    return out
  })()`
  // And a dialog is not a flat strip: it is no wider than FLATTEST times its height.
  const squat = new Set<string>()
  const FLATTEST = 2.5
  const FLAT = `[...document.querySelectorAll('#win .dialog')].map((e) => [e, e.getBoundingClientRect()]).filter(([, r]) => r.width > ${FLATTEST} * r.height).map(([e, r]) => e.className + ' is ' + Math.round(r.width) + ' by ' + Math.round(r.height) + ': ' + e.innerText.slice(0, 24))`
  // After every click, nothing may be wider than the menu, popover, dialog or toast it is in. One that holds an open
  // menu of its own is passed over: a submenu is meant to stand beside it.
  const OUT = `(() => { const e = [...document.querySelectorAll('.pop, .float, .dialog, .toast')].find((e) => !e.querySelector('.pop') && e.scrollWidth > e.clientWidth + 1); return e ? e.className + ': ' + e.innerText.slice(0, 30) : '' })()`
  const click = async (words: string, nth = 0) => {
    await press(words, nth)
    const seen = await js(`({ out: ${OUT}, loose: ${LOOSE}, dead: ${DEAD}, flat: ${FLAT} })`)
    must(!seen.out, 'after clicking ' + words + ', something sticks out of ' + seen.out)
    for (const row of seen.loose) loose.add(row)
    for (const e of seen.dead) dead.add(e)
    for (const e of seen.flat) squat.add(e)
  }
  // Brings the pointer over the row of the open menu saying words.
  const over = (words: string) => js(`[...document.querySelectorAll('.ctx .pop-row')].find((e) => e.innerText.includes(${JSON.stringify(words)})).dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))`)
  // Brings the pointer over the name of a menu in the title bar.
  const pointAt = (name: string) => js(`[...document.querySelectorAll('.menu-title')].find((e) => e.innerText === ${JSON.stringify(name)}).dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))`)
  // The tabs: the chapters open, left to right, which of them is shown, and whether its tab says it is not saved.
  const tabs = (): Promise<string> => js(`[...document.querySelectorAll('.sheet')].map((e) => e.dataset.tab + (e.classList.contains('on') ? '*' : '')).join(' ')`)
  const unsaved = (): Promise<boolean> => js(`!!document.querySelector('.sheet.on.dirty')`)
  const tabAt = (key: string): Promise<[number, number]> => js(`(() => { const r = document.querySelector('[data-tab="${key}"]').getBoundingClientRect(); return [r.x + 20, r.y + r.height / 2] })()`)
  const shut = (key: string) => js(`document.querySelector('[data-tab="${key}"] .sheet-x').click()`)
  // Where a row of the sidebar's tree is, and opening the outline of a volume from its menu: a click on it only folds it.
  const row = (words: string): Promise<[number, number]> => js(`(() => { const e = [...document.querySelectorAll('.tree-row')].find((e) => e.innerText.replace(/\\s+/g, '').includes(${JSON.stringify(words)})); e.scrollIntoView({ block: 'nearest' }); const r = e.getBoundingClientRect(); return [r.x + 60, r.y + r.height / 2] })()`)
  const outline = async (volume: string) => { await rclick(...(await row(volume))); await win('编辑卷纲'); await click('编辑卷纲') }
  // The prose as it is typed in: how many paragraphs, where the caret is, and putting it somewhere.
  const paras = (): Promise<number> => js(`document.querySelectorAll('[data-para]').length`)
  const caretAt = (): Promise<string> => js(`(() => { const s = spanNow(); return s ? s[1].para + ':' + s[1].off : 'nowhere' })()`)
  const caret = (para: number, off: number) => js(`putCaret({ para: ${para}, off: ${off} })`)
  // A flow is done when nothing in the window sticks out of it: what does can scroll the whole window aside.
  const done = async (step: string) => {
    must(await js(`(() => { const w = document.querySelector('#win'); return w.scrollWidth <= w.clientWidth && w.scrollHeight <= w.clientHeight && !w.scrollLeft && !w.scrollTop })()`), 'something sticks out of the window after: ' + step)
    said(step)
  }

  // Selects words in a paragraph of the open chapter, and says where they are on the page.
  const select = (para: number, words: string): Promise<[number, number]> => js(`(() => {
    const p = document.querySelector('[data-para="${para}"]')
    const a = p.textContent.indexOf(${JSON.stringify(words)}), b = a + ${words.length}
    if (a < 0) throw new Error('no such words: ' + ${JSON.stringify(words)})
    const walk = document.createTreeWalker(p, NodeFilter.SHOW_TEXT), r = document.createRange()
    let n, at = 0, begun = false
    while ((n = walk.nextNode())) {
      const end = at + n.data.length
      if (!begun && a < end) { r.setStart(n, a - at); begun = true }
      if (b <= end) { r.setEnd(n, b - at); break }
      at = end
    }
    const s = getSelection(); s.removeAllRanges(); s.addRange(r)
    const box = r.getBoundingClientRect()
    return [box.x + box.width / 2, box.y + box.height / 2]
  })()`)
  const menu = async (para: number, words: string) => { const [x, y] = await select(para, words); await rclick(x, y) }
  // How a sentence of the prose is marked: 'sel' picked, 'fs' a foreshadowing, 'fs on' the one looked at, '' plain, 'absent' not a piece of its own.
  const drawn = (words: string): Promise<string> => js(`(() => { const s = [...document.querySelectorAll('[data-para] span')].find((s) => s.textContent === ${JSON.stringify(words)}); return s ? s.className : 'absent' })()`)
  const greyed = (words: string): Promise<boolean> => js(`[...document.querySelectorAll('#win .row')].find((e) => e.innerText.trim() === ${JSON.stringify(words)}).classList.contains('off')`)

  await win('第87章：停电'); await margined(3); await pane('设定6'); await shot('01-open')

  // The margin: what this chapter has to answer, each note level with the words it is about, in their order.
  const notes = (): Promise<string> => js(`[...document.querySelectorAll('.mnote')].map((n) => n.querySelector('.mnote-title').innerText).join(' | ')`)
  must(await notes() === '林晚的设定从本章起有变化 | 小祥为什么突然改口叫她“姐” | 林晚怕黑，却独自走进黑巷', 'the margin reads: ' + await notes())
  const level = (): Promise<boolean> => js(`(() => {
    let below = -Infinity
    for (const n of document.querySelectorAll('.mnote')) {
      const r = spanAt(n.dataset.at).getBoundingClientRect(), top = n.getBoundingClientRect().top
      if (Math.abs(top + 3 - r.top) > 1 && top - below > 9) return false // where it does not stand by its words, the one above pushed it down
      below = n.getBoundingClientRect().bottom
    }
    return true
  })()`)
  must(await level(), 'a note does not stand by the words it is about')
  await js(`document.querySelector('.paper').scrollTop = 120`); await sleep(50); must(await level(), 'the notes do not follow the prose as it scrolls'); await js(`document.querySelector('.paper').scrollTop = 0`)
  await click('小祥为什么突然改口叫她“姐”'); must(await js(`document.querySelector('#pane .tab.on').firstChild.textContent`) === '伏笔', 'a note of a foreshadowing does not open it'); await pane('她把那句话又听了一遍'); await click('批注')
  await click('林晚怕黑，却独自走进黑巷'); await win('1 / 5'); await win('其他章节的类似情况'); await click('正文'); await margined(3)
  await click('忽略'); await margined(2); await pane('林晚的设定从本章起有变化', false); await win('已忽略'); await click('撤销'); await margined(3); await pane('林晚的设定从本章起有变化')
  await click('忽略'); await margined(2); await shot('01b-margin')
  // Taking the change the AI saw settles the mismatch the check found against the same setting in this chapter; one undo brings both back.
  await click('重新开始'); await click('采纳'); await margined(1); await win('已记到「林晚」· 第87章起'); await win('检查4'); await click('撤销'); await margined(3); await win('检查5'); await done('the margin')
  await click('重新开始'); await margined(3); await click('设定')

  // The setting, and going from an entry to where it was written.
  await click('周祥'); await pane('别名 小祥、saki'); await pane('AI 建议'); await pane('「第1章：巷口」“小祥把左手揣回兜里。”'); await shot('02-entry')
  await click('查看原文'); await win('第1章：巷口'); await win('梯子靠稳了'); must((await drawn('小祥把左手揣回兜里。')) === 'sel', 'the entry\'s sentence is not picked out'); await shot('03-entry-source'); await done('an entry to its source')
  await click('采纳'); await pane('AI 建议', false); await pane('设定5'); await done('take an idea on an entry')
  await click('修改'); await pane('名字'); await pane('分类'); await shot('04-edit')
  await type('内容', '新加一句。'); await click('人物'); await shot('05-cats'); await click('地点'); await click('保存')
  await pane('新加一句。'); await pane('删除设定'); await done('edit an entry')
  await click('全部设定'); await pane('地点')
  await click('待确认'); await pane('5 条 AI 建议'); await pane('关于 林晚 · 第87章起'); await shot('06-ideas')
  await click('忽略'); await pane('4 条 AI 建议'); await click('采纳'); await click('采纳'); await pane('2 条 AI 建议')
  // A change the AI saw is taken onto its entry, from its chapter on: a step of that entry, not an entry of its own.
  await click('采纳'); await pane('1 条 AI 建议'); await pane('关于 林晚 · 第87章起', false)
  // A question of the AI's is answered, not taken: the answer is the entry, in the question's category, and says what it answers.
  must(!(await js(`[...document.querySelectorAll('.pane .btn')].some((b) => b.innerText === '采纳')`)), 'a question can be taken as it is'); await click('回答'); await pane('林晚怕黑，是不是和她母亲有关？'); await pane('人物'); must(await js(`document.querySelector('[aria-label="名字"]').value`) === '林晚的母亲', 'the answer lost the name of the question'); await shot('06-answer')
  await type('内容', '母亲是在一次停电里出的事。'); await click('保存'); await pane('待确认', false); await click('母亲是在一次停电里出的事。'); await pane('回答：林晚怕黑，是不是和她母亲有关？'); await click('全部设定'); await done('decide the ideas')
  await click('更多'); await pane('修改记录'); await shot('07-menu'); await click('已删除'); await pane('老陈')
  await click('老陈'); await pane('恢复设定'); await click('查看原文'); await win('第2章：井沿'); must((await drawn('老陈把门卫室的灯拧亮。')) === 'sel', 'the deleted entry\'s sentence is not picked out')
  await click('恢复设定'); await pane('老陈', false); await pane('已删除'); await done('restore an entry')
  await click('全部设定'); await pane('老陈'); await click('更多'); await click('修改记录'); await pane('记在「小祥和林晚是怎么认识的」'); await pane('今天 10:40'); await shot('08-log')
  await click('查看讨论'); await win('2 条发言'); await win('设定里只写了他是她带出来的徒弟'); await click('正文'); await click('讨论', 1); await click('全部讨论'); await click('设定'); await done('the change log')
  await click('全部设定'); await type('搜索设定', '左手'); await pane('找到 2 条'); await shot('09-find')
  await clear('搜索设定'); await pane('找到', false)
  // An entry that changes from a chapter on: the list says from where beside it, the entry lists each change in the order of the
  // chapters, the one that holds for the chapter open is marked, and one whose chapter is in the trash holds nowhere.
  await pane('之后有变化'); await click('第87章：停电'); await pane('第87章起'); await click('周祥'); await pane('随章节的变化'); await pane('本章适用'); await pane('所在章节已删除 · 不起作用'); await shot('09b-timeline')
  await click('修改', 1); await pane('周祥 · 第87章起'); await pane('删除这处变化')
  // A change may start at a volume as well as a chapter: the volumes are among the chapters it may start from.
  await js(`document.querySelector('#pane [aria-label="从哪一章起"]').click()`); must(await js(`[...document.querySelectorAll('.choose-menu .pop-row')].map((r) => r.innerText).join('|').includes('第三卷：停电|第84章：刻字')`), 'a change cannot start at a volume'); await key('Escape'); await click('取消')
  await click('第86章：档案室'); await pane('本章适用上面原来的样子。'); must(!(await js(`!!document.querySelector('#pane .tag')`)), 'a change holds before its chapter'); await click('从本章起有变化'); await pane('周祥 · 第86章起')
  { const was = await js(`document.querySelector('[aria-label="变化后的内容"]').value`); must(was.includes('左手总揣在兜里') && !was.includes('改口'), 'a new change does not start from how the entry stood before: ' + was) }; await shot('09c-change')
  await clear('变化后的内容'); await type('变化后的内容', '配角，林晚的同事。在档案室外替她望风。'); await click('保存'); await pane('在档案室外替她望风'); must(await js(`!!document.querySelector('#pane .tag')`), 'the change of the chapter open is not marked as holding'); await pane('从本章起有变化', false)
  await click('修改', 3); await click('所在章节已删除'); await click('第46章：钥匙'); await click('保存'); await pane('第46章起'); await pane('所在章节已删除', false)
  await click('修改', 2); await click('删除这处变化'); await pane('在档案室外替她望风', false); await pane('从本章起有变化'); await click('第87章：停电'); await click('全部设定'); await done('an entry that changes')
  await click('新增'); await pane('新增设定'); must(await js(`document.activeElement.getAttribute('aria-label') === '内容'`), 'the text of a new entry is not ready to type in'); await type('内容', '老周是修表的，铺子在旧城小学对面。'); await type('名字', '老周'); await click('保存'); await pane('老周'); await done('add an entry')
  await click('周祥'); await click('删除设定'); await pane('左手总揣在兜里', false); await pane('物品'); await done('delete an entry')

  // The sidebar: chapters open, volumes fold.
  await click('第84章：刻字'); await win('灯罩内侧刻着'); await win('本章 3,102 字'); await shot('10-chapter')
  await outline('第三卷：停电'); await win('卷纲'); await win('4 项'); await win('本卷 1.2 万字 · 全书 31.2 万字'); await win('林晚趁何主任不在翻档案柜'); await win('巷口那盏灯又坏了'); await shot('10b-folder')
  await click('第三卷：停电'); must(!(await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第85章'))`)), 'the volume did not fold'); await click('第三卷：停电'); must((await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第85章'))`)), 'the volume did not unfold'); await click('第85章：灯数'); await win('卷纲', false); await click('上一个打开的章节'); await win('卷纲')
  await click('番外：晴天'); await win('雨城一年里只有十几个晴天'); await shot('11-loose-chapter'); await done('the sidebar')

  // Foreshadowings: from the pane to the prose and back.
  await click('伏笔'); await pane('久未回收'); await pane('未回收'); await shot('12-foreshadowings')
  await click('她怕黑的真正原因'); await pane('埋在「第1章：巷口」，已过 86 章'); await click('查看原文'); await win('第1章：巷口')
  must((await drawn('她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。')) === 'fs on', 'the open foreshadowing is not lit'); await shot('13-mark-source')
  must(await js(`[...document.querySelectorAll('#pane .btn')].find((e) => e.innerText === '记为回收').classList.contains('off')`), '记为回收 can be pressed with nothing picked'); await pane('请先在正文中选中回收的段落')
  await js(`[...document.querySelectorAll('#pane .btn')].find((e) => e.innerText === '记为回收').click()`); await sleep(300); must(!(await js(`!!document.querySelector('.toast')`)), 'a toast says again what the pane says'); await win('选择要回收的伏笔', false); await shot('14-pay-nothing-picked'); await done('pay off with nothing picked')
  await click('第87章：停电'); await select(5, '她往前走了一步。'); await js(`document.querySelector('[data-para="5"]').dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }))`); await click('记为回收'); await win('选择要回收的伏笔'); await win('86 章前埋下'); await shot('15-pay-from-pane')
  await click('确定回收'); await win('选择要回收的伏笔', false); await pane('回收在「第87章：停电」，相隔 86 章'); await pane('取消回收'); await pane('她往前走了一步。')
  must((await drawn('她往前走了一步。')) === 'fs on', 'the payoff is not lit'); await shot('16-paid'); await done('pay off from the pane')
  await click('修改备注'); await type('修改伏笔备注', '（改）'); await key('Enter'); await pane('她怕黑的真正原因（改）'); await pane('修改备注')
  await click('修改备注'); await key('Escape'); await pane('修改备注'); await done('rename a foreshadowing')
  await click('查看原文', 1); await win('第87章：停电'); await click('查看原文'); await win('第1章：巷口'); await done('both sources')

  // The right button in the prose.
  await click('第87章：停电')
  await menu(2, '灯一直没有亮。'); await win('记为伏笔…'); await win('记为回收…'); await win('查看伏笔', false); await win('剪切'); await win('Ctrl+A'); await shot('17-menu')
  must((await drawn('灯一直没有亮。')) === 'sel', 'what was picked is not shown'); await click('记为伏笔…'); await win('记为伏笔，填写备注'); await shot('18-bury')
  must(await js(`document.activeElement.getAttribute('aria-label') === '伏笔备注'`), 'the note of a new foreshadowing is not ready to type in')
  await type('伏笔备注', '灯为什么没亮'); await key('Enter'); await win('记为伏笔，填写备注', false); must((await drawn('灯一直没有亮。')) === 'fs', 'the new foreshadowing is not underlined')
  await click('全部伏笔'); await pane('灯为什么没亮'); await pane('本章'); await shot('19-buried'); await done('bury from the prose')
  await menu(2, '灯一直没有亮。'); await win('查看伏笔'); await click('查看伏笔'); await pane('灯为什么没亮'); await pane('埋在「第87章：停电」'); await pane('已过', false); await done('see a foreshadowing from the prose')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await key('Escape'); await win('记为伏笔，填写备注', false); must((await drawn('整条街的灯')) === 'absent', 'Escape left the pick behind')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await click('取消'); await win('记为伏笔，填写备注', false)
  await click('第86章：档案室'); await menu(4, '走廊里响起脚步声'); await click('记为回收…'); await type('查找伏笔', '哪扇'); await win('钥匙开哪扇门'); await win('小祥为什么突然改口', false); await shot('20-pay-search')
  await clear('查找伏笔'); await type('查找伏笔', '没有这个'); await win('没有匹配的伏笔'); await clear('查找伏笔'); await type('查找伏笔', '哪扇'); await key('Enter')
  await win('选择要回收的伏笔', false); await pane('钥匙开哪扇门'); await pane('回收在「第86章：档案室」，相隔 40 章'); await shot('21-paid-from-menu'); await done('pay off from the prose')
  await js('getSelection().removeAllRanges()')
  const [x, y]: [number, number] = await js(`(() => { const r = document.querySelector('[data-para="0"]').getBoundingClientRect(); return [r.x + 80, r.y + r.height / 2] })()`)
  await rclick(x, y); await win('记为伏笔…'); must(await greyed('记为伏笔…'), 'burying is not greyed'); await shot('22-nothing-picked'); await rclick(x, y); await win('记为伏笔…', false); await done('the menu with nothing picked')
  await menu(0, '档案室在电力局二楼'); await click('剪切'); await win('粘贴', false); await win('档案室在电力局二楼', false); must(await unsaved(), 'cutting left the chapter saved')
  await key('z', true); await win('档案室在电力局二楼'); await key('y', true); await win('档案室在电力局二楼', false); await key('z', true); await win('档案室在电力局二楼'); await done('cut from the menu, undo and redo')

  // A foreshadowing whose sentence is gone takes a new one.
  await click('全部伏笔'); await click('原句已删'); await click('小祥的左手'); await pane('「第1章：巷口」里的原句已删除'); await shot('23-lost')
  must(await js(`[...document.querySelectorAll('#pane .btn')].find((e) => e.innerText === '更换原句').classList.contains('off')`), '更换原句 can be pressed with nothing picked'); await click('查看原文'); await select(5, '小祥把左手揣回兜里。'); await js(`document.querySelector('[data-para="5"]').dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }))`)
  must(!(await js(`[...document.querySelectorAll('#pane .btn')].find((e) => e.innerText === '更换原句').classList.contains('off')`)), '更换原句 stays grey with words picked'); await click('更换原句')
  await pane('埋在「第1章：巷口」，已过 86 章'); await pane('记为回收'); must((await drawn('小祥把左手揣回兜里。')) === 'fs on', 'the new sentence is not lit'); await done('a new sentence')
  await click('全部伏笔'); await pane('此处暂无伏笔'); await click('全部伏笔'); await click('更多'); await shot('24-foreshadowing-menu'); await click('已放弃'); await click('红色钥匙'); await pane('已放弃'); await click('恢复伏笔'); await pane('放弃伏笔'); await done('give up and take back')

  // The pane folds away, and its switch stays where it was to bring it back.
  const fold = await spot('.pane-switch')
  await click('收起右栏'); must(!(await js(`!!document.querySelector('#pane')`)), 'the pane is still there'); await win('本章伏笔 1'); await shot('25-folded')
  must(await spot('.pane-switch') === fold, 'folding the pane moved its switch: ' + fold + ' to ' + await spot('.pane-switch'))
  await click('展开右栏'); await pane('放弃伏笔'); await click('收起右栏'); await click('本章伏笔 1'); await pane('未回收'); await done('fold the pane')

  // Discussions.
  await click('讨论', 1); await pane('最近'); await click('新讨论'); await pane('示例问题'); await shot('26-new-talk')
  await click('接下来可以怎么写'); await pane('正在思考…', false); await key('Enter', true); await pane('正在思考…'); await pane('取消请求'); await sleep(800); await pane('正在回复…'); await sleep(700); await pane('这是原型'); await pane('正在回复…', false)
  await type('输入消息', '那下一章呢'); await key('Enter', true); await pane('那下一章呢'); must(await js(`document.querySelector('.book').classList.contains('off')`), 'the work can be switched while the AI replies')
  await cats(); await win('“落点”指设定成熟后的去处'); await win('AI 正在思考…'); await click('正文'); await win('AI 正在思考…', false); await sleep(1500); await pane('先想清楚这一段要做什么'); await shot('27-talk'); await done('the rest free while the AI replies')
  await type('输入消息', '再说一句'); await key('Enter', true); await click('取消请求'); await pane('该消息尚未收到回复'); await pane('正在思考', false); await shot('27b-unanswered'); await click('重试回复'); await pane('该消息尚未收到回复', false); await sleep(1500); await pane('我会先留着')
  await pane('先看设定里和这句话有关的几条', false); await click('思考过程'); await pane('先看设定里和这句话有关的几条'); await shot('27c-thought'); await click('思考过程'); await pane('先看设定里和这句话有关的几条', false); await done('a reply cancelled and asked for again')
  await click('全部讨论'); await pane('接下来可以怎么写'); await click('这一章的节奏怎么样'); await pane('前半段偏慢'); await done('discussions')

  // Dark, and back to how it opened.
  await click('视图'); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('28-dark'); await key('Escape'); await click('设定'); await shot('29-dark-setting')
  await click('重新开始'); await margined(3); await pane('设定6'); await win('第87章：停电'); must(await js(`document.querySelector('#win').className === 'dark'`), 'starting over lost the look'); await click('视图'); await click('浅色'); await key('Escape'); await done('dark, and start over')

  const has = (sel: string): Promise<boolean> => js(`!!document.querySelector(${JSON.stringify(sel)})`)
  const value = (label: string): Promise<string> => js(`document.querySelector('[aria-label=${JSON.stringify(label)}]').value`)
  // Presses words in the open dialog, passing over the same words outside it (a tab of the pane, a row of the tree).
  const inDialog = async (words: string) => click(words, await js(`(() => {
    const flat = (t) => (t || '').replace(/\\s+/g, ''), own = (e) => [...e.childNodes].filter((n) => n.nodeType === 3).map((n) => n.data).join('')
    const out = []
    for (const h of document.querySelectorAll('body *')) {
      if (flat(h.innerText) !== ${JSON.stringify(words)} && flat(own(h)) !== ${JSON.stringify(words)}) continue
      const e = h.closest('[data-click]')
      if (e && !out.includes(e)) out.push(e)
    }
    return out.findIndex((e) => e.closest('.dialog'))
  })()`))
  // The middle of the row of the sidebar that reads so, to press the right button on.
  // Picks words the way a pointer does: down at their start, up at their end.
  const pick = async (para: number, words: string) => {
    const [a, b]: [[number, number], [number, number]] = await js(`(() => {
      const p = document.querySelector('[data-para="${para}"]'), from = p.textContent.indexOf(${JSON.stringify(words)}), to = from + ${words.length}
      const walk = document.createTreeWalker(p, NodeFilter.SHOW_TEXT), r = document.createRange()
      let n, at = 0, begun = false
      while ((n = walk.nextNode())) {
        const end = at + n.data.length
        if (!begun && from < end) { r.setStart(n, from - at); begun = true }
        if (to <= end) { r.setEnd(n, to - at); break }
        at = end
      }
      const box = r.getBoundingClientRect()
      return [[box.x + 1, box.y + box.height / 2], [box.right - 1, box.y + box.height / 2]]
    })()`)
    await drag(a, b)
  }

  // The sidebar: back and forth, new chapters and folders, the right button, the bin.
  await click('第84章：刻字'); await click('第85章：灯数'); await click('上一个打开的章节'); await win('灯罩内侧刻着'); await click('下一个打开的章节'); await win('何主任让他们把旧城的灯重新数一遍'); await done('back and forth')
  await click('第87章：停电'); await click('新建章节（Ctrl+N）'); must(await js(`document.activeElement.getAttribute('aria-label') === '章节标题'`), 'the name of a new chapter is not ready to type in'); await shot('30-new-chapter')
  await text('雨停'); await key('Enter'); await win('第88章：雨停'); await win('在此开始写作…'); await win('满 200 字后可检查'); await win('本章 0 字'); await done('a new chapter')
  await rclick(...(await row('第88章：雨停'))); await win('移到回收站'); await win('最上层', false); await over('移到'); await win('最上层'); await shot('31-tree-menu'); await click('重命名'); await text('天晴'); await key('Enter'); await win('天晴'); await win('第88章：天晴', false); await done('rename a chapter')
  await rclick(...(await row('天晴'))); await click('上移'); must(await js(`(() => { const t = document.querySelector('.tree').innerText; return t.indexOf('天晴') < t.indexOf('第87章') })()`), 'the chapter did not move up')
  await rclick(...(await row('天晴'))); await over('移到'); await click('最上层'); await win('天晴'); await done('move a chapter')
  await rclick(...(await row('天晴'))); await click('移到回收站'); await win('「天晴」已移到回收站'); await click('关闭提示'); must(!(await js(`document.querySelector('.tree').innerText.includes('天晴')`)), 'the chapter is still in the tree')
  await click('文件'); await click('新建文件夹'); await clear('章节标题'); await type('章节标题', '第四卷'); await key('Enter'); await win('第四卷')
  await bin(); await win('恢复后回到原文件夹末尾。'); await win('第83章：旧稿'); await shot('32-bin'); await click('恢复', 0); await win('已恢复「天晴」'); await click('关闭提示')
  await click('彻底删除'); await click('确认彻底删除'); await win('回收站为空'); await click('关闭'); must(await js(`document.querySelector('.tree').innerText.includes('天晴')`), 'the chapter did not come back'); await done('the bin')
  await click('切换作品'); await win('旧城灯匠'); await win('全部作品…'); await shot('33-book-menu'); await key('Escape'); await click('帮助'); await click('快捷键…'); await win('打开快捷键列表'); await win('专注写作：仅显示正文'); await shot('34-keys'); await click('关闭'); await win('打开快捷键列表', false)
  await key('/', true); await win('打开快捷键列表'); await key('Escape'); await win('打开快捷键列表', false); await done('the book menu, and the keys from the menu of help')
  await click('已备份 · 今天 10:32'); await win('今天 10:32 已备份'); await win('下次：关闭作品时'); await shot('35-backup-pop'); await click('立即备份'); await win('正在备份…'); await sleep(1400); must(await has('[title="已备份 · 今天 10:40"]'), 'the status bar does not say it was backed up at 今天 10:40'); await done('back up now')

  // The toolbar: the count and the goal, find and replace, the more menu, focus, what is pending.
  await click('重新开始'); await click('字数。点击设置每日目标'); await win('最近 7 天'); await win('今天+2,846 字'); await win('昨天+3,120 字'); await win('10-10'); await type('每日目标字数', '3000'); await win('今天 2,846 / 3,000'); await type('每日目标字数', '字'); await win('每日目标须为正整数，留空则不设。'); await win('今天 2,846 / 3,000'); await clear('每日目标字数'); await type('每日目标字数', '3000'); await win('每日目标须为正整数', false); await shot('36-goal'); await key('Escape'); await win('最近 7 天', false); await done('a goal for the day')
  await pick(3, '小祥'); await key('f', true); must(await js(`document.activeElement.getAttribute('aria-label') === '查找'`), 'Ctrl+F did not open the search')
  must((await value('查找')) === '小祥' && await js(`document.activeElement.selectionStart === 0 && document.activeElement.selectionEnd === 2`), 'the words picked are not what is looked for, ready to be typed over'); await win('1 / 2'); must((await drawn('小祥')) === 'got on', 'what was found is not marked'); await shot('37-find')
  await click('下一个'); await win('2 / 2'); await type('替换为', '阿祥'); await click('替换'); await win('阿祥从来不叫她姐'); await win('1 / 1'); await click('全部替换'); await win('已替换 1 处可撤销'); await win('阿祥在身后叫她'); await win('未找到'); await click('关闭提示')
  await win('本章有 1 处待处理，正文已再次修改'); await click('立即检查'); await win('正在检查本章…'); await sleep(1400); await win('本章检查完成，有 1 处待处理'); await done('find and replace in a chapter')
  await clear('查找'); await type('查找', '林晚'); await click('在全部章节中查找'); await win('全书 8 章、共 9 处'); await win('第1章：巷口 · 2 处'); await shot('38-find-book')
  await clear('替换为'); await type('替换为', '阿晚'); await click('全书替换'); await win('将全书 8 章中的 9 处“林晚”替换为“阿晚”？'); await shot('39-replace-book'); await click('确定替换'); await win('已在 8 章中替换 9 处'); await win('阿晚站在巷口'); await win('全书未找到')
  await click('关闭查找'); must(!(await has('.find-bar')), 'the search is still open'); await click('关闭提示'); await done('replace through the book')
  await click('编辑'); await win('Ctrl+J'); await shot('40-edit-menu'); await pointAt('文件'); await win('Ctrl+S'); await win('Ctrl+J', false); await shot('40b-file-menu'); await click('历史版本'); await win('历史版本 · 第87章：停电'); await win('全书替换前'); await win('打开时'); await shot('41-snaps')
  await click('与当前对比'); await win('红色删除线为历史版本中有而当前没有的内容'); must(await has('.snap-text .cut') && await has('.snap-text .add'), 'the comparison shows no change'); await shot('42-snap-diff')
  await click('今天 09:58'); await click('恢复这个版本'); await win('已恢复到 今天 09:58 的版本恢复前的内容已存为历史版本'); await win('小祥在身后叫她', false); await win('七岁那年的那个晚上'); await click('关闭提示'); await done('snapshots')
  await click('文件'); await click('导出…'); await win('首页写书名、笔名和字数，每章另起一页。'); await win('段首加两个全角空格', false); must((await value('笔名')) === '', 'a pen name is there before any was written')
  await type('笔名', '雨声'); await click('指定范围'); await win('第1章：巷口'); await win('第46章：钥匙'); await inDialog('第1章：巷口'); await inDialog('第2章：井沿'); await shot('43-export')
  await click('导出'); await win('已导出到 D:\\写作\\雨城来信-第2章 井沿至第46章 钥匙.docx'); await click('关闭提示')
  await click('文件'); await click('导出…'); must((await value('笔名')) === '雨声', 'the pen name is not kept'); await inDialog('大纲'); await win('勾选的几项各存为一个文件，放在同一文件夹。'); await click('指定范围'); await win('所选范围内还没有写章纲，不导出大纲。')
  await inDialog('正文'); await win('笔名', false); await win('按章节顺序列出各卷、各章的纲。'); must(await has('.dialog .btn.primary.off'), 'an outline with nothing in it can be exported'); await shot('43b-export-no-outline')
  await click('全书'); await win('所选范围内还没有写章纲', false); await inDialog('正文'); await inDialog('设定'); await shot('43c-export-three'); await click('纯文本'); await win('段首加两个全角空格'); await click('导出'); await win('已导出 3 个文件到 D:\\写作'); await click('关闭提示')
  await click('文件'); await click('导出…'); await win('勾选的几项各存为一个文件'); await inDialog('正文'); await inDialog('大纲'); await win('按分类列出已定的设定，不含待确认的。'); must(!(await has('[aria-label="导出范围"]')), 'the setting is offered a range')
  await click('Word'); await click('导出'); await win('已导出到 D:\\写作\\雨城来信-设定.docx'); await click('关闭提示'); await click('文件'); await click('导出…'); await inDialog('正文'); await inDialog('设定'); await click('取消'); await done('export')
  await click('仅显示正文，收起章节树和右栏（F11）'); must(!(await has('.sidebar')) && !(await has('#pane')), 'focus left something beside the prose'); await shot('44-focus'); await key('F11'); must(await has('.sidebar') && await has('#pane'), 'F11 did not leave focus'); await done('focus')
  await click('重新开始'); await click('待处理 8'); must(await js(`[...document.querySelectorAll('.todo-group')].map((e) => e.textContent).join(' ')`) === '检查5 伏笔3', 'the groups of what is pending read: ' + await js(`[...document.querySelectorAll('.todo-group')].map((e) => e.innerText).join(' ')`)); await win('「红色钥匙」', false); await win('「钥匙开哪扇门」41 章未回收'); await win('还有 2 条'); await shot('45-pending'); await click('还有 2 条'); await win('「旧城小学」尚未加入设定'); await click('收起')
  await click('本章 1'); await win('周翔', false); await win('林晚怕黑，却独自走进黑巷'); await click('全部 8'); await click('「小祥的左手」原句已删'); await pane('「第1章：巷口」里的原句已删除'); await win('全部忽略', false)
  // Ignoring all only takes them off what is pending: the margin, the check page and the prose keep them, and it is undone from the toast.
  await click('待处理 8'); await click('全部忽略'); must(!(await has('.todo-btn')), 'ignoring all left the button'); await win('已忽略 8 条待处理'); await click('批注'); await margined(3); await click('撤销'); await win('待处理 8')
  await click('待处理 8'); await click('全部忽略'); must(!(await has('.todo-btn')), 'ignoring all again left the button'); await done('what is pending')

  // Under the title: the outline.
  await click('重新开始'); await click('章纲 ▾'); must((await value('章纲')).startsWith('全城停电'), 'the outline is not shown'); await shot('46-outline'); await win('摘要', false); await click('章纲 ▴'); must(!(await has('.chapter-box')), 'the outline did not fold')
  await click('第84章：刻字'); await win('章纲 · 空 ▾'); await click('编辑'); must(await js(`[...document.querySelectorAll('.menu-pop .row')].find((e) => e.innerText.includes('起草本章')).classList.contains('off')`), 'drafting is not greyed without an outline'); await win('请先填写章纲'); await key('Escape'); await done('the outline')

  // AI in the prose: a sentence more, then a rewrite and a draft, each a talk of its own whose AI proposes in the prose.
  await click('重新开始'); await click('编辑'); await click('续写一句'); await win('续写中…'); await sleep(900); await win('Tab 采纳 · Esc 放弃'); must(await has('.ghost-text'), 'no grey sentence'); await shot('48-ghost')
  await key('Escape'); must(!(await has('.ghost-text')), 'Escape kept the sentence'); await win('她把手电咬在嘴里', false); await key('j', true); await sleep(900); await key('Tab'); must(!(await has('.ghost-text')), 'Tab kept it grey'); await win('她往前走了一步。她把手电咬在嘴里'); await done('a sentence more')
  // The prose alone: what a talk beside it quotes is in the window too.
  const proposals = (): Promise<number> => js(`S.props.length`), prose = reads('.prose')
  await pick(2, '灯一直没有亮。'); must(await has('.selbar'), 'nothing floats over what was picked'); await win('选中 7 字'); await shot('49-selection'); await click('AI 改写'); must(await js(`document.activeElement.getAttribute('aria-label') === '改写要求'`), 'the wish is not ready to type in')
  must(await has('.ai-bar .btn.primary.off'), 'a rewrite can go without a wish')
  await type('改写要求', '慢一点'); await key('Enter'); await pane('《第87章：停电》'); await pane('灯一直没有亮。'); await pane('慢一点'); await pane('跟着「第87章：停电」'); await win('AI 正在思考…')
  await sleep(1500); await win('正在写…'); must(await has('.prose .cut') && await has('.prose [data-prop]'), 'the proposal is not written into the prose as it comes'); await shot('50-rewriting')
  // As it comes, 放弃 stands where it will stay, 正在写… after it, and 全部采纳 is grey: nothing moves when it is done.
  must(await js(`[...document.querySelector('.prop-acts').children].map((e) => e.innerText).join('|') === '放弃|正在写…'`), '放弃 is not first as the proposal comes'); must(await has('.ai-bar .btn.primary.off'), '全部采纳 is not there, grey, as the proposal comes')
  await sleep(1800); await win('本章有 1处改稿建议'); must(await js(`document.querySelector('.ai-bar [title]').title.includes('划掉的是会被换掉的原文')`), 'the bar does not tell what the colours are'); await pane('换掉一句 · 20 字 · 等你决定'); await prose('她在巷口站了很久'); must(await js(`S.chapters.find((c) => c.key === S.ch).paras[2].includes('灯一直没有亮。')`), 'the proposal went into the prose before it was taken'); await shot('51-rewrite')
  await type('输入消息', '再短一点'); await key('Enter', true); await sleep(1500); await pane('已被新的建议替换'); await prose('她在巷口站了很久', false); must(await proposals() === 1, 'the proposal before was kept beside the new one')
  await sleep(1700); await click('采纳'); await win('已采纳改稿建议'); await win('可撤销恢复原文，原版本已存入历史版本'); await prose('她站了一会儿，往黑里迈了一步。'); await prose('灯一直没有亮。', false); await pane('已采纳')
  must(await js(`document.querySelectorAll('.prose .ai').length > 0`), 'what the AI wrote is not marked')
  await key('z', true); await prose('灯一直没有亮。'); await key('y', true); await prose('她站了一会儿'); await prose('灯一直没有亮。', false); await click('关闭提示')
  await pick(0, '同时灭的'); await click('AI 改写'); await key('Escape'); must(!(await has('.ai-bar')), 'Escape left the rewrite open'); must(await js(`document.activeElement === proseBox()`), 'Escape did not give the keyboard back to the prose'); await done('a rewrite')
  // A proposal whose sentence the author changes is void; one let go while it is written stops the AI.
  await pick(1, '手电在工具箱最底下。'); await click('AI 改写'); await type('改写要求', '换个说法'); await key('Enter'); await sleep(3300); await win('本章有 1处改稿建议')
  await caret(1, 9); await text('啊'); await win('1处改稿建议已作废'); await win('它要换掉的原文改动了'); await win('本章有 1处改稿建议', false); await pane('正文已改动，建议已作废'); await shot('52-void'); await key('z', true); await prose('手电在工具箱最底下。'); await click('关闭提示')
  await pick(1, '手电在工具箱最底下。'); await click('AI 改写'); await type('改写要求', '换个说法'); await key('Enter'); await sleep(1500); await win('正在写…'); await click('放弃'); must(!(await js(`!!S.waiting`)) && await proposals() === 0, 'letting go of a proposal being written left the AI writing'); await pane('该消息尚未收到回复'); await done('a proposal void, and one let go')
  await click('编辑'); await click('起草本章'); await win('AI 仅将章纲和本章的卡片扩写为正文'); await win('本章已有内容'); await win('将带上本章的 3 张卡片、上一章全文、本卷各章章纲等，约 5.8K token'); await win('用讨论模型 claude-opus-5-5'); await shot('53-draft')
  await type('目标字数', '1500'); await click('覆盖已有内容'); await click('开始起草'); await pane('按章纲起草本章，约 1500 字，覆盖已有内容'); await win('AI 正在思考…'); await sleep(1500); await win('AI 正在写改稿建议…'); await shot('54-drafting'); await sleep(2500); await win('本章有 1处改稿建议'); must(await js(`document.querySelector('.ai-bar [title]').title.includes('划掉的是会被换掉的原文')`), 'the bar does not tell what the colours are'); await pane('换掉全章')
  must(await js(`document.querySelectorAll('.prose .add').length === 3 && document.querySelectorAll('.prose > .cut').length === 6`), 'the draft is not shown against the prose'); must(await js(`S.chapters.find((c) => c.key === S.ch).paras[0] === '整条街的灯是同时灭的。'`), 'the draft went into the prose before it was taken'); await shot('55-drafted')
  await sleep(500); await click('全部采纳'); await win('已采纳改稿建议'); await win('整条街的灯是同时灭的。', false); await win('身后，小祥没有再出声。')
  must(await js(`document.querySelectorAll('.prose .ai').length === 3`), 'what the AI drafted is not marked')
  await key('z', true); await win('整条街的灯是同时灭的。'); await win('身后，小祥没有再出声。', false); await key('y', true); await win('身后，小祥没有再出声。'); await click('关闭提示')
  await click('编辑'); await click('撤销'); await win('整条街的灯是同时灭的。'); await win('身后，小祥没有再出声。', false); await click('编辑'); await click('重做'); await win('身后，小祥没有再出声。'); await win('整条街的灯是同时灭的。', false)
  await click('文件'); await click('历史版本'); await win('AI 改动前'); await win('采纳讨论「起草：第87章：停电」里的建议'); await key('Escape')
  await click('编辑'); await click('起草本章'); await win('推理', false); await type('目标字数', '很多'); await win('目标字数须为正整数，或留空'); must(await has('.dialog .note.danger') && await has('.dialog .btn.primary.off'), 'a draft can start with words that are not a count'); await clear('目标字数'); await click('开始起草'); await sleep(1500); await click('全部放弃'); await pane('该消息尚未收到回复'); must(!(await js(`!!S.waiting`)) && await proposals() === 0, 'the draft was not let go'); await done('a draft')
  // Leaving the work with a proposal not taken asks first.
  await click('编辑'); await click('起草本章'); await click('开始起草'); await sleep(4500); await click('雨城来信'); await click('新建作品…'); await win('还有 1处改稿建议没有处理'); await win('关闭后将丢弃。'); await shot('56-leave')
  await click('取消'); await win('还有 1处改稿建议没有处理', false); await win('本章有 1处改稿建议'); await click('全部放弃'); await win('本章有 1处改稿建议', false); await done('leave with a proposal')

  // What was picked goes to the setting, or to a discussion.
  await click('重新开始'); await pick(1, '手电在工具箱最底下'); await click('加入设定'); await win('选中的原文将记为出处'); must((await value('设定内容')) === '手电在工具箱最底下', 'what was picked is not in the box'); await click('人物'); await click('物品'); await shot('54-take')
  await click('加入'); await win('已加入设定正文'); await click('设定'); await pane('手电在工具箱最底下'); await click('关闭提示'); await click('手电在工具箱最底下'); await pane('「第87章：停电」“手电在工具箱最底下”'); await done('into the setting')
  await menu(3, '姐，你别进去。'); await win('AI 改写…'); await win('Ctrl+L'); await shot('55-menu'); await click('引用到讨论'); await pane('《第87章：停电》'); await pane('姐，你别进去。'); await shot('56-cited')
  await type('输入想讨论的内容', '这句要不要留'); await key('Enter'); await pane('这句要不要留', false); must(await js(`document.querySelector('#pane textarea').placeholder === '输入想讨论的内容，Ctrl+Enter 发送'`), 'the box beside the prose does not say what sends'); await key('Enter', true); await pane('这句要不要留'); await pane('《第87章：停电》'); await sleep(1500); await pane('这是原型'); await done('to a discussion')
  await click('本章检查完成，有 1 处待处理'); await win('其他章节的类似情况'); await click('正文'); must((await drawn('她往前走了一步。')) === 'doubt', 'what the check found is not marked in the prose')
  await click('写完一章后检查 ▴'); await win('检查时机'); await shot('57-when'); await click('手动检查'); await win('手动检查 ▴'); await done('the line under the prose')

  // The discussion page: a talk, what stands under a reply, a new talk, the model.
  await click('重新开始'); await click('讨论'); await win('8 条发言'); await win('前半段偏慢'); await pane('待确认'); await pane('伏笔', false); await shot('58-talk-page')
  // The talk reads the outline: a plot line's cards, in story order, with where each is written.
  await click('查了大纲「小祥」这条线，搜了全书「叫她姐」'); await win('2 张卡片：周祥来当学徒（今年·四月，「第1章：巷口」）→ 小祥改口叫她“姐”（十月六日夜，「第87章：停电」）'); await shot('58e-read-outline'); await click('查了大纲「小祥」这条线，搜了全书「叫她姐」')
  await win('读了「第87章：停电」，读了「第86章：档案室」'); await click('以上 2 轮已整理成摘要'); await win('作者的态度和偏好'); await win('摘要由 AI 写成，有误可在对话中指出。'); await win('和停电时「林晚」想起七岁那晚的写法。'); await win('等作者答了「林晚的母亲」那条提问再定。'); await win('[1]', false); await win('• 作者先改前半段。'); await win('• 再回来看「周祥」叫她“姐”那句。'); await win('- 作者', false); await shot('58a-digest')
  await click('林晚'); await pane('主角，靠维修路灯谋生'); await click('周祥'); await pane('配角，林晚的同事'); await click('林晚的母亲'); await pane('林晚怕黑，是不是和她母亲有关？'); await click('以上 2 轮已整理成摘要'); await win('作者的态度和偏好', false)
  await type('讨论输入', '先改哪一段'); await key('Enter', true); await win('正在压缩上下文…'); await win('正在思考', false); await shot('58d-compacting'); await sleep(1000); await win('以上 4 轮已整理成摘要'); await win('以上 2 轮已整理成摘要', false); await win('正在思考…')
  await click('取消请求'); await win('该消息尚未收到回复'); await win('以上 4 轮已整理成摘要'); await done('a long talk')
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('收起这段对话'); await win('已收起 1 段对话'); await win('主角有个说不出口的秘密', false); await click('已收起 1 段对话'); await win('不显示收起的对话') }
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('移回列表'); await win('不显示收起的对话', false); await win('主角有个说不出口的秘密'); await done('a talk put away and back') }
  await click('小祥和林晚是怎么认识的'); await win('2 条发言'); await win('268 token', false); await win('昨天 22:11 · claude-opus-5-5 · 28.3K token'); await win('昨天 22:10'); await click('AI 参考内容'); await win('本次发送约 9.6K token'); await win('这一轮共 3 次请求，这是最后一次。'); await win('规则 1.1K · 设定 12 条 3.1K'); await win('实际用量：输入 9.9K，其中缓存命中 9.3K；输出 268'); await win('附带全部 12 条设定'); await win('林晚、晚晚、林林宝：主角，靠维修路灯谋生，但她很怕黑。'); await win('已放弃：老陈：电力局的门卫，早先想让他知道井的事。后来这条线并给了周祥。')
  // An entry that changes is sent as it first read, each change under it: the last is not said twice, and what held before is not lost.
  must(await js(`(() => { const r = [...document.querySelectorAll('.saw-row')].find((x) => x.innerText.includes('周祥、小祥')); return !!r && !r.querySelector('.fit > div').innerText.includes('改口') && r.innerText.includes('第87章起：') && r.innerText.includes('改口') })()`), 'a talk is not sent how an entry first read, with its changes under it')
  await shot('58c-sent'); await done('what was sent')
  await click('查了设定「周祥」，搜了全书「小祥」，记下：待确认 · 人物：“小祥”是顺口的叫法，谁都能喊。'); await win('5 处，在「第1章：巷口」'); await click('查看设定'); await pane('“小祥”是顺口的叫法，谁都能喊'); await click('全部设定'); await done('what the AI looked at')
  await click('复制全文'); await win('已复制'); must((await toastMark('已复制')).includes('green'), 'what was done has no tick'); await click('关闭提示')
  await win('设定里只写了他是她带出来的徒弟', true); await click('摘取原文', 1); await win('拖选一段原文，作为已确定的设定加入正文。原文不会被修改。'); await win('尚未选中文字'); await click('收起原文'); await win('尚未选中文字', false); await click('摘取原文', 1)
  await js(`(() => { const box = document.querySelector('.source-text'), r = document.createRange(); r.setStart(box.firstChild, 0); r.setEnd(box.firstChild, 18); const s = getSelection(); s.removeAllRanges(); s.addRange(r); box.dispatchEvent(new MouseEvent('mouseup', { bubbles: true })) })()`); await win('已选 18 字'); await shot('58b-source')
  await click('加入设定'); await win('已加入设定正文'); await pane('设定里只写了他是她带出来的徒弟。可以'); await click('关闭提示'); await done('what stands under a reply')
  await click('新对话'); await win('接着聊这部作品'); await win('从一个初步的想法开始', false); await shot('59-new-talk-page'); await click('接下来的情节怎么走'); await win('正在思考…', false); must(await js(`(() => { const b = document.activeElement; return b.tagName === 'TEXTAREA' && b.value === '接下来的情节怎么走' && b.selectionStart === b.value.length })()`), 'the words offered are not in the box with the caret after them'); await click('发送（Ctrl+Enter）'); await win('正在思考…'); await sleep(1500); await win('这是原型'); await win('2 条发言')
  await type('讨论输入', '再想想'); await key('Enter', true); await win('3 条发言'); await sleep(500); await win('正在查阅…'); await win('搜了讨论「停电」'); await sleep(1000); await win('先想清楚这一段要做什么'); await done('a new talk on its page')
  await win('记下：待确认 · 情节：小祥改口叫“姐”，是因为他知道井边的事？'); await pane('待确认'); await pane('3 条 AI 建议', false); await click('待确认'); await pane('小祥改口叫“姐”，是因为他知道井边的事？'); await click('全部设定'); await done('what the AI keeps as it talks')
  await click('当前模型'); await win('思考程度'); await shot('60-model'); await click('claude-sonnet-5-5'); await win('思考程度', false); await win('claude-sonnet-5-5'); await click('当前模型'); await click('High'); await click('当前模型'); await click('AI 配置…'); await win('选用模型'); await done('the model of a discussion')

  // Settings: the models and the services, the look, reminders, backups.
  must((await value('讨论模型')) === 'claude-sonnet-5-5', 'the settings show another model'); await shot('61-settings')
  // The heading of the settings stays where it is: what is under it scrolls up to it, not over it.
  must(await js(`(() => { const q = (c) => document.querySelector(c), was = q('.prefs-title').getBoundingClientRect().top; q('.prefs-body').scrollTop = 200; return q('.prefs-body').scrollTop > 0 && q('.prefs-title').getBoundingClientRect().top === was && q('.prefs-body').getBoundingClientRect().top >= q('.prefs-head').getBoundingClientRect().bottom - 1 })()`), 'the settings scroll over their heading'); await shot('61b-settings-scrolled')
  await win('上下文预算', false); await click('和讨论相同'); await click('DeepSeek'); must((await value('续写检查模型')) === 'deepseek-flash', 'choosing a service chose no model'); must(await js(`document.querySelector('.services').innerText.split('本作品在用').length === 3`), 'the service taken is not said to be in use')
  await click('High', 1); await click('Medium'); must(await js(`S.uses[1].effort`) === 'Medium', 'the effort of writing did not change'); await win('用量'); must(await js(`[...document.querySelectorAll('.spent > *')].map((e) => e.firstElementChild.innerText).join(' ')`) === '讨论 起草 改写 续写 检查', 'the spending is not told by feature')
  await click(await js(`[...document.querySelectorAll('.link')].find((e) => e.innerText.startsWith('查看全部')).innerText`)); await win('AI 记录'); await win('不存原文'); await win('失败：无法连接模型服务'); await win('已取消'); await shot('61c-records')
  await click('失败：无法连接模型服务'); await win('实际用量：输入 8.6K'); await inDialog('检查'); must(await js(`[...document.querySelectorAll('.record-row')].every((e) => e.innerText.includes('检查 ·'))`), 'the records shown are not only of the check'); await click('‹ 用量'); await win('选用模型'); await click('停笔后自动续写'); must(await js(`document.querySelectorAll('.ticks .check.on').length === 1`), 'the box is not ticked')
  await click('编辑', 1); await win('内容会发到这个地址'); await win('key 存在系统钥匙串里，不进作品。'); await win('上下文上限'); await win('压缩阈值 %'); must(await js(`[...document.querySelectorAll('[aria-label="上下文上限"]')].map((e) => e.value).join(' ')`) === '1M 1M 200K', 'the numbers of the models are not from the table'); await shot('62-service')
  await clear('最大输出'); await type('最大输出', '2M'); await win('claude-opus-5-5 的最大输出须小于上下文上限'); must(await has('.service-form .note.danger') && await has('.service-form .btn.primary.off'), 'a model that breaks a rule can be saved'); await click('移除'); must(!(await has('.service-form .note.danger')), 'the model taken off is still said to be wrong'); must(await js(`document.querySelectorAll('[aria-label="模型 ID"]').length`) === 2, 'the model was not taken off'); await click('获取可用模型'); await win('正在读取可用模型…'); await sleep(1400); await win('已获取保存后生效')
  await click('保存'); await win('已保存'); await win('收起', false); await click('添加服务'); must(await js(`document.querySelector('[aria-label="API key"]').placeholder`) === '不填就不用 key', 'a new service says it keeps a key'); await type('名称', 'DeepSeek'); must(await js(`document.querySelector('[aria-label="Provider ID（小写）"]').placeholder`) === 'deepseek', 'a new service does not take its ID from its name'); await win('和「DeepSeek」重名了，换一个'); must(await has('.service-form .btn.primary.off'), 'a new service can take the place of another')
  await clear('名称'); await type('名称', '本地'); await win('重名了', false); must(await has('.service-form .btn.primary.off'), 'a service with no ID can be saved'); await type('Provider ID（小写）', 'local'); await type('服务地址', 'localhost:11434'); await win('地址要以 http:// 或 https:// 开头'); await clear('服务地址'); await type('服务地址', 'http://'); await win('地址不完整'); await clear('服务地址'); await type('服务地址', 'http://me@localhost:11434/v1'); await win('地址里不要带账号，也不要带 ? 或 # 后面的部分'); await clear('服务地址'); await type('服务地址', 'http://localhost:11434/v1'); await win('本机服务：确认它不转发到网上'); await click('添加模型'); await type('模型 ID', 'qwen3')
  must((await value('上下文上限')) === '32K' && (await value('最大输出')) === '4096' && (await value('压缩阈值')) === '80', 'a model not in the table does not start from the careful numbers')
  await clear('上下文上限'); await type('上下文上限', '一百万'); await win('qwen3 的上下文上限和最大输出须为正整数，可写成 128K、1M'); must(await has('.service-form .btn.primary.off'), 'a model that breaks a rule can be saved'); await shot('62b-service-wrong')
  await clear('上下文上限'); await type('上下文上限', '131072'); await clear('最大输出'); await type('最大输出', '8k'); await click('保存'); must(await js(`S.services.at(-1).models.map((m) => m.max + ' ' + m.out).join()`) === '131072 8K', 'the numbers written as 8k are not kept as 8K'); await win('localhost:11434 · 1 个模型'); await sleep(3300); await done('models and services')
  await click('重新读取'); await click('打开配置文件'); await click('编辑', 1); await click('清除 key'); await click('保存'); await win('还有 1 条'); await shot('63-toasts'); await sleep(3300); await win('还有 1 条', false)
  await done('more notices than fit')
  await click('外观'); await win('这台电脑上的所有作品'); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('64-look'); await click('浅色')
  if (await js(`OFFER_NO_GPU`)) {
    await click('关闭显卡加速'); await win('重启软件后生效。'); await shot('64b-no-gpu'); await click('关闭显卡加速'); await win('重启软件后生效。', false)
    await click('关闭显卡加速'); await click('立即重启'); must(!(await has('.prefs-nav')) && await js(`S.noGPU && S.ranNoGPU`), 'reopening left the settings open or the GPU on')
    await settings(); await click('外观'); await win('重启软件后生效。', false); await click('关闭显卡加速'); await win('重启软件后生效。'); await click('立即重启'); await settings()
  } else await win('关闭显卡加速', false)
  await click('提醒'); await win('当前有 2 个伏笔久未回收。'); await click('增加 5 章'); await click('增加 5 章'); await click('增加 5 章'); await win('45 章'); await win('当前有 1 个伏笔久未回收。'); await click('边写边检查'); await shot('65-reminders')
  await click('备份'); await win('31.2 万字 · 8 章 · 已存 12 份，最早 09-28'); await shot('66-backup'); await click('立即备份'); await win('正在备份…'); await sleep(1400); await win('今天 10:40 已备份'); await win('已存 13 份')
  await click('写作时每小时一次'); await click('最近 30 份'); await click('最近 90 份'); await win('最近 30 份', false); await click('关闭'); await win('为整部作品另存一份副本', false); await click('正文'); await win('边写边检查 ▴'); await done('the look, reminders and backups')
  // 扩展: one ticked off on its own page goes from every place it shows, at once and with no question; ticked on, it is back as it was.
  const onTab = (): Promise<string> => js(`document.querySelector('#pane .tab.on').firstChild.textContent`)
  const inMenu = async (menu: string, row: string): Promise<boolean> => { await click(menu); const on = await js(`document.querySelector('.menu-pop').innerText.includes(${JSON.stringify(row)})`); await key('Escape'); return on }
  const prefTabs = (): Promise<string> => js(`[...document.querySelectorAll('.prefs-nav .nav')].map((e) => e.innerText).join(' ')`)
  // An extension's name is in the left of the settings, in the overview and on the switch in the head of its page: which one is meant.
  const nthOf = (name: string, sel: string): Promise<number> => js(`[...document.querySelectorAll('[data-click]')].filter((e) => e.innerText.split('\\n')[0].replace(/\\s+/g, '') === ${JSON.stringify(name)}).findIndex((e) => e.closest(${JSON.stringify(sel)}))`)
  const extTick = async (name: string) => click(name, await nthOf(name, '.ext-head'))
  const extPage = async (name: string) => { await click(name, await nthOf(name, '.ext-cell')); await win('‹ 扩展') }
  await click('视图'); await click('手机预览'); must(await onTab() === '预览', 'Phone Preview did not open its tab')
  await settings(); await click('扩展'); must(await prefTabs() === '外观 语言 扩展 AI 提醒 名字补全 备份 全角标点', 'the settings of a work hold: ' + await prefTabs()); await win('作品与文件'); await win('其他功能靠它们运行。'); await shot('66b-extensions')
  await extPage('手机预览'); await win('看看在手机上的样子'); await win('这台电脑上的所有作品'); await shot('66c-extension'); await extTick('手机预览'); await win('关闭「手机预览」？', false); await click('‹ 扩展')
  await extPage('导出'); await win('配合大纲、设定更好用'); await extTick('导出'); await click('‹ 扩展')
  await extPage('备份'); await extTick('备份'); await win('不再自动备份，旧备份留着。'); await click('‹ 扩展'); await shot('66d-extensions-off')
  must(await js(`[...document.querySelectorAll('.ext-cell.off .fit')].map((e) => e.innerText).join(' ')`) === '备份 导出 手机预览', 'the extensions turned off are not said to be off')
  must(await prefTabs() === '外观 语言 扩展 AI 提醒 名字补全 全角标点', 'with backup off the settings still hold its page: ' + await prefTabs())
  await click('关闭'); must(await onTab() === '设定', 'with Phone Preview off the pane is on ' + await onTab()); must(!(await js(`[...document.querySelectorAll('#pane .tab')].some((e) => e.innerText.startsWith('预览'))`)), 'the Preview tab is still there')
  must(!(await has('.backup-line')), 'the backup is still in the status bar'); must(!(await inMenu('文件', '导出…')), 'Export is still in the File menu'); must(!(await inMenu('视图', '手机预览')), 'Phone Preview is still in the View menu')
  await settings(); await click('扩展'); for (const name of ['手机预览', '导出', '备份']) { await extPage(name); await extTick(name); await click('‹ 扩展') }
  await click('关闭'); must(await onTab() === '预览', 'Phone Preview came back on ' + await onTab()); must(await has('.backup-line'), 'the backup did not come back'); must(await inMenu('文件', '导出…'), 'Export did not come back'); await done('extensions off and on')
  // One that others need asks first and names them; set for this work alone, it leaves the other works as they were.
  await settings(); await click('扩展'); await extPage('资料库'); await win('名字补全、设定、大纲、伏笔、讨论、续写、起草、检查要用它'); await extTick('资料库'); await win('关闭「资料库」？'); await win('也会一起关闭。'); await shot('66e-extension-ask'); await click('取消'); await win('关闭「资料库」？', false); await click('‹ 扩展')
  await extPage('字数目标'); await click('单独设置'); await win('只管这部作品'); await extTick('字数目标'); must(await js(`!extOn('goal') && !S.extOff.includes('goal')`), 'set for this work alone, the switch moved the one of this computer'); await shot('66f-extension-own')
  await click('恢复默认'); await win('这台电脑上的所有作品'); must(await js(`extOn('goal')`), 'back to the default, the goal is still off'); await click('关闭'); await done('extensions that others need, and one work apart')
  await settings(); await clear('讨论模型'); await click('关闭'); await click('讨论', 1); await click('全部讨论'); await click('新讨论'); await type('输入想讨论的内容', '在吗'); await key('Enter', true); await win('请先配置讨论模型'); await sleep(3300); await win('请先配置讨论模型'); await shot('67-no-model'); await click('打开设置'); await win('选用模型'); await win('请先配置讨论模型', false); await key('Escape'); await done('no model to talk with')

  // The check page.
  await click('重新开始'); await click('检查'); await win('已检查至「番外：晴天」'); await win('全部 5'); await win('1 / 5'); await win('设定 · 人物 · 林晚'); await win('其他章节的类似情况'); await shot('68-check')
  await click('下一条'); await win('2 / 5'); await click('名字有误 1'); await win('应为「周祥」'); await win('1 / 1'); await shot('69-check-name'); await click('修改正文'); await win('第84章：刻字'); must((await drawn('周翔')) === 'sel', 'the wrong name is not picked out in the prose'); await done('from a finding to the prose')
  await click('检查'); await click('新出现 2'); await win('可加入设定 · 人物'); await win('其他章节中也出现'); await shot('70-check-new'); await click('加入设定'); await win('已加入设定'); await win('新出现 1'); await click('关闭提示'); await click('忽略'); await win('这一类没有待处理的'); await done('names not yet in the setting')
  await click('不一致 2'); await click('无需修改'); await win('不一致 1'); await click('修改设定'); await pane('名字'); await pane('别名'); await win('第87章：停电'); await click('取消')
  await click('检查'); await click('无需修改'); await win('这一类没有待处理的'); await click('名字有误 1'); await click('无需修改'); await win('没有发现不一致'); await win('写完一章后自动检查，结果显示在此处。'); await shot('71-check-none'); await done('settle every finding')

  // Categories.
  await cats(); await win('“落点”指设定成熟后的去处'); must(await js(`document.querySelector('.cat-rows').innerText.replace(/\\s+/g, '').startsWith('名称落点人物设定库')`), 'the table does not say once over its rows what a target is'); await win('情节大纲'); await win('落点：', false); await shot('72-cats'); await type('分类名称', '线索'); must(await has('.cat-form .btn.primary.off'), 'a category with no target can be saved'); await win('将添加新分类。'); await type('落点', '大纲'); must(!(await has('.cat-form .btn.off')), 'a category filled in cannot be saved'); await click('保存分类'); await win('分类“线索”已保存'); await click('关闭提示')
  must(await js(`(() => { const t = document.querySelector('.cat-rows').innerText; return t.includes('线索') && t.indexOf('线索') < t.indexOf('其他') })()`), 'the new category is not before the last one'); await click('编辑', 1); must((await value('分类名称')) === '人物', 'the category is not ready to change'); await win('将修改已有的分类“人物”，现在的落点是“设定库”。'); await shot('72-cat-change')
  await click('取消'); must((await value('分类名称')) === '' && (await value('落点')) === '', 'putting the category away left the form filled in'); await click('编辑', 1)
  await click('删除'); await win('7 条设定将移到“其他”'); await shot('73-cat-delete'); await click('确认删除'); await win('已删除分类“人物”'); must(!(await js(`document.querySelector('.cat-rows').innerText.includes('人物')`)), 'the category is still listed'); await click('关闭提示')
  // An empty one goes at once: nothing to move, nothing to ask.
  await js(`[...document.querySelectorAll('.cat-row')].find((r) => r.innerText.startsWith('线索')).querySelector('.hit.link:last-child').click()`); await win('确认删除', false); await win('已删除分类“线索”'); must(!(await js(`document.querySelector('.cat-rows').innerText.includes('线索')`)), 'the empty category is still listed'); await click('关闭提示'); await done('categories')

  // The outline: chapters down, plot lines across, a card where they cross. A card is written where it lands; Enter goes on to the next.
  const order = (): Promise<string> => js(`S.events.map((e) => e.title).join('|')`) // the order in the story, which the timeline lays out left to right
  const cellOf = (key: string, line: string): Promise<string> => js(`document.querySelector('[data-cell="${key}|${line}"]').innerText`)
  const rowOf = (key: string): Promise<string> => js(`document.querySelector('[data-k="row-${key}"]').innerText`)
  const focused = (): Promise<string> => js(`document.activeElement.getAttribute('aria-label')`)
  const dragTo = (card: string, at: string) => js(`(() => {
    const dt = new DataTransfer(), to = document.querySelector(${JSON.stringify(at)})
    document.querySelector('[data-card="${card}"]').dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: dt }))
    to.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt }))
    to.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }))
    document.getElementById('app').dispatchEvent(new DragEvent('dragend', { bubbles: true, dataTransfer: dt }))
  })()`)
  await click('重新开始'); await click('大纲'); await win('8 章 · 3 条情节线 · 11 张卡片，2 张未放进章节'); await win('第三卷：停电'); await win('4 章 · 十月三日 — 十月六日夜'); await win('旧城的灯一盏一盏查下去')
  must((await rowOf('87')).includes('回溯'), 'a chapter going back in time is not told'); must(!(await rowOf('86')).includes('回溯'), 'a chapter in order is told as going back')
  must((await rowOf('87')).includes('故事中 二十六年前·秋 — 十月六日夜'), 'a chapter does not say when it happens: ' + await rowOf('87'))
  must((await cellOf('87', 'l3')).includes('小祥改口叫她“姐”'), 'a card is not where its chapter and line cross'); await win('何主任为什么锁着档案柜'); await shot('100-plan')
  await js(`document.querySelector('[data-cell="85|l3"] .slot-add').click()`); await win('新卡片'); must(await focused() === '事件', 'a new card does not wait to be written'); must(await has('.dialog .btn.primary.off'), 'a card with nothing in it can be added')
  await type('事件', '小祥数到第十三盏'); await click('继续添加'); await win('已添加「小祥数到第十三盏」，接着记下一张。'); must(await focused() === '事件' && await value('事件') === '', 'adding one more does not start the next card')
  await type('事件', '他把数字报错了'); await shot('101-plan-card'); await inDialog('添加'); await win('新卡片', false)
  must((await cellOf('85', 'l3')).includes('小祥数到第十三盏') && (await cellOf('85', 'l3')).includes('他把数字报错了'), 'the cards written are not in their cell')
  await js(`document.querySelector('[data-cell="2|l1"] .slot-add').click()`); await key('Escape'); await win('新卡片', false); await win('未命名', false)
  await js(`document.querySelector('[data-k="row-85"] .ch-gist').click()`); must(await focused() === '章纲', 'the outline does not open where it is'); await type('章纲', '何主任要一份新的灯数。'); await key('Escape'); await win('何主任要一份新的灯数。')
  await dragTo('ev9', '[data-cell="87|l2"]'); must((await cellOf('87', 'l2')).includes('林晚下井'), 'a card dragged from below does not go into the cell'); await win('13 张卡片，1 张未放进章节')
  await dragTo('ev9', '.tray'); await win('13 张卡片，2 张未放进章节'); must(!(await cellOf('87', 'l2')).includes('林晚下井'), 'a card dragged below stays in its chapter')
  await click('新增情节线'); must(await focused() === '情节线的名字', 'a new plot line does not wait for its name'); await clear('情节线的名字'); await type('情节线的名字', '钥匙'); await key('Enter'); await win('4 条情节线')
  await js(`document.querySelector('[aria-label="情节线「钥匙」的操作"]').click()`); await click('删除情节线'); await win('已删除情节线「钥匙」'); await win('3 条情节线'); await click('关闭提示')
  await click('小祥'); await clear('情节线的名字'); await type('情节线的名字', '周祥'); await key('Enter'); await win('周祥')
  // Scrolled sideways, the chapters stay at the left and 新增情节线 at the right.
  const edges = (): Promise<string> => js(`[document.querySelector('.grid .board-corner').getBoundingClientRect().left, document.querySelector('[data-k="row-85"]').getBoundingClientRect().left, document.querySelector('.board-end').getBoundingClientRect().right].join()`)
  await click('新增情节线'); await key('Enter'); const was = await edges(); await js(`document.querySelector('.grid').scrollLeft = 400`); must(await edges() === was, 'the chapters or 新增情节线 scroll away: ' + was + ' / ' + await edges()); await js(`document.querySelector('.grid').scrollLeft = 0`)
  await js(`document.querySelector('[aria-label="情节线「新情节线」的操作"]').click()`); await click('删除情节线'); await click('关闭提示')
  await type('搜索大纲', '何主任'); await win('第85章：灯数'); await win('第1章：巷口', false); await win('找到 2 处章名或章纲、2 张卡片'); await clear('搜索大纲')
  // Nothing found: one line in the middle and a way back, no empty table.
  await type('搜索大纲', '是谁让他锁'); await win('找到 1 张卡片'); await win('章节里没有「是谁让他锁」'); must(await has('.tray'), 'a card found below is hidden with the table'); await clear('搜索大纲')
  await type('搜索大纲', '123232'); await win('没有找到「123232」'); must(!(await has('.board')) && !(await has('.tray')), 'an empty table is left when nothing is found'); await shot('100-plan-none')
  await click('清除搜索'); await win('8 章 · 3 条情节线'); await type('搜索大纲', '123232'); await key('Escape'); must(await value('搜索大纲') === '', 'Esc does not clear the search'); await done('the outline by chapter')

  // The timeline: the same cards, left to right in the order things happen in the story, one row a plot line; the time may be left out.
  const colAt = (when: string): Promise<number> => js(`[...document.querySelectorAll('.tl-time:not(.board-corner) .tl-when')].findIndex((w) => w.textContent === ${JSON.stringify(when)})`)
  const slot = async (when: string, line: string) => `[data-slot="${await colAt(when)}|${line}"]`
  await click('时间线'); await win('已放进章节'); await win('「番外：晴天」还没有卡片'); must((await order()).includes('何主任让重新数灯|小祥数到第十三盏|他把数字报错了|档案室'), 'cards written in a chapter are not in its place in time: ' + await order())
  must(await js(`[...document.querySelectorAll('.tl-when')].filter((w) => w.textContent === '十月六日夜').length`) === 1, 'cards at one time are not one column')
  must((await js(`document.querySelector(${JSON.stringify(await slot('十月六日夜', 'l3'))}).innerText`)).includes('小祥改口叫她“姐”'), 'a card is not where its time and line cross'); must(await has('[data-card="ev11"].later'), 'a card in no chapter does not look so'); await shot('102-timeline')
  await click('周祥来当学徒'); await win('修改卡片'); must(await value('事件') === '周祥来当学徒', 'a card does not open to change'); await inDialog('第1章：巷口'); await inDialog('番外：晴天'); await shot('103-timeline-card'); await click('保存'); await win('「番外：晴天」还没有卡片', false)
  await click('周祥来当学徒'); await type('事件', '，跟着林晚'); await click('取消'); await win('周祥来当学徒，跟着林晚', false)
  await click('周祥来当学徒'); await inDialog('番外：晴天'); await inDialog('第1章：巷口'); await click('保存'); await win('「番外：晴天」还没有卡片')
  await dragTo('ev9', await slot('十月六日夜', 'l2')); must(await js(`S.events.find((e) => e.id === 'ev9').when`) === '十月六日夜', 'a card dragged to a time does not take it')
  must((await js(`document.querySelector(${JSON.stringify(await slot('十月六日夜', 'l2'))}).innerText`)).includes('林晚下井'), 'a card dragged to a time and line is not there')
  await js(`document.querySelector('[data-slot="end|l1"] .slot-add').click()`); await type('事件', '雨季结束'); await key('Enter'); must((await order()).endsWith('雨季结束'), 'a card added after the last is not at the end: ' + await order())
  await click('雨季结束'); await click('删除卡片'); await win('已删除卡片「雨季结束」'); must(!(await order()).includes('雨季结束'), 'the card is not deleted'); await click('关闭提示')
  // The time of a column is clicked to change, all its cards at once; 未写时间 too.
  const atTime = async (when: string) => { const n = await js(`(() => { const ws = [...document.querySelectorAll('.tl-when')].filter((w) => w.textContent === ${JSON.stringify(when)}); ws.at(-1).scrollIntoView({ inline: 'center' }); return ws.length - 1 })()`); await click(when, n) } // the last column of that time
  await atTime('未写时间'); await type('这一列的时间', '十二月'); await click('保存'); must(await js(`S.events.find((e) => e.id === 'ev11').when`) === '十二月', 'a column takes no time')
  await atTime('十月六日夜'); await clear('这一列的时间'); await type('这一列的时间', '十月七日夜'); await key('Enter'); must(await js(`S.events.filter((e) => e.when === '十月七日夜').length`) === 3, 'not all of a column takes its new time')
  await atTime('十月七日夜'); await clear('这一列的时间'); await type('这一列的时间', '十月六日夜'); await key('Enter')
  await click('新增情节线'); await key('Enter'); await win('新情节线'); must(await has('.tl [data-slot="0|' + await js(`S.lines.at(-1).id`) + '"]'), 'a new line has no row on the timeline')
  await type('搜索大纲', '停电'); must(!(await has('[data-card="ev7"].dim')) && await has('[data-card="ev9"].dim'), 'a search does not pick out the cards'); await shot('104-timeline-find'); await clear('搜索大纲'); await done('the timeline')

  // The chapter says when in the story it is, under its title: clicked, the timeline opens at it.
  await click('正文'); await click('第87章：停电'); await win('故事中 二十六年前·秋 — 十月六日夜'); await click('故事中 二十六年前·秋 — 十月六日夜'); await win('已放进章节'); must((await js(`document.querySelector('.card.seen').innerText`)).includes('母亲在停电那晚失踪'), 'the timeline does not open at the chapter'); await shot('105-chapter-when'); await done('the chapter in the story')
  // An outline file broken outside: one line in place of the page, read again as the window comes back.
  await click('少见的情况'); await click('大纲文件读不出来'); await win('大纲文件读不出来'); await win('第 12 行附近有错'); must(!(await has('.board')) && !(await has('[aria-label="搜索大纲"]')), 'the broken outline still shows the page'); await win('重试', false); await shot('106-outline-broken')
  await click('打开所在文件夹'); await win('原型不会打开系统窗口'); await click('关闭提示'); await js(`window.dispatchEvent(new Event('focus'))`); await win('8 章 · '); must(await has('.board'), 'the outline does not come back as the window does'); await done('an outline file broken outside')

  // Names of the setting in the prose: a faint dotted line; clicked, the entry opens beside; put away in the settings.
  await click('正文'); await click('第87章：停电'); must(await js(`[...document.querySelectorAll('.prose .nm')].some((e) => e.textContent === '小祥')`), 'a name in the prose is not marked')
  await js(`[...document.querySelectorAll('.prose .nm')].find((e) => e.textContent === '小祥').click()`); await pane('配角，林晚的同事'); await shot('105-name')
  await settings(); await click('名字补全'); await click('标出设定里的名字'); await key('Escape'); must(!(await js(`!!document.querySelector('.prose .nm')`)), 'names are marked when put away')
  await settings(); await click('名字补全'); await click('标出设定里的名字'); await key('Escape'); await done('names in the prose')

  // The phone preview: the chapter as a reader's phone lays it out, a screen at a time. Two long paragraphs typed at the end make it
  // run over: it follows the caret there, shows the caret and the words picked in the prose, marks the long ones, a paragraph clicked
  // in it puts the caret there in the prose, it flips only within the chapter, and scrolled it scrolls along with the prose.
  await click('视图'); await click('手机预览'); await win('第 1 屏，共 1 屏'); must(await has('.phone.framed'), 'the phone has no frame'); await shot('107-phone')
  const long = '雨水顺着屋檐往下淌，巷子尽头那盏灯晃了两下，终于灭了。她数着自己的脚步，一步，两步，三步，数到第十七步的时候，听见身后有人喊她的名字，声音很轻，像是怕惊动了什么。她没有回头，只是把手电攥得更紧，指节都发了白。'
  await caret((await paras()) - 1, 999); await key('Enter'); await text(long + long); await key('Enter'); await text(long + long)
  const screens = await js(`phoneScreens`); must(screens > 1, 'two long paragraphs still fit one screen'); await win(`第 ${screens} 屏，共 ${screens} 屏`)
  must(await js(`!document.querySelector('.phone-caret').hidden`), 'the caret is not shown in the phone'); await win('2 段超过 5 行，看下一处'); await shot('107-phone-long')
  await select((await paras()) - 1, '她没有回头'); must(await js(`CSS.highlights.get('phone-pick').size === 1 && document.querySelector('.phone-caret').hidden`), 'the words picked in the prose are not picked in the phone'); await shot('107-phone-pick')
  await caret((await paras()) - 1, 999); await win(`第 ${screens} 屏，共`)
  await click('‹ 上一屏'); await win(`第 ${screens - 1} 屏，共`); await click('下一屏 ›'); await win(`第 ${screens} 屏，共`)
  // A word clicked in the phone puts the caret after the same word in the prose.
  await caret(0, 0); await win('第 1 屏，共'); for (let n = 1; n < screens; n++) await click('下一屏 ›')
  const [px, py] = await js(`(() => { const r = [...document.querySelector('[data-phone="${(await paras()) - 1}"]').getClientRects()].at(-1); return [r.left + r.width / 2, r.top + r.height / 2] })()`)
  await js(`document.elementFromPoint(${px}, ${py}).dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: ${px}, clientY: ${py} }))`)
  const [on, off] = (await caretAt()).split(':').map(Number); must(on === (await paras()) - 1 && off > 0, 'a word clicked in the phone did not put the caret there: ' + await caretAt()); await win(`第 ${screens} 屏，共`)
  await click('Aa'); await click('A+'); await click('夜间'); must(await has('.phone-screen.dark') && await js(`S.phone.fs`) === 21, 'the night screen, a size larger, did not come'); await shot('107-phone-look')
  await click('恢复默认'); must(await has('.phone-screen.light') && await js(`S.phone.fs`) === 20, 'the defaults did not come back'); await click('Aa')
  const rolled = (to: string) => js(`(() => { const p = document.querySelector('.editor .paper.scroll'); p.scrollTop = ${to}; p.dispatchEvent(new Event('scroll')); const m = document.querySelector('.phone-page'); return [m.scrollTop, m.scrollHeight - m.clientHeight] })()`)
  await click('滚动'); await win('约 '); must(await has('.phone-page.rolled'), 'scrolling does not scroll')
  const [down, most] = await rolled('p.scrollHeight'); must(down > 0 && Math.abs(down - most) <= 1, `the phone did not scroll to the end with the prose: ${down} of ${most}`); await shot('107-phone-scroll')
  must((await rolled('0'))[0] === 0, 'the phone did not scroll back to the top with the prose'); await click('翻页')
  const reading = await js(`S.ch`); for (let n = 0; n < 20 && !(await has('.phone-next.off')); n++) await click('下一屏 ›')
  await win(`第 ${screens} 屏，共 ${screens} 屏`); must(await js(`S.ch`) === reading && !(await js(`document.querySelector('.phone-view').innerText.includes('下一章')`)), 'the phone reads on past the chapter')
  await click('设定'); await done('the phone preview')

  // The shelf, new and empty works, and the three ways a backup can stand.
  await click('重新开始'); await click('切换作品'); await click('全部作品…'); await win('我的作品'); await win('4 部 · 作品库：D:\\写作'); await win('找不到文件夹'); await shot('74-shelf')
  await click('列表'); await win('重新定位…'); await win('D:\\写作\\旧城灯匠'); await shot('75-shelf-list'); await click('卡片'); await click('更多操作'); await win('打开所在文件夹'); await click('重命名…'); await win('仅修改作品名，文件夹不变。'); await clear('新作品名'); await type('新作品名', '雨城'); await click('保存'); await win('雨城来信', false)
  await click('移除'); await win('已从书架移除文件夹未改动：早年的稿子'); await win('3 部'); await click('关闭提示'); await done('the shelf')
  // A book brought in: a small form for its name, kind and place, then a preview of the file cut into volumes and chapters, one chapter to change
  // (its title and its words); a chapter joined to the one before, or begun again at the caret, from beside the title; the undo of one change goes away at the next.
  await click('导入书籍…'); await win('识别出 3 卷 18 章'); await win('原文件不会改动'); await shot('74a-import-form')
  await clear('作品名称'); await type('作品名称', '雨城'); await win('文件夹「雨城」中已有文件，请更换名称或位置。'); must(await has('.dialog .btn.primary.off'), 'a book can be brought in where there are files'); await shot('74b-import-taken')
  await clear('作品名称'); await type('作品名称', '雨夜行舟'); await win('将新建文件夹「雨夜行舟」。'); await click('下一步'); await win('共 3 卷 18 章'); await win('第二卷：旧城'); await win('预览 · 雨夜行舟.txt')
  must(await has('.import-acts > .link:first-child.off'), 'the first chapter of a volume can be joined to the one before'); await win('请先在正文中点选要另起一章的位置'); await shot('74c-import-preview')
  await click('第一卷：雨夜'); await win('第1章 渡口', false); await click('第一卷：雨夜'); await win('第1章 渡口')
  await click('第9章 旧信'); must(await has('.import-acts > .link:last-child.off') && !(await has('.import-acts > .link:first-child.off')), 'with the caret nowhere a chapter can be begun again, or this one cannot be joined to the one before')
  await click('第10章'); await click('并入上一章'); await win('共 3 卷 17 章'); await win('已将「第10章」并入上一章'); await shot('74d-import-merged'); await click('撤销'); await win('共 3 卷 18 章')
  await click('第9章 旧信'); await js(`putCaret({ para: 1, off: 6 }, false, impBox())`); await click('从光标处另起一章'); await win('共 3 卷 19 章'); await win('已另起一章「未命名」')
  must(await js(`document.activeElement === impBox()`), 'the caret left the words for a new chapter'); await shot('74e-import-split')
  await js(`putCaret({ para: 1, off: 0 }, false, impBox())`); await text('第十章 回信'); await key('Enter'); must(await js(`impCur().paras[1]`) === '第十章 回信', 'the words typed are not in the chapter'); await win('已另起一章「未命名」', false)
  await js(`putCaret({ para: 1, off: 0 }, false, impBox())`); await click('从光标处另起一章'); await win('共 3 卷 20 章'); await win('已另起一章「第十章 回信」')
  await clear('章节标题'); await type('章节标题', '新的章名'); await win('新的章名'); await win('已另起一章「第十章 回信」', false); await shot('74f-import-title')
  // The tree of the preview works as the sidebar's does: the menu on the right button, a name changed in its row, up and down, into a volume, a drag; only the bin is a delete here.
  const ctx = async (label: string) => { await win(label); await js(`[...document.querySelectorAll('[data-k="imp-menu"] .row')].find((e) => e.innerText.includes(${JSON.stringify(label)})).click()`); await sleep(150) }
  const at = (title: string): Promise<number> => js(`S.imp.chapters.findIndex((c) => c.title === ${JSON.stringify(title)})`)
  const pulled = (from: string, to: string) => js(`(() => { const row = (k) => document.querySelector('[data-imp-node="' + k + '"]'), d = new DataTransfer(); for (const [e, k] of [['dragstart', ${JSON.stringify(from)}], ['dragover', ${JSON.stringify(to)}], ['drop', ${JSON.stringify(to)}], ['dragend', ${JSON.stringify(from)}]]) row(k).dispatchEvent(new DragEvent(e, { bubbles: true, cancelable: true, dataTransfer: d })) })()`)
  await rclick(...(await row('第9章旧信'))); await win('重命名'); await win('新建文件夹'); await win('移到回收站', false); await win('删除'); await over('移到'); await win('第三卷：回潮'); await shot('74h-import-menu')
  await ctx('重命名'); must(await has('.tree-row.naming'), 'the name does not turn into a box in its row'); await type('重命名', '第9章 旧信（改）'); await key('Enter'); await win('旧信（改）'); must(!(await has('.tree-row.naming')), 'Enter did not settle the name')
  { const was = await at('第9章 旧信（改）'); await rclick(...(await row('旧信（改）'))); await ctx('上移'); must((await at('第9章 旧信（改）')) === was - 1, 'up did not move the chapter before the one above'); await rclick(...(await row('旧信（改）'))); await ctx('下移'); must((await at('第9章 旧信（改）')) === was, 'down did not bring it back') }
  await rclick(...(await row('旧信（改）'))); await over('移到'); await ctx('第三卷'); must(await js(`S.imp.chapters.find((c) => c.title === '第9章 旧信（改）').vol === S.imp.volumes[2][0]`), 'the chapter did not go into the volume picked'); await rclick(...(await row('旧信（改）'))); await over('移到'); await ctx('最上层'); must(await js(`S.imp.chapters.find((c) => c.title === '第9章 旧信（改）').vol === ''`), 'the chapter did not go to the top'); await shot('74i-import-top')
  await rclick(...(await row('旧信（改）'))); await ctx('删除'); await win('已删除「第9章 旧信（改）」'); must((await at('第9章 旧信（改）')) < 0, 'the chapter is still there'); await click('撤销'); must((await at('第9章 旧信（改）')) >= 0, 'undo did not bring the chapter back')
  await click('新建章节'); must(await has('.tree-row.naming'), 'a new chapter waits for no name'); await type('重命名', '楔子'); await key('Enter'); await win('楔子'); await win('共 3 卷 21 章')
  await rclick(...(await row('楔子'))); await ctx('新建文件夹'); await type('重命名', '番外'); await key('Enter'); await win('共 4 卷 21 章')
  await pulled(await js(`S.imp.chapters.find((c) => c.title === '楔子').id`), await js(`S.imp.volumes.find((v) => v[1] === '番外')[0]`)); must(await js(`S.imp.chapters.find((c) => c.title === '楔子').vol === S.imp.volumes.find((v) => v[1] === '番外')[0]`), 'a chapter dragged onto a volume did not go into it'); await shot('74j-import-dragged')
  await rclick(...(await row('番外'))); await ctx('删除'); await win('已删除「番外」'); await win('共 3 卷 20 章'); await click('关闭提示')
  await click('上一步'); await win('将新建文件夹「雨夜行舟」。'); await click('下一步'); await win('共 3 卷 20 章'); await js(`set({ look: '深色' })`); await shot('74g-import-dark'); await js(`set({ look: '跟随系统' })`)
  await click('确定')
  await win('已导入 20 章'); await win('暂无章节'); await click('切换作品'); await click('全部作品…'); await win('4 部'); await win('雨夜行舟'); await click('更多操作'); await click('从书架移除'); await click('关闭提示'); await win('3 部'); await done('a book brought in')
  await click('新建作品'); await win('将新建文件夹「未命名作品」。'); must((await value('作品位置')) === 'D:\\写作\\未命名作品', 'where the work goes is not offered'); await type('作品名称', '新书'); must((await value('作品位置')) === 'D:\\写作\\新书', 'where the work goes does not follow its name'); await click('网文'); await click('剧本'); await shot('76-new-book'); await click('创建')
  await win('暂无章节'); await win('新书'); await win('尚未备份'); await pane('打开一章后，这里列出它的批注'); await click('设定'); await pane('暂无设定'); await shot('77-empty-book'); await click('新建第一章'); await key('Enter'); await win('在此开始写作…'); await win('本章 0 字 · 全书 0 字'); await win('设定为空，无可对照内容'); await click('检查'); await click('检查全书'); await win('请先在讨论中确定设定，才能对照检查。'); await click('关闭提示'); await click('文件'); await click('导出…'); await inDialog('设定'); await win('还没有已定的设定，不导出设定。'); await inDialog('正文'); must(await has('.dialog .btn.primary.off'), 'a work with nothing in it can be exported'); await inDialog('正文'); await inDialog('设定'); await click('取消'); await click('讨论'); await win('从一个初步的想法开始'); await win('我想写一个总在下雨的城市'); await click('正文'); await done('a new work')
  await click('尚未备份'); await win('作品目前仅保存在本机。'); await click('选择位置…'); await win('选择其他位置，定期保存副本。'); await win('OneDrive 同步文件夹'); await shot('78-first-backup'); await click('开始备份'); await win('正在备份…'); await sleep(1400); must(await has('[title="已备份 · 今天 10:40"]'), 'the status bar does not say it was backed up at 今天 10:40'); await done('a first backup')
  await click('切换作品'); await click('井边'); await win('备份失败 · 找不到 E 盘'); await win('从左侧选择一章开始写作', false); await win('暂无章节'); await shot('79-backup-failed'); await click('待处理 1'); await win('备份失败，找不到 E 盘'); await win('点击重试 · 上次备份 昨天 21:40'); await key('Escape')
  await click('备份失败 · 找不到 E 盘'); await win('找不到 E 盘，本次备份未完成'); await win('作品未受影响，请接入硬盘后重试。'); await shot('80-backup-trouble'); await click('今日不再提醒'); must(!(await has('.warn-btn')), 'the warning is still on the toolbar')
  await click('备份失败'); await click('重试'); await sleep(1400); must(await has('[title="已备份 · 今天 10:40"]'), 'the backup, tried again, did not go through'); must(!(await has('.todo-btn')), 'the failed backup is still pending'); await done('a backup that failed')
  await click('切换作品'); await click('旧城灯匠'); await settings(); await click('备份'); await win('尚未备份'); await click('选择位置…'); await click('稍后再说'); await win('尚未备份'); await click('关闭')
  await click('切换作品'); await click('全部作品…'); for (let i = 0; i < 4; i++) { await click('更多操作'); await click('从书架移除'); await click('关闭提示') }
  await win('从一个想法开始'); await win('API key 保存在系统钥匙串中，不会写入作品文件夹。'); await shot('81-empty-shelf'); await click('新建作品'); await click('取消'); await click('重新开始'); await win('第87章：停电'); must(await has('[title="已备份 · 今天 10:32"]'), 'the status bar does not say it was backed up at 今天 10:32'); await done('an empty shelf, and start over')

  // A volume as a page: its outline, what it holds, a chapter made in it.
  await outline('第二卷：旧城'); await win('1 项'); await win('本卷 14.1 万字'); await type('卷纲', '林晚回到旧城，找到那把钥匙。'); await click('在此新建章节'); await key('Enter'); await win('第47章'); await win('第47章：', false); await win('满 200 字后可检查')
  await click('第二卷：旧城', 1); await win('2 项'); must((await value('卷纲')) === '林晚回到旧城，找到那把钥匙。', 'the outline of the volume is lost'); await pane('附上本卷内容', false); await click('讨论', 1); await click('新讨论'); await pane('附上本卷内容'); await click('全部讨论'); await click('设定'); await shot('82-folder-own'); await done('a volume as a page')

  // A short name picked in the prose goes into the setting as it is.
  await click('第85章：灯数'); await menu(0, '何主任'); await over('加入名字表'); must(await js(`!!document.querySelector('.side-pop')`), 'the pointer over the row opened nothing'); await over('加入设定'); must(await js(`!document.querySelector('.side-pop')`), 'the list stayed as the pointer went on'); await over('加入名字表'); await click('加入名字表'); must(await js(`document.querySelector('.side-pop').innerText.includes('组织') && document.querySelector('.side-pop').getBoundingClientRect().left > document.querySelector('.ctx').getBoundingClientRect().right - 8`), 'the categories of names are not beside the menu'); await shot('83-name-menu'); await js(`document.querySelector('.side-pop .row').click()`)
  await win('「何主任」已加入设定的人物输入时可自动补全'); await click('关闭提示'); await menu(0, '何主任'); await win('已在名字表中'); await key('Escape')
  await menu(0, '何主任让他们把旧城的灯重新数一遍'); must(await js(`[...document.querySelectorAll('.ctx .row')].find((e) => e.innerText.includes('加入名字表')).classList.contains('off')`), 'a long pick can be a name'); await key('Escape'); await done('a name from the prose')

  // Saving, and what is stale once the prose changes.
  await click('第86章：档案室'); must(!(await unsaved()), 'a chapter just opened is not saved'); await key('j', true); await sleep(900); await key('Tab'); must(await unsaved(), 'the tab does not say the chapter is not saved'); await win('本章已修改，尚未重新检查'); await sleep(1800); must(!(await unsaved()), 'the chapter was not saved by itself')
  await key('s', true); await win('停笔片刻也会自动保存'); await click('关闭提示'); await done('saving')

  // Rare things, shown from the bar over the window: a chapter changed outside, a check that failed.
  await click('第87章：停电'); await click('少见的情况'); await click('本章在软件外被修改'); await win('本章文件已被其他程序修改。请选择保留的版本，另一版本将存为历史版本。'); await shot('84-outside')
  await click('使用磁盘版本'); await win('已采用磁盘版本编辑器原内容已存为历史版本'); await win('（这一句是在别的程序里加上的。）'); await win('本章文件已被其他程序修改', false); await click('关闭提示')
  await click('文件'); await click('历史版本'); await win('外部改动'); await win('该历史版本与当前正文相同', false); await click('关闭')
  await click('少见的情况'); await click('本章在软件外被修改'); await click('保留当前版本'); await win('已保留编辑器版本磁盘内容已存为历史版本'); await click('关闭提示'); await done('a chapter changed outside')
  // A chapter too long for the model: the check says how much of it was read.
  await click('少见的情况'); await click('本章过长，仅检查前文'); await win('仅检查前 5,000 字'); await win('检查失败', false); await win('检查完成', false); await shot('86-read-in-part'); await done('a chapter read in part')
  await click('少见的情况'); await click('本章检查失败'); await win('检查失败：无法连接模型服务'); await shot('86-check-failed'); await click('检查'); await click('名字有误 1'); await click('无需修改'); await win('检查未完成'); await win('这一类没有待处理的', false); await win('章失败'); await win('已检查至', false); await shot('86-check-unfinished'); await click('正文'); await done('a check that failed')
  await click('少见的情况'); await click('未选择续写和检查模型'); await win('检查失败：请在设置中「AI」配置里选择续写和检查模型'); await done('no model to write with')

  // A narrow window narrows the sidebar, the pane and the margins of the paper, so that a line of prose still holds about thirty characters.
  // The sidebar lists what it always does: the chapters, and on the talk page the talks.
  const sides = (): Promise<string> => js(`[document.querySelector('.sidebar').offsetWidth, document.getElementById('pane').offsetWidth].join(' ')`)
  const perLine = (): Promise<number> => js(`(() => { const p = document.querySelector('.prose'); return Math.floor(p.clientWidth / parseFloat(getComputedStyle(p).fontSize)) })()`)
  await click('少见的情况'); await click('窗口变窄'); must(await sides() === '200 320', 'in a narrow window the sidebar and the pane are: ' + await sides())
  must(await perLine() >= 28, 'a line of prose in a narrow window holds only ' + await perLine() + ' characters'); await win('第84章：刻字'); await shot('87-narrow')
  await click('讨论'); must(await has('.sidebar .tree-row') && await sides() === '200 320', 'the talk page of a narrow window does not list the talks beside its pane'); await shot('87b-narrow-talk')
  await click('正文'); await click('少见的情况'); await click('窗口恢复原宽'); must(await sides() === '232 390', 'the sides stayed narrow: ' + await sides()); await done('a narrow window')

  // Ctrl+= zooms the whole window: it is laid out 1/z as large and shown z times larger. The window grows to hold at least 1000 by 680
  // of it, and 放大 stops at the step the screen cannot hold; a layout narrower than 1180 is a narrow window.
  const zoom = (): Promise<string> => js(`(() => { const w = document.getElementById('win'), r = w.getBoundingClientRect(); return [S.zoom, w.offsetWidth + 'x' + w.offsetHeight, Math.round(r.width) + 'x' + Math.round(r.height)].join(' ') + (w.classList.contains('narrow') ? ' narrow' : '') })()`)
  const menuGrey = (row: string): Promise<boolean> => js(`[...document.querySelectorAll('.menu-pop .row')].find((e) => e.innerText.includes(${JSON.stringify(row)})).classList.contains('off')`)
  await click('第87章：停电'); await key('=', true); await key('=', true); must(await zoom() === '1.25 1013x680 1266x850 narrow', 'zoomed to 125%: ' + await zoom()); await shot('88-zoom-125')
  { const [x, y] = await select(0, '整条街的灯'); await rclick(x, y); const at: number[] = await js(`(() => { const c = document.querySelector('.ctx').getBoundingClientRect(); return [c.left, c.top] })()`)
  must(Math.abs(at[0] - x) < 2 && Math.abs(at[1] - y) < 2, 'the menu of the prose opened at ' + at + ', not at ' + [x, y]); await key('Escape') }
  await click('视图'); await win('缩放 125%'); must(!(await menuGrey('实际大小')), '实际大小 is grey at 125%'); await shot('88b-zoom-menu'); await click('放大')
  must(await zoom() === '1.5 1000x680 1500x1020 narrow', 'zoomed to 150%: ' + await zoom()); await click('视图'); must(await menuGrey('放大'), '放大 goes past what the screen holds'); await key('Escape'); await shot('88c-zoom-150')
  await key('-', true); must((await zoom()).startsWith('1.25 '), 'Ctrl+- did not zoom out'); await key('0', true); must(await zoom() === '1 1266x780 1266x780', 'Ctrl+0 did not zoom back: ' + await zoom())
  await key('-', true); must(await zoom() === '0.9 1407x867 1266x780', 'zoomed to 90%: ' + await zoom()); await shot('88d-zoom-90')
  await click('视图'); must(await menuGrey('实际大小') === false, '实际大小 is grey at 90%'); await click('实际大小'); must((await zoom()).startsWith('1 '), '实际大小 did not zoom back'); await done('zoom the window')

  // The whole book checked: the chapters changed since, one after another.
  await click('检查'); await click('重新检查'); await win('2 章未检查或有改动'); await win('每章调用一次 AI，共 2 次。'); await shot('87-check-all'); await click('取消'); await win('2 章未检查或有改动', false)
  await click('重新检查'); await click('开始检查'); await win('正在检查「第86章：档案室」，还有 1 章'); await sleep(1000); await win('正在检查「第87章：停电」'); await win('还有', false); await click('停止'); await win('1 章失败'); await win('已检查至', false); await click('重新检查'); await win('1 章未检查或有改动'); await click('开始检查'); await sleep(1100)
  await click('重新检查'); await win('各章均已检查正文无新改动'); must(await toastMark('各章均已检查') === 'none', 'a word on what to do first wears a mark'); await shot('14-toast'); await click('关闭提示'); await click('正文'); await win('本章检查完成，有 1 处待处理'); await done('the whole book checked')

  // A chapter the disk lost: what is in the editor becomes a chapter of its own.
  await click('少见的情况'); await click('本章文件丢失'); await win('磁盘上已找不到本章文件：在软件外改名、移动或删除，均视为删除。编辑器中的内容仍保留。'); must(!(await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第87章'))`)), 'the lost chapter is still in the tree'); await shot('88-lost')
  await click('存为新章节'); await win('已存为新章节位于最上层末尾'); await win('磁盘上已找不到本章文件', false); must(await js(`document.querySelector('.tree').lastElementChild.innerText.includes('停电')`), 'the chapter was not kept at the end'); await click('关闭提示')
  // A talk that proposes in a chapter not shown opens it in a tab beside, and stays where the author is.
  await click('少见的情况'); await click('讨论里提议修改另一章'); await pane('建议修改「第86章：档案室」'); await pane('换掉一句 · 23 字 · 等你决定'); must((await tabs()).includes('86') && !(await tabs()).includes('86*'), 'the chapter proposed in is not opened beside'); await shot('89-elsewhere')
  await click('在正文中查看'); await win('本章有 1处改稿建议'); await win('白天也得开着灯'); await click('全部放弃'); await pane('已放弃'); await win('本章有 1处改稿建议', false); await done('a proposal in another chapter')
  await click('少见的情况'); await click('回复超出最大输出'); await win('回复超出模型的最大输出 128K（含思考）'); await win('请在设置中「AI」配置里调大'); await click('打开设置'); await win('选用模型'); await key('Escape'); await click('重新开始'); await done('a chapter the disk lost')

  // The pane: a renamed entry and the prose, its width.
  await click('林晚'); await click('修改'); await clear('名字'); await type('名字', '林婉'); await click('保存'); await click('全部设定'); await pane('处「林晚」'); await shot('89-renamed')
  await click('全书替换'); must((await value('查找')) === '林晚' && (await value('替换为')) === '林婉', 'the old name is not ready to replace'); await win('全书 '); await pane('处「林晚」', false); await click('关闭查找')
  await click('周祥'); await click('修改'); await clear('名字'); await type('名字', '周详'); await click('保存'); await click('全部设定'); await click('不替换'); await pane('正文中仍有', false); await done('a name changed in the setting')
  await click('少见的情况'); await click('讨论查阅达到上限'); await pane('已达到本次查阅上限，以上回答仅依据已查到的内容。'); await pane('搜了全书「井」'); await shot('90-capped'); await done('a reply that looked as far as it may')
  await click('少见的情况'); await click('设定较多，只附目录'); await pane('设定较多，本次只附目录'); await pane('人物 · 林婉：主角，靠维修路灯谋生，但她很怕黑'); await pane('悬而未决：地点：旧城小学对面有一家钟表铺，老周在那里修表'); await shot('90b-crowded'); await click('设定'); await done('a setting too big to send whole')
  { const [x, y] = await js(`(() => { const r = document.querySelector('.pane-edge').getBoundingClientRect(); return [r.x + 3, r.y + 300] })()`) as [number, number]; await drag([x, y], [x - 120, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 510, 'the pane is not wider'); await shot('91-pane-wide')
    await drag([x - 120, y], [x + 400, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 300, 'the pane is narrower than it may be'); await drag([x + 90, y], [x, y]); await done('the width of the pane') }

  // Foreshadowings: the one picked to pay off shown in the pane, a list with nothing in it.
  await click('伏笔'); await click('第87章：停电'); await menu(5, '她往前走了一步。'); await click('记为回收…'); await click('井沿上的刻痕'); await win('第2章：井沿 · 已回收'); await pane('「第85章：灯数」里已有一处回收'); await shot('92-paying'); await key('Escape')
  await click('更多'); await click('已放弃'); await click('红色钥匙'); await click('恢复伏笔'); await click('全部伏笔'); await pane('此处暂无伏笔'); await click('全部伏笔'); await done('the foreshadowing picked')

  // The tree: a chapter dragged into a volume, and before another chapter.
  const pull = (from: string, to: string) => js(`(() => { const row = (k) => document.querySelector('[data-node="' + k + '"]'), d = new DataTransfer(); for (const [e, k] of [['dragstart', ${JSON.stringify(from)}], ['dragover', ${JSON.stringify(to)}], ['drop', ${JSON.stringify(to)}], ['dragend', ${JSON.stringify(from)}]]) row(k).dispatchEvent(new DragEvent(e, { bubbles: true, cancelable: true, dataTransfer: d })) })()`)
  await pull('87', 'v2'); must(await js(`document.querySelector('[data-node="87"]').classList.contains('deep') && document.querySelector('[data-node="87"]').previousElementSibling.dataset.node === '46'`), 'the chapter is not at the end of the volume')
  await pull('87', '46'); must(await js(`document.querySelector('[data-node="87"]').nextElementSibling.dataset.node === '46'`), 'the chapter is not before the other'); await shot('93-dragged'); await done('chapters dragged')

  // Settings: a local service, a variable found.
  await settings()
  await click('添加服务'); await win('ID（小写字母、数字、- _）'); await win('将发送到该地址', false); await type('服务地址', 'http://localhost:11434/v1'); await win('本机服务：确认它不转发到网上'); await type('环境变量名称', 'DEEPSEEK_API_KEY'); await win('已检测到环境变量 DEEPSEEK_API_KEY。'); await shot('94-service-local'); await click('取消'); await win('ID（小写字母、数字、- _）', false); await win('添加服务')
  await click('少见的情况'); await click('模型目录读取失败'); await win('读取模型目录失败'); await win('打开设置'); await click('重试'); await win('读取模型目录失败', false); await win('正在读取可用模型…'); await sleep(1400); await click('关闭提示'); await click('关闭'); await done('settings that do not hold')

  // Backups that fail in the other two ways.
  await click('少见的情况'); await click('备份文件夹丢失'); await click('备份失败 · 找不到备份文件夹'); await win('找不到备份文件夹，本次备份未完成'); await win('作品未受影响，文件夹可能已被移动或改名。上次备份于今天 10:32。'); await key('Escape')
  await click('少见的情况'); await click('备份文件夹不可写'); await click('备份失败 · 没有权限写入'); await win('备份文件夹无法写入，本次备份未完成'); await win('作品未受影响，该文件夹没有写入权限。'); await shot('95-backup-denied'); await click('重试'); await sleep(1400); must(await has('[title="已备份 · 今天 10:40"]'), 'the status bar does not say it was backed up at 今天 10:40'); await done('backups that fail otherwise')

  // The shelf: a work added from disk, a place taken, a name left empty.
  await click('切换作品'); await click('全部作品…'); await click('添加已有作品…'); await win('已加入书架'); await win('井边的旧稿'); await win('尚未打开'); await win('5 部'); await click('关闭提示'); await click('添加已有作品…'); await win('该作品已在书架上'); await click('关闭提示')
  await click('少见的情况'); await click('添加的文件夹中没有作品'); await win('该文件夹中没有作品（缺少 work.yaml）'); await click('关闭提示')
  await click('新建作品'); await type('作品名称', '雨城来信'); await win('文件夹「雨城来信」中已有文件，请更换名称或位置。'); must(await has('.dialog .note.danger'), 'a place with files in it is not said in red'); must(await has('.dialog .btn.primary.off'), 'a work can be made where there are files'); await shot('96-place-taken'); await clear('作品位置'); await win('请填写作品位置。'); must(!(await has('.dialog .note.danger')) && await has('.dialog .btn.primary.off'), 'a work can be made nowhere'); await click('取消')
  await click('更多操作'); await click('重命名…'); await clear('新作品名'); must(await has('.dialog .btn.primary.off'), 'a work can be named nothing'); await click('取消'); await click('重新开始'); await done('the shelf, where things go wrong')

  // Tabs: a chapter opened gets one beside the one shown, and they close one by one, or all but one.
  must((await tabs()) === '87*', 'a book opens with more than its last chapter: ' + (await tabs()))
  await click('第三卷：停电'); must((await tabs()) === '87*', 'a volume clicked took a tab: ' + (await tabs())); await click('第三卷：停电')
  await click('第86章：档案室'); await click('第84章：刻字'); must((await tabs()) === '87 86 84*', 'opening two more: ' + (await tabs())); await win('巷口那盏灯又坏了'); await shot('97-tabs')
  await click('第87章：停电', 1); must((await tabs()) === '87* 86 84', 'a tab clicked: ' + (await tabs())); await win('整条街的灯是同时灭的')
  await click('第85章：灯数'); must((await tabs()) === '87 85* 86 84', 'a new one goes beside the one shown: ' + (await tabs()))
  await shut('85'); must((await tabs()) === '87 86* 84', 'the one shown closed: ' + (await tabs())); await shut('84'); must((await tabs()) === '87 86*', 'another closed: ' + (await tabs()))
  await click('第84章：刻字'); await rclick(...(await tabAt('87'))); await win('关闭其他'); await win('关闭右侧'); await shot('98-tab-menu'); await click('关闭其他'); must((await tabs()) === '87*', 'all but one closed: ' + (await tabs()))
  await shut('87'); must((await tabs()) === '', 'the last one closed: ' + (await tabs())); await win('从左侧选择一章开始写作'); await click('上一个打开的章节'); must((await tabs()) === '87*', 'going back: ' + (await tabs()))
  await outline('第一卷：雨季'); must((await tabs()) === '87 v1*', 'the outline of a volume has no tab: ' + (await tabs())); await win('卷纲'); await shut('v1'); await done('tabs')

  // Typing in the prose. The tab says the chapter is not saved until the pause after the last key, or Ctrl+S.
  await click('重新开始'); await click('第84章：刻字'); const had = await paras()
  await caret(0, 4); await text('今天'); await win('巷口那盏今天灯又坏了'); must(await unsaved(), 'typing left the tab saying saved'); await win('本章 3,104 字'); must((await caretAt()) === '0:6', 'the caret is not after what was typed: ' + (await caretAt())); await shot('99-typed')
  must((await drawn('灯罩内侧刻着一个“晚”字，笔画很浅，像是用钥匙尖划的。')) === 'fs', 'typing above a foreshadowing lost its line')
  await key('s', true); await win('停笔片刻也会自动保存'); must(!(await unsaved()), 'Ctrl+S left the dot'); await click('关闭提示'); await caret(0, 6)
  await text('的'); await sleep(900); await text('的'); await sleep(900); must(await unsaved(), 'saved while the author was still typing'); await sleep(1000); must(!(await unsaved()), 'not saved after the pause'); await win('巷口那盏今天的的灯又坏了')
  await key('Enter'); must((await paras()) === had + 1, 'Enter did not start a paragraph'); must((await caretAt()) === '1:0', 'the caret is not at the head of the new paragraph: ' + (await caretAt())); must((await drawn('灯罩内侧刻着一个“晚”字，笔画很浅，像是用钥匙尖划的。')) === 'fs', 'a paragraph more lost the foreshadowing below')
  await key('Backspace'); must((await paras()) === had, 'Backspace did not join the paragraphs'); await win('巷口那盏今天的的灯又坏了'); must((await caretAt()) === '0:8', 'the caret is not where the paragraphs joined: ' + (await caretAt()))
  await key('Backspace'); await key('Delete'); await win('巷口那盏今天的又坏了'); await key('z', true); must((await paras()) === had + 1, 'undoing did not bring the paragraph back'); await key('z', true); await win('巷口那盏今天的的灯又坏了'); must((await caretAt()) === '0:8', 'the caret is not where the undoing ended: ' + (await caretAt()))
  await js(`(() => { const d = new DataTransfer(); d.setData('text/plain', '甲\\r\\n乙'); document.querySelector('.prose').dispatchEvent(new ClipboardEvent('paste', { clipboardData: d, bubbles: true, cancelable: true })) })()`)
  must((await paras()) === had + 1, 'two lines pasted are not two paragraphs'); await win('巷口那盏今天的的甲'); must((await caretAt()) === '1:1', 'the caret is not after what was pasted: ' + (await caretAt())); must(await unsaved(), 'pasting left the tab saying saved')
  await click('第87章：停电'); must(!(await unsaved()), 'the chapter gone to is said not to be saved'); await click('第84章：刻字', 1); must(!(await unsaved()), 'the chapter left was not saved'); await win('巷口那盏今天的的甲')
  await click('第87章：停电', 1); await caret(0, 0); await key('Enter'); must((await drawn('她往前走了一步。')) === 'doubt', 'a paragraph more above lost what the check found')
  await click('新建章节（Ctrl+N）'); await key('Enter'); await win('在此开始写作…'); await caret(0, 0); await text('雨停了'); await win('在此开始写作…', false); await win('本章 3 字'); await key('Enter'); await text('她'); must((await paras()) === 2, 'the second paragraph of a new chapter'); await win('本章 4 字')
  await key('j', true); await sleep(900); must(await js(`!document.querySelector('.prose[contenteditable]')`), 'the prose takes typing while a sentence waits to be taken'); await key('Escape'); must(await js(`!!document.querySelector('.prose[contenteditable]')`), 'the prose does not take typing again'); await done('typing')

  // The name of a new chapter: its row stays a row, the number is offered with the name left to type, and leaving
  // the box settles it as Enter does.
  const naming = (): Promise<{ value: string, from: number, to: number, row: string, pic: boolean, off: number, high: number } | null> => js(`(() => {
    const i = document.querySelector('.tree-row.naming .in'), other = [...document.querySelectorAll('.tree-row.deep:not(.naming) .fit')].at(-1)
    return i && { value: i.value, from: i.selectionStart, to: i.selectionEnd, row: i.parentElement.className, pic: !!i.parentElement.querySelector('svg'), off: i.getBoundingClientRect().left + parseFloat(getComputedStyle(i).paddingLeft) - other.getBoundingClientRect().left, high: i.parentElement.offsetHeight - other.parentElement.offsetHeight }
  })()`)
  await click('重新开始'); await click('第87章：停电'); await click('新建章节（Ctrl+N）')
  let box = await naming()
  must(!!box && box.value === '第88章：' && box.from === 5 && box.to === 5, 'a new chapter is not offered its number, the name left to type: ' + JSON.stringify(box))
  must(!box!.pic && box!.row.includes('deep') && Math.abs(box!.off) < 1 && box!.high === 0, 'the box for the name is not in line with the other rows: ' + JSON.stringify(box)); await shot('99c-naming')
  const inProse = await js(`(() => { const r = document.querySelector('.prose').getBoundingClientRect(); return [r.x + 40, r.y + 10] })()`) as [number, number]
  await text('雨停'); await drag(inProse, inProse); must(!(await naming()), 'leaving the box did not settle the name'); await win('第88章：雨停')
  await click('新建章节（Ctrl+N）'); await click('第84章：刻字'); must(!(await naming()), 'a click on another row did not settle the name'); await win('第89章')
  must(await js(`document.querySelector('.sheet.on').innerText.includes('刻字')`), 'the click that settled the name did not open the row it was on')
  await rclick(...(await row('第88章：雨停'))); await click('重命名'); box = await naming()
  must(!!box && box.value === '第88章：雨停' && box.from === 0 && box.to === 7, 'renaming does not start with the whole name picked: ' + JSON.stringify(box))
  await key('Escape'); must(!(await naming()), 'Esc left the box open'); await win('第88章：雨停')
  // What is typed over the title is the title: a number of the author's own is not given another before it, and a
  // title written as 第 N 章 is numbered again.
  await rclick(...(await row('第88章：雨停'))); await click('重命名'); await text('第一章：雨'); await key('Enter'); await win('第一章：雨'); await win('第88章：第一章', false)
  await rclick(...(await row('第一章：雨'))); await click('重命名'); await text('第90章：晴'); await key('Enter'); await win('第90章：晴'); await win('第一章', false)
  await click('新建章节（Ctrl+N）'); await text('第九十一章 雪'); await key('Enter'); await win('第九十一章 雪'); await win('：第九十一章', false)
  // F2 names the chapter that is open, wherever the caret is, its whole title picked.
  await click('第84章：刻字'); await drag(inProse, inProse); await key('F2'); box = await naming()
  must(!!box && box.value === '第84章：刻字' && box.from === 0 && box.to === 7, 'F2 does not start naming the open chapter: ' + JSON.stringify(box)); await key('Escape'); await win('第84章：刻字')
  await click('新建章节（Ctrl+N）'); await key('Enter'); must(await js(`document.activeElement.matches('.prose')`), 'named with Enter, the new chapter does not take the caret')
  await click('文件'); await click('新建文件夹'); box = await naming(); must(!!box && box.value === '新文件夹' && box.from === 0 && box.to === 4, 'a new folder is not offered a name to type over: ' + JSON.stringify(box)); await key('Enter'); await done('naming in the tree')

  // Closing a tab whose words are not saved asks first; the saving that comes by itself waits for the answer.
  const shutOn = () => js(`document.querySelector('.sheet.on .sheet-x').click()`)
  const asked = '保存对「第84章：刻字」的修改？'
  await click('重新开始'); await click('第84章：刻字'); await caret(0, 0); await text('甲乙'); await shutOn(); await win(asked); await win('不保存将丢失上次保存后的修改。'); await shot('99d-unsaved')
  must(await js(`[...document.querySelectorAll('.dialog .buttons .btn')].map((e) => e.innerText).join(' ')`) === '保存 不保存 取消', 'the question of saving is not in the order of the system')
  await sleep(1800); must(await unsaved(), 'the chapter saved itself while the question was open'); await click('取消'); await win(asked, false); must(await unsaved(), 'not closing lost that the chapter is unsaved'); await win('甲乙巷口那盏灯又坏了')
  await shutOn(); await click('不保存'); await win(asked, false); must(await js(`!document.querySelector('.sheet.on') || !document.querySelector('.sheet.on').innerText.includes('刻字')`), 'the tab is still open'); await click('第84章：刻字'); await win('甲乙巷口', false); must(!(await unsaved()), 'the chapter put back is said not to be saved')
  await caret(0, 0); await text('丙'); await shutOn(); await click('保存'); await click('第84章：刻字'); await win('丙巷口那盏灯又坏了'); must(!(await unsaved()), 'the chapter saved on closing is said not to be saved')
  await caret(0, 0); await text('丁'); await sleep(1800); await shutOn(); await win(asked, false); await done('closing with words not saved')

  // The title bar is a row of its own: the sidebar's switch and the menus at its left, the pane's switch and the window's buttons at its right.
  // Each switch works on every page and stays put whether its side is out or away, and nothing under the bar keeps room for the window's buttons.
  const bar = (): Promise<string> => js(`(() => { const b = document.querySelector('.titlebar').getBoundingClientRect(), w = document.querySelector('#win').getBoundingClientRect(), k = document.querySelector('.window-buttons').getBoundingClientRect(); return [b.top - w.top, b.left - w.left, w.right - b.right, w.right - k.right, k.top - b.top].map(Math.round).join(' ') + ' ' + [...document.querySelectorAll('.win-body > *')].every((e) => e.getBoundingClientRect().top >= b.bottom) })()`)
  await click('重新开始'); must(await bar() === '0 0 0 0 0 true', 'the title bar is not a row across the top of the window, with the window buttons at its right: ' + await bar())
  must(await js(`(() => { const r = (sel) => document.querySelector(sel).getBoundingClientRect(), s = r('.pane-switch'); return r('.titlebar .tools').right <= s.left && r('.window-buttons').left - s.right <= 12 && !document.querySelector('#pane .icon-btn[aria-label$="右栏"]') })()`), 'the switch of the pane is not just left of the window buttons with the tools before it, or the pane keeps a switch of its own')
  const side = await spot('.titlebar > .icon-btn'), right = await spot('.pane-switch')
  await click('收起侧栏'); must(!(await has('.sidebar')), 'the sidebar is still there'); await win('第87章：停电'); await shot('99b-no-sidebar'); await click('第84章：刻字').then(() => { throw new Error('the tree is still clickable') }, () => {})
  must(await spot('.titlebar > .icon-btn') === side, 'putting the sidebar away moved its switch'); await click('展开侧栏'); must(await has('.sidebar'), 'the sidebar did not come back')
  for (const page of ['讨论', '检查', '作品分类']) { await go(page); await click('收起侧栏'); must(!(await has('.sidebar')) && await spot('.titlebar > .icon-btn') === side, 'the sidebar is still there, or its switch moved, on ' + page); await click('展开侧栏'); must(await has('.sidebar'), 'the sidebar did not come back on ' + page) }
  // The pane's switch is in the same spot on every page: it puts away the pane of the talks and of the outline, and is grey where a page has none.
  for (const [page, has] of [['讨论', true], ['大纲', true], ['检查', false], ['作品分类', false]] as const) {
    await go(page); must(await spot('.pane-switch') === right && await js(`document.querySelector('.pane-switch').classList.contains('off')`) === !has, 'the switch of the pane moved, or is wrongly grey, on ' + page)
    if (has) { const was = await js(`!!document.getElementById('pane')`); await js(`document.querySelector('.pane-switch').click()`); must(await js(`!!document.getElementById('pane')`) === !was && await spot('.pane-switch') === right, 'the switch did not turn the pane over in place on ' + page); await js(`document.querySelector('.pane-switch').click()`) }
  }
  await click('正文'); await done('the title bar, and both sides put away on every page')

  // The menus: every command has a row, greyed where it cannot be done; with one open, the pointer over another opens that one.
  const menuRows = (): Promise<string> => js(`[...document.querySelectorAll('.menu-pop .pop-row')].map((e) => e.querySelector('.grow').innerText + (e.classList.contains('off') ? '-' : '')).join(' ')`)
  await click('重新开始'); await pointAt('编辑'); must(!(await has('.menu-pop')), 'a menu opened under the pointer with none open')
  await click('文件'); must(await menuRows() === '新建章节 新建文件夹 保存 历史版本 导出… 作品分类 回收站… 设置…', 'the menu of the file reads: ' + await menuRows())
  await pointAt('编辑'); must(await menuRows() === '撤销- 重做- 查找替换 重命名 续写一句 起草本章', 'the pointer over 编辑 did not open it: ' + await menuRows())
  await pointAt('视图'); must(await menuRows() === '收起侧栏 收起右栏 专注写作 手机预览 放大 缩小 实际大小-', 'the menu of the view reads: ' + await menuRows()); await win('跟随系统'); await pointAt('帮助'); must(await menuRows() === '快捷键…', 'the menu of help reads: ' + await menuRows())
  await key('Escape'); must(!(await has('.menu-pop')), 'Esc left the menu open'); await click('帮助'); await click('帮助'); must(!(await has('.menu-pop')), 'a second click on its name left the menu open')
  // The title bar gives the caret back: a menu opened and put away, the sidebar's switch, a dialog from a menu, and the prose is typed in still.
  const penned = (): Promise<boolean> => js(`document.activeElement === proseBox()`)
  await caret(0, 0); await click('视图'); await key('Escape'); must(await penned(), 'a menu opened and put away took the caret from the prose'); await click('视图'); await click('视图'); must(await penned(), 'a menu shut by its name took the caret from the prose')
  await click('收起侧栏'); await click('展开侧栏'); must(await penned(), 'the switch of the sidebar took the caret from the prose'); await click('帮助'); await click('快捷键…'); await key('Escape'); must(await penned(), 'a dialog from a menu, shut, did not leave the caret in the prose')
  // So does every button that opens a dialog: the caret comes back where it was.
  await caret(0, 2); await settings(); await key('Escape'); must(await penned() && await caretAt() === '0:2', 'the settings from the menu, shut, did not give the caret back: ' + await caretAt())
  await bin(); await key('Escape'); must(await penned() && await caretAt() === '0:2', 'the trash, shut, did not give the caret back: ' + await caretAt())
  await click('编辑'); await click('查找替换'); must(await has('[aria-label="查找"]') && !(await has('.menu-pop')), 'the menu did not open the find bar'); await key('Escape')
  await click('编辑'); await click('重命名'); must(await has('.tree-row.naming'), 'the menu did not start naming the chapter'); await key('Escape')
  await caret(0, 0); await text('戊'); must(await unsaved(), 'typing did not mark the chapter'); await win('戊整条街的灯')
  await click('编辑'); must((await menuRows()).startsWith('撤销 重做- '), 'after typing the menu does not offer to undo: ' + await menuRows()); await click('撤销'); await win('戊整条街的灯', false); must(!(await has('.menu-pop')), 'undoing left the menu open')
  await click('编辑'); must((await menuRows()).startsWith('撤销- 重做 '), 'after undoing the menu does not offer to redo: ' + await menuRows()); await click('重做'); await win('戊整条街的灯')
  // What a notice calls undoable is: a replacement goes back in one step, from the menu.
  await key('f', true); await type('查找', '小祥'); await type('替换为', '阿祥'); await click('全部替换'); await win('可撤销'); await win('阿祥在身后叫她'); await click('关闭提示'); await click('编辑'); await click('撤销'); await win('小祥在身后叫她'); await win('阿祥', false); await win('戊整条街的灯'); await click('关闭查找')
  await click('文件'); await click('保存'); must(!(await unsaved()), 'the menu did not save the chapter'); await win('已保存'); await click('关闭提示')
  await click('文件'); await click('新建章节'); must(await has('.tree-row.naming'), 'the menu did not make a chapter'); await key('Escape')
  await click('文件'); await click('设置…'); await win('AI 服务'); await win('颜色', false); await click('关闭')
  await click('视图'); await click('收起侧栏', 1); must(!(await has('.sidebar')), 'the menu did not put the sidebar away'); await click('视图'); must((await menuRows()).startsWith('展开侧栏 '), 'the menu does not offer the sidebar back'); await click('展开侧栏', 1); must(await has('.sidebar'), 'the menu did not bring the sidebar back')
  await click('视图'); await click('收起右栏'); must(!(await has('#pane')), 'the menu did not put the pane away'); await click('视图'); await click('展开右栏'); must(await has('#pane'), 'the menu did not bring the pane back')
  // Focus puts the menus away with the rest: the title bar keeps the chapter and its count in the middle, its tools and the window's buttons at the right end.
  const focusAt = await spot('[aria-label="专注写作"]')
  await click('视图'); await click('专注写作'); must(!(await has('.sidebar')) && !(await has('#pane')) && !(await has('.menu-title')) && !(await has('.sheet-row')), 'focus from the menu left something beside the prose'); await shot('99c-focus-no-bar')
  must(await js(`document.querySelector('.bar-title').innerText === titleOf(chapterOf(S.ch))`), 'in focus the title bar does not say the chapter: ' + await js(`document.querySelector('.bar-title').innerText`))
  must(await js(`(() => { const r = (sel) => document.querySelector(sel).getBoundingClientRect(), w = r('#win'), k = r('.window-buttons'), t = r('.titlebar'); return Math.round(k.top - w.top) === 0 && Math.round(w.right - k.right) === 0 && Math.round(k.bottom) === Math.round(t.bottom) && r('.titlebar .tools').right <= k.left })()`), 'in focus the window buttons are not at the right end of the title bar, or its tools run under them')
  must(await spot('[aria-label="退出专注"]') === focusAt, 'the switch out of focus is not where the switch into it was: ' + focusAt + ' to ' + await spot('[aria-label="退出专注"]'))
  await click('退出专注，恢复章节树和右栏（F11）'); must(await has('.sidebar') && await has('.menu-title'), 'leaving focus did not bring the menus back')
  await click('检查'); await click('文件'); must(await menuRows() === '新建章节- 新建文件夹- 保存- 历史版本- 导出… 作品分类 回收站… 设置…', 'off the page of prose the menu of the file reads: ' + await menuRows()); await shot('99d-menu-greyed')
  await pointAt('编辑'); must(await menuRows() === '撤销- 重做- 查找替换- 重命名- 续写一句- 起草本章-', 'off the page of prose the menu of editing reads: ' + await menuRows()); await pointAt('视图'); must(await menuRows() === '收起侧栏 展开右栏- 专注写作- 手机预览- 放大 缩小 实际大小-', 'on the page of checks the menu of the view reads: ' + await menuRows()); await key('Escape')
  await click('切换作品'); await click('全部作品…'); must(await has('.titlebar') && await js(`document.querySelector('.titlebar > .icon-btn').classList.contains('off')`), 'the shelf has no title bar, or its switch of the sidebar is not greyed'); await click('文件'); must(await menuRows() === '新建章节- 新建文件夹- 保存- 历史版本- 导出…- 作品分类- 回收站…- 设置…', 'on the shelf the menu of the file reads: ' + await menuRows()); await shot('99e-shelf-bar'); await key('Escape')
  await click('帮助'); await click('快捷键…'); await win('打开快捷键列表'); await click('关闭'); await done('the menus')

  // AI turned off. The settings open on the shelf too, with the two parts that hold for the whole computer; the switch
  // is one for every work, and asks first, both ways. With it off nothing in the window offers AI or tells of it: what
  // the author wrote (the prose) aside, no word of it is shown, nor in a tooltip. The settings, whose part named AI holds
  // the switch alone then, are looked at by themselves.
  const AI_WORDS = ['AI', '讨论', '检查', '续写', '改写', '起草', '摘要', '模型', '整理']
  const aiSaid = (): Promise<string[]> => js(`(() => {
    const out = new Set(), walk = document.createTreeWalker(document.querySelector('#win'), NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT)
    for (let n; (n = walk.nextNode());) {
      const e = n.nodeType === 3 ? n.parentElement : n
      if (e.closest('.prose') || !e.getClientRects().length) continue
      const said = (n.nodeType === 3 ? n.data : ['title', 'aria-label', 'placeholder'].map((a) => n.getAttribute(a) || '').join(' ')).split('连续写').join('') // 连续写作不是续写
      for (const w of ${JSON.stringify(AI_WORDS)}) if (said.includes(w)) out.add(w + ' in ' + (e.className || e.tagName) + ': ' + said.trim().slice(0, 30))
    }
    return [...out]
  })()`)
  const noAI = async (where: string) => { const said = await aiSaid(); must(!said.length, 'with AI off, ' + where + ' tells of AI:\n  ' + said.join('\n  ')) }
  const navs = (): Promise<string> => js(`[...document.querySelectorAll('.pages > div')].map((e) => e.firstChild.textContent).join(' ')`)
  const paneTabs = (): Promise<string> => js(`[...document.querySelectorAll('#pane .tab')].map((e) => e.firstChild.textContent).join(' ')`)
  const aiOff = async () => { await click('文件'); await click('设置…'); await extTick('AI'); await win('关闭 AI？'); await click('关闭 AI'); await win('关闭 AI？', false) }
  await click('文件'); await click('设置…'); must(await prefTabs() === '外观 语言 扩展 AI 名字补全 全角标点', 'on the shelf the settings hold: ' + await prefTabs()); await win('AI 服务'); await win('选用模型', false)
  await extTick('AI'); await win('关闭 AI？'); await win('进行中的任务会停下'); await shot('99f-ask-ai-off'); await click('取消'); await win('关闭 AI？', false); await win('AI 服务')
  await extTick('AI'); must(await js(`getComputedStyle(document.querySelector('.scrim:not(.top)')).backgroundColor === 'rgba(0, 0, 0, 0)'`), 'the question over the settings shades the window twice'); await key('Escape'); await win('关闭 AI？', false); must(await has('.prefs'), 'Esc on the question shut the settings under it too'); await click('关闭')
  // Off while AI is at work: the proposal being written stops and goes, as one not taken would; what the author said stays.
  await click('雨城来信'); await win('第87章：停电'); const before = await paras()
  await click('编辑'); await click('起草本章'); await click('开始起草'); await sleep(1500); must(await has('.prose [data-prop]'), 'the draft wrote nothing')
  await aiOff(); must(await prefTabs() === '外观 语言 扩展 AI 提醒 名字补全 备份 全角标点', 'with AI off the settings of a work hold: ' + await prefTabs()); await win('AI 服务', false); await win('选用模型', false)
  must(!(await has('[data-ext="ai"] .check.on')), 'the switch is still ticked'); await win('这台电脑上的所有作品'); await shot('99g-ai-off')
  await click('提醒'); await win('超过多少章算久未回收'); await win('何时检查', false); await win('AI已关闭，暂时用不了'); await click('关闭')
  await sleep(3000); must(await paras() === before && !(await has('[data-prop]')), 'the draft went on or stayed with AI off'); must(await js(`S.talks[0].msgs.length === 1 && !S.waiting && !S.props.length`), 'the draft went on with AI off')
  // Off, the page of prose and all that opens from it.
  must(await navs() === '正文 大纲', 'with AI off the title bar offers: ' + await navs()); must(await paneTabs() === '批注 设定 伏笔 预览', 'with AI off the pane offers: ' + await paneTabs())
  must(await js(`notesOf(chapterOf(S.ch)).map((n) => n.title).join(' | ')`) === '小祥为什么突然改口叫她“姐”', 'with AI off the margin still has what the AI found')
  must(await js(`document.querySelector('#pane .tab.on').firstChild.textContent`) === '设定', 'with AI off the pane is not on the setting')
  must(await js(`S.tab`) === '讨论', 'with AI off the pane forgot it was on the talk, to go back to when AI is on')
  await noAI('the page of prose'); await shot('99h-ai-off-prose')
  await click('伏笔'); await noAI('the foreshadowings'); await click('设定'); await click('更多'); await click('修改记录'); await win('手动修改'); await win('查看讨论', false); await noAI('the change log'); await click('全部设定')
  await click('周祥'); await click('修改', 1); await pane('从这一章起，这一条就是下面这段话。'); await noAI('the form of a change'); await click('取消'); await click('全部设定')
  for (const name of ['文件', '编辑', '视图', '帮助']) { await click(name); await noAI('the menu ' + name); await key('Escape') }
  await menu(0, '整条街的灯'); await noAI('the menu of the prose'); await key('Escape')
  await pick(1, '手电在工具箱最底下'); must(await js(`document.querySelector('.selbar').innerText.trim()`) === '加入设定', 'with AI off the bar over picked words offers more than 加入设定'); await key('Escape')
  await key('j', true); await key('l', true); await sleep(800); must(await js(`!S.ai && !S.cite`), 'Ctrl+J or Ctrl+L did something with AI off')
  await key('/', true); await win('打开快捷键列表'); await noAI('the shortcuts'); await click('关闭')
  await click(await js(`document.querySelector('.todo-btn').innerText`)); await win('伏笔'); await noAI('what waits'); await key('Escape')
  await click('文件'); await click('历史版本'); await win('全书替换前都会保存一份'); await noAI('the snapshots'); await key('Escape')
  await cats(); await noAI('the categories'); await click('正文')
  await click('少见的情况'); must(!(await js(`[...document.querySelectorAll('.rare-pop .pop-row')].some((e) => ${JSON.stringify(AI_WORDS)}.some((w) => e.innerText.includes(w)))`)), 'with AI off the rare cases hold those of AI'); await key('Escape')
  // On again, asked first: all is back as it was, and the reply let go waits to be tried again.
  await click('文件'); await click('设置…'); await extTick('AI'); await win('打开 AI？'); await shot('99i-ask-ai-on'); await click('打开 AI'); await win('打开 AI？', false); await win('AI 服务'); await click('关闭')
  must(await navs() === '讨论 正文 大纲 检查', 'with AI on again the title bar offers: ' + await navs()); must(await paneTabs() === '批注 设定 讨论 伏笔 预览', 'with AI on again the pane offers: ' + await paneTabs())
  await click('讨论', 1); await pane('该消息尚未收到回复'); await pane('重试回复')
  // Off on the talk page, the prose shows; on again while it does, the talk page is back.
  await js(`document.querySelectorAll('.pages > div')[0].click()`); must(await js(`S.page`) === 'talk', 'the talk page did not open')
  await aiOff(); await click('关闭'); must(await js(`S.page`) === 'write', 'with AI off the talk page stayed')
  await click('文件'); await click('设置…'); await extTick('AI'); await click('打开 AI'); await click('关闭'); must(await js(`S.page`) === 'talk', 'with AI on again the talk page is not back')
  await js(`document.querySelectorAll('.pages > div')[1].click()`); must(await js(`S.page`) === 'write', 'the prose did not open')
  // The shelf, and the shelf with nothing on it, say nothing of AI either.
  await aiOff(); await click('关闭'); await click('切换作品'); await click('全部作品…'); await noAI('the shelf')
  for (;;) {
    if (await has('.shelf-card [aria-label="更多操作"]')) { await click('更多操作'); await click('从书架移除') } else if (await has('.shelf-gone')) await click('移除'); else break
    await click('关闭提示')
  }
  await win('新建一部作品，或添加已有的作品文件夹。'); await noAI('the empty shelf'); await shot('99j-ai-off-shelf')
  await click('重新开始'); await win('第87章：停电'); must(await navs() === '讨论 正文 大纲 检查', 'starting over did not bring AI back: ' + await navs()); await done('AI turned off')

  // macOS: the window's buttons are the system's three lights at the left, the menus are in the screen's menu bar
  // and not in the window, with the system's Inkgo and 窗口 menus too, settings under Inkgo and the look as three
  // ticked rows. Shortcuts are written the Mac way, paths too, and the question of saving puts 不保存 apart at the
  // left. Switching is like another computer: all starts over. The keys stay the same, so Ctrl still works.
  const saveOrder = (): Promise<string> => js(`[...document.querySelectorAll('.dialog .buttons .btn')].map((e) => e.innerText).join(' ')`)
  const macMenus = reads('.mac-bar')
  await click('macOS'); await win('第87章：停电')
  must(await has('#win .lights') && !(await has('#win .window-buttons')), 'macOS shows the buttons of a Windows window'); must(!(await has('#win .menu-title')), 'on macOS the menus are still in the window')
  await macMenus('Inkgo文件编辑视图窗口帮助'); await shot('a0-mac')
  await click('文件'); await macMenus('⌘N'); await macMenus('Ctrl+N', false); await macMenus('设置…', false); await shot('a0b-mac-menu'); await key('Escape')
  await click('Inkgo'); await macMenus('⌘,'); await macMenus('退出 Inkgo'); await click('设置…'); await win('AI 服务'); await click('关闭')
  await click('编辑'); await macMenus('⇧⌘Z'); await key('Escape')
  await click('视图'); await click('深色'); must(await js(`S.look`) === '深色', 'the look in the menu of macOS did not turn dark'); await click('视图'); await macMenus('⌃⌘F'); await shot('a0f-mac-look'); await click('跟随系统')
  await key('/', true); await win('⌃Tab'); await win('⌘↩'); await win('⇧⌘Z'); await win('⌃⌘F'); await win('F2', false); await win('重命名当前章节', false); await shot('a0c-mac-keys'); await click('关闭')
  await click('第84章：刻字'); await caret(0, 0); await text('甲乙'); await shutOn(); await win(asked)
  must(await saveOrder() === '不保存 取消 保存', 'on macOS the question of saving is not in the order of the system: ' + await saveOrder()); await shot('a0d-mac-unsaved'); await click('不保存')
  await key('f', true); must(await has('.find-bar'), 'Ctrl+F did not open find on macOS'); await key('Escape')
  await click('切换作品'); await click('全部作品…'); await win('~/写作'); await shot('a0e-mac-shelf')
  await click('Windows'); await win('第87章：停电'); must(await has('#win .window-buttons') && !(await has('#win .lights')) && !(await has('.mac-bar')), 'back on Windows the window is still a Mac one')
  await click('文件'); await win('Ctrl+N'); await key('Escape'); await click('编辑'); await win('Ctrl+Shift+Z'); await win('Ctrl+Y', false); await key('Escape'); await done('macOS')

  // Languages: the stage bar switches the words of the window without starting over. What has been taken into
  // locales/ (so far the title bar, the menus and the toasts) follows; 伪英文 (en-XA) brackets and lengthens every
  // word taken, so a word still in Chinese without brackets has not been. Both platforms.
  await click('English'); await win('第87章：停电'); await win('FileEditViewHelp'); await win('Write')
  await click('File'); await win('New Chapter'); await win('Ctrl+N'); await shot('a1-en-file'); await key('Escape')
  await click('View'); await win('Zoom 100%'); await win('Appearance'); await shot('a1b-en-view'); await key('Escape')
  await click('伪英文'); await click('[文件 one]'); await win('[新建章节 one]'); await shot('a1c-pseudo-file'); await key('Escape')
  await click('English'); await click('macOS'); await macMenus('InkgoFileEditViewWindowHelp'); await click('Inkgo'); await macMenus('Quit Inkgo'); await shot('a1d-mac-en'); await key('Escape')
  await click('Windows'); await click('中文'); await win('文件编辑视图帮助'); await done('languages')

  must(!loose.size, 'buttons that are not at the right of their row, or not in order:\n  ' + [...loose].join('\n  '))
  must(!dead.size, 'things to click that do not show it:\n  ' + [...dead].join('\n  '))
  must(!squat.size, 'dialogs flatter than ' + FLATTEST + ' to 1:\n  ' + [...squat].join('\n  '))
}
