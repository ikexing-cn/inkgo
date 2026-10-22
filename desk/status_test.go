package desk

import (
	"errors"
	"testing"
)

func TestStatusCountsFailuresInARow(t *testing.T) {
	s := status{}
	a, b := errors.New("网络失败"), errors.New("连接超时")
	s.fail(a)
	if n := s.repeated(); n != 1 {
		t.Fatalf("first failure counts %d", n)
	}
	s.fail(a)
	if n := s.repeated(); n != 2 {
		t.Fatalf("the same failure counts %d", n)
	}
	s.fail(b)
	if n := s.repeated(); n != 1 || s.errorText() != b.Error() {
		t.Fatalf("another failure counts %d, shows %q", n, s.errorText())
	}
}

func TestStatusASuccessEndsTheRun(t *testing.T) {
	s := status{}
	a := errors.New("网络失败")
	s.fail(a)
	s.fail(a)
	s.succeed()
	if n := s.repeated(); n != 0 {
		t.Fatalf("after a success the run counts %d", n)
	}
	s.fail(a)
	if n := s.repeated(); n != 1 {
		t.Fatalf("a failure after a success counts %d", n)
	}
}

func TestStatusClosingTheErrorClearsItsRun(t *testing.T) {
	s := status{}
	a := errors.New("网络失败")
	s.fail(a)
	s.fail(a)
	s.closeError()
	if s.err() != nil || s.errorText() != "" || s.repeated() != 0 {
		t.Fatalf("closing left %v, %q, %d", s.err(), s.errorText(), s.repeated())
	}
	s.fail(a)
	if n := s.repeated(); n != 1 {
		t.Fatalf("a failure after closing counts %d", n)
	}
}

// 「再试一次」 is offered while the error shown is the last request's: not
// for another error, and not after that request succeeded.
func TestStatusOffersRetryForTheShownFailure(t *testing.T) {
	s := status{}
	ran := false
	s.retry(func() { ran = true })
	a := errors.New("网络失败")
	s.fail(a)
	again := s.retryAction()
	if again == nil {
		t.Fatal("no retry for the failure shown")
	}
	again()
	if !ran {
		t.Fatal("the retry did not run")
	}
	s.setError(errors.New("连接超时"))
	if s.retryAction() != nil {
		t.Fatal("another error offers the retry")
	}
	s.setError(a)
	if s.retryAction() == nil {
		t.Fatal("the last failure's error does not offer the retry")
	}
	s.succeed()
	if s.retryAction() != nil {
		t.Fatal("a success offers the retry")
	}
}

func TestStatusClearDropsEverything(t *testing.T) {
	s := status{}
	s.fail(errors.New("网络失败"))
	s.retry(func() {})
	s.did("已保存")
	s.hint("请先选中")
	s.clear()
	if s.err() != nil || s.errorText() != "" || s.noticeText() != "" || s.hintText() != "" || s.repeated() != 0 || s.retryAction() != nil {
		t.Fatalf("clear left %+v", s)
	}
}
