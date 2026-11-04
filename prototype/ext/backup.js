// 扩展：backup。界面、动作和登记都在这里。
'use strict'

// ───────── 备份 ─────────

// 备份为什么没成功：[短的, 标题, 怎么办]。why 空着的是找不到那块盘。
const backupFails = () => ({
  disk: [L.backupDiskShort(), L.backupDiskTitle(), L.backupDiskFix()],
  dir: [L.backupDirShort(), L.backupDirTitle(), L.backupDirFix()],
  deny: [L.backupDenyShort(), L.backupDenyTitle(), L.backupDenyFix()],
})
const backupWhy = () => backupFails()[S.backup.why || 'disk']

// 立即备份：还没选过地方，就先问放哪。
function backupNow() {
  if (S.backup.state === 'none') { set({ dialog: { kind: 'backup', place: PATHS().places[0][1] }, ...SHUT }); return }
  set((s) => ({ backup: { ...s.backup, state: 'busy' }, ...SHUT }))
  later(1200, () => set((s) => ({ backup: { ...s.backup, state: 'ok', why: '', at: stamp(), copies: s.backup.copies + 1 } })))
}
const backupSet = (change) => set((s) => ({ backup: { ...s.backup, ...change } }))
const backupNext = (b) => (b.close ? L.backupWhenClose() : b.hourly ? L.backupNextHour() : b.daily ? L.backupNextDay() : L.backupNextManual())
const backupTimes = (b, change) => tickbox(b.close, L.backupWhenClose(), () => change({ close: !b.close })) + tickbox(b.daily, L.backupTickDaily(), () => change({ daily: !b.daily })) + tickbox(b.hourly, L.backupTickHourly(), () => change({ hourly: !b.hourly }))

// 侧栏里那一行点开的：备份得怎么样，没成功的话怎么办。
function backupPop() {
  const b = S.backup
  if (b.state === 'none') return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + t(L.backupNone()) + '</div><div class="note">' + t(L.backupLocalOnly()) + '</div>' + `<div class="pop-buttons">${button(L.backupChoose(), backupNow, 'primary')}</div></div>`
  if (b.state === 'fail') {
    return '<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">' + t(backupWhy()[1]) + '</div>' + `<div class="note">${t(backupWhy()[2])}${b.at ? t(L.backupLastAt(timeShown(b.at))) : t(L.backupNoneYet())}</div>` +
      `<div class="pop-buttons"><div class="hit link"${onClick(() => set((s) => ({ backup: { ...s.backup, hush: true }, ...SHUT })))}>${t(L.backupHushToday())}</div>${button(L.backupChangePlace(), osAsk(L.dialogOsBackupFolder()))}${button(L.backupRetry(), backupNow, 'primary')}</div></div>`
  }
  return `<div class="pop backup-pop" data-k="backup-pop"><div class="pop-title">${t(L.backupDone(timeShown(b.at)))}</div><div class="note">${t(L.backupTo(b.dir))}<br>${t(L.backupNextAt(backupNext(b)))}</div>` +
    `<div class="pop-buttons">${button(L.backupSettings(), () => set({ dialog: { kind: 'settings', tab: '备份', edit: '' }, ...SHUT }))}${button(b.state === 'busy' ? L.backupBusy() : L.backupRunNow(), b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary')}</div></div>`
}

// 第一次备份：问放哪、备份时机。
function backupDialog(d) {
  const type = (change) => set((s) => ({ dialog: { ...s.dialog, ...change } }))
  const times = { close: d.close ?? true, daily: d.daily ?? true, hourly: d.hourly ?? false }
  const leave = () => set({ dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null, ...SHUT })
  const start = () => {
    set((s) => ({ backup: { ...s.backup, ...times, state: 'busy', dir: d.place, copies: 0 }, dialog: d.back ? { kind: 'settings', tab: '备份', edit: '' } : null }))
    later(1200, () => backupSet({ state: 'ok', at: stamp(), copies: 1 }))
  }
  return dialog('small', dialogHead(L.backupDialogTitle(), L.backupDialogAbout()) +
    `<div class="places">${PATHS().places.map(([label, dir]) => `<div class="place hit${d.place === dir ? ' on' : ''}"${onClick(() => type({ place: dir }))}><div class="ring${d.place === dir ? ' on' : ''}"></div><div class="fit"><div>${t(label)}</div><div class="note ell">${t(L.backupDetected(dir))}</div></div></div>`).join('')}` +
    `<div class="place hit"${onClick(osAsk(L.dialogOsBackupFolder()))}><div class="ring"></div><div class="fit"><div>${t(L.backupOtherFolder())}</div><div class="note">${t(L.backupOtherFolderNote())}</div></div></div></div>` +
    `<div class="field"><div class="label">${t(L.backupTiming())}</div><div class="ticks">${backupTimes(times, type)}</div></div>` +
    `<div class="buttons"><div class="hit link"${onClick(leave)}>${t(L.backupLater())}</div><div class="grow"></div>${button(L.backupStart(), d.place ? start : null, d.place ? 'primary' : 'primary off')}</div>`)
}

function settingsBackup() {
  const b = S.backup
  if (b.state === 'none') {
    return `<div class="backup-card"><div class="fit"><div class="group-title">${t(L.backupNone())}</div><div class="note">${t(L.backupLocalOnly())}</div></div>${button(L.backupChoose(), () => set({ dialog: { kind: 'backup', place: PATHS().places[0][1], back: true }, ...SHUT }), 'primary')}</div>`
  }
  const title = { ok: L.backupDone(timeShown(b.at)), busy: L.backupBusy(), fail: L.backupFailed() }[b.state]
  const row = (label, inner) => `<div class="prefs-row"><div class="prefs-label">${t(label)}</div>${inner}</div>`
  return `<div class="backup-card"><div class="fit"><div class="group-title">${t(title)}</div><div class="note">${b.state === 'fail' ? t(L.backupFailNote(backupWhy()[0], backupWhy()[2])) : t(L.backupSummary(wordsShown(S.shelf.find((x) => x.id === S.book).words), S.chapters.length, b.copies)) + (b.copies > 1 ? t(L.backupOldest()) : '')}</div></div>` +
    button(b.state === 'fail' ? L.backupRetry() : L.backupRunNow(), b.state === 'busy' ? null : backupNow, b.state === 'busy' ? 'primary off' : 'primary') + '</div>' +
    row(L.backupDestLabel(), `<div class="fit ell">${t(b.dir)}</div>${button(L.backupChange(), osAsk(L.dialogOsBackupFolder()))}`) +
    row(L.backupTiming(), `<div class="ticks">${backupTimes(b, backupSet)}</div>`) +
    row(L.backupKeep(), `<div class="keep">${choose('keep', b.keep, [['最近 10 份', L.backupKeep10()], ['最近 30 份', L.backupKeep30()], ['最近 90 份', L.backupKeep90()]], (keep) => backupSet({ keep }), L.backupKeep())}</div><div class="note">${t(L.backupSize(b.copies * 1.1))}</div>`)
}

const failBackup = (why) => () => set((s) => ({ ...SHUT, backup: { ...s.backup, state: 'fail', why, at: s.backup.at || '昨天 21:40', dir: s.backup.dir || PATHS().backup, hush: false } }))

// 底栏右边、字数前面：备份到哪一步了。好好的时候只三个字「已备份」，时间在悬停和点开里；点开还有下次什么时候，和「立即备份」。
function backupLine() {
  const b = S.backup, words = { ok: L.statusBackupOK(), busy: L.statusBackupBusy(), fail: L.statusBackupFailed(), none: L.statusBackupNone() }[b.state]
  return `<div class="anchor up backup"><div class="backup-line hit"${b.state === 'ok' ? ` title="${t(L.statusBackedUpAt(timeShown(b.at)))}"` : ''}${onClick(() => set({ open: S.open === 'backup' ? '' : 'backup' }))}><div class="dot${b.state === 'ok' || b.state === 'busy' ? ' grey' : ''}"></div><div>${t(words)}</div></div>${S.open === 'backup' ? backupPop() : ''}</div>`
}

contribute('SettingsSections', 'backup', () => inBook() ? `<div class="ext-prefs">${settingsBackup()}</div>` : '', 0)

contribute('PendingGroups', 'backup', () => {
  const list = []
  if (S.backup.state === 'fail') list.push({ id: 'backup', group: '备份', name: L.statusBackupWhy(backupWhy()[0]), about: L.statusRetryBackup(timeShown(S.backup.at)), key: '', open: backupNow })
  return list
}, 2)

contribute('Dialogs', 'backup', { kind: 'backup', draw: backupDialog }, 0)

contribute('ToolHints', 'backup', (cur, wide, here, tool, flip) => S.backup.state === 'fail' && !S.backup.hush ? `<div class="anchor"><button class="warn-btn" aria-label="${L.statusBackupTip()}"${onClick(flip('chip'))}>${DOT}<span>${L.statusBackupWarn(backupWhy()[0])}</span></button>${S.open === 'chip' ? backupPop() : ''}</div>` : '', 1)

contribute('State', 'backup', ({ kept }) => ({
  backup: { state: kept, at: kept === 'ok' ? '今天 10:32' : kept === 'fail' ? '昨天 21:40' : '', dir: kept === 'none' ? '' : PATHS().backup, copies: kept === 'none' ? 0 : 12, close: true, daily: true, hourly: false, keep: '最近 30 份', hush: false }
}))

contribute('FootStatus', 'backup', () => backupLine(), 0)

contribute('SettingsPages', 'backup', { name: '备份', order: 30 }, 30)
