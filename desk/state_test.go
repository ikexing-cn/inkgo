package desk

import (
	"time"

	"writer/editor"
	"writer/workspace"
)

// Tests read and set the app's state through these, so that a later move
// of a field changes one helper rather than every test.

// The book's content.
func chapters(a *App) workspace.Tree            { return a.book.Tree() }
func setChapterCounts(a *App, n map[string]int) { a.book.SetCounts(n) }
func setting(a *App) workspace.Setting          { return a.book.Setting() }
func rereadBook(a *App)                         { a.warn(a.book.ReloadAll()) }

// The open chapter.
func shownEditor(a *App) *editor.Editor  { return a.write.cur().ed }
func shownChapter(a *App) string         { return a.write.node }
func forgetChapter(a *App)               { a.write.node = "" }
func savedText(a *App) string            { return a.write.cur().saved }
func savedVersion(a *App) int            { return a.write.cur().savedVer }
func chapterOutline(a *App) string       { return a.write.cur().outline.Text }
func setChapterOutline(a *App, s string) { a.write.cur().outline.Text = s }
func diskChanged(a *App) bool            { return a.write.cur().external != nil }
func chapterLost(a *App) bool            { return a.write.cur().lost }
func openTabs(a *App) []string           { return a.write.tabs }
func markUnchecked(a *App)               { a.write.checked = time.Time{} }

// The setting panel.
func shownEntry(a *App) string     { return a.doc.route.entry() }
func showEntry(a *App, id string)  { a.doc.route.openEntry(id) }
func historyShown(a *App) bool     { return a.doc.route.showingHistory() }
func editingEntry(a *App) string   { return a.doc.route.editID() }
func editEntry(a *App, id string)  { a.doc.route.editEntry(id) }
func editChange(a *App, id string) { a.doc.route.editChange(id) }
func todoShown(a *App) bool        { return a.doc.route.showingWaiting() }

func showHistory(a *App, on bool) {
	if on {
		a.doc.route.showHistory()
	} else {
		a.doc.route.closeHistory()
	}
}

func showTodo(a *App, on bool) {
	if on {
		a.doc.route.showWaiting()
	} else {
		a.doc.route.closeWaiting()
	}
}

// Status.
func appError(a *App) error      { return a.status.Err() }
func noticeOf(a *App) string     { return a.status.NoticeText() }
func setNotice(a *App, s string) { a.status.Did(s) }
func hintOf(a *App) string       { return a.status.HintText() }
func setHint(a *App, s string)   { a.status.Hint(s) }

// The page and the tab of the right column.
func shownPage(a *App) page     { return a.page }
func showPage(a *App, p page)   { a.page = p }
func shownTab(a *App) paneTab   { return a.doc.tab }
func showTab(a *App, t paneTab) { a.doc.tab = t }
