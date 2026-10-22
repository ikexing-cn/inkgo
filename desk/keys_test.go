package desk

import (
	"testing"

	"writer/internal/testshot"
)

// SHOT=dir go test ./desk -run TestShotKeys draws the shortcuts, with the
// AI on and with it off.
func TestShotKeys(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	a.keysOpen = true
	shot(t, tt, "keys.png")
	a.shelf.NoAI = true
	tt.Frame()
	shot(t, tt, "keys-off.png")
}
