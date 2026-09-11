package resource

import (
	"context"

	"go.uber.org/multierr"
)

// MultiAPIResource is a resource that serves more than one API from a single identity — a
// "composite". On the client side, ResourceByName and the dependencies map return one composite
// object for a multi-API resource; consumers get a typed handle for any of its APIs via that API
// package's FromResource helper (which calls FromResourceForAPI), so custom/module APIs work the
// same way builtin ones do — no closed set of embedded clients required.
type MultiAPIResource interface {
	Resource

	// ResourceForAPI returns the sub-resource that serves the given API, and whether this composite
	// serves it. The returned sub-resource is the concrete typed client/instance for that API.
	ResourceForAPI(api API) (Resource, bool)

	// APIs returns the set of APIs this composite serves, in a stable order.
	APIs() []API
}

// FromResourceForAPI returns a typed handle for api from res. If res is a composite
// (MultiAPIResource) that serves api, its sub-resource for that API is used; otherwise res itself
// is asserted. This is the general, open-world access path: each API package wraps it in a
// FromResource helper (e.g. arm.FromResource), so the "does this satisfy T" knowledge lives in the
// API's own package rather than in a central superclient.
func FromResourceForAPI[T Resource](res Resource, api API) (T, error) {
	return AsType[T](subresourceForAPI(res, api))
}

// APIsOf returns the set of APIs a resource handle serves. For a composite (multi-API) resource it
// returns every API it was registered under, in a stable order; for an ordinary resource it returns
// the single API of its Name. It lets a consumer discover a handle's capabilities without a
// MultiAPIResource type assertion or trial-and-error AsType, and is the runtime counterpart to
// APIsForModel (which answers the same question from a model, before construction).
func APIsOf(res Resource) []API {
	if mar, ok := res.(MultiAPIResource); ok {
		return mar.APIs()
	}
	return []API{res.Name().API}
}

// compositeResource is the default MultiAPIResource: one identity holding a typed sub-resource per
// API. The client builds one of these for a multi-API resource by dialing each advertised API's
// RPCClient; on the robot/in-process side the sub-resources may all be the same underlying object.
type compositeResource struct {
	name  Name
	apis  []API // declared order, for deterministic DoCommand routing and APIs()
	byAPI map[API]Resource
}

// NewMultiAPIResource assembles a composite from a name and a per-API set of sub-resources. apis
// gives the stable order (first entry is the canonical/DoCommand-preferred API). Every api in apis
// must have an entry in byAPI.
func NewMultiAPIResource(name Name, apis []API, byAPI map[API]Resource) MultiAPIResource {
	ordered := append([]API(nil), apis...)
	return &compositeResource{name: name, apis: ordered, byAPI: byAPI}
}

func (c *compositeResource) Name() Name { return c.name }

func (c *compositeResource) APIs() []API { return c.apis }

func (c *compositeResource) ResourceForAPI(api API) (Resource, bool) {
	sub, ok := c.byAPI[api]
	return sub, ok
}

// DoCommand routes a bare command on the composite to the first sub-resource whose DoCommand is not
// the unimplemented default. On the server the composite's APIs all resolve to one instance with one
// DoCommand method, so every route converges; the choice only needs to be deterministic. Accessing a
// specific API via FromResource sidesteps this entirely.
func (c *compositeResource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	var lastErr error
	for _, api := range c.apis {
		sub, ok := c.byAPI[api]
		if !ok {
			continue
		}
		res, err := sub.DoCommand(ctx, cmd)
		if err == ErrDoUnimplemented {
			lastErr = err
			continue
		}
		return res, err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrDoUnimplemented
}

// Status routes to the canonical (first-declared) sub-resource. As with DoCommand, all APIs of a
// composite resolve to one instance on the server, so any route reports the same status.
func (c *compositeResource) Status(ctx context.Context) (map[string]interface{}, error) {
	for _, api := range c.apis {
		if sub, ok := c.byAPI[api]; ok {
			return sub.Status(ctx)
		}
	}
	return map[string]interface{}{}, nil
}

// Close closes each distinct sub-resource once. Sub-resources may share an underlying object, so a
// resource is closed at most once even when several APIs map to it.
func (c *compositeResource) Close(ctx context.Context) error {
	seen := make(map[Resource]bool, len(c.byAPI))
	var errs error
	for _, api := range c.apis {
		sub, ok := c.byAPI[api]
		if !ok || seen[sub] {
			continue
		}
		seen[sub] = true
		if err := sub.Close(ctx); err != nil {
			errs = multierr.Combine(errs, err)
		}
	}
	return errs
}
