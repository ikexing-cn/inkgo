package hostapi

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
)

// DrawItem draws inline on the frame thread, through ext.Call.
type DrawItem struct{ Draw func(*ui.Context) }

// ChapterHeadItems draws beneath the writing page's chapter title in
// registration order; each Draw runs through ext.Call on the frame thread.
const ChapterHeadItems ext.PointKey[DrawItem] = "host.chapter-head"

// BeforeChapterChange runs synchronously through ext.Call before the
// displayed chapter or its tab changes.
const BeforeChapterChange ext.PointKey[func()] = "host.before-chapter-change"

// FrameTick runs once per frame with a book open, on the frame thread.
type FrameTick struct{ Tick func(*ui.Context) }

// PendingItem describes an actionable item; Open runs on the frame thread
// through ext.Guard.
type PendingItem struct {
	Key, Text, Sub string
	Here           bool
	Open           func(ui.Services)
}

// PendingGroup describes a group before the host filters ignored keys.
type PendingGroup struct {
	ID, Name string
	Color    ui.Color
	Items    []PendingItem
}

// PendingSource supplies a comparable cache key and current group on the frame
// thread through ext.Guard. Key must change whenever Group would return
// different content.
type PendingSource struct {
	Key   func() string
	Group func() (PendingGroup, bool)
}

// SettingsPage registers a persistent sidebar page, even while inactive or on
// the shelf. Label is its stable UI test label and shared-page identity. The
// smallest Order places the page among the host's (AI 10, reminders 20); it must
// be positive. Sections sort by Order then assembly order.
// Title supplies display metadata on the frame thread without session state.
type SettingsPage struct {
	Title func() string
	Label string
	Order int
}

// SettingsSection draws settings on the frame thread through ext.Call.
// Inactive extensions are omitted; Work also omits it on the shelf.
// Draw must not retain Context or frame elements.
type SettingsSection struct {
	Work bool
	Draw func(*ui.Context)
}

// Desktop exposes the writing page, footer animation, settings navigation and
// platform folder comparison on the frame thread.
type Desktop interface {
	// HasPage reports an available contributed page on the frame thread.
	HasPage(string) bool
	// Focused reports the layout focus state on the frame thread.
	Focused() bool
	// SetFocus changes layout focus on the frame thread.
	SetFocus(bool)
	// Selected returns the displayed node, whether it is lost, and whether saving waits for a close decision; frame thread only.
	Selected() (id string, lost, paused bool)
	// Writing reports whether the writing page is active.
	Writing() bool
	// SlideEdge applies the footer's current transition transform.
	SlideEdge(ui.Element)
	// OpenSettings opens the page registered by a qualified SettingsPages
	// contribution ID, such as backup.settings. Unknown IDs open Extensions.
	OpenSettings(id string)
	// ShowPage selects a registered page by qualified ID, such as cats.categories.
	// Call directly on the frame thread, without ext.Call or ext.Guard.
	ShowPage(id string)
	// OpenWriting selects the writing page and opens a chapter on the frame thread.
	OpenWriting(string)
	// MenuOpen queries the shared menu state directly on the frame thread,
	// without ext.Call or ext.Guard.
	MenuOpen(string) bool
	// DropMenu draws a shared menu on the frame thread; frame values must not escape.
	DropMenu(*ui.Context, ui.Element, string, func(ui.Element), func(*kit.Menu))
	// SettingColumn draws the shared right column on the frame thread.
	SettingColumn(*ui.Context)
	// SameFolder compares paths using the host platform's rules.
	SameFolder(string, string) bool
}

// Pending manages ignored items in the open book's desktop preferences.
type Pending interface {
	// ClearIgnored removes an extension's own key, on the frame thread.
	ClearIgnored(string)
}

// FooterItems collects inline footer draws in registration order; frame thread.
const FooterItems ext.PointKey[DrawItem] = "host.footer"

// TitleItems collects inline title bar draws in registration order; frame thread.
const TitleItems ext.PointKey[DrawItem] = "host.title"

// FrameTicks collects book frame callbacks in registration order; frame thread.
const FrameTicks ext.PointKey[FrameTick] = "host.ticks"

// BeforeLeave runs synchronously on the frame thread before sessions close.
const BeforeLeave ext.PointKey[func()] = "host.before-leave"

// PendingGroups collects pending sources queried on the frame thread.
const PendingGroups ext.PointKey[PendingSource] = "host.pending"

// SettingsPages collects persistent sidebar registrations, including inactive ones.
const SettingsPages ext.PointKey[SettingsPage] = "host.settings"

// SettingsSections collects active settings content, associated by module owner.
const SettingsSections ext.PointKey[SettingsSection] = "host.settings-sections"

// DesktopService resolves frame-thread presentation helpers.
const DesktopService ext.ServiceKey[Desktop] = "host.desktop"

// PendingService resolves frame-thread ignored-item management.
const PendingService ext.ServiceKey[Pending] = "host.pending-state"

// WindowActivity carries activity sampled before the frame.
type WindowActivity struct {
	// Active reports whether the native window is active in this frame.
	Active bool
	// Returned includes blur/focus pairs between consecutive frames.
	Returned bool
}

// WindowActivities invokes frame-thread listeners through ext.Call before drawing.
const WindowActivities ext.PointKey[func(WindowActivity)] = "host.window-activity"

// SaveEdits flushes extension drafts on the frame thread through ext.Call; errors keep close/save pending.
const SaveEdits ext.PointKey[func() error] = "host.save-edits"

// ChapterEdits participates in chapter tab close decisions on the frame thread.
type ChapterEdits struct {
	// Dirty queries unsaved state through ext.Guard.
	Dirty func(string) bool
	// Discard abandons unsaved text through ext.Guard.
	Discard func(string)
}

// ChapterEditStates collects chapter draft close participants.
const ChapterEditStates ext.PointKey[ChapterEdits] = "host.chapter-edits"

// ChapterOpened contains the node freshly read when a writing tab opens.
type ChapterOpened struct {
	// ID is the selected node ID, including folders.
	ID string
}

// ChapterOpens notifies frame-thread listeners through ext.Call after a node is read.
const ChapterOpens ext.PointKey[func(ChapterOpened)] = "host.chapter-open"

// CountSuffix supplies the inline count suffix and tooltip on the frame thread.
type CountSuffix struct {
	Text    func() string
	Tooltip func(string) string
}

// WordPopovers draws count popovers on the frame thread through ext.Call.
// The anchor is borrowed for this frame only and clicked reports its click.
const WordPopovers ext.PointKey[WordPopover] = "host.word-popovers"

// WordPopover draws against a borrowed count anchor on the frame thread.
type WordPopover struct {
	Draw func(*ui.Context, ui.Element, bool)
}

// WordSuffix collects suffixes on the frame thread through ext.Guard.
const WordSuffix ext.PointKey[CountSuffix] = "host.word-suffix"

// Tool draws a writing tool on the frame thread, ordered among host tools.
type Tool struct {
	Order int
	Draw  func(*ui.Context)
}

// Tools collects writing tools on the frame thread through ext.Call.
const Tools ext.PointKey[Tool] = "host.tools"
