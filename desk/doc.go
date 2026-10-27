// Package desk builds Inkgo's native interface: the window, its pages, and
// App, which holds them. Some parts are packages under it:
//
// desk/book: the book's content as the pages read it, the outline drafts, and timing.
// desk/kit: what the pages draw with: colors, sizes, controls, menus, status, toast cards, text, and Markdown.
// desk/phone: the phone preview panel.
// desk/exportdialog: the export dialog.
// desk/outline: the 大纲 page, card editing, chapter table, and timeline.
//
// The files of desk:
//
// ai.go: inline continuation, rewriting, drafting, and the prose AI record.
// aiswitch.go: the machine-wide AI switch and shutdown handling.
// app.go: App state, work lifecycle, the AI's background work, and errors.
// backup.go: backup tasks, policy wiring, folder selection, and settings.
// beside.go: right-pane width and tabs, discussion lists, and prose quotes.
// cache.go: values built once per key.
// categories.go: work-category forms and deletion confirmation.
// change.go: setting changes from a chapter, timelines, and forms.
// chapters.go: opening chapters, navigation, editors, and text size.
// chat.go: discussion drawing, streaming replies, tool views, and digest sections.
// check.go: check scheduling and persistence wiring, findings, prose marks, and the 检查 page.
// checkrunner.go: cancellable background checks and their result mailbox.
// commands.go: the one table of commands, for the keys, the menus, and the shortcut help.
// counts.go: book and daily word counts, goals, and recent days.
// doc.go: this package index.
// find.go: the prose find-and-replace bar.
// findbook.go: whole-book search results and replacement.
// foreshadow.go: live-mark wiring, foreshadowing column, popovers, actions, and reminders.
// host.go: appHost, what the pages of the packages under desk reach of App.
// ideas.go: setting ideas, questions, and changes awaiting the author.
// jobs.go: work off the view thread and what it hands back.
// keys.go: keyboard-shortcut help.
// markdown.go: Markdown views, setting links, and digest text.
// menu.go: command-table rows for the menus.
// names.go: setting-name completion, rename notices, and whole-book replacement.
// navigation.go: shared tab entries, availability, counts, and fallback choices.
// opened.go: a chapter opened in this run: its editor, what the disk has, and saving.
// pane.go: the search box atop right-pane lists, and chapter titles.
// pages.go: page entry, exit, drawing, and counts.
// pending.go: pending groups, ignored items, and navigation.
// prefs.go: applying and collecting per-work preferences.
// propose.go: discussion proposals in prose, accepting, dropping, and leaving.
// record.go: AI cost totals, request records, and request details.
// save.go: prose and 章纲 saving, disk changes, conflicts, and lost chapters.
// serviceform.go: service draft loading, model rows, validation, and clearing.
// services.go: AI services, models, keys, limits, and forms.
// setting.go: the right-pane 设定 list, entries, editing, and revisions.
// settings.go: the settings dialog, tabs, and appearance settings.
// shell.go: page types, window frame, sidebar navigation, and discussion switching.
// sidebar.go: chapter tree, menus, drag and drop, creation, and renaming.
// snapshots.go: snapshots, comparisons, and restoration.
// tabs.go: chapter tabs, keeping them across runs, closing, and save confirmation.
// take.go: taking a prose selection into the 设定.
// titlebar.go: title bar, its menus, and keyboard focus.
// toast.go: toast selection, errors, progress, notices, and their actions.
// toolbar.go: appearance switching and prose-selection tools.
// trash.go: chapter trash, restoration, and permanent deletion.
// welcome.go: 书架, work creation, opening, renaming, and relocation.
// write.go: writing-page state, frame, prose, 章纲, volumes, and footer.
package desk
