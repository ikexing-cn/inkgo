package ai

import (
	"writer/ext"
	"writer/ext/ai/aiapi"
)

// accountingBackend captures receiving activations at each request start.
type accountingBackend struct {
	aiapi.Backend
	calls      *ext.PointReader[func(aiapi.Call)]
	activation *ext.Session
}

func (b accountingBackend) prepare() {
	observer, ok := b.Backend.(interface{ BindCalls(func() func(aiapi.Call)) })
	if !ok {
		return
	}
	observer.BindCalls(func() func(aiapi.Call) {
		items, sessions := b.calls.Capture()
		return func(call aiapi.Call) {
			if !b.activation.Active() {
				return
			}
			for _, item := range items {
				session := sessions[item.Owner]
				if session.Active() {
					observed := call
					observed.Activation = session
					_, _ = ext.CallReadPoint(b.calls, item.ID, observed, func(c aiapi.Call) struct{} { item.Value(c); return struct{}{} })
				}
			}
		}
	})
}

// BackendSource returns the configured backend for native integration fixtures.
func (b accountingBackend) BackendSource() aiapi.Backend { return b.Backend }
