// 扩展：classic。假装另装的，「少见的情况」里装上；和竖排导出都要替换「导出 docx」，还要 Ctrl+L（讨论的引用已经在用）。
'use strict'

contribute('Injections', 'classic', { where: () => L.modInjectDocx(), kind: 'overwrite', at: 'docx' }, 0)

contribute('KeyClaims', 'classic', { key: 'Ctrl+L', over: 'talk' }, 0)
