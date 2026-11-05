package hostapi

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/workspace"
)

// DrawItem draws inline on the frame thread, through ext.Call.
type DrawItem struct {
	// Placement selects a host slot; FooterItems accepts "prose" before its spacer.
	Placement string
	Draw      func(*ui.Context)
}

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
// the shelf. Pages with the same nonempty Key merge; the lowest Order wins the
// title, label and position, with registration order breaking ties. An empty Key
// keeps the page separate under its contribution ID; a nonempty Key is also the
// shared page's navigation ID. Order must be positive.
// Label is the stable accessibility/test label. Title supplies display metadata
// on the frame thread without session state. Sections sort by page order then
// registration order.
type SettingsPage struct {
	Key string
	// Title is localization only, like Module.Name; safe while disabled,
	// without accessing session state or services.
	Title func() string
	Label string
	Order int
}

// SettingsSection draws settings on the frame thread through ext.Call.
// Inactive extensions are omitted; Work also omits it on the shelf.
// Draw must not retain Context or frame elements.
type SettingsSection struct {
	// Placement optionally embeds a section in a named settings slot. Draw runs
	// on the frame thread through ext.Call at that slot and in module details.
	Placement string
	Work      bool
	Draw      func(*ui.Context)
}

// Navigation groups related frame-thread host capabilities.
type Navigation interface {
	// OpenExtension opens Settings > Extensions at the registered extension details.
	// Unknown IDs open the overview. Call directly on the frame thread.
	OpenExtension(id string)
	// Page returns the current qualified page identity on the frame thread.
	Page() string

	// EntryAvailable reports whether the host can navigate to an entry; frame thread only.
	EntryAvailable() bool

	// ShowEntry navigates to an entry using the installed entry provider; frame thread only.
	ShowEntry(workspace.Entry)

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

	// OpenSettings opens the page registered by a qualified SettingsPages
	// contribution ID, such as backup.settings. Unknown IDs open Extensions.
	OpenSettings(id string)

	// ShowPage selects a registered page by qualified ID, such as cats.categories.
	// Call directly on the frame thread, without ext.Call or ext.Guard.
	ShowPage(id string)

	// OpenWriting selects the writing page and opens a chapter on the frame thread.
	OpenWriting(string)
}

// NativeUI groups related frame-thread host capabilities.
type NativeUI interface {
	// ModelLabel formats historical model metadata through the owning API on the frame thread.
	ModelLabel(workspace.AIRecord) string
	// SlideEdge applies the footer's current transition transform.
	SlideEdge(ui.Element)

	// MenuOpen queries the shared menu state directly on the frame thread,
	// without ext.Call or ext.Guard.
	MenuOpen(string) bool

	// MenuState returns persistent shared menu state on the frame thread.
	MenuState() *kit.MenuState

	// DropMenu draws a shared menu on the frame thread; frame values must not escape.
	DropMenu(*ui.Context, ui.Element, string, func(ui.Element), func(*kit.Menu))

	// SettingColumn draws the shared right column on the frame thread.
	SettingColumn(*ui.Context)
}

// MachinePaths groups related frame-thread host capabilities.
type MachinePaths interface {
	// Dir returns the current work directory; capture it before starting a worker.
	Dir() string
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

// NavigationService resolves page and chapter navigation.
const NavigationService ext.ServiceKey[Navigation] = "host.navigation"

// NativeUIService resolves menus, animation and pane drawing.
const NativeUIService ext.ServiceKey[NativeUI] = "host.native-ui"

// MachinePathsService resolves platform path comparison.
const MachinePathsService ext.ServiceKey[MachinePaths] = "host.machine-paths"

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

// SettingsDetail replaces a settings page body while its detail is selected.
// All callbacks run on the frame thread through the host failure boundary.
type SettingsDetail struct {
	// Enter observes settings navigation on the frame thread.
	Enter       func(string)
	Matches     func(string) bool
	Title, Note func() string
	Draw        func(*ui.Context)
	Close       func()
}

// SettingsDetails collects nested settings pages, such as AI records.
const SettingsDetails ext.PointKey[SettingsDetail] = "host.settings-details"
