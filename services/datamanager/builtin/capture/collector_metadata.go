package capture

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

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
// api is the method's owning API. It resolves that API itself (via captureAPIForMethod) so every caller
// keys the collector the same way.
func newCollectorMetadata(res resource.Resource, c datamanager.DataCaptureConfig) collectorMetadata {
	return collectorMetadata{
		ResourceName: c.Name.ShortName(),
		MethodParams: fmt.Sprintf("%v", c.AdditionalParams),
		MethodMetadata: data.MethodMetadata{
			API:        captureAPIForMethod(res, c.Method, c.Name.API),
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

// unresolvedCaptureMethodError returns an actionable error when a composite capture method cannot be
// resolved to a single owning API, or nil when it can (a single-API resource, or exactly one owner).
func unresolvedCaptureMethodError(res resource.Resource, cfg datamanager.DataCaptureConfig) error {
	apis := resource.APIsOf(res)
	if len(apis) < 2 {
		return nil
	}
	owners := make([]string, 0, len(apis))
	for _, api := range apis {
		if data.CollectorLookup(data.MethodMetadata{API: api, MethodName: cfg.Method}) != nil {
			owners = append(owners, api.String())
		}
	}
	switch len(owners) {
	case 0:
		return errors.Errorf("composite %q serves no API that implements capture method %q",
			cfg.Name.ShortName(), cfg.Method)
	case 1:
		return nil
	default:
		example := fmt.Sprintf("%s/%s", owners[0], cfg.Name.ShortName())
		return errors.Errorf(
			"capture method %q on composite %q is served by multiple APIs (%s); set the capture config name to the full name of the API to capture, e.g. %q",
			cfg.Method, cfg.Name.ShortName(), strings.Join(owners, ", "), example)
	}
}
