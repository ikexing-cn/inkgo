/**
 * Clicks through everything the prototype can do, the way its author would,
 * and throws where it stops behaving. Each line of steps ends by saying what
 * should now be on the page; change the prototype, change its steps here.
 */
import type { Page } from './browser.ts'

export async function walk({ js, shot, click, rclick, type, key, clear, sleep }: Page, said: (step: string) => void): Promise<void> {
  const flat = (t: string) => t.replace(/\s+/g, '')
  const reads = (where: string) => async (words: string, want = true) => {
    const text = await js(`(document.querySelector('${where}') || { innerText: '' }).innerText`)
    if (flat(text).includes(flat(words)) !== want) throw new Error((want ? 'missing in ' : 'unexpected in ') + where + ': ' + words)
  }
  const pane = reads('#pane'), win = reads('#win')
  const must = (ok: boolean, words: string) => { if (!ok) throw new Error(words) }
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

  await win('第87章：停电'); await pane('设定4'); await shot('01-open')

  // The setting, and going from an entry to where it was written.
  await click('周祥'); await pane('别名 小祥、saki'); await pane('AI 建议'); await pane('第 1 章“小祥把左手揣回兜里。”'); await shot('02-entry')
  await click('看原文'); await win('第1章：巷口'); await win('梯子靠稳了'); must((await drawn('小祥把左手揣回兜里。')) === 'sel', 'the entry\'s sentence is not picked out'); await shot('03-entry-source'); await done('an entry to its source')
  await click('采纳'); await pane('AI 建议', false); await pane('设定3'); await done('take an idea on an entry')
  await click('修改'); await pane('名字'); await pane('分类'); await shot('04-edit')
  await type('内容', '新加一句。'); await click('人物'); await shot('05-cats'); await click('地点'); await click('保存')
  await pane('新加一句。'); await pane('删除设定'); await done('edit an entry')
  await click('全部设定'); await pane('地点')
  await click('等你决定'); await pane('3 条 AI 建议'); await shot('06-ideas')
  await click('忽略'); await pane('2 条 AI 建议'); await click('采纳'); await click('采纳'); await pane('等你决定', false); await done('decide the ideas')
  await click('更多'); await pane('修改记录'); await shot('07-menu'); await click('已删除'); await pane('老陈')
  await click('老陈'); await pane('恢复设定'); await click('看原文'); await win('第2章：井沿'); must((await drawn('老陈把门卫室的灯拧亮。')) === 'sel', 'the deleted entry\'s sentence is not picked out')
  await click('恢复设定'); await pane('老陈', false); await pane('已删除'); await done('restore an entry')
  await click('全部设定'); await pane('老陈'); await click('更多'); await click('修改记录'); await pane('这几轮没整理成功'); await pane('刚刚'); await shot('08-log')
  await click('重试'); await pane('这几轮没整理成功', false); await done('retry')
  await click('全部设定'); await type('搜索设定', '左手'); await pane('找到 2 条'); await shot('09-find')
  await clear('搜索设定'); await pane('找到', false)
  await click('新增'); await pane('新增设定'); await type('内容', '老周是修表的，铺子在旧城小学对面。'); await type('名字', '老周'); await click('保存'); await pane('老周'); await done('add an entry')
  await click('周祥'); await click('删除设定'); await pane('左手总揣在兜里', false); await pane('物品'); await done('delete an entry')

  // The sidebar: chapters open, volumes fold.
  await click('第84章：刻字'); await win('灯罩内侧刻着'); await win('本章 3,102 字'); await shot('10-chapter')
  await click('第三卷：停电'); await win('第85章：灯数', false); await click('第三卷：停电'); await win('第85章：灯数')
  await click('番外：晴天'); await win('雨城一年里只有十几个晴天'); await shot('11-loose-chapter'); await done('the sidebar')

  // Foreshadowings: from the pane to the prose and back.
  await click('伏笔'); await pane('久未回收'); await pane('未回收'); await shot('12-foreshadowings')
  await click('她怕黑的真正原因'); await pane('第 1 章埋下，已过 86 章'); await click('看原文'); await win('第1章：巷口')
  must((await drawn('她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。')) === 'fs on', 'the open foreshadowing is not lit'); await shot('13-mark-source')
  await click('记为回收'); await win('先在正文里选中回收伏笔的那一段'); await shot('14-toast'); await sleep(3300); await win('先在正文里选中回收伏笔的那一段', false); await done('pay off with nothing picked')
  await click('第87章：停电'); await select(5, '她往前走了一步。'); await click('记为回收'); await win('回收的是哪个伏笔'); await win('86 章前埋下'); await shot('15-pay-from-pane')
  await click('确定回收'); await win('回收的是哪个伏笔', false); await pane('第 87 章回收，相隔 86 章'); await pane('取消回收'); await pane('她往前走了一步。')
  must((await drawn('她往前走了一步。')) === 'fs on', 'the payoff is not lit'); await shot('16-paid'); await done('pay off from the pane')
  await click('改备注'); await type('改伏笔备注', '（改）'); await key('Enter'); await pane('她怕黑的真正原因（改）'); await pane('改备注')
  await click('改备注'); await key('Escape'); await pane('改备注'); await done('rename a foreshadowing')
  await click('看原文', 1); await win('第87章：停电'); await click('看原文'); await win('第1章：巷口'); await done('both sources')

  // The right button in the prose.
  await click('第87章：停电')
  await menu(2, '灯一直没有亮。'); await win('记为伏笔…'); await win('记为回收…'); await win('查看伏笔', false); await shot('17-menu')
  must((await drawn('灯一直没有亮。')) === 'sel', 'what was picked is not shown'); await click('记为伏笔…'); await win('记为伏笔，写句备注'); await shot('18-bury')
  must(await js(`document.activeElement.getAttribute('aria-label') === '伏笔备注'`), 'the note of a new foreshadowing is not ready to type in')
  await type('伏笔备注', '灯为什么没亮'); await key('Enter'); await win('记为伏笔，写句备注', false); must((await drawn('灯一直没有亮。')) === 'fs', 'the new foreshadowing is not underlined')
  await click('全部伏笔'); await pane('灯为什么没亮'); await pane('本章'); await shot('19-buried'); await done('bury from the prose')
  await menu(2, '灯一直没有亮。'); await win('查看伏笔'); await click('查看伏笔'); await pane('灯为什么没亮'); await pane('第 87 章埋下'); await pane('已过', false); await done('see a foreshadowing from the prose')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await key('Escape'); await win('记为伏笔，写句备注', false); must((await drawn('整条街的灯')) === 'absent', 'Escape left the pick behind')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await click('取消'); await win('记为伏笔，写句备注', false)
  await click('第86章：档案室'); await menu(4, '走廊里响起脚步声'); await click('记为回收…'); await type('找伏笔', '哪扇'); await win('钥匙开哪扇门'); await win('小祥为什么突然改口', false); await shot('20-pay-search')
  await clear('找伏笔'); await type('找伏笔', '没有这个'); await win('没有叫这个的伏笔'); await clear('找伏笔'); await type('找伏笔', '哪扇'); await key('Enter')
  await win('回收的是哪个伏笔', false); await pane('钥匙开哪扇门'); await pane('第 86 章回收，相隔 40 章'); await shot('21-paid-from-menu'); await done('pay off from the prose')
  await js('getSelection().removeAllRanges()')
  const [x, y]: [number, number] = await js(`(() => { const r = document.querySelector('[data-para="0"]').getBoundingClientRect(); return [r.x + 80, r.y + r.height / 2] })()`)
  await rclick(x, y); await win('记为伏笔…'); must(await greyed('记为伏笔…'), 'burying is not greyed'); await shot('22-nothing-picked'); await rclick(x, y); await win('记为伏笔…', false); await done('the menu with nothing picked')

  // A foreshadowing whose sentence is gone takes a new one.
  await click('全部伏笔'); await click('原句已删'); await click('小祥的左手'); await pane('第 1 章的原句已删除'); await shot('23-lost')
  await click('换个原句'); await win('先在正文里选中要挂上去的句子'); await click('看原文'); await select(5, '小祥把左手揣回兜里。'); await click('换个原句')
  await pane('第 1 章埋下，已过 86 章'); await pane('记为回收'); must((await drawn('小祥把左手揣回兜里。')) === 'fs on', 'the new sentence is not lit'); await done('a new sentence')
  await click('全部伏笔'); await click('更多'); await shot('24-foreshadowing-menu'); await click('已放弃'); await click('红色钥匙'); await pane('已放弃'); await click('恢复伏笔'); await pane('放弃伏笔'); await done('give up and take back')

  // The pane folds away.
  await click('收起右栏'); must(!(await js(`!!document.querySelector('#pane')`)), 'the pane is still there'); await win('本章伏笔 1'); await shot('25-folded')
  await click('展开右栏'); await pane('放弃伏笔'); await click('收起右栏'); await click('本章伏笔 1'); await pane('未回收'); await done('fold the pane')

  // Discussions.
  await click('讨论'); await pane('最近'); await click('新讨论'); await pane('试试这样问'); await shot('26-new-talk')
  await click('接下来可以怎么写'); await pane('正在回复…'); await sleep(1500); await pane('这里是原型'); await pane('正在回复…', false)
  await type('输入消息', '那下一章呢'); await key('Enter'); await pane('那下一章呢'); await sleep(1500); await pane('先想清楚这一段要做什么'); await shot('27-talk')
  await click('全部讨论'); await pane('接下来可以怎么写'); await click('这一章的节奏怎么样'); await pane('前半段偏慢'); await done('discussions')

  // Dark, and back to how it opened.
  await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('28-dark'); await click('设定'); await shot('29-dark-setting')
  await click('重新开始'); await pane('设定4'); await pane('等你决定'); await win('第87章：停电'); await click('浅色'); await done('dark, and start over')
}
