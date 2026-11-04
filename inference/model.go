// Package inference holds the model and answer rules used by the talk and the pen.
package inference

import (
	"context"
	"errors"

	"writer/model"
)

// Model is a model client with the settings it runs at. A request sends
// at most Input tokens, and its reply is cut at Output.
type Model struct {
	model.Completer
	model.Model
	Name   string // provider/model, recorded with what it writes
	Effort model.Effort
}

// Ready tells whether m has a client: a work with no model set up has none.
func (m Model) Ready() bool { return m.Completer != nil }

// Complete asks of the reply all the model gives, or less where req asks
// less. Cut off at all the model gives, it says so, and where to give it
// more.
func (m Model) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	whole := req.MaxTokens == 0 || req.MaxTokens >= m.Output
	if whole {
		req.MaxTokens = m.Output
	}
	reply, err := m.Completer.Complete(ctx, req, on)
	if whole && errors.Is(err, model.ErrTruncated) {
		err = &model.SetupError{Err: &OutputLimitError{Limit: m.Output}}
	}
	return reply, err
}
