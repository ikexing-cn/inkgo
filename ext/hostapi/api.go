// Package hostapi contains stateless contracts between desk and extensions.
package hostapi

import (
	"github.com/egoist/mygo/ui"
	"time"

	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/workspace"
)

// GroupWrite identifies writing extensions in the host overview.
const GroupWrite = "write"

// GroupLore identifies reference and planning extensions in the host overview.
const GroupLore = "lore"

// GroupAI identifies AI extensions in the host overview.
const GroupAI = "ai"

// GroupFiles identifies work and file extensions in the host overview.
const GroupFiles = "files"

// Menu names a host menu; read by desk on the frame thread.
type Menu string

// MenuFile is the file menu in both title and system menus; frame thread only.
const MenuFile Menu = "file"

// MenuView is the view menu in both title and system menus.
const MenuView Menu = "view"

// MenuPosition places a command after a host command ID, such as focus.
// Empty After appends. Desk reads it on the frame thread.
type MenuPosition struct {
	Menu  Menu
	After string
}

// Command is invoked by desk on the frame thread. Name may reflect live state;
// nil Can means always enabled. Run is dispatched through ext.Call.
// Mods and MacMods are native UI modifiers; OnMac selects the macOS override.
type Command struct {
	// HelpGroup and HelpText optionally customize the shortcut row on the frame thread.
	HelpGroup, HelpText func() string
	// HelpAfter anchors the row after a host shortcut row's description on the frame thread.
	HelpAfter func() string
	Name      func() string
	Can       func() bool
	Run       func()
	Menu      MenuPosition
	Key       ui.Key
	Mods      ui.Modifiers
	MacKey    ui.Key
	MacMods   ui.Modifiers
	OnMac     bool
}

// PaneTab is a right pane contributed by an extension. Desk calls Title and
// Available on the frame thread; nil Available means visible. Draw runs through
// ext.Call in the frame and must not retain Context or frame elements.
type PaneTab struct {
	// Key optionally preserves an existing desktop preference identity. Empty uses the contribution ID.
	Key string
	// Before places this tab before a qualified pane ID; empty appends.
	Before string
	// Fallback makes this tab the default when no chosen pane is available.
	Fallback bool
	// Count queries the badge on the frame thread through ext.Guard.
	Count     func() int
	Title     func() string
	Draw      func(*ui.Context)
	Available func() bool
}

// Page is a title-bar page contributed by an extension. Metadata and
// availability are read on the frame thread; Draw uses ext.Call.
type Page struct {
	// MenuOnly excludes the page from the title switch on the frame thread.
	MenuOnly bool
	// Title supplies display text on the frame thread through ext.Guard.
	Title func() string
	// Icon is drawn by the host on the frame thread.
	Icon *ui.SVG
	// Order sorts this page among host pages.
	Order int
	// Label is a stable UI test label.
	Label string
	// Enter and Leave run through ext.Guard on the frame thread.
	Enter func()
	Leave func()
	// Draw runs through ext.Call on the frame thread.
	Draw func(*ui.Context)
	// Available is queried through ext.Guard on the frame thread.
	Available func() bool
	// HasPane reports whether the page has a right column, through ext.Guard on the frame thread.
	HasPane func() bool
	// PaneOpen reports the page's right-column state, through ext.Guard on the frame thread.
	PaneOpen func() bool
	// TogglePane changes that state, through ext.Guard on the frame thread.
	TogglePane func()
}

// Book exposes current book content to extensions on the frame thread only.
type Book interface {
	// Held reports a blocking book operation on the frame thread.
	Held() bool
	// CategoriesChanged reloads derived setting data and notifies the open book on the frame thread.
	CategoriesChanged() error
	// Flashed returns when book content last received a recent-change notice.
	// Call directly on the frame thread, without ext.Call or ext.Guard.
	Flashed(string) (time.Time, bool)
	// ClearFlash removes that content notice. Call directly on the frame thread,
	// without ext.Call or ext.Guard; this does not change persisted book data.
	ClearFlash(string)
	// Store returns the current store, or nil on the shelf; frame thread only.
	Store() *workspace.Store
	// Content returns borrowed UI content, or nil on the shelf; frame thread only.
	Content() *book.Book
	// Flush saves the displayed chapter; frame thread only.
	Flush()
	// Chapter returns the selected editor; id is empty off the writing page.
	// open requires an existing chapter, rather than a folder.
	Chapter() (id string, ed *editor.Editor, open bool)
	// ChapterTitle returns a chapter title, false for a missing node or folder.
	ChapterTitle(id string) (string, bool)
	// Dir returns the current work folder, or empty on the shelf.
	Dir() string
}

// Palette provides desktop and preview colors on the frame thread only.
type Palette interface {
	// Colors returns the current desktop palette.
	Colors() *kit.Palette
	// ScreenColors returns light and dark palettes for a reading screen.
	ScreenColors() (*kit.Palette, *kit.Palette)
}

// Pane is called by extensions on the frame thread only.
type Pane interface {
	// Show opens the right pane, selects a qualified tab ID (or its preserved
	// preference Key), exits focus and saves the choice. Selecting a non-talk
	// tab closes the conversation list. Call directly on the frame thread,
	// without ext.Call or ext.Guard.
	Show(tab string)
	// RevealPane selects a tab and unfolds the current page's column, including
	// a contributed page's column. It preserves focus and does not save the new
	// selection. Selecting a non-talk tab closes the conversation list. Call
	// directly on the frame thread, without ext.Call or ext.Guard.
	RevealPane(tab string)
}

// Commands collects commands; desk enumerates it on the frame thread.
const Commands ext.PointKey[Command] = "host.commands"

// PaneTabs collects right pane tabs; desk enumerates it on the frame thread.
const PaneTabs ext.PointKey[PaneTab] = "host.panes"

// Pages collects title-bar pages, sorted with host pages by Order.
const Pages ext.PointKey[Page] = "host.pages"

// BookService resolves current book content; methods require the frame thread.
const BookService ext.ServiceKey[Book] = "host.book"

// PaletteService resolves palettes; methods require the frame thread.
const PaletteService ext.ServiceKey[Palette] = "host.palette"

// PaneService resolves navigation; methods require the frame thread.
const PaneService ext.ServiceKey[Pane] = "host.pane"

// Overlay draws above pages on the frame thread through ext.Call. Draw checks
// its own open state and must not retain Context or frame elements.
type Overlay struct {
	// Open optionally reports a modal above settings; called on the frame thread through ext.Guard.
	Open func() bool
	Draw func(*ui.Context)
}

// Notices exposes status messages on the frame thread only.
type Notices interface {
	// Notices returns the status bar; use it on the frame thread only.
	Notices() *kit.Status
}

// Jobs dispatches background work from the frame thread.
type Jobs interface {
	// StartJob runs fn on a worker and its result on the frame thread.
	// Capture Services in the initiating frame; never retain its Context.
	StartJob(ui.Services, func() func())
}

// Dialogs exposes native dialogs; accessors require the frame thread.
type Dialogs interface {
	// CanPickFolder reports whether a native folder picker is installed; frame thread only.
	CanPickFolder() bool
	// Dialogs returns optional save-file and reveal callbacks, called on workers.
	Dialogs() (save func(string, string) (string, error), reveal func(string) error)
	// PickInto starts a folder picker on the frame thread. The picker runs on a
	// worker; picked runs on the frame thread. False means no picker is available.
	PickInto(*ui.Context, string, func(string)) bool
}

// Overlays collects page overlays; desk enumerates it on the frame thread.
const Overlays ext.PointKey[Overlay] = "host.overlays"

// NoticesService resolves frame-thread status messages.
const NoticesService ext.ServiceKey[Notices] = "host.notices"

// JobsService resolves background dispatch; call StartJob on the frame thread.
const JobsService ext.ServiceKey[Jobs] = "host.jobs"

// DialogsService resolves native dialogs; accessors require the frame thread.
const DialogsService ext.ServiceKey[Dialogs] = "host.dialogs"

// ErrorTranslator formats a known error using reason for nested causes.
// It runs on the frame thread and must not retain frame objects.
type ErrorTranslator func(error, func(error) string) (string, bool)

// ErrorTexts collects active extension error translators.
const ErrorTexts ext.PointKey[ErrorTranslator] = "host.error-texts"
