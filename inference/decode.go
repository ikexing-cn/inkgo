package inference

import (
	"errors"
	"strings"

	"writer/internal/strictjson"
)

// Decode reads exactly one strict JSON value from a model's answer,
// allowing a ```json fence.
func Decode(s string, v any) error {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json\n") && strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "```json\n"), "```")
	}
	if err := strictjson.Decode([]byte(s), v); err != nil {
		if errors.Is(err, strictjson.ErrTrailing) {
			return ErrTrailingJSON
		}
		return &JSONError{Err: err}
	}
	return nil
}
