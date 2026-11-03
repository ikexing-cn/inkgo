package kit

import (
	"cmp"
	"os"
	"testing"

	"github.com/egoist/mygo/i18n"
	"writer/internal/testtmp"
)

func TestMain(m *testing.M) {
	i18n.SetLocaleFunc(testLocale)
	testtmp.Main(m)
}

// testLocale is the language the tests say their words in: Chinese, which
// they compare against, whatever the system's. MYGO_LOCALE=en draws the
// screenshots (SHOT) in another; the tests that compare words then fail.
func testLocale() string { return cmp.Or(os.Getenv("MYGO_LOCALE"), "zh-Hans") }
