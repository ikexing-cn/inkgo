package kit

import (
	"errors"
	"testing"
)

func TestStatusCountsFailuresInARow(t *testing.T) {
	s := Status{}
	a, b := errors.New("网络失败"), errors.New("连接超时")
	s.Fail(a)
	if n := s.Repeated(); n != 1 {
		t.Fatalf("first failure counts %d", n)
	}
	s.Fail(a)
	if n := s.Repeated(); n != 2 {
		t.Fatalf("the same failure counts %d", n)
	}
	s.Fail(b)
	if n := s.Repeated(); n != 1 || s.ErrorText() != b.Error() {
		t.Fatalf("another failure counts %d, shows %q", n, s.ErrorText())
	}
}

func TestStatusASuccessEndsTheRun(t *testing.T) {
	s := Status{}
	a := errors.New("网络失败")
	s.Fail(a)
	s.Fail(a)
	s.Succeed()
	if n := s.Repeated(); n != 0 {
		t.Fatalf("after a success the run counts %d", n)
	}
	s.Fail(a)
	if n := s.Repeated(); n != 1 {
		t.Fatalf("a failure after a success counts %d", n)
	}
}

func TestStatusClosingTheErrorClearsItsRun(t *testing.T) {
	s := Status{}
	a := errors.New("网络失败")
	s.Fail(a)
	s.Fail(a)
	s.CloseError()
	if s.Err() != nil || s.ErrorText() != "" || s.Repeated() != 0 {
		t.Fatalf("closing left %v, %q, %d", s.Err(), s.ErrorText(), s.Repeated())
	}
	s.Fail(a)
	if n := s.Repeated(); n != 1 {
		t.Fatalf("a failure after closing counts %d", n)
	}
}

// 「再试一次」 is offered while the error shown is the last request's: not
// for another error, and not after that request succeeded.
func TestStatusOffersRetryForTheShownFailure(t *testing.T) {
	s := Status{}
	ran := false
	s.Retry(func() { ran = true })
	a := errors.New("网络失败")
	s.Fail(a)
	again := s.RetryAction()
	if again == nil {
		t.Fatal("no retry for the failure shown")
	}
	again()
	if !ran {
		t.Fatal("the retry did not run")
	}
	s.SetError(errors.New("连接超时"))
	if s.RetryAction() != nil {
		t.Fatal("another error offers the retry")
	}
	s.SetError(a)
	if s.RetryAction() == nil {
		t.Fatal("the last failure's error does not offer the retry")
	}
	s.Succeed()
	if s.RetryAction() != nil {
		t.Fatal("a success offers the retry")
	}
}

func TestStatusClearDropsEverything(t *testing.T) {
	s := Status{}
	s.Fail(errors.New("网络失败"))
	s.Retry(func() {})
	s.Did("已保存")
	s.Hint("请先选中")
	s.Clear()
	if s.Err() != nil || s.ErrorText() != "" || s.NoticeText() != "" || s.HintText() != "" || s.Repeated() != 0 || s.RetryAction() != nil {
		t.Fatalf("clear left %+v", s)
	}
}
