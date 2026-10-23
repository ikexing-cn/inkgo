package desk

// status is what the toasts show: the error, the failures in a row, the
// request that can run again, and what was done or is to be done first.
type status struct {
	shown  error  // the error shown
	failed string // the error the last request ended with,
	fails  int    // and how many times in a row
	again  func() // runs that request again
	notice string // what was done
	tip    string // what to do first
}

// setError shows err in a toast until the author closes it; nil clears it.
func (s *status) setError(err error) { s.shown = err }

func (s *status) err() error { return s.shown }

func (s *status) errorText() string {
	if s.shown == nil {
		return ""
	}
	return s.shown.Error()
}

// fail shows err and counts it with the failures in a row.
func (s *status) fail(err error) {
	s.setError(err)
	if text := s.errorText(); text == s.failed {
		s.fails++
	} else {
		s.failed, s.fails = text, 1
	}
}

// succeed ends the failures in a row.
func (s *status) succeed() { s.failed, s.fails = "", 0 }

// repeated is how many times in a row the error shown failed; 0 while it
// is not the last request's.
func (s *status) repeated() int {
	if s.failed != s.errorText() {
		return 0
	}
	return s.fails
}

// retry offers fn to run the last request again.
func (s *status) retry(fn func()) { s.again = fn }

// retryAction is that offer while the error shown is the last request's.
func (s *status) retryAction() func() {
	if s.again == nil || s.failed != s.errorText() {
		return nil
	}
	return s.again
}

// did says what was done.
func (s *status) did(what string) { s.notice = what }

func (s *status) noticeText() string { return s.notice }

// closeNotice clears what was done, if it is still what the toast said.
func (s *status) closeNotice(what string) {
	if s.notice == what {
		s.notice = ""
	}
}

// hint says what to do first.
func (s *status) hint(what string) { s.tip = what }

func (s *status) hintText() string { return s.tip }

// closeHint clears the hint, if it is still what the toast said.
func (s *status) closeHint(what string) {
	if s.tip == what {
		s.tip = ""
	}
}

// closeError closes the error toast: the error goes, and a run of
// failures with it.
func (s *status) closeError() { s.shown, s.failed, s.fails = nil, "", 0 }

// clear drops everything, as turning the AI off does.
func (s *status) clear() { *s = status{} }

func (s *status) warn(err error) {
	if err != nil {
		s.setError(err)
	}
}
