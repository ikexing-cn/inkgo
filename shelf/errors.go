package shelf

// NoBookError identifies a folder without book metadata.
type NoBookError struct{ Path string }

func (e *NoBookError) Error() string { return "shelf: no work.yaml in " + e.Path }
