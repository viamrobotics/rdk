package capture

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

// collectorMetadata identifies a collector by resource short name, method params, and (API, method).
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

// newCollectorMetadata keys a collector by resource short name, method params, and (api, method).
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

// apisByShortName groups dependency names by short name, giving the set of APIs each resource serves,
// canonical-first. A composite appears under one short name with several APIs; an ordinary resource has one.
func apisByShortName(deps resource.Dependencies) map[string][]resource.API {
	byName := map[string][]resource.API{}
	for name := range deps {
		byName[name.ShortName()] = append(byName[name.ShortName()], name.API)
	}
	for _, apis := range byName {
		resource.SortAPIs(apis)
	}
	return byName
}

// OwningResource resolves the resource and API that own cfg's capture method. For a composite it picks the
// API that implements the method; for an ordinary resource it is the resource's own API. It errors if the
// method cannot be resolved to one API, or the resolved resource is missing from deps.
func OwningResource(deps resource.Dependencies, cfg datamanager.DataCaptureConfig) (resource.Resource, resource.API, error) {
	apis := apisByShortName(deps)[cfg.Name.ShortName()]
	owningAPI := captureAPIForMethod(apis, cfg.Method, cfg.Name.API)
	// The owning API must be one the resource serves; if not, the config pinned an API it doesn't have.
	if len(apis) > 0 && !slices.Contains(apis, owningAPI) {
		return nil, resource.API{}, errors.Errorf("capture config for %q names API %q, which it does not serve (serves %v)",
			cfg.Name.ShortName(), owningAPI, apis)
	}
	// The resolved API doesn't collect this method, which for a composite means it isn't owned by exactly
	// one of the composite's APIs — report whether several own it (pin one) or none do.
	if data.CollectorLookup(data.MethodMetadata{API: owningAPI, MethodName: cfg.Method}) == nil {
		if err := unresolvedCaptureMethodError(apis, cfg); err != nil {
			return nil, resource.API{}, err
		}
	}
	sub, err := deps.Lookup(resource.NewName(owningAPI, cfg.Name.ShortName()))
	if err != nil {
		return nil, resource.API{}, err
	}
	return sub, owningAPI, nil
}

// owningResourceForAutoEnable resolves the owning resource and API for a (shortName, method) the control
// sensor named, where no configured API is available (so the canonical API is the fallback). Returns the
// resource, its owning API, and whether it resolved.
func owningResourceForAutoEnable(deps resource.Dependencies, shortName, method string) (resource.Resource, resource.API, bool) {
	apis := apisByShortName(deps)[shortName]
	if len(apis) == 0 {
		return nil, resource.API{}, false
	}
	owningAPI := captureAPIForMethod(apis, method, apis[0])
	sub, err := deps.Lookup(resource.NewName(owningAPI, shortName))
	if err != nil {
		return nil, resource.API{}, false
	}
	return sub, owningAPI, true
}

// captureAPIForMethod resolves which of a resource's APIs owns a capture method, so its data is tagged
// under the implementing API. A single-API resource yields its one API. For a composite, a method
// exactly one API implements resolves to that API (Readings -> sensor, GetImage -> camera); a method
// that several APIs share or collide on by name (e.g. DoCommand), or that no API implements, stays under
// the configured API.
func captureAPIForMethod(apis []resource.API, method string, configuredAPI resource.API) resource.API {
	switch len(apis) {
	case 0:
		return configuredAPI
	case 1:
		return apis[0]
	}
	if owners := captureMethodOwners(apis, method); len(owners) == 1 {
		return owners[0]
	}
	return configuredAPI
}

// captureMethodOwners returns every API in apis that registers a collector for method, in the given
// order. Exactly one owner means the method unambiguously belongs to that API.
func captureMethodOwners(apis []resource.API, method string) []resource.API {
	var owners []resource.API
	for _, api := range apis {
		if data.CollectorLookup(data.MethodMetadata{API: api, MethodName: method}) != nil {
			owners = append(owners, api)
		}
	}
	return owners
}

// unresolvedCaptureMethodError returns an actionable error when a composite capture method cannot be
// resolved to a single owning API, or nil when it can (a single-API resource, or exactly one owner).
func unresolvedCaptureMethodError(apis []resource.API, cfg datamanager.DataCaptureConfig) error {
	if len(apis) < 2 {
		return nil
	}
	owners := captureMethodOwners(apis, cfg.Method)
	switch len(owners) {
	case 0:
		return errors.Errorf("composite %q serves no API that implements capture method %q",
			cfg.Name.ShortName(), cfg.Method)
	case 1:
		return nil
	default:
		names := make([]string, len(owners))
		for i, api := range owners {
			names[i] = api.String()
		}
		example := fmt.Sprintf("%s/%s", names[0], cfg.Name.ShortName())
		return errors.Errorf("capture method %q on composite %q is served by multiple APIs (%s); pin the name to the owning API, e.g. %q",
			cfg.Method, cfg.Name.ShortName(), strings.Join(names, ", "), example)
	}
}
