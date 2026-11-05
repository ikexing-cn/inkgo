package hooks

// cached keeps the last value built for a key and builds again only when
// the key changes. What it returns is shared by its callers, so a caller
// that sorts, filters or appends works on a copy.
type cached[K comparable, V any] struct {
	key    K
	val    V
	ok     bool
	builds int
}

// get returns the value for key, building it when this is another key than
// the last one. builds counts the builds, for the frames test.
func (c *cached[K, V]) get(key K, build func() V) V {
	if c.ok && c.key == key {
		return c.val
	}
	c.key, c.val, c.ok = key, build(), true
	c.builds++
	return c.val
}
