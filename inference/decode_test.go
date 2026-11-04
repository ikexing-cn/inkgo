package inference

import "testing"

func TestDecodeMessages(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"```json\n{\"n\":1}\n```", ""},
		{`{"extra":1}`, `inference: unexpected JSON: json: unknown field "extra"`},
		{`{"n":1} {}`, "inference: multiple JSON values or trailing content"},
		{`{"n":1} trailing`, "inference: multiple JSON values or trailing content"},
	} {
		var out struct {
			N int `json:"n"`
		}
		err := Decode(c.text, &out)
		if c.want == "" {
			if err != nil || out.N != 1 {
				t.Fatalf("decode(%q) = %+v, %v", c.text, out, err)
			}
		} else if err == nil || err.Error() != c.want {
			t.Errorf("decode(%q) = %v, want %q", c.text, err, c.want)
		}
	}
}
