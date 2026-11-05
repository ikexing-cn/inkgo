// Package desk builds Inkgo's native interface: the window, its pages, and
// App, which holds them. Some parts are packages under it:
//
// desk/book: the book's content as the pages read it, the outline drafts, and timing.
// desk/kit: what the pages draw with: colors, sizes, controls, menus, status, toast cards, text, and Markdown.
// ext/phone: the phone preview panel.
// ext/export: the export extension.
// ext/lore: setting entries, changes, timelines, and forms.
// ext/outline: the 大纲 page, card editing, chapter table, and timeline.
//
// The files of desk:
//
// ai.go: inline continuation, rewriting, drafting, and the prose AI record.
// aiswitch.go: the machine-wide AI switch and shutdown handling.
// app.go: App state, work lifecycle, content refresh, and errors.
// extensions_lifecycle.go: extension status, settings, pending groups and lifecycle.
// beside.go: right-pane width and tabs, discussion lists, and prose quotes.
// cache.go: values built once per key.
// categories.go: work-category forms and deletion confirmation.
// chapters.go: opening chapters, navigation, editors, and text size.
// chat.go: discussion page, session list and digest sections.
// chatmessage.go: messages, streaming replies, tool results and request details.
// chatcompose.go: composer, discussion model menu and extracting text into settings.
// check.go: check state, scheduling and persistence wiring.
// checkitems.go: findings, pending groups, prose marks and chapter check status.
// checkview.go: the 检查 page, finding details and actions.
// checkrunner.go: cancellable background checks and their result mailbox.
// commands.go: the one table of commands, for the keys, the menus, and the shortcut help.
// counts.go: book and daily word counts, goals, and recent days.
// doc.go: this package index.
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
// record.go: AI usage snapshot, list state, cost totals, and request details.
// save.go: prose and 章纲 saving, disk changes, conflicts, and lost chapters.
// serviceform.go: service draft loading, model rows, validation, and clearing.
// services.go: AI services, models, keys, limits, and forms.
// setting.go: the right-pane 设定 list, entries, editing, and revisions.
// settings.go: settings state and lifecycle, dialog, tabs, and appearance settings.
// shell.go: page types, window frame, sidebar navigation, and discussion switching.
// sidebar.go: chapter tree, menus, drag and drop, creation, and renaming.
// tabs.go: chapter tabs, keeping them across runs, closing, and save confirmation.
// take.go: taking a prose selection into the 设定.
// talkrun.go: a discussion turn's state, stream, requests, cancellation and completion.
// titlebar.go: title bar, its menus, and keyboard focus.
// toast.go: toast selection, errors, progress, notices, and their actions.
// toolbar.go: appearance switching and prose-selection tools.
// trash.go: tree deletion into the core trash.
// version_points.go: writing presentation hooks for extensions.
// version_services.go: editor restoration and book storage coordination.
// welcome.go: 书架, work creation, opening, renaming, and relocation.
// write.go: writing-page state, frame, prose, 章纲, volumes, and footer.
package desk
