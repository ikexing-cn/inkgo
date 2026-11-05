package desk

// Busy reports a running author operation on the frame thread.
func (h extensionHost) Busy() bool { return h.a.busy != "" }

// Held reports a blocking book operation on the frame thread.
func (h extensionHost) Held() bool { return h.a.held() }
