// Package hostapi contains stateless contracts between desk and extensions.
package hostapi

import (
	"github.com/egoist/mygo/ui"

	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
)

// Menu names a host menu; read by desk on the frame thread.
type Menu string

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
	Name    func() string
	Can     func() bool
	Run     func()
	Menu    MenuPosition
	Key     ui.Key
	Mods    ui.Modifiers
	MacKey  ui.Key
	MacMods ui.Modifiers
	OnMac   bool
}

// PaneTab is a right pane contributed by an extension. Desk calls Title and
// Available on the frame thread; nil Available means visible. Draw runs through
// ext.Call in the frame and must not retain Context or frame elements.
type PaneTab struct {
	Title     func() string
	Draw      func(*ui.Context)
	Available func() bool
}

// Book exposes current book content to extensions on the frame thread only.
type Book interface {
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
	// Show opens the right pane, selects a fully qualified tab ID and exits focus.
	Show(tab string)
}

// Commands collects commands; desk enumerates it on the frame thread.
const Commands ext.PointKey[Command] = "host.commands"

// PaneTabs collects right pane tabs; desk enumerates it on the frame thread.
const PaneTabs ext.PointKey[PaneTab] = "host.panes"

// BookService resolves current book content; methods require the frame thread.
const BookService ext.ServiceKey[Book] = "host.book"

// PaletteService resolves palettes; methods require the frame thread.
const PaletteService ext.ServiceKey[Palette] = "host.palette"

// PaneService resolves navigation; methods require the frame thread.
const PaneService ext.ServiceKey[Pane] = "host.pane"
