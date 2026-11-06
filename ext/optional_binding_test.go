package ext_test

import (
	"testing"
	"writer/ext"
)

type boundService struct{ session *ext.Session }

func (b boundService) BindSession(s *ext.Session) any { return boundService{s} }
func TestOptionalServiceBindsConsumerSession(t *testing.T) {
	key := ext.ServiceKey[boundService]("provider.service")
	var optional *ext.OptRef[boundService]
	var required *ext.Ref[boundService]
	var session *ext.Session
	h := host(t, ext.Options{}, ext.Module{ID: "provider", Setup: func(r *ext.Registrar) { ext.Provide(r, key, func(*ext.Session) boundService { return boundService{} }) }}, ext.Module{ID: "consumer", Requires: []string{"provider"}, Setup: func(r *ext.Registrar) {
		optional = ext.UseOptional(r, key)
		required = ext.Use(r, key)
		r.OnStart(func(s *ext.Session) error { session = s; return nil })
	}})
	check := func() {
		t.Helper()
		v, ok := optional.Get()
		if !ok || v.session != session || required.Get().session != session {
			t.Fatal("service not bound to consumer")
		}
	}
	check()
	old := session
	must(t, h.Open(ext.Book{Dir: "book"}))
	check()
	if session == old {
		t.Fatal("session not renewed")
	}
}
