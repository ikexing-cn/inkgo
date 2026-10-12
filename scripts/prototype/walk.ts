/**
 * Clicks through everything the prototype can do, the way its author would,
 * and throws where it stops behaving. Each line of steps ends by saying what
 * should now be on the page; change the prototype, change its steps here.
 */
import type { Page } from './browser.ts'

export async function walk({ js, shot, click: press, rclick, drag, type, key, clear, sleep }: Page, said: (step: string) => void): Promise<void> {
  const flat = (t: string) => t.replace(/\s+/g, '')
  const reads = (where: string) => async (words: string, want = true) => {
    const text = await js(`(document.querySelector('${where}') || { innerText: '' }).innerText`)
    if (flat(text).includes(flat(words)) !== want) throw new Error((want ? 'missing in ' : 'unexpected in ') + where + ': ' + words)
  }
  const pane = reads('#pane'), win = reads('#win')
  const MORE = '续写、起草、快照、导出和外观' // the toolbar's "more" button, by its title
  const must = (ok: boolean, words: string) => { if (!ok) throw new Error(words) }
  // After every click, nothing may be wider than the menu, popover, dialog or toast it is in. One that holds an open
  // menu of its own is passed over: a submenu is meant to stand beside it.
  const click = async (words: string, nth = 0) => {
    await press(words, nth)
    const out = await js(`(() => { const e = [...document.querySelectorAll('.pop, .float, .dialog, .toast')].find((e) => !e.querySelector('.pop') && e.scrollWidth > e.clientWidth + 1); return e ? e.className + ': ' + e.innerText.slice(0, 30) : '' })()`)
    must(!out, 'after clicking ' + words + ', something sticks out of ' + out)
  }
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
  await click('查看原文'); await win('第1章：巷口'); await win('梯子靠稳了'); must((await drawn('小祥把左手揣回兜里。')) === 'sel', 'the entry\'s sentence is not picked out'); await shot('03-entry-source'); await done('an entry to its source')
  await click('采纳'); await pane('AI 建议', false); await pane('设定3'); await done('take an idea on an entry')
  await click('修改'); await pane('名字'); await pane('分类'); await shot('04-edit')
  await type('内容', '新加一句。'); await click('人物'); await shot('05-cats'); await click('地点'); await click('保存')
  await pane('新加一句。'); await pane('删除设定'); await done('edit an entry')
  await click('全部设定'); await pane('地点')
  await click('待确认'); await pane('3 条 AI 建议'); await shot('06-ideas')
  await click('忽略'); await pane('2 条 AI 建议'); await click('采纳'); await click('采纳'); await pane('待确认', false); await done('decide the ideas')
  await click('更多'); await pane('修改记录'); await shot('07-menu'); await click('已删除'); await pane('老陈')
  await click('老陈'); await pane('恢复设定'); await click('查看原文'); await win('第2章：井沿'); must((await drawn('老陈把门卫室的灯拧亮。')) === 'sel', 'the deleted entry\'s sentence is not picked out')
  await click('恢复设定'); await pane('老陈', false); await pane('已删除'); await done('restore an entry')
  await click('全部设定'); await pane('老陈'); await click('更多'); await click('修改记录'); await pane('最近几轮整理失败'); await pane('刚刚'); await shot('08-log')
  await click('重试'); await win('正在重新整理…'); await sleep(1400); await pane('最近几轮整理失败', false); await pane('后台整理（重试）'); await done('retry')
  await click('全部设定'); await type('搜索设定', '左手'); await pane('找到 2 条'); await shot('09-find')
  await clear('搜索设定'); await pane('找到', false)
  await click('新增'); await pane('新增设定'); await type('内容', '老周是修表的，铺子在旧城小学对面。'); await type('名字', '老周'); await click('保存'); await pane('老周'); await done('add an entry')
  await click('周祥'); await click('删除设定'); await pane('左手总揣在兜里', false); await pane('物品'); await done('delete an entry')

  // The sidebar: chapters open, volumes fold.
  await click('第84章：刻字'); await win('灯罩内侧刻着'); await win('本章 3,102 字'); await shot('10-chapter')
  await click('第三卷：停电'); await win('卷纲'); await win('4 项'); await win('本卷 1.2 万字 · 全书 31.2 万字'); await win('林晚趁何主任不在翻档案柜'); await win('巷口的灯又坏了'); await shot('10b-folder')
  await click('第三卷：停电'); must(!(await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第85章'))`)), 'the volume did not fold'); await click('第三卷：停电'); must((await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第85章'))`)), 'the volume did not unfold'); await click('第85章：灯数'); await win('卷纲', false); await click('上一个打开的章节'); await win('卷纲')
  await click('番外：晴天'); await win('雨城一年里只有十几个晴天'); await shot('11-loose-chapter'); await done('the sidebar')

  // Foreshadowings: from the pane to the prose and back.
  await click('伏笔'); await pane('久未回收'); await pane('未回收'); await shot('12-foreshadowings')
  await click('她怕黑的真正原因'); await pane('第 1 章埋下，已过 86 章'); await click('查看原文'); await win('第1章：巷口')
  must((await drawn('她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。')) === 'fs on', 'the open foreshadowing is not lit'); await shot('13-mark-source')
  await click('记为回收'); await win('请先在正文中选中回收伏笔的段落'); await shot('14-toast'); await sleep(3300); await win('请先在正文中选中回收伏笔的段落', false); await done('pay off with nothing picked')
  await click('第87章：停电'); await select(5, '她往前走了一步。'); await click('记为回收'); await win('选择要回收的伏笔'); await win('86 章前埋下'); await shot('15-pay-from-pane')
  await click('确定回收'); await win('选择要回收的伏笔', false); await pane('第 87 章回收，相隔 86 章'); await pane('取消回收'); await pane('她往前走了一步。')
  must((await drawn('她往前走了一步。')) === 'fs on', 'the payoff is not lit'); await shot('16-paid'); await done('pay off from the pane')
  await click('修改备注'); await type('修改伏笔备注', '（改）'); await key('Enter'); await pane('她怕黑的真正原因（改）'); await pane('修改备注')
  await click('修改备注'); await key('Escape'); await pane('修改备注'); await done('rename a foreshadowing')
  await click('查看原文', 1); await win('第87章：停电'); await click('查看原文'); await win('第1章：巷口'); await done('both sources')

  // The right button in the prose.
  await click('第87章：停电')
  await menu(2, '灯一直没有亮。'); await win('记为伏笔…'); await win('记为回收…'); await win('查看伏笔', false); await shot('17-menu')
  must((await drawn('灯一直没有亮。')) === 'sel', 'what was picked is not shown'); await click('记为伏笔…'); await win('记为伏笔，填写备注'); await shot('18-bury')
  must(await js(`document.activeElement.getAttribute('aria-label') === '伏笔备注'`), 'the note of a new foreshadowing is not ready to type in')
  await type('伏笔备注', '灯为什么没亮'); await key('Enter'); await win('记为伏笔，填写备注', false); must((await drawn('灯一直没有亮。')) === 'fs', 'the new foreshadowing is not underlined')
  await click('全部伏笔'); await pane('灯为什么没亮'); await pane('本章'); await shot('19-buried'); await done('bury from the prose')
  await menu(2, '灯一直没有亮。'); await win('查看伏笔'); await click('查看伏笔'); await pane('灯为什么没亮'); await pane('第 87 章埋下'); await pane('已过', false); await done('see a foreshadowing from the prose')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await key('Escape'); await win('记为伏笔，填写备注', false); must((await drawn('整条街的灯')) === 'absent', 'Escape left the pick behind')
  await menu(0, '整条街的灯'); await click('记为伏笔…'); await click('取消'); await win('记为伏笔，填写备注', false)
  await click('第86章：档案室'); await menu(4, '走廊里响起脚步声'); await click('记为回收…'); await type('查找伏笔', '哪扇'); await win('钥匙开哪扇门'); await win('小祥为什么突然改口', false); await shot('20-pay-search')
  await clear('查找伏笔'); await type('查找伏笔', '没有这个'); await win('没有匹配的伏笔'); await clear('查找伏笔'); await type('查找伏笔', '哪扇'); await key('Enter')
  await win('选择要回收的伏笔', false); await pane('钥匙开哪扇门'); await pane('第 86 章回收，相隔 40 章'); await shot('21-paid-from-menu'); await done('pay off from the prose')
  await js('getSelection().removeAllRanges()')
  const [x, y]: [number, number] = await js(`(() => { const r = document.querySelector('[data-para="0"]').getBoundingClientRect(); return [r.x + 80, r.y + r.height / 2] })()`)
  await rclick(x, y); await win('记为伏笔…'); must(await greyed('记为伏笔…'), 'burying is not greyed'); await shot('22-nothing-picked'); await rclick(x, y); await win('记为伏笔…', false); await done('the menu with nothing picked')

  // A foreshadowing whose sentence is gone takes a new one.
  await click('全部伏笔'); await click('原句已删'); await click('小祥的左手'); await pane('第 1 章的原句已删除'); await shot('23-lost')
  await click('更换原句'); await win('请先在正文中选中要关联的句子'); await click('关闭提示'); await win('请先在正文中选中要关联的句子', false); await click('查看原文'); await select(5, '小祥把左手揣回兜里。'); await click('更换原句')
  await pane('第 1 章埋下，已过 86 章'); await pane('记为回收'); must((await drawn('小祥把左手揣回兜里。')) === 'fs on', 'the new sentence is not lit'); await done('a new sentence')
  await click('全部伏笔'); await pane('此处暂无伏笔'); await click('全部伏笔'); await click('更多'); await shot('24-foreshadowing-menu'); await click('已放弃'); await click('红色钥匙'); await pane('已放弃'); await click('恢复伏笔'); await pane('放弃伏笔'); await done('give up and take back')

  // The pane folds away.
  await click('收起右栏'); must(!(await js(`!!document.querySelector('#pane')`)), 'the pane is still there'); await win('本章伏笔 1'); await shot('25-folded')
  await click('展开右栏'); await pane('放弃伏笔'); await click('收起右栏'); await click('本章伏笔 1'); await pane('未回收'); await done('fold the pane')

  // Discussions.
  await click('讨论', 1); await pane('最近'); await click('新讨论'); await pane('示例问题'); await shot('26-new-talk')
  await click('接下来可以怎么写'); await pane('正在思考…'); await pane('取消请求'); await sleep(800); await pane('正在回复…'); await sleep(700); await pane('这是原型'); await pane('正在回复…', false)
  await type('输入消息', '那下一章呢'); await key('Enter'); await pane('那下一章呢'); await sleep(1500); await pane('先想清楚这一段要做什么'); await shot('27-talk')
  await type('输入消息', '再说一句'); await key('Enter'); await click('取消请求'); await pane('该消息尚未收到回复'); await pane('正在思考', false); await shot('27b-unanswered'); await click('重试回复'); await pane('该消息尚未收到回复', false); await sleep(1500); await pane('我会先留着')
  await pane('先看设定里和这句话有关的几条', false); await click('思考过程'); await pane('先看设定里和这句话有关的几条'); await shot('27c-thought'); await click('思考过程'); await pane('先看设定里和这句话有关的几条', false); await done('a reply cancelled and asked for again')
  await click('全部讨论'); await pane('接下来可以怎么写'); await click('这一章的节奏怎么样'); await pane('前半段偏慢'); await done('discussions')

  // Dark, and back to how it opened.
  await click(MORE); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('28-dark'); await key('Escape'); await click('设定'); await shot('29-dark-setting')
  await click('重新开始'); await pane('设定4'); await pane('待确认'); await win('第87章：停电'); must(await js(`document.querySelector('#win').className === 'dark'`), 'starting over lost the look'); await click(MORE); await click('浅色'); await key('Escape'); await done('dark, and start over')

  const has = (sel: string): Promise<boolean> => js(`!!document.querySelector(${JSON.stringify(sel)})`)
  const value = (label: string): Promise<string> => js(`document.querySelector('[aria-label=${JSON.stringify(label)}]').value`)
  // The middle of the row of the sidebar that reads so, to press the right button on.
  const row = (words: string): Promise<[number, number]> => js(`(() => { const r = [...document.querySelectorAll('.tree-row')].find((e) => e.innerText.replace(/\\s+/g, '').includes(${JSON.stringify(words)})).getBoundingClientRect(); return [r.x + 60, r.y + r.height / 2] })()`)
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
  await clear('章节标题'); await type('章节标题', '雨停'); await key('Enter'); await win('第88章：雨停'); await win('在此开始写作…'); await win('满 200 字后可检查'); await win('本章 0 字'); await done('a new chapter')
  await rclick(...(await row('第88章：雨停'))); await win('移到回收站'); await win('最上层'); await shot('31-tree-menu'); await click('重命名'); await clear('章节标题'); await type('章节标题', '天晴'); await key('Enter'); await win('第88章：天晴'); await done('rename a chapter')
  await rclick(...(await row('第88章：天晴'))); await click('上移'); must(await js(`(() => { const t = document.querySelector('.tree').innerText; return t.indexOf('天晴') < t.indexOf('第87章') })()`), 'the chapter did not move up')
  await rclick(...(await row('第88章：天晴'))); await click('最上层'); await win('第88章：天晴', false); await win('天晴'); await done('move a chapter')
  await rclick(...(await row('天晴'))); await click('移到回收站'); await win('「天晴」已移到回收站'); await click('关闭提示'); must(!(await js(`document.querySelector('.tree').innerText.includes('天晴')`)), 'the chapter is still in the tree')
  await click('新建文件夹'); await clear('章节标题'); await type('章节标题', '第四卷'); await key('Enter'); await win('第四卷')
  await click('回收站'); await win('已删除的章节及其内容暂存于此'); await win('第83章：旧稿'); await shot('32-bin'); await click('恢复', 0); await win('已恢复「天晴」'); await click('关闭提示')
  await click('彻底删除'); await click('确认彻底删除'); await win('回收站为空'); await click('关闭'); must(await js(`document.querySelector('.tree').innerText.includes('天晴')`), 'the chapter did not come back'); await done('the bin')
  await click('切换作品'); await win('旧城灯匠'); await win('全部作品…'); await shot('33-book-menu'); await click('快捷键…'); await win('打开快捷键列表'); await win('专注写作：仅显示正文'); await shot('34-keys'); await click('关闭'); await win('打开快捷键列表', false)
  await key('/', true); await win('打开快捷键列表'); await key('Escape'); await win('打开快捷键列表', false); await done('the book menu and the keys')
  await click('已备份 · 今天 10:32'); await win('今天 10:32 已备份'); await win('下次：关闭作品时'); await shot('35-backup-pop'); await click('立即备份'); await win('正在备份…'); await sleep(1400); await win('已备份 · 刚刚'); await done('back up now')

  // The toolbar: the count and the goal, find and replace, the more menu, focus, what is pending.
  await click('重新开始'); await click('字数。点击设置每日目标'); await win('最近 7 天'); await win('+2,846 字'); await type('每日目标字数', '3000'); await win('今天 2,846 / 3000'); await shot('36-goal'); await key('Escape'); await win('最近 7 天', false); await done('a goal for the day')
  await key('f', true); must(await js(`document.activeElement.getAttribute('aria-label') === '查找'`), 'Ctrl+F did not open the search'); await type('查找', '小祥'); await win('1 / 2'); must((await drawn('小祥')) === 'got on', 'what was found is not marked'); await shot('37-find')
  await click('下一个'); await win('2 / 2'); await type('替换为', '阿祥'); await click('替换'); await win('阿祥从来不叫她姐'); await win('1 / 1'); await click('全部替换'); await win('已替换 1 处可撤销'); await win('阿祥在身后叫她'); await win('未找到'); await click('关闭提示')
  await win('第87章有 1 处待处理，正文已再次修改'); await click('立即检查'); await win('正在检查第87章…'); await sleep(1400); await win('第87章检查完成，有 1 处待处理'); await done('find and replace in a chapter')
  await clear('查找'); await type('查找', '林晚'); await click('在全部章节中查找'); await win('全书 8 章、共 9 处'); await win('第1章：巷口 · 2 处'); await shot('38-find-book')
  await clear('替换为'); await type('替换为', '阿晚'); await click('全书替换'); await win('将全书 8 章中的 9 处“林晚”替换为“阿晚”？'); await shot('39-replace-book'); await click('确定替换'); await win('已在 8 章中替换 9 处'); await win('阿晚站在巷口'); await win('全书未找到')
  await click('关闭查找'); must(!(await has('.find-bar')), 'the search is still open'); await click('关闭提示'); await done('replace through the book')
  await click(MORE); await win('Ctrl+J'); await shot('40-more'); await click('快照'); await win('快照 · 第87章：停电'); await win('全书替换前'); await win('打开时'); await shot('41-snaps')
  await click('与当前对比'); await win('红色删除线为快照中有而当前没有的内容'); must(await has('.snap-text .cut') && await has('.snap-text .add'), 'the comparison shows no change'); await shot('42-snap-diff')
  await click('今天 09:58'); await click('恢复这个版本'); await win('已恢复到 今天 09:58 的版本恢复前的内容已存为快照'); await win('小祥在身后叫她', false); await win('七岁那年的那个晚上'); await click('关闭提示'); await done('snapshots')
  await click(MORE); await click('导出…'); await win('按章节树顺序合并为一个 .txt 文件'); await click('全书'); await shot('43-export'); await click('导出'); await win('已导出到 D:\\写作\\雨城来信.txt'); await click('关闭提示'); await done('export')
  await click('仅显示正文，收起章节树和右栏（F11）'); must(!(await has('.sidebar')) && !(await has('#pane')), 'focus left something beside the prose'); await shot('44-focus'); await key('F11'); must(await has('.sidebar') && await has('#pane'), 'F11 did not leave focus'); await done('focus')
  await click('重新开始'); await click('待处理 8'); await win('设定不一致'); await win('「红色钥匙」', false); await win('「钥匙开哪扇门」41 章未回收'); await win('还有 2 条'); await shot('45-pending'); await click('还有 2 条'); await win('「旧城小学」尚未加入设定'); await click('收起')
  await click('本章 1'); await win('周翔', false); await win('林晚怕黑，却独自走进黑巷'); await click('全部 8'); await click('「小祥的左手」原句已删'); await pane('第 1 章的原句已删除'); await win('全部忽略', false)
  await click('待处理 8'); await click('全部忽略'); must(!(await has('.todo-btn')), 'ignoring all left the button'); await done('what is pending')

  // Under the title: the outline and the summary.
  await click('重新开始'); await click('章纲 ▾'); must((await value('章纲')).startsWith('全城停电'), 'the outline is not shown'); await shot('46-outline'); await click('本章摘要 · 暂无 ▾'); await win('正文满 200 字后，离开本章时自动生成；也可手动填写。')
  await click('生成摘要'); await win('AI 正在阅读本章并生成摘要…'); await sleep(1400); must((await value('摘要')).includes('小祥第一次叫她“姐”'), 'no summary came'); await win('AI 生成 · Opus 5.5 · 刚刚'); await win('重新生成'); await shot('47-summary')
  await type('摘要', '（我加的）'); await win('手动填写'); await click('本章摘要 ▴'); must(!(await has('.chapter-box')), 'the summary did not fold')
  await click('第84章：刻字'); await win('章纲 · 空 ▾'); await click('本章摘要 · 可能过时 ▾'); await win('正文已修改'); await click(MORE); must(await js(`[...document.querySelectorAll('.more-pop .row')].find((e) => e.innerText.includes('起草本章')).classList.contains('off')`), 'drafting is not greyed without an outline'); await win('请先填写章纲'); await key('Escape'); await done('the outline and the summary')

  // AI in the prose: a sentence more, a rewrite, a draft.
  await click('重新开始'); await click(MORE); await click('续写一句'); await win('续写中…'); await sleep(900); await win('Tab 采纳 · Esc 放弃'); must(await has('.ghost-text'), 'no grey sentence'); await shot('48-ghost')
  await key('Escape'); must(!(await has('.ghost-text')), 'Escape kept the sentence'); await win('她把手电咬在嘴里', false); await key('j', true); await sleep(900); await key('Tab'); must(!(await has('.ghost-text')), 'Tab kept it grey'); await win('她往前走了一步。她把手电咬在嘴里'); await done('a sentence more')
  await pick(2, '灯一直没有亮。'); must(await has('.selbar'), 'nothing floats over what was picked'); await win('选中 7 字'); await shot('49-selection'); await click('AI 改写'); must(await js(`document.activeElement.getAttribute('aria-label') === '改写要求'`), 'the wish is not ready to type in')
  await type('改写要求', '慢一点'); await key('Enter'); await win('AI 正在改写选中内容'); await sleep(1400); await win('红色删除线为原文，绿色为改写。'); must(await has('.prose .cut') && await has('.prose .add'), 'the rewrite is not shown against the original'); await shot('50-rewrite')
  await click('重新改写'); await win('AI 正在改写选中内容'); await sleep(1400); await click('采纳'); await win('已采纳改写'); await win('可撤销恢复原文，原版本已存入快照'); await win('她在巷口站了很久'); await win('灯一直没有亮。', false); await click('关闭提示')
  await pick(0, '同时灭的'); await click('AI 改写'); await key('Escape'); must(!(await has('.ai-bar')), 'Escape left the rewrite open'); await done('a rewrite')
  await click(MORE); await click('起草本章'); await win('AI 仅将章纲扩写为正文'); await win('前文有 3 章尚无摘要'); await win('本章已有内容'); await shot('51-draft'); await click('补齐摘要（3 次调用）'); await win('正在补齐摘要：0 / 3。补齐前起草，AI 只能参考已有摘要。'); await sleep(2700); await win('前文有 3 章尚无摘要', false); await win('起草时 AI 会参考前 6 章的摘要，以衔接前文。')
  await type('目标字数', '1500'); await click('覆盖已有内容'); await click('开始起草'); await win('AI 正在按章纲起草'); await shot('52-drafting'); await sleep(3800); await win('起草完成，共 38 字'); await win('整条街的灯是同时灭的。', false); await win('身后，小祥没有再出声。')
  must(await js(`document.querySelectorAll('.prose .ai').length === 3`), 'what the AI drafted is not marked'); await shot('53-drafted'); await click('关闭提示')
  await click(MORE); await click('快照'); await win('AI 改动前'); await win('按章纲起草'); await win('约 1500 字'); await key('Escape')
  await click(MORE); await click('起草本章'); await type('目标字数', '很多'); await click('开始起草'); await win('目标字数须为正整数或留空'); await win('复制详情'); await click('关闭提示'); await clear('目标字数'); await click('开始起草'); await sleep(1000); await click('停止'); await win('已停止起草已写部分保留'); await click('关闭提示'); must(!(await has('.ai-bar')), 'the draft did not stop'); await done('a draft')

  // What was picked goes to the setting, or to a discussion.
  await click('重新开始'); await pick(1, '手电在工具箱最底下'); await click('加入设定'); await win('选中的原文将记为出处'); must((await value('设定内容')) === '手电在工具箱最底下', 'what was picked is not in the box'); await click('人物'); await click('物品'); await shot('54-take')
  await click('加入'); await win('已加入设定正文'); await pane('手电在工具箱最底下'); await click('关闭提示'); await click('手电在工具箱最底下'); await pane('第87章“手电在工具箱最底下”'); await done('into the setting')
  await menu(3, '姐，你别进去。'); await win('AI 改写…'); await win('Ctrl+L'); await shot('55-menu'); await click('引用到讨论'); await pane('《第87章：停电》'); await pane('姐，你别进去。'); await shot('56-cited')
  await type('输入想讨论的内容', '这句要不要留'); await key('Enter'); await pane('这句要不要留'); await pane('《第87章：停电》'); await sleep(1500); await pane('这是原型'); await done('to a discussion')
  await click('第87章检查完成，有 1 处待处理'); await win('其他章节的类似情况'); await click('正文'); must((await drawn('她往前走了一步。')) === 'doubt', 'what the check found is not marked in the prose')
  await click('写完一章后检查 ▴'); await win('检查时机'); await shot('57-when'); await click('手动检查'); await win('手动检查 ▴'); await done('the line under the prose')

  // The discussion page: a talk, what stands under a reply, a new talk, the model.
  await click('重新开始'); await click('讨论'); await win('总讨论'); await win('4 条发言'); await win('前半段偏慢'); await pane('待确认'); await pane('伏笔', false); await shot('58-talk-page')
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('收起这段对话'); await win('已收起 1 段对话'); await win('主角有个说不出口的秘密', false); await click('已收起 1 段对话'); await win('不显示收起的对话') }
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('移回列表'); await win('不显示收起的对话', false); await win('主角有个说不出口的秘密'); await done('a talk put away and back') }
  await click('小祥和林晚是怎么认识的'); await win('2 条发言'); await click('AI 参考内容'); await win('回复时附带 4 条设定，因预算不足省略 0 条'); await click('复制全文'); await win('已复制'); await click('关闭提示')
  await win('设定里只写了他是她带出来的徒弟', true); await click('摘取原文', 1); await win('拖选一段原文，作为已确定的设定加入正文。原文不会被修改。'); await win('尚未选中文字'); await click('收起原文'); await win('尚未选中文字', false); await click('摘取原文', 1)
  await js(`(() => { const box = document.querySelector('.source-text'), r = document.createRange(); r.setStart(box.firstChild, 0); r.setEnd(box.firstChild, 18); const s = getSelection(); s.removeAllRanges(); s.addRange(r); box.dispatchEvent(new MouseEvent('mouseup', { bubbles: true })) })()`); await win('已选 18 字'); await shot('58b-source')
  await click('加入设定'); await win('已加入设定正文'); await pane('设定里只写了他是她带出来的徒弟。可以'); await click('关闭提示'); await done('what stands under a reply')
  await click('新对话'); await win('从一个初步的想法开始'); await shot('59-new-talk-page'); await click('这个故事的冲突在哪'); await win('正在思考…'); await sleep(1500); await win('这是原型'); await win('2 条发言')
  await type('讨论输入', '再想想'); await key('Enter', true); await win('3 条发言'); await sleep(1500); await win('先想清楚这一段要做什么'); await done('a new talk on its page')
  await click('当前模型'); await win('思考程度'); await shot('60-model'); await click('claude-sonnet-5-5'); await win('思考程度', false); await win('claude-sonnet-5-5'); await click('当前模型'); await click('High'); await click('当前模型'); await click('模型设置…'); await win('为本作品选择模型。'); await done('the model of a discussion')

  // Settings: the models and the services, the look, reminders, backups.
  must((await value('讨论模型')) === 'claude-sonnet-5-5', 'the settings show another model'); await shot('61-settings')
  await click('沿用讨论模型'); await click('DeepSeek'); must((await value('整理模型')) === 'deepseek-chat', 'choosing a service chose no model'); must(await js(`document.querySelector('.services').innerText.split('本作品使用中').length === 3`), 'the service taken is not said to be in use')
  await click('Low'); await click('Medium'); await win('Low', false); await clear('上下文预算'); await type('上下文预算', '100'); await click('保存'); await win('上下文预算至少为 8192'); await click('关闭提示'); await click('停笔后自动续写'); must(await js(`document.querySelectorAll('.ticks .check.on').length === 2`), 'the box is not ticked')
  await click('编辑'); await win('远程服务：讨论内容将发送到该地址'); await win('key 保存在系统钥匙串中，不会写入配置文件或作品。'); await shot('62-service'); await click('获取可用模型'); await win('正在读取可用模型…'); await sleep(1400); await win('已获取模型目录保存提供商后生效')
  await click('保存提供商'); await win('提供商已保存'); await win('收起', false); await click('添加服务'); await type('显示名称', '本地'); await type('API 地址', 'http://localhost:11434/v1'); await click('保存提供商'); await win('localhost:11434 · 0 个模型'); await sleep(3300); await done('models and services')
  await click('重新加载配置'); await click('打开配置文件'); await click('保存'); await clear('上下文预算'); await type('上下文预算', '很多'); await click('保存'); await win('上下文预算须为整数'); await win('还有 1 条'); await shot('63-toasts'); await sleep(3300); await win('还有 1 条', false); await done('more notices than fit')
  await click('外观'); await win('仅在本机生效，适用于所有作品。'); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('64-look'); await click('浅色')
  await click('提醒与检查'); await win('当前有 2 个伏笔久未回收。'); await click('增加 5 章'); await click('增加 5 章'); await click('增加 5 章'); await win('45 章'); await win('当前有 1 个伏笔久未回收。'); await click('边写边检查'); await shot('65-reminders')
  await click('备份'); await win('为整部作品另存一份副本，以防设备故障。'); await win('31.2 万字 · 8 章 · 已存 12 份，最早 9 月 28 日'); await shot('66-backup'); await click('立即备份'); await win('正在备份…'); await sleep(1400); await win('刚刚 已备份'); await win('已存 13 份')
  await click('写作时每小时一次'); await click('最近 30 份'); await click('最近 90 份'); await win('最近 30 份', false); await click('关闭设置'); await win('为整部作品另存一份副本', false); await click('正文'); await win('边写边检查 ▴'); await done('the look, reminders and backups')
  await click('设置'); await clear('讨论模型'); await click('关闭设置'); await click('讨论', 1); await click('全部讨论'); await click('新讨论'); await type('输入想讨论的内容', '在吗'); await key('Enter'); await win('请先配置讨论模型'); await sleep(3300); await win('请先配置讨论模型'); await shot('67-no-model'); await click('打开设置'); await win('为本作品选择模型。'); await win('请先配置讨论模型', false); await key('Escape'); await done('no model to talk with')

  // The check page.
  await click('重新开始'); await click('检查'); await win('已检查至番外：晴天'); await win('全部 5'); await win('1 / 5'); await win('设定 · 人物 · 林晚'); await win('其他章节的类似情况'); await shot('68-check')
  await click('下一条'); await win('2 / 5'); await click('名字有误 1'); await win('应为「周祥」'); await win('1 / 1'); await shot('69-check-name'); await click('修改正文'); await win('第84章：刻字'); must((await drawn('周翔')) === 'sel', 'the wrong name is not picked out in the prose'); await done('from a finding to the prose')
  await click('检查'); await click('新出现 2'); await win('可加入设定 · 人物'); await shot('70-check-new'); await click('加入设定'); await win('已加入设定'); await win('新出现 1'); await click('关闭提示'); await click('忽略'); await win('这一类没有待处理的'); await done('names not yet in the setting')
  await click('不一致 2'); await click('有意为之'); await win('不一致 1'); await click('修改设定'); await pane('名字'); await pane('别名'); await win('第87章：停电'); await click('取消')
  await click('检查'); await click('有意为之'); await win('这一类没有待处理的'); await click('名字有误 1'); await click('有意为之'); await win('没有发现不一致'); await win('写完一章后自动检查，结果显示在此处。'); await shot('71-check-none'); await done('settle every finding')

  // Categories.
  await click('作品分类'); await win('设定文档按分类分节'); await win('落点：大纲'); await shot('72-cats'); await type('分类名称', '线索'); await type('落点', '大纲'); await click('保存分类'); await win('分类“线索”已保存'); await click('关闭提示')
  must(await js(`(() => { const t = document.querySelector('.cat-rows').innerText; return t.includes('线索') && t.indexOf('线索') < t.indexOf('其他') })()`), 'the new category is not before the last one'); await click('编辑'); must((await value('分类名称')) === '人物', 'the category is not ready to change')
  await click('删除'); await win('5 条设定将移到“其他”'); await shot('73-cat-delete'); await click('确认删除'); await win('已删除分类“人物”'); must(!(await js(`document.querySelector('.cat-rows').innerText.includes('人物')`)), 'the category is still listed'); await click('关闭提示'); await done('categories')

  // The shelf, new and empty works, and the three ways a backup can stand.
  await click('重新开始'); await click('切换作品'); await click('全部作品…'); await win('我的作品'); await win('4 部 · 作品库：D:\\写作'); await win('找不到文件夹'); await shot('74-shelf')
  await click('列表'); await win('重新定位…'); await win('D:\\写作\\旧城灯匠'); await shot('75-shelf-list'); await click('卡片'); await click('更多操作'); await win('打开所在文件夹'); await click('重命名…'); await win('仅修改作品名，文件夹不变。'); await clear('新作品名'); await type('新作品名', '雨城'); await click('保存'); await win('雨城来信', false)
  await click('移除'); await win('已从书架移除文件夹未改动：早年的稿子'); await win('3 部'); await click('关闭提示'); await done('the shelf')
  await click('新建作品'); await win('将新建该文件夹；该文件夹须不存在或为空。'); must((await value('作品位置')) === 'D:\\写作\\未命名作品', 'where the work goes is not offered'); await type('作品名称', '新书'); must((await value('作品位置')) === 'D:\\写作\\新书', 'where the work goes does not follow its name'); await click('网文'); await click('剧本'); await shot('76-new-book'); await click('创建')
  await win('暂无章节'); await win('新书'); await win('尚未备份'); await pane('暂无设定'); await shot('77-empty-book'); await click('新建第一章'); await key('Enter'); await win('在此开始写作…'); await win('本章 0 字 · 全书 0 字'); await win('设定为空，无可对照内容'); await click('检查'); await click('检查全书'); await win('请先在讨论中确定设定，才能对照检查。'); await click('关闭提示'); await click('正文'); await done('a new work')
  await click('尚未备份'); await win('作品目前仅保存在本机。'); await click('选择位置…'); await win('选择其他位置，定期保存副本。'); await win('OneDrive 同步文件夹'); await shot('78-first-backup'); await click('开始备份'); await win('正在备份…'); await sleep(1400); await win('已备份 · 刚刚'); await done('a first backup')
  await click('切换作品'); await click('井边'); await win('备份失败 · 找不到 E 盘'); await win('从左侧选择一章开始写作', false); await win('暂无章节'); await shot('79-backup-failed'); await click('待处理 1'); await win('备份失败，找不到 E 盘'); await win('点击重试 · 上次备份 昨天 21:40'); await key('Escape')
  await click('备份失败 · 找不到 E 盘'); await win('找不到 E 盘，本次备份未完成'); await win('作品未受影响，请接入硬盘后重试。'); await shot('80-backup-trouble'); await click('今日不再提醒'); must(!(await has('.warn-btn')), 'the warning is still on the toolbar')
  await click('备份失败'); await click('重试'); await sleep(1400); await win('已备份 · 刚刚'); must(!(await has('.todo-btn')), 'the failed backup is still pending'); await done('a backup that failed')
  await click('切换作品'); await click('旧城灯匠'); await click('设置'); await click('备份'); await win('尚未备份'); await click('选择位置…'); await click('稍后再说'); await win('尚未备份'); await click('关闭设置')
  await click('切换作品'); await click('全部作品…'); for (let i = 0; i < 4; i++) { await click('更多操作'); await click('从书架移除'); await click('关闭提示') }
  await win('从一个想法开始'); await win('API key 保存在系统钥匙串中，不会写入作品文件夹。'); await shot('81-empty-shelf'); await click('新建作品'); await click('取消'); await click('重新开始'); await win('第87章：停电'); await win('已备份 · 今天 10:32'); await done('an empty shelf, and start over')

  // A volume as a page: its outline, what it holds, a chapter made in it.
  await click('第二卷：旧城'); await win('1 项'); await win('本卷 14.1 万字'); await type('卷纲', '林晚回到旧城，找到那把钥匙。'); await click('在此新建章节'); await key('Enter'); await win('第47章：新章节'); await win('满 200 字后可检查')
  await click('第二卷：旧城'); await win('2 项'); must((await value('卷纲')) === '林晚回到旧城，找到那把钥匙。', 'the outline of the volume is lost'); await pane('附上本卷内容', false); await click('讨论', 1); await click('新讨论'); await pane('附上本卷内容'); await click('全部讨论'); await click('设定'); await shot('82-folder-own'); await done('a volume as a page')

  // A short name picked in the prose goes into the setting as it is.
  await click('第三卷：停电'); await click('第三卷：停电'); await click('第85章：灯数'); await menu(0, '何主任'); await click('加入名字表'); must(await js(`document.querySelector('.side-pop').innerText.includes('组织') && document.querySelector('.side-pop').getBoundingClientRect().left > document.querySelector('.ctx').getBoundingClientRect().right - 8`), 'the categories of names are not beside the menu'); await shot('83-name-menu'); await js(`document.querySelector('.side-pop .row').click()`)
  await win('「何主任」已加入设定的人物输入时可自动补全'); await click('关闭提示'); await menu(0, '何主任'); await win('已在名字表中'); await key('Escape')
  await menu(0, '何主任让他们把旧城的灯重新数一遍'); must(await js(`[...document.querySelectorAll('.ctx .row')].find((e) => e.innerText.includes('加入名字表')).classList.contains('off')`), 'a long pick can be a name'); await key('Escape'); await done('a name from the prose')

  // Saving, and what is stale once the prose changes.
  await click('第86章：档案室'); await win('已保存'); await key('j', true); await sleep(900); await key('Tab'); await win('未保存'); await win('第86章已修改，尚未重新检查'); await win('本章摘要 · 可能过时 ▾'); await sleep(1800); await win('已保存')
  await key('s', true); await win('停笔片刻也会自动保存'); await click('关闭提示'); await done('saving')

  // Rare things, shown from the bar over the window: a chapter changed outside, a check and a summary that failed.
  await click('第87章：停电'); await click('少见的情况'); await click('本章在软件外被修改'); await win('本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。'); await shot('84-outside')
  await click('使用磁盘版本'); await win('已采用磁盘版本编辑器原内容已存为快照'); await win('（这一句是在别的程序里加上的。）'); await win('本章文件已被其他程序修改', false); await click('关闭提示')
  await click(MORE); await click('快照'); await win('外部改动'); await win('该快照与当前正文相同', false); await click('关闭')
  await click('少见的情况'); await click('本章在软件外被修改'); await click('保留当前版本'); await win('已保留编辑器版本磁盘内容已存为快照'); await click('关闭提示'); await done('a chapter changed outside')
  await click('少见的情况'); await click('本章摘要生成失败'); await win('上次生成失败：无法连接模型服务'); await shot('85-summary-failed'); await click('生成摘要'); await win('本章摘要 · 正在生成 ▴'); await win('AI 正在阅读本章并生成摘要…'); await sleep(1500); await win('AI 生成 · Opus 5.5 · 刚刚'); await win('上次生成失败', false)
  await click('少见的情况'); await click('本章检查失败'); await win('检查失败：无法连接模型服务'); await shot('86-check-failed'); await done('a summary and a check that failed')

  // The whole book checked: the chapters changed since, one after another.
  await click('检查'); await click('重新检查'); await win('2 章未检查或有改动'); await win('每章调用一次 AI，共 2 次。'); await shot('87-check-all'); await click('取消'); await win('2 章未检查或有改动', false)
  await click('重新检查'); await click('开始检查'); await win('正在检查第86章，还有 1 章'); await sleep(1000); await win('正在检查第87章'); await win('还有', false); await click('停止'); await win('已检查至番外：晴天'); await click('重新检查'); await win('1 章未检查或有改动'); await click('开始检查'); await sleep(1100)
  await click('重新检查'); await win('各章均已检查正文无新改动'); await click('关闭提示'); await click('正文'); await win('第87章检查完成，有 1 处待处理'); await done('the whole book checked')

  // A chapter the disk lost: what is in the editor becomes a chapter of its own.
  await click('少见的情况'); await click('本章文件丢失'); await win('磁盘上已找不到本章文件：在软件外改名、移动或删除，均视为删除。编辑器中的内容仍保留。'); must(!(await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第87章'))`)), 'the lost chapter is still in the tree'); await shot('88-lost')
  await click('存为新章节'); await win('已存为新章节位于最上层末尾'); await win('磁盘上已找不到本章文件', false); must(await js(`document.querySelector('.tree').lastElementChild.innerText.includes('停电')`), 'the chapter was not kept at the end'); await click('关闭提示')
  await click('少见的情况'); await click('AI 改写出错'); await win('模型未返回改写结果'); await win('复制详情'); await click('关闭提示'); await click('重新开始'); await done('a chapter the disk lost')

  // The pane: a renamed entry and the prose, the bookkeeper's sign, its width.
  await click('林晚'); await click('修改'); await clear('名字'); await type('名字', '林婉'); await click('保存'); await click('全部设定'); await pane('处「林晚」'); await shot('89-renamed')
  await click('全书替换'); must((await value('查找')) === '林晚' && (await value('替换为')) === '林婉', 'the old name is not ready to replace'); await win('全书 '); await pane('处「林晚」', false); await click('关闭查找')
  await click('周祥'); await click('修改'); await clear('名字'); await type('名字', '周详'); await click('保存'); await click('全部设定'); await click('不替换'); await pane('正文中仍有', false); await done('a name changed in the setting')
  await click('讨论', 1); await click('新讨论'); await type('输入想讨论的内容', '这一章慢不慢'); await key('Enter'); await sleep(1400); must(await has('.tidy'), 'no sign of the bookkeeper at work'); await sleep(1700); must(!(await has('.tidy')), 'the bookkeeper never ends')
  await click('设定'); await click('更多'); await click('修改记录'); await pane('无需修改'); await click('整理思考过程'); await pane('没有确定新的设定'); await shot('90-log')
  await click('少见的情况'); await click('设定整理失败'); must(await has('.tidy.warn'), 'no warning of the bookkeeping that failed'); await click('全部设定'); await click('上次整理失败，点击查看'); await pane('最近几轮整理失败：无法连接 AI'); await done('the bookkeeper')
  { const [x, y] = await js(`(() => { const r = document.querySelector('.pane-edge').getBoundingClientRect(); return [r.x + 3, r.y + 300] })()`) as [number, number]; await drag([x, y], [x - 120, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 510, 'the pane is not wider'); await shot('91-pane-wide')
    await drag([x - 120, y], [x + 400, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 300, 'the pane is narrower than it may be'); await drag([x + 90, y], [x, y]); await done('the width of the pane') }

  // Foreshadowings: the one picked to pay off shown in the pane, a list with nothing in it.
  await click('伏笔'); await click('第87章：停电'); await menu(5, '她往前走了一步。'); await click('记为回收…'); await click('井沿上的刻痕'); await win('第2章埋下，第85章回收'); await pane('第 85 章已有一处回收'); await shot('92-paying'); await key('Escape')
  await click('更多'); await click('已放弃'); await click('红色钥匙'); await click('恢复伏笔'); await click('全部伏笔'); await pane('此处暂无伏笔'); await click('全部伏笔'); await done('the foreshadowing picked')

  // The tree: a chapter dragged into a volume, and before another chapter.
  const pull = (from: string, to: string) => js(`(() => { const row = (k) => document.querySelector('[data-node="' + k + '"]'), d = new DataTransfer(); for (const [e, k] of [['dragstart', ${JSON.stringify(from)}], ['dragover', ${JSON.stringify(to)}], ['drop', ${JSON.stringify(to)}], ['dragend', ${JSON.stringify(from)}]]) row(k).dispatchEvent(new DragEvent(e, { bubbles: true, cancelable: true, dataTransfer: d })) })()`)
  await pull('87', 'v2'); must(await js(`document.querySelector('[data-node="87"]').classList.contains('deep') && document.querySelector('[data-node="87"]').previousElementSibling.dataset.node === '46'`), 'the chapter is not at the end of the volume')
  await pull('87', '46'); must(await js(`document.querySelector('[data-node="87"]').nextElementSibling.dataset.node === '46'`), 'the chapter is not before the other'); await shot('93-dragged'); await done('chapters dragged')

  // Settings: models not filled in, a local service, a variable found.
  await click('设置'); await clear('讨论模型'); await click('保存'); await win('讨论模型需要同时选择服务和填写模型 ID'); await click('关闭提示'); await type('讨论模型', 'claude-opus-5-5')
  await click('添加服务'); await win('ID（小写字母、数字、- _）'); await win('将发送到该地址', false); await type('API 地址', 'http://localhost:11434/v1'); await win('本机地址：请确认该服务不会转发到云端'); await type('环境变量名称', 'DEEPSEEK_API_KEY'); await win('已检测到环境变量 DEEPSEEK_API_KEY。'); await shot('94-service-local')
  await click('少见的情况'); await click('模型目录读取失败'); await win('读取模型目录失败'); await win('打开设置'); await click('重试'); await win('读取模型目录失败', false); await win('正在读取可用模型…'); await sleep(1400); await click('关闭提示'); await click('关闭设置'); await done('settings that do not hold')

  // Backups that fail in the other two ways.
  await click('少见的情况'); await click('备份文件夹丢失'); await click('备份失败 · 找不到备份文件夹'); await win('找不到备份文件夹，本次备份未完成'); await win('作品未受影响，文件夹可能已被移动或改名。上次备份于今天 10:32。'); await key('Escape')
  await click('少见的情况'); await click('备份文件夹不可写'); await click('备份失败 · 没有权限写入'); await win('备份文件夹无法写入，本次备份未完成'); await win('作品未受影响，该文件夹没有写入权限。'); await shot('95-backup-denied'); await click('重试'); await sleep(1400); await win('已备份 · 刚刚'); await done('backups that fail otherwise')

  // The shelf: a work added from disk, a place taken, a name left empty.
  await click('切换作品'); await click('全部作品…'); await click('添加已有作品…'); await win('已加入书架'); await win('井边的旧稿'); await win('尚未打开'); await win('5 部'); await click('关闭提示'); await click('添加已有作品…'); await win('该作品已在书架上'); await click('关闭提示')
  await click('少见的情况'); await click('添加的文件夹中没有作品'); await win('该文件夹中没有作品（缺少 work.yaml）'); await click('关闭提示')
  await click('新建作品'); await type('作品名称', '雨城来信'); await click('创建'); await win('D:\\写作\\雨城来信 中已有文件请更换名称或位置'); await shot('96-place-taken'); await click('关闭提示'); await clear('作品位置'); await click('创建'); await win('请选择作品位置'); await click('关闭提示'); await click('取消')
  await click('更多操作'); await click('重命名…'); await clear('新作品名'); await click('保存'); await win('作品名不能为空'); await click('关闭提示'); await click('取消'); await click('重新开始'); await done('the shelf, where things go wrong')
}
