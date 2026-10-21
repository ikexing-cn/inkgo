package inference

import (
	"errors"
	"fmt"
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
			return errors.New("返回了多个 JSON 值或尾随内容")
		}
		return fmt.Errorf("JSON 不符合约定：%w", err)
	}
	return nil
}
