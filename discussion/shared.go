package discussion

// Locate finds quote in text verbatim or, failing that, ignoring spaces
// and the marks a model drops or adds copying words, and returns the
// text's own words for it.
func Locate(text, quote string) (string, bool) { return locate(text, quote) }

// Decode reads exactly one strict JSON value from a model's answer,
// allowing a ```json fence.
func Decode(s string, v any) error { return decode(s, v) }
