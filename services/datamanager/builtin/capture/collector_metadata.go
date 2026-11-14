package capture

import (
	"fmt"
	"slices"

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

// ResolvedCollector pairs a collector's resolved identity (metadata) with its config. ResolveCollector is
// its only producer, so resolution and keying happen once and downstream sites reuse the metadata.
type ResolvedCollector struct {
	Metadata collectorMetadata
	Config   datamanager.DataCaptureConfig
}

// CollectorResolver resolves capture configs against a dependency set, indexing each resource's served
// APIs once so resolving many methods (and every capture-control poll) doesn't rebuild the index.
type CollectorResolver struct {
	deps       resource.Dependencies
	apisByName map[string][]resource.API
}

// NewCollectorResolver indexes deps by short name for repeated resolution.
func NewCollectorResolver(deps resource.Dependencies) *CollectorResolver {
	return &CollectorResolver{deps: deps, apisByName: apisByShortName(deps)}
}

// Resolve resolves the resource serving cfg's capture method and returns it alongside a ResolvedCollector
// (the collector's metadata plus cfg pinned to the owning API). For a composite it picks the API that
// implements the method; for an ordinary resource it is the resource's own API. It errors if the method
// cannot be resolved to one API, or the resolved resource is missing from deps.
func (r *CollectorResolver) Resolve(cfg datamanager.DataCaptureConfig) (ResolvedCollector, resource.Resource, error) {
	owningAPI, err := resolveOwningAPI(r.apisByName[cfg.Name.ShortName()], cfg)
	if err != nil {
		return ResolvedCollector{}, nil, err
	}
	sub, err := r.deps.Lookup(resource.NewName(owningAPI, cfg.Name.ShortName()))
	if err != nil {
		return ResolvedCollector{}, nil, err
	}
	// Pin the config to the resolved owning API so the metadata and every downstream key use it.
	cfg.Name = cfg.Name.AsAPI(owningAPI)
	return ResolvedCollector{Metadata: newCollectorMetadata(cfg), Config: cfg}, sub, nil
}

// resolveOwningAPI picks the API that owns cfg's capture method from the APIs its resource serves. A real
// pin (cfg.PinnedAPI) must be one of the resource's APIs and must capture the method, else it errors rather
// than resolve elsewhere. Without a pin, a method exactly one API captures resolves to that API
// (Readings -> sensor); a method several APIs share (e.g. DoCommand) stays under the configured API that
// name resolution filled into cfg.Name; a single-API resource uses its one API.
func resolveOwningAPI(apis []resource.API, cfg datamanager.DataCaptureConfig) (resource.API, error) {
	short := cfg.Name.ShortName()
	owners := captureMethodOwners(apis, cfg.Method)
	if cfg.PinnedAPI.Validate() == nil {
		pin := cfg.PinnedAPI
		if len(apis) > 0 && !slices.Contains(apis, pin) {
			return resource.API{}, errors.Errorf(
				"capture config for %q pins API %q, which it does not serve (serves %v)", short, pin, apis,
			)
		}
		if len(apis) > 1 && !slices.Contains(owners, pin) {
			return resource.API{}, errors.Errorf(
				"capture config for %q pins API %q, which does not capture method %q (captured by %v)",
				short, pin, cfg.Method, owners,
			)
		}
		return pin, nil
	}
	switch {
	case len(apis) == 0:
		// Not in deps; return the configured API so the caller's Lookup fails with a clear missing-resource error.
		return cfg.Name.API, nil
	case len(apis) == 1:
		return apis[0], nil
	case len(owners) == 1:
		return owners[0], nil
	case len(owners) == 0:
		return resource.API{}, errors.Errorf(
			"composite %q serves no API that captures method %q", short, cfg.Method,
		)
	default:
		// Several of the composite's APIs capture the method (e.g. DoCommand). Keep it under the
		// configured API that name resolution filled into cfg.Name when that API is one of them;
		// otherwise it is genuinely ambiguous and the user must pin one.
		if slices.Contains(owners, cfg.Name.API) {
			return cfg.Name.API, nil
		}
		return resource.API{}, errors.Errorf(
			"capture method %q on composite %q is captured by multiple APIs (%v); pin the name to one",
			cfg.Method, short, owners,
		)
	}
}

// apisByShortName groups dependency names by short name, giving the sorted set of APIs each resource serves.
// A composite appears under one short name with several APIs; an ordinary resource has one.
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
