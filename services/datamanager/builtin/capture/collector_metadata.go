package capture

import (
	"fmt"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
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
		"[Resource Name: %s, API: %s, Method Name: %s, Method Params: %s]",
		r.ResourceName, r.MethodMetadata.API, r.MethodMetadata.MethodName, r.MethodParams,
	)
}

// newCollectorMetadata keys a collector by resource short name, method params, and the (api, method)
// pair. api is the API that owns the method (see captureAPIForMethod), not necessarily the resource's
// configured API: a composite's methods therefore key into distinct per-API streams even though they
// share one resource name, and a method name shared across a composite's APIs never collides.
func newCollectorMetadata(c datamanager.DataCaptureConfig, api resource.API) collectorMetadata {
	return collectorMetadata{
		ResourceName: c.Name.ShortName(),
		MethodParams: fmt.Sprintf("%v", c.AdditionalParams),
		MethodMetadata: data.MethodMetadata{
			API:        api,
			MethodName: c.Method,
		},
	}
}

// captureAPIForMethod resolves which of a resource's co-equal APIs owns a capture method, so a
// composite's data is tagged and streamed under the API that actually implements the method rather
// than under the resource's configured API. A single-API resource always yields its one API. For a
// composite: a method implemented by exactly one served API resolves to that API (e.g. Readings ->
// sensor, GetImage -> camera); a method several served APIs share (e.g. DoCommand) stays under the
// configured API, matching the single shared implementation the server routes such calls to; a method
// no served API implements also falls back to the configured API, where the collector lookup then
// fails with a clear error.
func captureAPIForMethod(res resource.Resource, method string, configuredAPI resource.API) resource.API {
	apis := resource.APIsOf(res)
	if len(apis) == 1 {
		return apis[0]
	}

	var owner resource.API
	owners := 0
	for _, api := range apis {
		if data.CollectorLookup(data.MethodMetadata{API: api, MethodName: method}) != nil {
			owner = api
			owners++
		}
	}
	if owners == 1 {
		return owner
	}
	return configuredAPI
}
