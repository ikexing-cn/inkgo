package desk

import "testing"

func TestCachedBuildsOncePerKey(t *testing.T) {
	var c cached[int, string]
	built := 0
	build := func() string {
		built++
		return "值"
	}
	if got := c.get(1, build); got != "值" || built != 1 || c.builds != 1 {
		t.Fatalf("first get = %q, built %d, builds %d", got, built, c.builds)
	}
	if got := c.get(1, build); got != "值" || built != 1 || c.builds != 1 {
		t.Fatalf("same key built again: %q, built %d, builds %d", got, built, c.builds)
	}
	if got := c.get(2, build); got != "值" || built != 2 || c.builds != 2 {
		t.Fatalf("another key = %q, built %d, builds %d", got, built, c.builds)
	}
	if got := c.get(1, build); got != "值" || built != 3 || c.builds != 3 {
		t.Fatalf("a key come back = %q, built %d, builds %d", got, built, c.builds)
	}
}

func TestCachedZeroKeyBuilds(t *testing.T) {
	var c cached[string, int]
	built := 0
	if got := c.get("", func() int { built++; return 0 }); got != 0 || built != 1 {
		t.Fatalf("zero key: %d, built %d", got, built)
	}
	if got := c.get("", func() int { built++; return 1 }); got != 0 || built != 1 {
		t.Fatalf("zero key built again: %d, built %d", got, built)
	}
}
