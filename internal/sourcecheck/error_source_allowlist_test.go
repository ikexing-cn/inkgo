package sourcecheck

// bookLanguageExceptions is the sole allowlist. Fragments identify book-language
// prompts and recorded context descriptions without depending on line numbers.
var bookLanguageExceptions = map[string]struct {
	reason    string
	fragments []string
}{
	"ext/propose/tool.go":           {reason: "Moved proposal model prompt, schema and responses follow the book language."},
	"ext/ai/aiapi/proposal_text.go": {reason: "Shared proposal validation reports book-language tool errors."},

	"ext/draft/view.go":            {reason: "Model prompt follows the book language.", fragments: []string{"按章纲起草本章", "，约 %d 字", "，覆盖已有内容", "，接在已有内容之后"}},
	"desk/extensions.go":           {reason: "Stable reminder settings test identity.", fragments: []string{"设置-提醒"}},
	"desk/app.go":                  {reason: "Default genre is saved in the book.", fragments: []string{"网文"}},
	"desk/welcome.go":              {reason: "Default title and genre values are saved in books.", fragments: []string{"网文", "出版小说", "剧本", "非虚构", "未命名作品"}},
	"ext/check/checkitems.go":      {reason: "Persisted pending-check key.", fragments: []string{"检查/"}},
	"ext/hooks/module.go":          {reason: "Persisted pending-foreshadowing keys.", fragments: []string{"伏笔/失去锚点/", "伏笔/没回收/"}},
	"desk/save.go":                 {reason: "Recovered title becomes saved book content in saveLost.", fragments: []string{"找回的章节"}},
	"desk/sidebar.go":              {reason: "Default node names are book content.", fragments: []string{"新章节", "新文件夹"}},
	"ext/outline/plan_layout.go":   {reason: "A CJK glyph measured for the line height, never drawn.", fragments: []string{"字"}},
	"desk/toast.go":                {reason: "Test hook name of a toast action (kit.ToastAction.Name), never drawn.", fragments: []string{"撤销提示"}},
	"ext/backup/view.go":           {reason: "Persisted folder names and pending-backup identity.", fragments: []string{"写作备份", "我的坚果云", "备份/没成功"}},
	"ext/backup/archive/backup.go": {reason: "Fallback archive filename follows the book language.", fragments: []string{"作品"}},
	"ext/export/export.go":         {reason: "Exported file and folder names follow the book language.", fragments: []string{"大纲", "设定", "导出"}},
	"ext/export/format/docx.go":    {reason: "Exported document cover and fonts follow the book language.", fragments: []string{"作者：", "字数：", "宋体", "黑体"}},
	"ext/export/format/export.go":  {reason: "Exported document titles follow the book language.", fragments: []string{" · 大纲", " · 设定"}},
	"prompt/prompt.go":             {reason: "Model prompt truncation text follows the book language."},
	"prompt/setting.go":            {reason: "Model prompt lore context follows the book language."},
	"shelf/shelf.go":               {reason: "Persisted library directory name.", fragments: []string{"写作"}},
	"workspace/paths.go":           {reason: "Default saved filename follows the book language.", fragments: []string{"未命名"}},
	"workspace/numbering.go":       {reason: "Chapter numbering follows the book language."},
	"workspace/types.go":           {reason: "Default saved categories, genres and quoted model context follow the book language."},

	"writing/check.go":           {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"你在帮一位作者核对小说的一章和设定是否一致。", "｜已放弃", "【设定】\n", "\n\n【分类】", "\n\n【作者说过是故意的】", "\n\n【章名】", "\n\n【正文】", "（正文太长，只给了前面一部分）"}},
	"writing/rules.go":           {reason: "Model prompts and tool descriptions follow the book language."},
	"writing/writer.go":          {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"正文", "设定正文（作者定下的，必须遵守）：", "已放弃", "已放弃（不能出现在正文里）：", "\n\n以上是写到本章时的设定；后面的章节里还有 %d 处变化，未列出。", "【所在卷：", "】卷纲：", "【本章：", "】章纲：", "【本章前文】", "【本章后文】", "【上一章《", "》的结尾】", "【要求】请接着本章前文写下一句。"}},
	"ext/talk/engine/beside.go":  {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"【作者正在看的卷：", "所在卷：", "【卷纲】", "卷纲：", "【所在卷：", "】卷纲：", "【作者正在写的章：", "所在章：", "【章纲】", "章纲：", "【光标前的正文】\n", "本章光标前 %d %s", "字", "词", "【作者说】\n"}},
	"ext/talk/engine/context.go": {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"设定正文（作者定下的，必须遵守）：", "已放弃", "已放弃（不要再推荐，换个说法也不行）：", "悬而未决", "悬而未决（作者还没表态，可以讨论）：", "设定较多，这里只列目录：每条的编号、分类、名字和第一句。", "之前的讨论已经整理成下面的摘要。原话还在，要用时用 search_talks 查。", "」起：", "」起（悬而未决）：", "\n\n以上是写到「%s」时的设定；后面的章节里还有 %d 处变化，没有列出。", "\n\n【这一轮在正文里的提议】", "换掉「", "接在「", "」之后"}},
	"ext/talk/engine/engine.go":  {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"\n\n（已达到本次查阅的上限，不能再查了，请用已经查到的内容回答。）", "（这次查到的内容太多，放不下了；不能再查了，请用已经查到的内容回答。）"}},
	"ext/talk/engine/fixed.go":   {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"起草", "改写", "【这段讨论", "的章：", "\n章纲：\n", "章纲：", "\n（这一章没有章纲）", "【所在卷：", "】卷纲：\n", "卷纲：", "【本章的卡片】\n", "本章的卡片 %d 张", "【本章全文】\n", "本章全文 %d %s", "【前面各卷的卷纲】\n", "前面各卷的卷纲 %d 卷", "【本卷前面各章的章纲】\n", "本卷前面各章的章纲 %d 章", "【上一章全文：", "上一章全文：%s %d %s", "【本章已有的正文】\n", "本章已有正文 %d %s"}},

	"ext/talk/engine/prompts.go":  {reason: "Model prompts and tool descriptions follow the book language."},
	"ext/talk/engine/propose.go":  {reason: "Book-language prompt, tool response or saved context identity.", fragments: []string{"换掉一段", "接在一句之后", "接在章末", "换掉全章", "读不出章节：%v", "text 要写新的正文", "已在「", "」的正文里显示，等作者决定；作者采纳或放弃了，下次说话时会告诉你。", "提议修改「", "at 只能是 replace、after、end、all", "」是一卷，没有正文；提议改它下面的章。", " 要在 quote 里逐字复制「", "」里的原文", "读不出「%s」：%v", "」里找不到 quote 这段原文；要逐字复制，可以先用 read_prose 读一下。", "quote 这段原文在「%s」里出现了 %d 次；多引几个字，让它只出现一次。", "已采纳", "被作者放弃", "作者改动了它要换掉的原文，已作废", "作者关闭作品时丢弃了", "作者关闭 AI 时丢弃了", "换掉「", "接在「", "」之后", "的提议："}},
	"ext/talk/engine/tools.go":    {reason: "Model prompts and tool descriptions follow the book language."},
	"ext/lore/note.go":            {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{`"entry":{"type":"integer","minimum":1,"description":"edit、move、name、change 改的条目的编号，即设定里方括号中的数字"},`, `"chapter":{"type":"string","description":"change 从哪一章（或哪一卷）起，章名和搜全书结果里写的一样"},`, `"place":{"type":"string","enum":["body","open","dropped"],"description":"body 是设定正文，open 是悬而未决，dropped 是已放弃"},`, `"text":{"type":"string","description":"一句能独立成立的设定；change 写从那一章起这一条的全文"},`, `"names":{"type":"array","items":{"type":"string"},"description":"条目说的专有名词，第一个是正式名称"},`, `"sources":{"type":"array","items":{"type":"string"},"description":"这条出自的原话，逐字复制自本次讨论里已经说过的话"},`, `"author_said":{"type":"string","description":"作者拍板、接受、拒绝或改口的原话，逐字复制"}`, `"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`, `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`, "正文", "待确认", "已删除", "改写「", "」为：", "名称改为：", "移到", "改归", "」起：", "读不出设定：%v", "读不出这次讨论：%v", "这次讨论里已经说过的话", "作者在这次讨论里说过的话", "%s「%s」不是%s里逐字的原文；你这次回复里的话还不能引用", "sources 里的", "要在 author_said 里引作者拍板、接受、拒绝或改口的原话", "没有「%s」这个分类", "没有 %q 这个位置", "add 不带 entry", "add 要有 place、category、text 和 sources", "设定里没有编号 %d 的条目", "edit 要有改好的 text，不带 place、category（换位置或分类用 move）", "move 要有 place 或 category，不带 text、sources（改写用 edit）", "name 要有 names，不带 text、place、category、sources", "name 只在原有名字后面加别名，原有名字 %s 要原样排在前面", "change 只给设定正文里的条目记变化", "change 要有 chapter、place（body 或 open）、text 和 sources，不带 category", "读不出章节：%v", "change 要写从哪一章起（chapter）", "没有 %q 这个操作", " 不带 names", "只有%s分类的条目带 names", "名字「%s」没有在条目或对话里出现过", "名字「%s」已经在编号 %d 的条目里，一个名字只放在一条里", "没有记下：%s。", "没有记下：%v", "已记下，编号 %d：%s", "记下：", "设定正文", "悬而未决", "已放弃", "query 不能为空", "这一次的讨论", "查了设定「", "讨论「", "正文「", "  出处：「", "设定里没有「", "\n还有 %d 条没有列出，换一个更具体的词再查。", "参数不是约定的 JSON：%v", "look_up_setting 查设定的全文和出处", "要看全文和出处，用 look_up_setting 查。", "按名字或词查设定，返回条目的编号、位置、分类、名字、全文和出处。", "把这次讨论里定下的东西记进设定文档，一次一个操作：add 新增，edit 改写，move 换位置或分类，name 补名字，change 记一条从某一章起的变化。规则见系统提示里的「记设定」。", "新对话"}},
	"ext/outline/tools.go":        {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{`"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`, `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`, `{"type":"object","properties":{"line":{"type":"string","description":"情节线名"},"chapter":{"type":"string","description":"章名或卷名，和搜全书结果里写的一样"},`, "大纲文件读不出来，暂时不能查阅。", "读不出章节，暂时不能查阅大纲。", "没有叫「%s」的情节线；大纲还没有情节线。", "没有叫「%s」的情节线；现有的情节线：%s。", "情节线「", "这条线没有卡片。", "读不出「%s」的纲。", "章纲", "卷纲", "（没有", "这一章没有卡片。", "\n这一卷没有卡片。", "没有含「", "」这个词的卡片。", "大纲是空的。", "大纲共 %d 张卡片，按故事里发生的先后排列。", "情节线「%s」：%d 张。", "没有情节线：%d 张。未放进章节：%d 张。", "查了大纲「", "」这条线", "」的卡片", "查了大纲", "后面没有卡片了。", "这里列的是第 %d–%d 张，共 %d 张。", "要看后面的，from 填 %d 再查一次。", "参数不是约定的 JSON：%v", "read_outline 查大纲的卡片和情节线", "读作者在大纲里排的卡片和情节线，卡片按故事里发生的先后排，可能和章节先后不同。line 按情节线名查，chapter 按章名或卷名查并附对应的纲，query 按一个词搜发生了什么、说明和故事中的时间；都不给则先给概览再列全部卡片。多个筛选参数按 line > chapter > query 只用第一个。一次最多 200 张，from 跳过前面多少张接着查。查卷时按卷里各章的先后分组，各章内仍按故事先后；#序号始终是全书故事里的位置。"}},
	"ext/ai/aiapi/snapshot.go":    {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{"工具调用失败：%v"}},
	"talkbook/setting.go":         {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{"」起：", "」起（悬而未决）："}},
	"talkbook/cards.go":           {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{"时间：", "发生：", "未放进章节", "情节线：", "说明："}},
	"talkbook/book.go":            {reason: "Tool output follows the book language; moved with its owner.", fragments: []string{"」对得上好几章：", "；写全章名。", "没有叫「", "」的章；相近的有", "」的章；章名要和搜全书的结果里写的一样。"}},
	"ext/lore/tools_prompt.go":    {reason: "Model prompts follow the book language."},
	"ext/outline/tools_prompt.go": {reason: "Model prompts follow the book language."},
}
