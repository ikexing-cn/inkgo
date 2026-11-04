package inference

import (
	"errors"
	"fmt"
)

// ErrTrailingJSON reports multiple JSON values or trailing content.
var ErrTrailingJSON = errors.New("inference: multiple JSON values or trailing content")

// JSONError wraps an answer that violates the expected JSON contract.
type JSONError struct{ Err error }

func (e *JSONError) Error() string { return fmt.Sprintf("inference: unexpected JSON: %v", e.Err) }

func (e *JSONError) Unwrap() error { return e.Err }

// OutputLimitError records the maximum reply token count.
type OutputLimitError struct{ Limit int }

func (e *OutputLimitError) Error() string {
	return fmt.Sprintf("inference: reply exceeds maximum output %d", e.Limit)
}
