package hostapi

import "context"

// Run starts worker-only work owned and joined by the Jobs activation.
func Run(jobs Jobs, work func(context.Context)) { jobs.Run(work) }
