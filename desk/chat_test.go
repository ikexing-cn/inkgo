package desk

import "testing"

func TestReplyLabel(t *testing.T) {
	a := newApp("")
	defer a.Close()
	for _, tc := range []struct {
		name, effort, want string
	}{
		{"deepseek/deepseek-flash", "", "deepseek-flash"},
		{"relay/vendor/deepseek-flash", "high", "deepseek-flash · High"},
		{"bare", "low", "bare · Low"},
		{"", "high", ""}, {"provider/", "high", ""},
	} {
		if got := a.replyLabel(tc.name, tc.effort); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
