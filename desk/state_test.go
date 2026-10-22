package desk

import (
	"time"

	"writer/editor"
	"writer/workspace"
)

// Tests read and set the app's state through these, so that a later move
// of a field changes one helper rather than every test.

// The book's content.
func chapters(a *App) workspace.Tree            { return a.write.tree }
func setChapterCounts(a *App, n map[string]int) { a.write.counts = n }
func countsView(n map[string]int) *writeView    { return &writeView{counts: n} }
func setting(a *App) workspace.Setting          { return a.set }
func outline(a *App) workspace.Plan             { return a.plan.data }
func forgetOutline(a *App)                      { a.plan.loaded = false }
func outlineErr(a *App) error                   { return a.plan.loadErr }
func treeRead(a *App) bool                      { return a.write.entered }

// The open chapter.
func shownEditor(a *App) *editor.Editor  { return a.write.ed }
func shownChapter(a *App) string         { return a.write.node }
func forgetChapter(a *App)               { a.write.node = "" }
func savedText(a *App) string            { return a.write.saved }
func savedVersion(a *App) int            { return a.write.savedVer }
func chapterOutline(a *App) string       { return a.write.outline }
func setChapterOutline(a *App, s string) { a.write.outline = s }
func diskChanged(a *App) bool            { return a.write.external != nil }
func chapterLost(a *App) bool            { return a.write.lost }
func openTabs(a *App) []string           { return a.write.tabs }
func markUnchecked(a *App)               { a.write.checked = time.Time{} }
