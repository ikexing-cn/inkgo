// Package desk builds Inkgo's native interface.
// Shared drawing controls, colors, status, and text helpers live in desk/kit.
//
// ai.go: inline continuation, rewriting, drafting, and the prose AI record.
// aiswitch.go: the machine-wide AI switch and shutdown handling.
// app.go: App state, work lifecycle, background discussion jobs, and errors.
// backup.go: backup tasks, policy wiring, folder selection, and settings.
// beside.go: right-pane width and tabs, discussion lists, and prose quotes.
// categories.go: work-category forms and deletion confirmation.
// change.go: setting changes from a chapter, timelines, and forms.
// chapters.go: opening chapters, navigation, editors, and text size.
// chat.go: discussion drawing, streaming replies, tool views, and digest sections.
// check.go: check scheduling and persistence wiring, findings, prose marks, and the 检查 page.
// checkrunner.go: cancellable background checks and their result mailbox.
// counts.go: book and daily word counts, goals, and recent days.
// doc.go: this package index.
// export.go: export dialog choices, cached plan, and file writing.
// find.go: the prose find-and-replace bar.
// findbook.go: whole-book search results and replacement.
// foreshadow.go: live-mark wiring, foreshadowing column, popovers, actions, and reminders.
// ideas.go: setting ideas, questions, and changes awaiting the author.
// keys.go: keyboard-shortcut help.
// markdown.go: Markdown styles, code blocks, and setting links.
// menu.go: command-table rows for the menus.
// names.go: setting-name completion, rename notices, and whole-book replacement.
// navigation.go: shared tab entries, availability, counts, and fallback choices.
// pane.go: shared right-pane lists, details, forms, and quotes.
// pages.go: page entry, exit, drawing, and counts.
// pending.go: pending groups, ignored items, and navigation.
// phone.go: phone preview panel, settings, painting, and selection.
// plan.go: the 大纲 page, card editing, chapter table, and timeline.
// plan_layout.go: outline-table geometry, scrolling, and drag and drop.
// prefs.go: applying and collecting per-work preferences.
// propose.go: discussion proposals in prose, accepting, dropping, and leaving.
// record.go: AI cost totals, request records, and request details.
// save.go: prose and 章纲 saving, disk changes, conflicts, and lost chapters.
// services.go: AI services, models, keys, limits, and forms.
// setting.go: the right-pane 设定 list, entries, editing, and revisions.
// settings.go: the settings dialog, tabs, and appearance settings.
// shell.go: page types, window frame, sidebar navigation, and discussion switching.
// sidebar.go: chapter tree, menus, drag and drop, creation, and renaming.
// snapshots.go: snapshots, comparisons, and restoration.
// tabs.go: chapter tabs, editor parking, closing, and save confirmation.
// take.go: taking a prose selection into the 设定.
// titlebar.go: title bar, menu commands, and keyboard focus.
// toast.go: errors, progress, notices, and their actions.
// toolbar.go: appearance switching and prose-selection tools.
// trash.go: chapter trash, restoration, and permanent deletion.
// welcome.go: 书架, work creation, opening, renaming, and relocation.
// write.go: writing-page state, frame, prose, 章纲, volumes, and footer.
package desk
