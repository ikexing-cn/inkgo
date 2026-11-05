package hostapi

import (
	"context"
	"writer/ext"
)

// Run starts worker-only work owned and joined by the activation. It must not wait for a frame.
func Run(jobs Jobs, s *ext.Session, work func(context.Context)) {
	if runner, ok := jobs.(interface{ Run(func(context.Context)) }); ok {
		runner.Run(work)
		return
	}
	s.Go(work)
}
