// Package desk builds Inkgo's native interface: the window, its pages, and
// App, which holds them. Shared support lives under desk; feature pages
// and AI workflows live in extensions under ext/:
//
// desk/book: the book's content as the pages read it, the outline drafts, and timing.
// desk/kit: what the pages draw with: colors, sizes, controls, menus, status, toast cards, text, and Markdown.
// ext/phone: the phone preview panel.
// ext/export: the export extension.
// ext/lore: setting entries, changes, timelines, and forms.
// ext/talk: discussions, messages, streaming replies, and the composer.
// ext/check: background checks, findings, and the 检查 page.
// ext/continue, ext/draft, ext/propose: prose continuation, drafts, and proposals.
// ext/usage: AI usage records and request details.
// ext/outline: the 大纲 page, card editing, chapter table, and timeline.
//
// The files of desk:
//
// ai.go: recorded AI model and effort labels.
// ai_settings.go: extension-host busy and blocking-operation queries.
// aiswitch.go: the machine-wide AI switch and shutdown handling.
// app.go: App state, work lifecycle, content refresh, and errors.
// authoring.go: contributed prose-selection menus and beside-page content.
// authoring_services.go: extension-host author operations, pane navigation, and preferences.
// beside.go: right-pane width, resizing, tabs, and finding quoted chapter text.
// cache.go: values built once per key.
// categories.go: work-category names and the default category for new entries.
// chapters.go: opening chapters, navigation, editors, marks, and text size.
// commands.go: the command table, shortcut dispatch, menu labels, and shortcut help rows.
// context.go: optional chapter outlines and setting entries supplied by extensions.
// counts.go: live chapter, selection, folder, and book word counts and footer contributions.
// entry_navigation.go: optional setting-entry navigation through the lore extension.
// errtext.go: localized error formatting and contributed error translators.
// extension_pages.go: contributed pages, window activity, chapter events, and edit coordination.
// extension_store.go: extension settings, switches, injection preferences, and shelf-data migration.
// extensions.go: extension-host setup, capabilities, contribution tables, commands, panes, and overlays.
// extensions_lifecycle.go: extension lifecycle, frame hooks, settings, pending groups, and writing tools.
// host.go: appHost access to book content, editors, dialogs, navigation, and shared UI.
// ideas.go: cached setting ideas awaiting the author.
// jobs.go: background jobs, frame-thread callbacks, and their mailbox.
// keys.go: keyboard-shortcut help.
// leave.go: extension questions before leaving undecided author work.
// menu.go: command-table rows for the menus.
// motion.go: sidebar and right-pane folding animations and prose movement.
// names.go: contributed name candidates and links fed to the editor.
// navigation.go: shared tab entries, availability, counts, and fallback choices.
// opened.go: an open chapter's editor, disk state, saving, snapshots, and conflicts.
// pages.go: page entry, exit, drawing, and count callbacks.
// pane.go: the search box atop right-pane lists, and chapter titles.
// pending.go: pending groups, chapter marks, ignored items, and navigation.
// prediction.go: extension-provided prose prediction and editor predictor updates.
// prefs.go: applying and collecting per-work preferences.
// prose_marks.go: contributed prose marks and editor mark updates.
// prose_ui.go: extension-host prose bounds, pane state, chapter text, tabs, and layout helpers.
// revisions.go: author setting revisions and content refresh.
// save.go: prose and contributed-edit saving, disk changes, conflicts, and lost chapters.
// setting.go: right-pane state, tab identities, and sidebar and pane visibility.
// settings.go: settings state, dialog, tabs, appearance, language, and relaunch handling.
// settings_declared.go: controls for declarative extension settings.
// settings_extensions.go: extension overview, detail pages, switches, and confirmation dialogs.
// shell.go: page identities, frame lifecycle, window layout, sidebar shell, and window titles.
// sidebar.go: chapter tree, menus, drag and drop, creation, and renaming.
// system_menu.go: native-menu snapshots, shortcuts, and queued command dispatch.
// tabs.go: chapter tabs, keeping them across runs, closing, and save confirmation.
// take.go: taking a prose selection into the 设定.
// titlebar.go: title bar, its menus, page switch, and keyboard focus.
// toast.go: toast selection, errors, progress, notices, and their actions.
// toolbar.go: appearance switching and prose-selection tools.
// trash.go: chapter-tree deletion into the book's trash.
// undo.go: undoable setting decisions and coordinated chapter-check restoration.
// version_points.go: writing keyboard, escape, selection, and pre-prose hooks for extensions.
// version_services.go: extension-host snapshot and trash restoration, model labels, and book text replacement.
// welcome.go: 书架, work creation, opening, renaming, and relocation.
// work.go: background requests, retries, cancellation, completion, and blocking state.
// write.go: writing-page state, frame, prose, folders, right pane, tools, and footer.
// zoom.go: machine-wide interface scale, display limits, and scale changes.
package desk
