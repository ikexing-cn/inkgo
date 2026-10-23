package desk

import (
	"writer/desk/kit"
)

// item adds a row for cmd: what it is called, its hint, and whether it
// can run now, all from the table. Do says what picking it does; a menu
// whose action is its own gives that instead of the command's.
func (c *command) item(a *App, m *kit.Menu) *kit.MenuItem {
	return m.Item(c.name(a)).Hint(c.menuHint(a)).Disabled(!c.available(a))
}
