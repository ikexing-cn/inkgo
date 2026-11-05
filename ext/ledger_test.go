package ext_test

import (
	"testing"
	"writer/ext"
)

type bindingFixture [2]string

func (b bindingFixture) ShortcutBindings() [2]string { return b }

func TestKeyClaimsPreferBundledThenRegistrationAndRefresh(t *testing.T) {
	p := ext.PointKey[bindingFixture]("host.keys")
	module := func(id string, installed bool) ext.Module {
		return ext.Module{ID: id, Installed: installed, Setup: func(r *ext.Registrar) { ext.Contribute(r, p, "key", bindingFixture{"ctrl:l", "cmd:l"}) }}
	}
	h := host(t, ext.Options{DeclareHost: func(r *ext.Registrar) { ext.Declare(r, p, ext.Hidden) }}, module("installed", true), module("bundled", false), module("later", false))
	check := func(id, winner string) {
		t.Helper()
		claims := h.Ledger(id).Keys
		if len(claims) != 2 {
			t.Fatal(claims)
		}
		for _, c := range claims {
			if c.Winner != winner {
				t.Fatal(c)
			}
		}
	}
	check("installed", "bundled.key")
	check("later", "bundled.key")
	must(t, h.Set("bundled", false))
	check("installed", "later.key")
	must(t, h.Set("later", false))
	check("installed", "installed.key")
	if len(h.Ledger("bundled").Keys) != 0 {
		t.Fatal("disabled claim remains")
	}
}

func TestExplicitDefaultKeepsOriginalWithoutUnresolvedConflict(t *testing.T) {
	h := host(t, hookOptions(), overwriter("a", 10), overwriter("b", 20))
	must(t, h.Choose(string(hostHook), ext.DefaultChoice))
	value, err := ext.Call(h, hostHook, 2, identity)
	must(t, err)
	if value != 2 {
		t.Fatal(value)
	}
	for _, id := range []string{"a", "b"} {
		i := h.Ledger(id).Injections[0]
		if i.Active || i.Conflict || i.Reason.ID != ext.DefaultChoice {
			t.Fatal(i)
		}
	}
}

func TestPointPlaceValidated(t *testing.T) {
	_, err := ext.New(ext.Options{DeclareHost: func(r *ext.Registrar) { ext.Declare(r, ext.PointKey[int]("host.test"), "bad") }})
	if err == nil {
		t.Fatal("accepted an unknown place")
	}
}

func TestInjectionKindWords(t *testing.T) {
	for k, want := range map[ext.InjectionKind]string{ext.BeforeKind: "before", ext.AfterKind: "after", ext.AroundKind: "around", ext.OverwriteKind: "overwrite", ext.ReplaceKind: "overwrite"} {
		if k.KindWord() != want {
			t.Fatal(k, k.KindWord(), want)
		}
	}
}

func TestHostBindingsAlwaysWinAndRecordOwner(t *testing.T) {
	p := ext.PointKey[bindingFixture]("host.keys")
	h := host(t, ext.Options{DeclareHost: func(r *ext.Registrar) {
		ext.Declare(r, p, "keys")
		ext.Contribute(r, p, "save", bindingFixture{"ctrl:s", "cmd:s"})
	}}, ext.Module{ID: "bundled", Setup: func(r *ext.Registrar) { ext.Contribute(r, p, "save", bindingFixture{"ctrl:s", "cmd:s"}) }})
	for _, id := range []string{ext.HostOwner, "bundled"} {
		for _, c := range h.Ledger(id).Keys {
			if c.Winner != "host.save" || c.WinnerOwner != ext.HostOwner {
				t.Fatal(c)
			}
		}
	}
}
