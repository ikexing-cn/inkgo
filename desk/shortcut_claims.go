package desk

import (
	"fmt"
	"slices"
	"strings"

	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
)

func hostCommandID(cmd *command) string {
	return fmt.Sprintf("host.command-%d", slices.Index(commandList, cmd))
}

// hostKeys gives the host's own commands' bindings to the key claims.
func hostKeys() []ext.HostKey {
	out := make([]ext.HostKey, 0, len(commandList))
	for _, cmd := range commandList {
		out = append(out, ext.HostKey{ID: hostCommandID(cmd), Bindings: hostapi.Command{Key: cmd.key, Mods: cmd.mods, MacKey: cmd.macKey, MacMods: cmd.macMods, OnMac: cmd.onMac}.ShortcutBindings()})
	}
	return out
}

func (a *App) applyKeyClaim(cmd *command, id, owner string) {
	cmd.keyLost = false
	cmd.clash = nil
	for _, claim := range a.extensions.Ledger(owner).Keys {
		if claim.ID != id || claim.Mac != onMacOS {
			continue
		}
		cmd.keyLost = claim.Winner != id
		if cmd.keyLost || len(claim.Owners) < 2 {
			continue
		}
		// The host already resolved ownership and precedence. Keep localization
		// functions so an open help dialog follows a language switch as well.
		var others []func() string
		var winner func() string
		if claim.WinnerOwner == ext.HostOwner {
			winner = cmd.label
		}
		for _, mod := range a.extensions.Modules() {
			name := mod.Name
			if name == nil {
				moduleID := mod.ID
				name = func() string { return moduleID }
			}
			if mod.ID == claim.WinnerOwner {
				winner = name
			}
			if slices.Contains(claim.Owners, mod.ID) && mod.ID != claim.WinnerOwner {
				others = append(others, name)
			}
		}
		cmd.clash = func() string {
			names := make([]string, 0, len(others))
			for _, name := range others {
				names = append(names, name())
			}
			return locales.KeysConflict(strings.Join(names, locales.ModListSep()), winner())
		}
	}
}
