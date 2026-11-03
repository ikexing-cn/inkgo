package phone

import (
	"cmp"
	"github.com/egoist/mygo/i18n"
	"os"
	"testing"

	"writer/internal/testtmp"
)

func TestMain(m *testing.M) {
	i18n.SetLocaleFunc(testLocale)
	testtmp.Main(m)
}

// Tests default to Chinese; MYGO_LOCALE selects screenshot languages.
func testLocale() string { return cmp.Or(os.Getenv("MYGO_LOCALE"), "zh-Hans") }
