package workspace

import "fmt"

// Error is a stable workspace failure independent of the interface language.
type Error string

func (e Error) Error() string { return "workspace: " + string(e) }

// Workspace validation and concurrent-edit failures.
const (
	ErrTitleEmpty     Error = "title is empty"
	ErrMoveIntoSelf   Error = "cannot move a folder into itself"
	ErrCheckChanged   Error = "check results changed again"
	ErrSettingChanged Error = "lore changed again"
	ErrLegacySchema   Error = "legacy schema 1 needs migration"
)

// TrashMissingError identifies a missing trash entry.
type TrashMissingError struct{ ID string }

func (e *TrashMissingError) Error() string {
	return fmt.Sprintf("workspace: trash entry %q missing", e.ID)
}
