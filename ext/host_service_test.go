package ext_test

import (
	"testing"
	"writer/ext"
)

func TestUseHostServiceNeedsNoRequires(t *testing.T) {
	key := ext.ServiceKey[int]("host.number")
	var ref *ext.Ref[int]
	builds := 0
	h := host(t, ext.Options{DeclareHost: func(r *ext.Registrar) {
		ext.Provide(r, key, func(s *ext.Session) int {
			builds++
			if s.ID() != ext.HostOwner || s.Book().Dir != "" {
				t.Error("invalid host session")
			}
			return 42
		})
	}}, ext.Module{ID: "consumer", Setup: func(r *ext.Registrar) { ref = ext.Use(r, key) }})
	if ref.Get() != 42 {
		t.Fatal("host service unavailable")
	}
	must(t, h.Set("consumer", false))
	h.Close()
	must(t, h.Open(ext.Book{Dir: "book"}))
	if ref.Get() != 42 || builds != 1 {
		t.Fatal("host service followed module or book lifetime", builds)
	}
}

func TestHostServiceAvailableBeforeExtensionOnStart(t *testing.T) {
	key := ext.ServiceKey[string]("host.text")
	started := false
	host(t, ext.Options{DeclareHost: func(r *ext.Registrar) { ext.Provide(r, key, func(*ext.Session) string { return "ready" }) }}, ext.Module{ID: "consumer", Setup: func(r *ext.Registrar) {
		ref := ext.Use(r, key)
		r.OnStart(func(*ext.Session) error {
			started = true
			if ref.Get() != "ready" {
				t.Error("host service not ready")
			}
			return nil
		})
	}})
	if !started {
		t.Fatal("consumer did not start")
	}
}

func TestHostServiceGetDuringSetupStillPanics(t *testing.T) {
	_, err := ext.New(ext.Options{DeclareHost: func(r *ext.Registrar) {
		ext.Provide(r, ext.ServiceKey[int]("host.number"), func(*ext.Session) int { return 1 })
	}}, ext.Module{ID: "consumer", Setup: func(r *ext.Registrar) { ext.Use(r, ext.ServiceKey[int]("host.number")).Get() }})
	if err == nil {
		t.Fatal("Setup access accepted")
	}
}

func TestOpenAfterCloseResetsBookPreference(t *testing.T) {
	var p *ext.Pref[int]
	h := host(t, ext.Options{}, ext.Module{ID: "prefs", Setup: func(r *ext.Registrar) {
		p = ext.Setting(r, "n", ext.Schema[int]{Scope: ext.BookScope})
	}})
	must(t, h.Open(ext.Book{Dir: "one"}))
	p.Set(7)
	h.Close()
	must(t, h.Open(ext.Book{}))
	if p.Get() != 0 {
		t.Fatal("book preference reset lost", p.Get())
	}
}
