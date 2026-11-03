package desk

import (
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"writer/locales"
)

// SystemMenuItem is a value snapshot, independent of the native menu toolkit.
// IDs, roles, shortcuts and ordering are structural; the other fields are state.
type SystemMenuItem struct {
	ID, Label, Accelerator, Role                string
	Enabled, Checked, Visible, Separator, Radio bool
	Children                                    []SystemMenuItem
}

type systemMenuQueue struct {
	mu  sync.Mutex
	ids []string
}

var systemCommands = []struct {
	id  string
	cmd *command
}{
	{"settings", cmdSettings}, {"new-chapter", cmdNewChapter}, {"new-folder", cmdNewFolder},
	{"save", cmdSave}, {"snapshots", cmdSnapshots}, {"export", cmdExport},
	{"categories", cmdCategories}, {"trash", cmdTrash}, {"undo", cmdUndo}, {"redo", cmdRedo},
	{"find", cmdFind}, {"rename", cmdRename}, {"continue", cmdContinue}, {"draft", cmdDraft},
	{"side", cmdSide}, {"pane", cmdPane}, {"focus", cmdFocus},
	{"zoom-in", cmdZoomIn}, {"zoom-out", cmdZoomOut}, {"zoom-reset", cmdZoomReset}, {"shortcuts", cmdHelp},
}

// systemAccelerator writes the command's macOS key as mygo's menus take it
// ("Shift+Cmd+Z"); "" when it has none there.
func systemAccelerator(cmd *command) string {
	mods, key := cmd.bindingOn(true)
	if key == 0 {
		return ""
	}
	var parts []string
	for _, m := range []struct {
		bit  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}, {ui.Super, "Cmd"}} {
		if mods&m.bit != 0 {
			parts = append(parts, m.name)
		}
	}
	name := keyText(key)
	if key == ui.KeyComma {
		name = ","
	}
	parts = append(parts, name)
	return strings.Join(parts, "+")
}

// systemMenuOwns reports whether the system menu shows the command with its
// key, and so takes that key before the window does.
func systemMenuOwns(cmd *command) bool {
	if cmd.systemMenu {
		return systemAccelerator(cmd) != ""
	}
	for _, entry := range systemCommands {
		if entry.cmd == cmd {
			return systemAccelerator(cmd) != ""
		}
	}
	return false
}

// SystemMenu computes state on the view thread, after the frame's actions.
func (a *App) SystemMenu() []SystemMenuItem {
	commands := a.systemCommands()
	row := func(id string) SystemMenuItem {
		for _, entry := range commands {
			if entry.id == id {
				c := entry.cmd
				item := SystemMenuItem{ID: id, Label: c.name(a), Accelerator: systemAccelerator(c), Enabled: c.available(a), Visible: !c.ai || a.aiOn()}
				if id == "undo" || id == "redo" {
					item.Role, item.Label = id, ""
				}
				return item
			}
		}
		panic(id)
	}
	sep := func(id string) SystemMenuItem { return SystemMenuItem{ID: id, Separator: true, Visible: true} }
	role := func(id, r string) SystemMenuItem {
		return SystemMenuItem{ID: id, Role: r, Enabled: true, Visible: true}
	}
	menu := func(id, label, r string, children ...SystemMenuItem) SystemMenuItem {
		return SystemMenuItem{ID: id, Label: label, Role: r, Enabled: true, Visible: true, Children: children}
	}
	aiSep := sep("edit-ai-separator")
	aiSep.Visible = a.aiOn()
	zoom := SystemMenuItem{ID: "zoom-label", Label: locales.ViewZoom(int(math.Round(float64(a.Zoom()) * 100))), Visible: true}
	look := SystemMenuItem{ID: "look-label", Label: locales.ViewLook(), Visible: true}
	looks := []SystemMenuItem{}
	for _, v := range []struct{ id, value, name string }{{"look-system", "", locales.LookSystem()}, {"look-light", "light", locales.LookLight()}, {"look-dark", "dark", locales.LookDark()}} {
		looks = append(looks, SystemMenuItem{ID: v.id, Label: v.name, Enabled: true, Visible: true, Radio: true, Checked: a.Theme() == v.value})
	}
	var view []SystemMenuItem
	for _, entry := range a.viewCommands() {
		switch entry.id {
		case "view-zoom-separator", "view-look-separator":
			view = append(view, sep(entry.id))
		case "zoom-label":
			view = append(view, zoom)
		case "look-label":
			view = append(view, look)
			view = append(view, looks...)
		default:
			view = append(view, row(entry.id))
		}
	}
	return []SystemMenuItem{
		menu("app", "", "appMenu", role("about", "about"), sep("app-settings-separator"), row("settings"), sep("app-hide-separator"), role("hide", "hide"), role("hide-others", "hideOthers"), sep("app-quit-separator"), role("quit", "quit")),
		menu("file", locales.MenuFile(), "", row("new-chapter"), row("new-folder"), sep("file-save-separator"), row("save"), row("snapshots"), row("export"), sep("file-organize-separator"), row("categories"), row("trash")),
		menu("edit", locales.MenuEdit(), "", row("undo"), row("redo"), sep("edit-find-separator"), row("find"), row("rename"), aiSep, row("continue"), row("draft")),
		menu("view", locales.MenuView(), "", view...),
		menu("window", locales.MenuWindow(), "windowMenu", role("minimize", "minimize"), role("window-zoom", "zoom"), sep("window-front-separator"), role("front", "front")),
		menu("help", "", "help", row("shortcuts")),
	}
}

// QueueSystemMenu may be called on the main thread. It retains no frame objects.
func (a *App) QueueSystemMenu(id string) {
	a.nativeMenu.mu.Lock()
	a.nativeMenu.ids = append(a.nativeMenu.ids, id)
	a.nativeMenu.mu.Unlock()
}

func (a *App) systemMenuAct() {
	a.nativeMenu.mu.Lock()
	ids := a.nativeMenu.ids
	a.nativeMenu.ids = nil
	a.nativeMenu.mu.Unlock()
	for _, id := range ids {
		switch id {
		case "look-system":
			a.setTheme("")
		case "look-light":
			a.setTheme("light")
		case "look-dark":
			a.setTheme("dark")
		default:
			for _, entry := range a.systemCommands() {
				c := entry.cmd
				if entry.id == id && id != "undo" && id != "redo" && c.available(a) && (!c.ai || a.aiOn()) && c.run != nil {
					c.run(a)
				}
			}
		}
	}
}

func (a *App) systemCommands() []namedCommand { a.syncExtensions(); return a.extViews.system }
