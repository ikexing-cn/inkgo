package strictjson

import (
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, c := range []struct {
		text, message string
		trailing      bool
	}{
		{`{"n":1}`, "", false},
		{"{\"n\":1}\n\t ", "", false},
		{`{"extra":1}`, `json: unknown field "extra"`, false},
		{`{"n":"x"}`, `json: cannot unmarshal string into Go struct field .n of type int`, false},
		{"", "EOF", false},
		{`{"n":`, "unexpected EOF", false},
		{`{"n":1} {}`, ErrTrailing.Error(), true},
		{`{"n":1} null`, ErrTrailing.Error(), true},
		{`{"n":1} trailing`, "invalid character 'a' in literal true (expecting 'u')", true},
		{`{"n":1} {`, "unexpected EOF", true},
	} {
		t.Run(c.text, func(t *testing.T) {
			var out struct {
				N int `json:"n"`
			}
			err := Decode([]byte(c.text), &out)
			if c.message == "" {
				if err != nil || out.N != 1 {
					t.Fatalf("decoded %+v, %v", out, err)
				}
				return
			}
			if err == nil || err.Error() != c.message || errors.Is(err, ErrTrailing) != c.trailing {
				t.Fatalf("error = %v, trailing %v; want %q, %v", err, errors.Is(err, ErrTrailing), c.message, c.trailing)
			}
		})
	}
}

func TestErrorIdentity(t *testing.T) {
	var out any
	if err := Decode([]byte(`{"n":`), &out); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("incomplete line lost its recovery error: %v", err)
	}
	var syntax *json.SyntaxError
	if err := Decode([]byte(`{} x`), &out); !errors.Is(err, ErrTrailing) || !errors.As(err, &syntax) {
		t.Fatalf("trailing syntax error lost its identity: %v", err)
	}
}
