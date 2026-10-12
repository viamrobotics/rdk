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

// newCollectorMetadata keys a collector by resource short name, method params, and (api, method), where
// api is the method's owning API (see captureAPIForMethod).
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

// captureAPIForMethod resolves which of a resource's APIs owns a capture method, so its data is tagged
// under the implementing API. A single-API resource yields its one API. For a composite, a method
// exactly one API implements resolves to that API (Readings -> sensor, GetImage -> camera); a method
// that several APIs share or collide on by name (e.g. DoCommand), or that no API implements, stays under
// the configured API.
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
