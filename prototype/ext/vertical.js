// 扩展：vertical。假装另装的，「少见的情况」里装上；和古籍版式都要替换「导出 docx」。
'use strict'

contribute('Injections', 'vertical', { where: () => L.modInjectDocx(), kind: 'overwrite', at: 'docx' }, 0)
