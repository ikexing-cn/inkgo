package desk

import (
	"testing"
	"writer/internal/testshot"
)

func TestShotAPIDependencyDetails(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := fixture(t)
	extensionDetails(t, a, tt, "check")
	shot(t, tt, "ext-check-api-details.png")
	extensionDetails(t, a, tt, "snaps")
	shot(t, tt, "ext-snaps-api-details.png")
}
