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
  await click('重试'); await win('正在重新整理…'); await sleep(1400); await pane('这几轮没整理成功', false); await pane('后台整理（重试）'); await done('retry')
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
  await click('换个原句'); await win('先在正文里选中要挂上去的句子'); await click('关闭提示'); await win('先在正文里选中要挂上去的句子', false); await click('看原文'); await select(5, '小祥把左手揣回兜里。'); await click('换个原句')
  await pane('第 1 章埋下，已过 86 章'); await pane('记为回收'); must((await drawn('小祥把左手揣回兜里。')) === 'fs on', 'the new sentence is not lit'); await done('a new sentence')
  await click('全部伏笔'); await pane('这里还没有'); await click('全部伏笔'); await click('更多'); await shot('24-foreshadowing-menu'); await click('已放弃'); await click('红色钥匙'); await pane('已放弃'); await click('恢复伏笔'); await pane('放弃伏笔'); await done('give up and take back')

  // The pane folds away.
  await click('收起右栏'); must(!(await js(`!!document.querySelector('#pane')`)), 'the pane is still there'); await win('本章伏笔 1'); await shot('25-folded')
  await click('展开右栏'); await pane('放弃伏笔'); await click('收起右栏'); await click('本章伏笔 1'); await pane('未回收'); await done('fold the pane')

  // Discussions.
  await click('讨论', 1); await pane('最近'); await click('新讨论'); await pane('试试这样问'); await shot('26-new-talk')
  await click('接下来可以怎么写'); await pane('正在思考…'); await pane('取消请求'); await sleep(800); await pane('正在回复…'); await sleep(700); await pane('这里是原型'); await pane('正在回复…', false)
  await type('输入消息', '那下一章呢'); await key('Enter'); await pane('那下一章呢'); await sleep(1500); await pane('先想清楚这一段要做什么'); await shot('27-talk')
  await type('输入消息', '再说一句'); await key('Enter'); await click('取消请求'); await pane('这条还没有得到回复'); await pane('正在思考', false); await shot('27b-unanswered'); await click('重试回复'); await pane('这条还没有得到回复', false); await sleep(1500); await pane('我会先留着')
  await pane('先看设定里和这句话有关的几条', false); await click('思考过程'); await pane('先看设定里和这句话有关的几条'); await shot('27c-thought'); await click('思考过程'); await pane('先看设定里和这句话有关的几条', false); await done('a reply cancelled and asked for again')
  await click('全部讨论'); await pane('接下来可以怎么写'); await click('这一章的节奏怎么样'); await pane('前半段偏慢'); await done('discussions')

  // Dark, and back to how it opened.
  await click(MORE); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('28-dark'); await key('Escape'); await click('设定'); await shot('29-dark-setting')
  await click('重新开始'); await pane('设定4'); await pane('等你决定'); await win('第87章：停电'); must(await js(`document.querySelector('#win').className === 'dark'`), 'starting over lost the look'); await click(MORE); await click('浅色'); await key('Escape'); await done('dark, and start over')

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
  await clear('章节标题'); await type('章节标题', '雨停'); await key('Enter'); await win('第88章：雨停'); await win('从这里开始写…'); await win('满 200 字后可以检查'); await win('本章 0 字'); await done('a new chapter')
  await rclick(...(await row('第88章：雨停'))); await win('移到回收站'); await win('最上层'); await shot('31-tree-menu'); await click('重命名'); await clear('章节标题'); await type('章节标题', '天晴'); await key('Enter'); await win('第88章：天晴'); await done('rename a chapter')
  await rclick(...(await row('第88章：天晴'))); await click('上移'); must(await js(`(() => { const t = document.querySelector('.tree').innerText; return t.indexOf('天晴') < t.indexOf('第87章') })()`), 'the chapter did not move up')
  await rclick(...(await row('第88章：天晴'))); await click('最上层'); await win('第88章：天晴', false); await win('天晴'); await done('move a chapter')
  await rclick(...(await row('天晴'))); await click('移到回收站'); await win('「天晴」已移到回收站'); await click('关闭提示'); must(!(await js(`document.querySelector('.tree').innerText.includes('天晴')`)), 'the chapter is still in the tree')
  await click('新建文件夹'); await clear('章节标题'); await type('章节标题', '第四卷'); await key('Enter'); await win('第四卷')
  await click('回收站'); await win('删掉的章节连同它里面的内容先放在这里'); await win('第83章：旧稿'); await shot('32-bin'); await click('恢复', 0); await win('已恢复「天晴」'); await click('关闭提示')
  await click('彻底删除'); await click('确认彻底删除'); await win('回收站是空的'); await click('关闭'); must(await js(`document.querySelector('.tree').innerText.includes('天晴')`), 'the chapter did not come back'); await done('the bin')
  await click('切换作品'); await win('旧城灯匠'); await win('全部作品…'); await shot('33-book-menu'); await click('快捷键…'); await win('打开这张表'); await win('专注写作：只留正文'); await shot('34-keys'); await click('关闭'); await win('打开这张表', false)
  await key('/', true); await win('打开这张表'); await key('Escape'); await win('打开这张表', false); await done('the book menu and the keys')
  await click('已备份 · 今天 10:32'); await win('今天 10:32 备份过'); await win('下次：关闭作品时'); await shot('35-backup-pop'); await click('立即备份'); await win('正在备份…'); await sleep(1400); await win('已备份 · 刚刚'); await done('back up now')

  // The toolbar: the count and the goal, find and replace, the more menu, focus, what is pending.
  await click('重新开始'); await click('字数。点击设每日目标'); await win('最近 7 天'); await win('+2,846 字'); await type('每日目标字数', '3000'); await win('今天 2,846 / 3000'); await shot('36-goal'); await key('Escape'); await win('最近 7 天', false); await done('a goal for the day')
  await key('f', true); must(await js(`document.activeElement.getAttribute('aria-label') === '查找'`), 'Ctrl+F did not open the search'); await type('查找', '小祥'); await win('1 / 2'); must((await drawn('小祥')) === 'got on', 'what was found is not marked'); await shot('37-find')
  await click('下一个'); await win('2 / 2'); await type('替换为', '阿祥'); await click('替换'); await win('阿祥从来不叫她姐'); await win('1 / 1'); await click('全部替换'); await win('已替换 1 处可以撤销'); await win('阿祥在身后叫她'); await win('没有找到'); await click('关闭提示')
  await win('第87章有 1 处要看，正文又改过了'); await click('现在查'); await win('正在检查第87章…'); await sleep(1400); await win('第87章查完了，有 1 处要看'); await done('find and replace in a chapter')
  await clear('查找'); await type('查找', '林晚'); await click('在所有章节里找'); await win('全书 8 章、共 9 处'); await win('第1章：巷口 · 2 处'); await shot('38-find-book')
  await clear('替换为'); await type('替换为', '阿晚'); await click('全书替换'); await win('把全书 8 章里的 9 处“林晚”换成“阿晚”？'); await shot('39-replace-book'); await click('确定替换'); await win('已在 8 章里替换 9 处'); await win('阿晚站在巷口'); await win('全书都没有找到')
  await click('关闭查找'); must(!(await has('.find-bar')), 'the search is still open'); await click('关闭提示'); await done('replace through the book')
  await click(MORE); await win('Ctrl+J'); await shot('40-more'); await click('快照'); await win('快照 · 第87章：停电'); await win('全书替换前'); await win('打开时'); await shot('41-snaps')
  await click('和现在对比'); await win('红色划掉的是快照里有、现在没有的'); must(await has('.snap-text .cut') && await has('.snap-text .add'), 'the comparison shows no change'); await shot('42-snap-diff')
  await click('今天 09:58'); await click('恢复这个版本'); await win('已恢复到 今天 09:58 的版本恢复前的内容也存了快照'); await win('小祥在身后叫她', false); await win('七岁那年的那个晚上'); await click('关闭提示'); await done('snapshots')
  await click(MORE); await click('导出…'); await win('按章节树的顺序拼成一个 .txt'); await click('全书'); await shot('43-export'); await click('导出'); await win('已导出到 D:\\写作\\雨城来信.txt'); await click('关闭提示'); await done('export')
  await click('只留正文，收起章节树和右栏（F11）'); must(!(await has('.sidebar')) && !(await has('#pane')), 'focus left something beside the prose'); await shot('44-focus'); await key('F11'); must(await has('.sidebar') && await has('#pane'), 'F11 did not leave focus'); await done('focus')
  await click('重新开始'); await click('待处理 8'); await win('设定不一致'); await win('「红色钥匙」', false); await win('「钥匙开哪扇门」41 章没回收'); await win('还有 2 条'); await shot('45-pending'); await click('还有 2 条'); await win('「旧城小学」还不在设定里'); await click('收起')
  await click('本章 1'); await win('周翔', false); await win('林晚怕黑，却独自走进黑巷'); await click('全部 8'); await click('「小祥的左手」原句已删'); await pane('第 1 章的原句已删除'); await win('全部忽略', false)
  await click('待处理 8'); await click('全部忽略'); must(!(await has('.todo-btn')), 'ignoring all left the button'); await done('what is pending')

  // Under the title: the outline and the summary.
  await click('重新开始'); await click('章纲 ▾'); must((await value('章纲')).startsWith('全城停电'), 'the outline is not shown'); await shot('46-outline'); await click('本章摘要 · 还没有 ▾'); await win('正文满 200 字后，离开这一章时会自动生成；也可以自己写。')
  await click('生成摘要'); await win('AI 正在读这一章写摘要…'); await sleep(1400); must((await value('摘要')).includes('小祥第一次叫她“姐”'), 'no summary came'); await win('AI 写的 · Opus 5.5 · 刚刚'); await win('重新生成'); await shot('47-summary')
  await type('摘要', '（我加的）'); await win('你写的'); await click('本章摘要 ▴'); must(!(await has('.chapter-box')), 'the summary did not fold')
  await click('第84章：刻字'); await win('章纲 · 空 ▾'); await click('本章摘要 · 可能过时 ▾'); await win('正文改过了'); await click(MORE); must(await js(`[...document.querySelectorAll('.more-pop .row')].find((e) => e.innerText.includes('起草本章')).classList.contains('off')`), 'drafting is not greyed without an outline'); await win('先写章纲'); await key('Escape'); await done('the outline and the summary')

  // AI in the prose: a sentence more, a rewrite, a draft.
  await click('重新开始'); await click(MORE); await click('续写一句'); await win('续写中…'); await sleep(900); await win('Tab 采纳 · Esc 放弃'); must(await has('.ghost-text'), 'no grey sentence'); await shot('48-ghost')
  await key('Escape'); must(!(await has('.ghost-text')), 'Escape kept the sentence'); await win('她把手电咬在嘴里', false); await key('j', true); await sleep(900); await key('Tab'); must(!(await has('.ghost-text')), 'Tab kept it grey'); await win('她往前走了一步。她把手电咬在嘴里'); await done('a sentence more')
  await pick(2, '灯一直没有亮。'); must(await has('.selbar'), 'nothing floats over what was picked'); await win('选中 7 字'); await shot('49-selection'); await click('AI 改写'); must(await js(`document.activeElement.getAttribute('aria-label') === '改写要求'`), 'the wish is not ready to type in')
  await type('改写要求', '慢一点'); await key('Enter'); await win('AI 正在改写选中的部分'); await sleep(1400); await win('红色划掉的是原文，绿色是改写。'); must(await has('.prose .cut') && await has('.prose .add'), 'the rewrite is not shown against the original'); await shot('50-rewrite')
  await click('重新改写'); await win('AI 正在改写选中的部分'); await sleep(1400); await click('采纳'); await win('已采纳改写'); await win('撤销可以换回原文，原来的版本也在快照里'); await win('她在巷口站了很久'); await win('灯一直没有亮。', false); await click('关闭提示')
  await pick(0, '同时灭的'); await click('AI 改写'); await key('Escape'); must(!(await has('.ai-bar')), 'Escape left the rewrite open'); await done('a rewrite')
  await click(MORE); await click('起草本章'); await win('AI 只把章纲写成文字'); await win('前面有 3 章还没有摘要'); await win('这一章已经有内容'); await shot('51-draft'); await click('补齐摘要（3 次调用）'); await win('正在补摘要：0 / 3。补完之前起草，AI 只看得到已有的摘要。'); await sleep(2700); await win('前面有 3 章还没有摘要', false); await win('起草时 AI 会读前面 6 章的摘要，接上前情。')
  await type('目标字数', '1500'); await click('覆盖已写的内容'); await click('开始起草'); await win('AI 正在按章纲起草'); await shot('52-drafting'); await sleep(3800); await win('起草完成，写了 38 字'); await win('整条街的灯是同时灭的。', false); await win('身后，小祥没有再出声。')
  must(await js(`document.querySelectorAll('.prose .ai').length === 3`), 'what the AI drafted is not marked'); await shot('53-drafted'); await click('关闭提示')
  await click(MORE); await click('快照'); await win('AI 改动前'); await win('按章纲起草'); await win('约 1500 字'); await key('Escape')
  await click(MORE); await click('起草本章'); await type('目标字数', '很多'); await click('开始起草'); await win('目标字数需要是正整数或者留空'); await win('复制详情'); await click('关闭提示'); await clear('目标字数'); await click('开始起草'); await sleep(1000); await click('停止'); await win('已停止起草写出的部分保留了'); await click('关闭提示'); must(!(await has('.ai-bar')), 'the draft did not stop'); await done('a draft')

  // What was picked goes to the setting, or to a discussion.
  await click('重新开始'); await pick(1, '手电在工具箱最底下'); await click('加入设定'); await win('选中的原文作为出处记下来'); must((await value('设定内容')) === '手电在工具箱最底下', 'what was picked is not in the box'); await click('人物'); await click('物品'); await shot('54-take')
  await click('加入'); await win('已加入设定正文'); await pane('手电在工具箱最底下'); await click('关闭提示'); await click('手电在工具箱最底下'); await pane('第87章“手电在工具箱最底下”'); await done('into the setting')
  await menu(3, '姐，你别进去。'); await win('AI 改写…'); await win('Ctrl+L'); await shot('55-menu'); await click('拿去讨论'); await pane('《第87章：停电》'); await pane('姐，你别进去。'); await shot('56-cited')
  await type('想聊什么', '这句要不要留'); await key('Enter'); await pane('这句要不要留'); await pane('《第87章：停电》'); await sleep(1500); await pane('这里是原型'); await done('to a discussion')
  await click('第87章查完了，有 1 处要看'); await win('别的章也有类似的地方'); await click('正文'); must((await drawn('她往前走了一步。')) === 'doubt', 'what the check found is not marked in the prose')
  await click('写完一章就查 ▴'); await win('什么时候检查'); await shot('57-when'); await click('我点了才查'); await win('点了才查 ▴'); await done('the line under the prose')

  // The discussion page: a talk, what stands under a reply, a new talk, the model.
  await click('重新开始'); await click('讨论'); await win('总讨论'); await win('4 条发言'); await win('前半段偏慢'); await pane('等你决定'); await pane('伏笔', false); await shot('58-talk-page')
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('收起这段对话'); await win('已收起 1 段对话'); await win('主角有个说不出口的秘密', false); await click('已收起 1 段对话'); await win('不显示收起的对话') }
  { const [x, y] = await row('主角有个说不出口的秘密'); await rclick(x, y); await click('放回列表'); await win('不显示收起的对话', false); await win('主角有个说不出口的秘密'); await done('a talk put away and back') }
  await click('小祥和林晚是怎么认识的'); await win('2 条发言'); await click('AI 看到了什么'); await win('回复时带上了 4 条设定，预算不足省略 0 条'); await click('复制全文'); await win('已复制'); await click('关闭提示')
  await win('设定里只写了他是她带出来的徒弟', true); await click('摘取原文', 1); await win('拖选一段原话，作为你定下的设定加入正文。原话不会被改动。'); await win('还没有选中文字'); await click('收起原文'); await win('还没有选中文字', false); await click('摘取原文', 1)
  await js(`(() => { const box = document.querySelector('.source-text'), r = document.createRange(); r.setStart(box.firstChild, 0); r.setEnd(box.firstChild, 18); const s = getSelection(); s.removeAllRanges(); s.addRange(r); box.dispatchEvent(new MouseEvent('mouseup', { bubbles: true })) })()`); await win('已选 18 字'); await shot('58b-source')
  await click('加入设定'); await win('已加入设定正文'); await pane('设定里只写了他是她带出来的徒弟。可以'); await click('关闭提示'); await done('what stands under a reply')
  await click('新对话'); await win('从一个模糊的念头聊起'); await shot('59-new-talk-page'); await click('这个故事的冲突在哪'); await win('正在思考…'); await sleep(1500); await win('这里是原型'); await win('2 条发言')
  await type('讨论输入', '再想想'); await key('Enter', true); await win('3 条发言'); await sleep(1500); await win('先想清楚这一段要做什么'); await done('a new talk on its page')
  await click('当前模型'); await win('思考程度'); await shot('60-model'); await click('claude-sonnet-5-5'); await win('思考程度', false); await win('claude-sonnet-5-5'); await click('当前模型'); await click('High'); await click('当前模型'); await click('模型设置…'); await win('给这部作品选模型。'); await done('the model of a discussion')

  // Settings: the models and the services, the look, reminders, backups.
  must((await value('讨论模型')) === 'claude-sonnet-5-5', 'the settings show another model'); await shot('61-settings')
  await click('沿用讨论模型'); await click('DeepSeek'); must((await value('整理模型')) === 'deepseek-chat', 'choosing a service chose no model'); must(await js(`document.querySelector('.services').innerText.split('这部作品在用').length === 3`), 'the service taken is not said to be in use')
  await click('Low'); await click('Medium'); await win('Low', false); await clear('上下文预算'); await type('上下文预算', '100'); await click('保存'); await win('上下文预算至少为 8192'); await click('关闭提示'); await click('停笔后自动续写'); must(await js(`document.querySelectorAll('.ticks .check.on').length === 2`), 'the box is not ticked')
  await click('编辑'); await win('远程服务：你的讨论内容会发送到这个地址'); await win('key 存入系统钥匙串，不会写进配置文件或作品。'); await shot('62-service'); await click('获取可用模型'); await win('正在读取可用模型…'); await sleep(1400); await win('已获取模型目录保存提供商后生效')
  await click('保存提供商'); await win('提供商已保存'); await win('收起', false); await click('添加服务'); await type('显示名称', '本地'); await type('API 地址', 'http://localhost:11434/v1'); await click('保存提供商'); await win('localhost:11434 · 0 个模型'); await sleep(3300); await done('models and services')
  await click('重新加载配置'); await click('打开配置文件'); await click('保存'); await clear('上下文预算'); await type('上下文预算', '很多'); await click('保存'); await win('上下文预算需要是整数'); await win('还有 1 条'); await shot('63-toasts'); await sleep(3300); await win('还有 1 条', false); await done('more notices than fit')
  await click('外观'); await win('只在这台电脑上生效，所有作品都一样。'); await click('深色'); must(await js(`document.querySelector('#win').className === 'dark'`), 'the window is not dark'); await shot('64-look'); await click('浅色')
  await click('提醒与检查'); await win('现在有 2 个伏笔久未回收。'); await click('多 5 章'); await click('多 5 章'); await click('多 5 章'); await win('45 章'); await win('现在有 1 个伏笔久未回收。'); await click('边写边查'); await shot('65-reminders')
  await click('备份'); await win('把整部作品多存一份，防电脑坏。'); await win('31.2 万字 · 8 章 · 已存 12 份，最早 9 月 28 日'); await shot('66-backup'); await click('立即备份'); await win('正在备份…'); await sleep(1400); await win('刚刚 备份过'); await win('已存 13 份')
  await click('写作时每小时一次'); await click('最近 30 份'); await click('最近 90 份'); await win('最近 30 份', false); await click('关闭设置'); await win('把整部作品多存一份', false); await click('正文'); await win('边写边查 ▴'); await done('the look, reminders and backups')
  await click('设置'); await clear('讨论模型'); await click('关闭设置'); await click('讨论', 1); await click('全部讨论'); await click('新讨论'); await type('想聊什么', '在吗'); await key('Enter'); await win('请先配置讨论模型'); await sleep(3300); await win('请先配置讨论模型'); await shot('67-no-model'); await click('去设置'); await win('给这部作品选模型。'); await win('请先配置讨论模型', false); await key('Escape'); await done('no model to talk with')

  // The check page.
  await click('重新开始'); await click('检查'); await win('已检查至番外：晴天'); await win('全部 5'); await win('1 / 5'); await win('设定 · 人物 · 林晚'); await win('别的章也有类似的地方'); await shot('68-check')
  await click('下一条'); await win('2 / 5'); await click('名字有误 1'); await win('应为「周祥」'); await win('1 / 1'); await shot('69-check-name'); await click('改正文'); await win('第84章：刻字'); must((await drawn('周翔')) === 'sel', 'the wrong name is not picked out in the prose'); await done('from a finding to the prose')
  await click('检查'); await click('新出现 2'); await win('可以加入设定 · 人物'); await shot('70-check-new'); await click('加入设定'); await win('已加入设定'); await win('新出现 1'); await click('关闭提示'); await click('不用记'); await win('这一类没有待处理的'); await done('names not yet in the setting')
  await click('不一致 2'); await click('是故意的'); await win('不一致 1'); await click('改设定'); await pane('名字'); await pane('别名'); await win('第87章：停电'); await click('取消')
  await click('检查'); await click('是故意的'); await win('这一类没有待处理的'); await click('名字有误 1'); await click('是故意的'); await win('没有发现不一致'); await win('写完一章会自动检查，结果列在这里。'); await shot('71-check-none'); await done('settle every finding')

  // Categories.
  await click('作品分类'); await win('设定文档按分类分节'); await win('落点：大纲'); await shot('72-cats'); await type('分类名称', '线索'); await type('落点', '大纲'); await click('保存分类'); await win('分类“线索”已保存'); await click('关闭提示')
  must(await js(`(() => { const t = document.querySelector('.cat-rows').innerText; return t.includes('线索') && t.indexOf('线索') < t.indexOf('其他') })()`), 'the new category is not before the last one'); await click('编辑'); must((await value('分类名称')) === '人物', 'the category is not ready to change')
  await click('删除'); await win('5 条设定将移到“其他”'); await shot('73-cat-delete'); await click('确认删除'); await win('已删除分类“人物”'); must(!(await js(`document.querySelector('.cat-rows').innerText.includes('人物')`)), 'the category is still listed'); await click('关闭提示'); await done('categories')

  // The shelf, new and empty works, and the three ways a backup can stand.
  await click('重新开始'); await click('切换作品'); await click('全部作品…'); await win('我的作品'); await win('4 部 · 作品库：D:\\写作'); await win('找不到文件夹'); await shot('74-shelf')
  await click('列表'); await win('重新定位…'); await win('D:\\写作\\旧城灯匠'); await shot('75-shelf-list'); await click('卡片'); await click('更多操作'); await win('打开所在文件夹'); await click('重命名…'); await win('只改作品名，文件夹不动。'); await clear('新作品名'); await type('新作品名', '雨城'); await click('保存'); await win('雨城来信', false)
  await click('移除'); await win('已从书架移除文件夹没有动：早年的稿子'); await win('3 部'); await click('关闭提示'); await done('the shelf')
  await click('新建作品'); await win('会新建这个文件夹；它必须不存在或是空的。'); must((await value('作品位置')) === 'D:\\写作\\未命名作品', 'where the work goes is not offered'); await type('作品名称', '新书'); must((await value('作品位置')) === 'D:\\写作\\新书', 'where the work goes does not follow its name'); await click('网文'); await click('剧本'); await shot('76-new-book'); await click('创建')
  await win('还没有章节'); await win('新书'); await win('还没备份过'); await pane('还没有设定'); await shot('77-empty-book'); await click('新建第一章'); await key('Enter'); await win('从这里开始写…'); await win('本章 0 字 · 全书 0 字'); await win('设定还是空的，没有可以对照的'); await click('检查'); await click('检查全书'); await win('先在讨论里把设定定下来，才有可以对照的。'); await click('关闭提示'); await click('正文'); await done('a new work')
  await click('还没备份过'); await win('稿子现在只存在这台电脑上。'); await click('选一个地方…'); await win('选个别的地方，定时多存一份。'); await win('OneDrive 同步文件夹'); await shot('78-first-backup'); await click('开始备份'); await win('正在备份…'); await sleep(1400); await win('已备份 · 刚刚'); await done('a first backup')
  await click('切换作品'); await click('井边'); await win('备份没成功 · 找不到 E 盘'); await win('从左边选一章开始写', false); await win('还没有章节'); await shot('79-backup-failed'); await click('待处理 1'); await win('备份没成功，找不到 E 盘'); await win('点一下重试 · 上次备份 昨天 21:40'); await key('Escape')
  await click('备份没成功 · 找不到 E 盘'); await win('找不到 E 盘，这次没备份上'); await win('稿子没丢，插上硬盘再重试。'); await shot('80-backup-trouble'); await click('今天不提醒'); must(!(await has('.warn-btn')), 'the warning is still on the toolbar')
  await click('备份没成功'); await click('重试'); await sleep(1400); await win('已备份 · 刚刚'); must(!(await has('.todo-btn')), 'the failed backup is still pending'); await done('a backup that failed')
  await click('切换作品'); await click('旧城灯匠'); await click('设置'); await click('备份'); await win('还没有备份'); await click('选一个地方…'); await click('以后再说'); await win('还没有备份'); await click('关闭设置')
  await click('切换作品'); await click('全部作品…'); for (let i = 0; i < 4; i++) { await click('更多操作'); await click('从书架移除'); await click('关闭提示') }
  await win('从一个念头开始'); await win('API key 存在系统钥匙串里，不会写进作品文件夹。'); await shot('81-empty-shelf'); await click('新建作品'); await click('取消'); await click('重新开始'); await win('第87章：停电'); await win('已备份 · 今天 10:32'); await done('an empty shelf, and start over')

  // A volume as a page: its outline, what it holds, a chapter made in it.
  await click('第二卷：旧城'); await win('1 项'); await win('本卷 14.1 万字'); await type('卷纲', '林晚回到旧城，找到那把钥匙。'); await click('在这里新建章节'); await key('Enter'); await win('第47章：新章节'); await win('满 200 字后可以检查')
  await click('第二卷：旧城'); await win('2 项'); must((await value('卷纲')) === '林晚回到旧城，找到那把钥匙。', 'the outline of the volume is lost'); await pane('附上本卷内容', false); await click('讨论', 1); await click('新讨论'); await pane('附上本卷内容'); await click('全部讨论'); await click('设定'); await shot('82-folder-own'); await done('a volume as a page')

  // A short name picked in the prose goes into the setting as it is.
  await click('第三卷：停电'); await click('第三卷：停电'); await click('第85章：灯数'); await menu(0, '何主任'); await click('加入名字表'); must(await js(`document.querySelector('.side-pop').innerText.includes('组织') && document.querySelector('.side-pop').getBoundingClientRect().left > document.querySelector('.ctx').getBoundingClientRect().right - 8`), 'the categories of names are not beside the menu'); await shot('83-name-menu'); await js(`document.querySelector('.side-pop .row').click()`)
  await win('「何主任」已加入设定的人物打字时可以补全'); await click('关闭提示'); await menu(0, '何主任'); await win('已在名字表里'); await key('Escape')
  await menu(0, '何主任让他们把旧城的灯重新数一遍'); must(await js(`[...document.querySelectorAll('.ctx .row')].find((e) => e.innerText.includes('加入名字表')).classList.contains('off')`), 'a long pick can be a name'); await key('Escape'); await done('a name from the prose')

  // Saving, and what is stale once the prose changes.
  await click('第86章：档案室'); await win('已保存'); await key('j', true); await sleep(900); await key('Tab'); await win('还没保存'); await win('第86章改过，还没再查'); await win('本章摘要 · 可能过时 ▾'); await sleep(1800); await win('已保存')
  await key('s', true); await win('停笔片刻也会自动保存'); await click('关闭提示'); await done('saving')

  // Rare things, shown from the bar over the window: a chapter changed outside, a check and a summary that failed.
  await click('第87章：停电'); await click('少见的情况'); await click('这一章在软件外面被改了'); await win('磁盘上的这一章被别的程序改过了。保留哪一边？另一边会存成快照，不会丢。'); await shot('84-outside')
  await click('用磁盘上的'); await win('已换成磁盘上的版本编辑器里原来的内容存了快照'); await win('（这一句是在别的程序里加上的。）'); await win('磁盘上的这一章被别的程序改过了', false); await click('关闭提示')
  await click(MORE); await click('快照'); await win('外部改动'); await win('这一份和现在的正文一样', false); await click('关闭')
  await click('少见的情况'); await click('这一章在软件外面被改了'); await click('保留我的'); await win('已保留编辑器里的版本磁盘上的内容存了快照'); await click('关闭提示'); await done('a chapter changed outside')
  await click('少见的情况'); await click('这一章的摘要没生成出来'); await win('上次生成失败：连不上模型服务'); await shot('85-summary-failed'); await click('生成摘要'); await win('本章摘要 · 正在生成 ▴'); await win('AI 正在读这一章写摘要…'); await sleep(1500); await win('AI 写的 · Opus 5.5 · 刚刚'); await win('上次生成失败', false)
  await click('少见的情况'); await click('这一章检查没成功'); await win('检查没成功：连不上模型服务'); await shot('86-check-failed'); await done('a summary and a check that failed')

  // The whole book checked: the chapters changed since, one after another.
  await click('检查'); await click('重新检查'); await win('2 章未检查或有改动'); await win('每章用一次 AI，共 2 次。'); await shot('87-check-all'); await click('取消'); await win('2 章未检查或有改动', false)
  await click('重新检查'); await click('开始检查'); await win('正在检查第86章，还有 1 章'); await sleep(1000); await win('正在检查第87章'); await win('还有', false); await click('停止'); await win('已检查至番外：晴天'); await click('重新检查'); await win('1 章未检查或有改动'); await click('开始检查'); await sleep(1100)
  await click('重新检查'); await win('各章都查过了正文没有再改'); await click('关闭提示'); await click('正文'); await win('第87章查完了，有 1 处要看'); await done('the whole book checked')

  // A chapter the disk lost: what is in the editor becomes a chapter of its own.
  await click('少见的情况'); await click('这一章在磁盘上找不到了'); await win('磁盘上找不到这一章了：在软件外改名、移动或删除，都会被当成删掉了。编辑器里的文字还在。'); must(!(await js(`[...document.querySelectorAll('.tree-row')].some((r) => r.innerText.includes('第87章'))`)), 'the lost chapter is still in the tree'); await shot('88-lost')
  await click('存为新章节'); await win('已存为新章节放在最上层的末尾'); await win('磁盘上找不到这一章了', false); must(await js(`document.querySelector('.tree').lastElementChild.innerText.includes('停电')`), 'the chapter was not kept at the end'); await click('关闭提示')
  await click('少见的情况'); await click('AI 改写没成功'); await win('模型没有给出改写'); await win('复制详情'); await click('关闭提示'); await click('重新开始'); await done('a chapter the disk lost')

  // The pane: a renamed entry and the prose, the bookkeeper's sign, its width.
  await click('林晚'); await click('修改'); await clear('名字'); await type('名字', '林婉'); await click('保存'); await click('全部设定'); await pane('处「林晚」'); await shot('89-renamed')
  await click('全书替换'); must((await value('查找')) === '林晚' && (await value('替换为')) === '林婉', 'the old name is not ready to replace'); await win('全书 '); await pane('处「林晚」', false); await click('关闭查找')
  await click('周祥'); await click('修改'); await clear('名字'); await type('名字', '周详'); await click('保存'); await click('全部设定'); await click('不换'); await pane('正文里还有', false); await done('a name changed in the setting')
  await click('讨论', 1); await click('新讨论'); await type('想聊什么', '这一章慢不慢'); await key('Enter'); await sleep(1400); must(await has('.tidy'), 'no sign of the bookkeeper at work'); await sleep(1700); must(!(await has('.tidy')), 'the bookkeeper never ends')
  await click('设定'); await click('更多'); await click('修改记录'); await pane('没有需要改的'); await click('整理时的思考'); await pane('没有定下新的设定'); await shot('90-log')
  await click('少见的情况'); await click('设定没整理成功'); must(await has('.tidy.warn'), 'no warning of the bookkeeping that failed'); await click('全部设定'); await click('上次整理没成功，点开看看'); await pane('这几轮没整理成功：连不上 AI'); await done('the bookkeeper')
  { const [x, y] = await js(`(() => { const r = document.querySelector('.pane-edge').getBoundingClientRect(); return [r.x + 3, r.y + 300] })()`) as [number, number]; await drag([x, y], [x - 120, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 510, 'the pane is not wider'); await shot('91-pane-wide')
    await drag([x - 120, y], [x + 400, y]); must((await js(`document.getElementById('pane').offsetWidth`)) === 300, 'the pane is narrower than it may be'); await drag([x + 90, y], [x, y]); await done('the width of the pane') }

  // Foreshadowings: the one picked to pay off shown in the pane, a list with nothing in it.
  await click('伏笔'); await click('第87章：停电'); await menu(5, '她往前走了一步。'); await click('记为回收…'); await click('井沿上的刻痕'); await win('第2章埋，第85章回收'); await pane('第 85 章已经回收过一处'); await shot('92-paying'); await key('Escape')
  await click('更多'); await click('已放弃'); await click('红色钥匙'); await click('恢复伏笔'); await click('全部伏笔'); await pane('这里还没有'); await click('全部伏笔'); await done('the foreshadowing picked')

  // The tree: a chapter dragged into a volume, and before another chapter.
  const pull = (from: string, to: string) => js(`(() => { const row = (k) => document.querySelector('[data-node="' + k + '"]'), d = new DataTransfer(); for (const [e, k] of [['dragstart', ${JSON.stringify(from)}], ['dragover', ${JSON.stringify(to)}], ['drop', ${JSON.stringify(to)}], ['dragend', ${JSON.stringify(from)}]]) row(k).dispatchEvent(new DragEvent(e, { bubbles: true, cancelable: true, dataTransfer: d })) })()`)
  await pull('87', 'v2'); must(await js(`document.querySelector('[data-node="87"]').classList.contains('deep') && document.querySelector('[data-node="87"]').previousElementSibling.dataset.node === '46'`), 'the chapter is not at the end of the volume')
  await pull('87', '46'); must(await js(`document.querySelector('[data-node="87"]').nextElementSibling.dataset.node === '46'`), 'the chapter is not before the other'); await shot('93-dragged'); await done('chapters dragged')

  // Settings: models not filled in, a local service, a variable found.
  await click('设置'); await clear('讨论模型'); await click('保存'); await win('讨论模型需要同时选择服务和填写模型 ID'); await click('关闭提示'); await type('讨论模型', 'claude-opus-5-5')
  await click('添加服务'); await win('ID（小写字母、数字、- _）'); await win('会发送到这个地址', false); await type('API 地址', 'http://localhost:11434/v1'); await win('本机地址：请确认这个服务不会再转发到云端'); await type('环境变量名称', 'DEEPSEEK_API_KEY'); await win('已检测到环境变量 DEEPSEEK_API_KEY。'); await shot('94-service-local')
  await click('少见的情况'); await click('读取模型目录没成功'); await win('读取模型目录失败'); await win('去设置'); await click('重试'); await win('读取模型目录失败', false); await win('正在读取可用模型…'); await sleep(1400); await click('关闭提示'); await click('关闭设置'); await done('settings that do not hold')

  // Backups that fail in the other two ways.
  await click('少见的情况'); await click('备份文件夹找不到了'); await click('备份没成功 · 找不到备份文件夹'); await win('找不到备份文件夹，这次没备份上'); await win('稿子没丢，文件夹可能被移走或改名。上次备份在今天 10:32。'); await key('Escape')
  await click('少见的情况'); await click('备份文件夹不让写'); await click('备份没成功 · 没有权限写入'); await win('备份文件夹不让写，这次没备份上'); await win('稿子没丢，这个文件夹不让写入。'); await shot('95-backup-denied'); await click('重试'); await sleep(1400); await win('已备份 · 刚刚'); await done('backups that fail otherwise')

  // The shelf: a work added from disk, a place taken, a name left empty.
  await click('切换作品'); await click('全部作品…'); await click('添加已有作品…'); await win('已加入书架'); await win('井边的旧稿'); await win('还没打开过'); await win('5 部'); await click('关闭提示'); await click('添加已有作品…'); await win('这部作品已经在书架上了'); await click('关闭提示')
  await click('少见的情况'); await click('添加的文件夹里没有作品'); await win('这个文件夹里没有作品（缺少 work.yaml）'); await click('关闭提示')
  await click('新建作品'); await type('作品名称', '雨城来信'); await click('创建'); await win('D:\\写作\\雨城来信 里已经有文件了换个名字或位置'); await shot('96-place-taken'); await click('关闭提示'); await clear('作品位置'); await click('创建'); await win('请选择作品放在哪里'); await click('关闭提示'); await click('取消')
  await click('更多操作'); await click('重命名…'); await clear('新作品名'); await click('保存'); await win('作品名不能为空'); await click('关闭提示'); await click('取消'); await click('重新开始'); await done('the shelf, where things go wrong')
}
