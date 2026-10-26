package kit

// Status is what the toasts show: the error, the failures in a row, the
// request that can run again, and what was done or is to be done first.
type Status struct {
	shown  error  // the error shown
	failed string // the error the last request ended with,
	fails  int    // and how many times in a row
	again  func() // runs that request again
	notice string // what was done
	undo   func() error
	serial int
	tip    string // what to do first
}

// SetError shows err in a toast until the author closes it; nil clears it.
func (s *Status) SetError(err error) { s.shown = err }

func (s *Status) Err() error { return s.shown }

func (s *Status) ErrorText() string {
	if s.shown == nil {
		return ""
	}
	return s.shown.Error()
}

// Fail shows err and counts it with the failures in a row.
func (s *Status) Fail(err error) {
	s.SetError(err)
	if text := s.ErrorText(); text == s.failed {
		s.fails++
	} else {
		s.failed, s.fails = text, 1
	}
}

// Succeed ends the failures in a row.
func (s *Status) Succeed() { s.failed, s.fails = "", 0 }

// Repeated is how many times in a row the error shown failed; 0 while it
// is not the last request's.
func (s *Status) Repeated() int {
	if s.failed != s.ErrorText() {
		return 0
	}
	return s.fails
}

// Retry offers fn to run the last request again.
func (s *Status) Retry(fn func()) { s.again = fn }

// RetryAction is that offer while the error shown is the last request's.
func (s *Status) RetryAction() func() {
	if s.again == nil || s.failed != s.ErrorText() {
		return nil
	}
	return s.again
}

// Did says what was done.
func (s *Status) Did(what string, undo ...func() error) {
	s.notice, s.undo = what, nil
	s.serial++
	if len(undo) > 0 {
		s.undo = undo[0]
	}
}

func (s *Status) NoticeSerial() int { return s.serial }

func (s *Status) CanUndo() bool { return s.undo != nil }

// Undo retires the offer only after its writes succeed.
func (s *Status) Undo() {
	if s.undo == nil {
		return
	}
	if err := s.undo(); err != nil {
		s.Warn(err)
		return
	}
	s.notice, s.undo = "", nil
}

func (s *Status) NoticeText() string { return s.notice }

// CloseNotice clears what was done, if it is still what the toast said.
func (s *Status) CloseNotice(what string) {
	if s.notice == what {
		s.notice = ""
		s.undo = nil
	}
}

// Hint says what to do first.
func (s *Status) Hint(what string) { s.tip = what }

func (s *Status) HintText() string { return s.tip }

// CloseHint clears the hint, if it is still what the toast said.
func (s *Status) CloseHint(what string) {
	if s.tip == what {
		s.tip = ""
	}
}

// CloseError closes the error toast: the error goes, and a run of
// failures with it.
func (s *Status) CloseError() { s.shown, s.failed, s.fails = nil, "", 0 }

// Clear drops everything, as turning the AI off does.
func (s *Status) Clear() { *s = Status{} }

func (s *Status) Warn(err error) {
	if err != nil {
		s.SetError(err)
	}
}
