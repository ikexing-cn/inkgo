package ai

import (
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// accountingBackend captures observers when each model handle is constructed.
// Existing handles retain the exact observers and activations they started with.
type accountingBackend struct {
	aiapi.Backend
	calls      *ext.PointReader[func(aiapi.Call)]
	activation *ext.Session
}

func (b accountingBackend) prepare() {
	observer, ok := b.Backend.(interface{ BindCalls(func(aiapi.Call)) })
	if !ok {
		return
	}
	items := b.calls.Items()
	sessions := make([]*ext.Session, len(items))
	for i, item := range items {
		sessions[i] = b.calls.Activation(item.Owner)
	}
	observer.BindCalls(func(call aiapi.Call) {
		if !b.activation.Active() {
			return
		}
		for i, item := range items {
			if sessions[i].Active() {
				observed := call
				observed.Activation = sessions[i]
				_ = b.calls.Guard(item.Owner, item.ID, func() { item.Value(observed) })
			}
		}
	})
}
func (b accountingBackend) Model(s *workspace.Store, r aiapi.Role) (aiapi.Handle, error) {
	b.prepare()
	return b.Backend.Model(s, r)
}
func (b accountingBackend) Pen(s *workspace.Store, m aiapi.Material) (aiapi.Pen, error) {
	b.prepare()
	return b.Backend.Pen(s, m)
}

// BackendSource returns the configured backend for native integration fixtures.
func (b accountingBackend) BackendSource() aiapi.Backend { return b.Backend }
