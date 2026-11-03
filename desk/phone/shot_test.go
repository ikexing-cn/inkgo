package phone

import (
	"strings"
	"testing"
	"time"
	"writer/internal/testshot"
)

func TestShotPhone(t *testing.T) {
	testshot.Dir(t)
	_, p, tt, _ := previewFixture(t, strings.Repeat("A paragraph of book text for the phone preview. ", 30))
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "phone-preview.png", tt.Image())
	p.look = true
	tt.Frame()
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	testshot.Save(t, "phone-typesetting.png", tt.Image())
}
