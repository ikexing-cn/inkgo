package desk

import (
	"cmp"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/i18n"
	"writer/internal/testtmp"
)

// TestMain keeps a first shelf from finding the author's own works: the
// library is in the run's own temporary folder, which os.TempDir gives
// once testtmp.Main has set it, so it is asked for when called.
func TestMain(m *testing.M) {
	i18n.SetLocaleFunc(testLocale)
	Library = func() string { return filepath.Join(os.TempDir(), "library") }
	testtmp.Main(m)
}

// testLocale is the language the tests say their words in: Chinese, which
// they compare against, whatever the system's. MYGO_LOCALE=en draws the
// screenshots (SHOT) in another; the tests that compare words then fail.
func testLocale() string { return cmp.Or(os.Getenv("MYGO_LOCALE"), "zh-Hans") }
