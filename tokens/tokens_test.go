package tokens

import "testing"

func TestFormat(t *testing.T) {
	for n, want := range map[int]string{1_000_000: "1M", 1_050_000: "1050K", 128_000: "128K", 384_000: "384K", 131_072: "131072", 4096: "4096", 0: "0"} {
		if got := Format(n); got != want {
			t.Errorf("Format(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	good := map[string]int{"1M": 1_000_000, "1m": 1_000_000, "128K": 128_000, "128k": 128_000, " 128 K ": 128_000, "1.5M": 1_500_000, "0.5K": 500, "131072": 131_072, "1050K": 1_050_000}
	for s, want := range good {
		if got, ok := Parse(s); !ok || got != want {
			t.Errorf("Parse(%q) = %d, %v, want %d", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "K", "一百万", "1.5", "1.0001K", "-1K", "0", "0K", "1e6", "1,000", "1.K", ".5M", "1MB", "9999999999999K"} {
		if n, ok := Parse(s); ok {
			t.Errorf("Parse(%q) = %d, want a failure", s, n)
		}
	}
	for _, n := range []int{1, 4096, 128_000, 131_072, 1_000_000, 1_050_000} {
		if got, ok := Parse(Format(n)); !ok || got != n {
			t.Errorf("Parse(Format(%d)) = %d, %v", n, got, ok)
		}
	}
}

func TestRound(t *testing.T) {
	for n, want := range map[int]string{0: "0", 268: "268", 999: "999", 1000: "1K", 9600: "9.6K", 9649: "9.6K", 10_140: "10.1K", 59_200: "59.2K", 999_960: "1M", 1_700_000: "1.7M", 2_500_000_000: "2.5B"} {
		if got := Round(n); got != want {
			t.Errorf("Round(%d) = %q, want %q", n, got, want)
		}
	}
}
