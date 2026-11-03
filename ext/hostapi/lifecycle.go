package hostapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
)

// DrawItem draws inline on the frame thread, through ext.Call.
type DrawItem struct{ Draw func(*ui.Context) }

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

// SettingsPage contributes a custom page; all callbacks run on the frame thread.
// Label is a stable UI test label. Equal Order values keep registration order.
type SettingsPage struct {
	Title func() string
	Label string
	Order int
	Work  bool
	Draw  func(*ui.Context)
}

// Desktop exposes the writing page, footer animation, settings navigation and
// platform folder comparison on the frame thread.
type Desktop interface {
	// Writing reports whether the writing page is active.
	Writing() bool
	// SlideEdge applies the footer's current transition transform.
	SlideEdge(ui.Element)
	// OpenSettings opens a qualified extension settings page.
	OpenSettings(string)
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

// SettingsPages collects settings pages sorted by Order on the frame thread.
const SettingsPages ext.PointKey[SettingsPage] = "host.settings"

// DesktopService resolves frame-thread presentation helpers.
const DesktopService ext.ServiceKey[Desktop] = "host.desktop"

// PendingService resolves frame-thread ignored-item management.
const PendingService ext.ServiceKey[Pending] = "host.pending-state"
