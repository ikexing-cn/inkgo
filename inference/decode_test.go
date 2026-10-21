package inference

import "testing"

func TestDecodeMessages(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"```json\n{\"n\":1}\n```", ""},
		{`{"extra":1}`, `JSON 不符合约定：json: unknown field "extra"`},
		{`{"n":1} {}`, "返回了多个 JSON 值或尾随内容"},
		{`{"n":1} trailing`, "返回了多个 JSON 值或尾随内容"},
	} {
		var out struct {
			N int `json:"n"`
		}
		err := decode(c.text, &out)
		if c.want == "" {
			if err != nil || out.N != 1 {
				t.Fatalf("decode(%q) = %+v, %v", c.text, out, err)
			}
		} else if err == nil || err.Error() != c.want {
			t.Errorf("decode(%q) = %v, want %q", c.text, err, c.want)
		}
	}
}
