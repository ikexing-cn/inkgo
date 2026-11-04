// 扩展：goal。界面、动作和登记都在这里。
'use strict'

contribute('WordSuffix', 'goal', () => S.goalSet ? L.statusTodayGoal(BOOK.days[0][1], Number(S.goalSet)) : '', 0)

contribute('WordPopovers', 'goal', () => S.open === 'goal' ? goal() : '', 0)

contribute('State', 'goal', () => ({
  goal: '',
  goalSet: ''
}))

const goal = () => '<div class="pop goal-pop" data-k="goal-pop"><div class="pop-label">' + L.statusLast7() + '</div>' + BOOK.days.map(([day, n]) => `<div class="goal-day"><div class="grow">${timeShown(day)}</div><div>${(n ? '+' : '') + L.statusWords(n)}</div></div>`).join('') + HAIR +
    field(L.statusDailyGoal(), line(S.goal, (v) => set({ goal: v, ...(countOrNone(v) ? { goalSet: v.trim() } : {}) }), L.statusGoalHint(), L.statusGoalLabel())) + (countOrNone(S.goal) ? '<div class="note">' + L.statusGoalNote() + '</div>' : '<div class="note danger">' + L.statusGoalError() + '</div>') + '</div>'
