package aiport

import (
	"errors"
	"testing"
)

func TestProblemsFirstKeepsErrorIdentityAndFieldOrder(t *testing.T) {
	first, second := errors.New("first"), errors.New("second")
	for _, tc := range []struct {
		problems Problems
		want     error
	}{
		{Problems{}, nil},
		{Problems{ID: first, Name: second}, first},
		{Problems{Name: first, Protocol: second}, first},
		{Problems{Protocol: first, BaseURL: second}, first},
		{Problems{BaseURL: first, Env: second}, first},
		{Problems{Env: first, Models: second}, first},
		{Problems{Models: first}, first},
	} {
		if tc.problems.First() != tc.want {
			t.Errorf("%+v: %v", tc.problems, tc.problems.First())
		}
	}
}
