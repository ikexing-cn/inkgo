package hostapi

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"writer/ext"
)

// CommandModifier represents the platform command modifier in portable bindings.
const CommandModifier ui.Modifiers = 1 << 7

// ShortcutBindings identifies each platform's effective command binding.
func (c Command) ShortcutBindings() [2]string {
	var out [2]string
	for n := range 2 {
		key, mods := c.Key, c.Mods
		if n == 1 && c.OnMac {
			key, mods = c.MacKey, c.MacMods
		}
		if mods&CommandModifier != 0 {
			mods &^= CommandModifier
			if n == 1 {
				mods |= ui.Super
			} else {
				mods |= ui.Ctrl
			}
		}
		if key != 0 {
			out[n] = fmt.Sprintf("%d:%d", mods, key)
		}
	}
	return out
}

// CommandPlaces translates command declarations into visible host places.
// Commands share one point but occupy several prototype contribution places.
func CommandPlaces(c Command) []ext.Place {
	var places []ext.Place
	switch c.Menu.Menu {
	case MenuFile:
		places = append(places, "file")
	case MenuEdit:
		places = append(places, "edit")
	case MenuView:
		places = append(places, "view")
	}
	if c.Key != 0 || c.MacKey != 0 {
		places = append(places, "keys")
	}
	return places
}
