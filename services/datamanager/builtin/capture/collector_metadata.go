package capture

import (
	"fmt"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/services/datamanager"
)

// method parameters, and method name.
type collectorMetadata struct {
	ResourceName   string
	MethodParams   string
	MethodMetadata data.MethodMetadata
}

func (r collectorMetadata) String() string {
	return fmt.Sprintf(
		"[API: %s, Resource Name: %s, Method Name: %s, Method Params: %s]",
		r.MethodMetadata.API, r.ResourceName, r.MethodMetadata.MethodName, r.MethodParams)
}

func newCollectorMetadata(c datamanager.DataCaptureConfig) collectorMetadata {
	return collectorMetadata{
		ResourceName: c.Name.ShortName(),
		MethodParams: fmt.Sprintf("%v", c.AdditionalParams),
		MethodMetadata: data.MethodMetadata{
			API:        c.Name.API,
			MethodName: c.Method,
		},
	}
}
