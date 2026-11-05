package check

import (
	"encoding/json"
	"fmt"
)

func pendingKey(k checkKey) string { return fmt.Sprint(k) }
func (a *Panel) loadState(s map[string]json.RawMessage) {
	_ = json.Unmarshal(s["checkWhen"], &a.State.When)
}
func (a *Panel) saveState() map[string]json.RawMessage {
	b, _ := json.Marshal(a.State.When)
	return map[string]json.RawMessage{"checkWhen": b}
}
