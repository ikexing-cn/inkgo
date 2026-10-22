package workspace

import "testing"

func TestModelRef(t *testing.T) {
	for _, tc := range []struct {
		name string
		ref  ModelRef
		ok   bool
	}{
		{"service/model", ModelRef{"service", "model"}, true},
		{"service/vendor/id", ModelRef{"service", "vendor/id"}, true},
		{"service/vendor/team/id", ModelRef{"service", "vendor/team/id"}, true},
		{"service/", ModelRef{"service", ""}, true},
		{"/model", ModelRef{"", "model"}, true},
		{"/", ModelRef{}, true},
		{"bare", ModelRef{Model: "bare"}, false},
		{"", ModelRef{}, false},
	} {
		got, ok := ParseModelRef(tc.name)
		if got != tc.ref || ok != tc.ok {
			t.Fatal(tc.name, got, ok)
		}
		if ok && got.String() != tc.name {
			t.Fatal(tc.name, got.String())
		}
	}
	if got := (ModelRef{Provider: "service", Model: "vendor/id"}).String(); got != "service/vendor/id" {
		t.Fatal(got)
	}
}
