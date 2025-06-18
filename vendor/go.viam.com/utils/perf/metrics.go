package perf

import (
	"go.opencensus.io/plugin/runmetrics"
	"go.uber.org/multierr"

	"go.viam.com/utils"
)

// registerApplicationViews registers all the default views we may need for the application. gRPC, MongoDB, HTTP, etc...
func registerApplicationViews() error {
	utils.UncheckedError(runmetrics.Enable(runmetrics.RunMetricOptions{
		EnableCPU:    true,
		EnableMemory: true,
	}))

	return multierr.Combine(
		registerGrpcViews(),
		registerHTTPViews(),
	)
}
