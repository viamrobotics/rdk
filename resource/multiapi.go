package resource

import (
	"context"

	"go.uber.org/multierr"
)

// MultiAPIResource is a resource that serves more than one co-equal API from a single identity — a
// "composite". Consumers get a typed handle for any of its APIs via AsType (or an API package's
// FromProvider helper), both of which unwrap the composite to the sub-resource for the requested
// API, so custom/module APIs work the same way builtin ones do.
type MultiAPIResource interface {
	Resource

	// ResourceForAPI returns the sub-resource serving the given API, and whether this composite
	// serves it. The returned sub-resource is the concrete typed client/instance for that API.
	ResourceForAPI(api API) (Resource, bool)

	// APIs returns the set of APIs this composite serves, in a stable (declared) order.
	APIs() []API
}

// compositeResource is the default MultiAPIResource: one identity holding a typed sub-resource per
// API. On the robot/in-process side the sub-resources may all be the same underlying instance; the
// client builds one per advertised API on a shared connection. Composites always rebuild, never
// reconfigure in place (they are reassembled from their sub-resources), hence AlwaysRebuild.
type compositeResource struct {
	AlwaysRebuild
	name  Name
	apis  []API
	byAPI map[API]Resource
}

// NewMultiAPIResource assembles a composite from a name and a per-API set of sub-resources. apis
// gives the stable order (first entry is the canonical/DoCommand-preferred API). Every api in apis
// must have an entry in byAPI.
func NewMultiAPIResource(name Name, apis []API, byAPI map[API]Resource) MultiAPIResource {
	return &compositeResource{name: name, apis: append([]API(nil), apis...), byAPI: byAPI}
}

func (c *compositeResource) Name() Name { return c.name }

func (c *compositeResource) APIs() []API { return c.apis }

func (c *compositeResource) ResourceForAPI(api API) (Resource, bool) {
	sub, ok := c.byAPI[api]
	return sub, ok
}

// DoCommand routes a bare command to the first sub-resource whose DoCommand is implemented. On the
// server every API of a composite resolves to one instance with one DoCommand, so all routes
// converge; the choice only needs to be deterministic. Access a specific API via AsType to sidestep
// this entirely.
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
		errs = multierr.Combine(errs, sub.Close(ctx))
	}
	return errs
}

// subresourceForAPI unwraps a composite to the sub-resource serving api. If res is not a composite
// (or does not serve api) it is returned unchanged. This is the general, open-world access path used
// by AsType, FromDependencies, and FromProvider.
func subresourceForAPI(res Resource, api API) Resource {
	if mar, ok := res.(MultiAPIResource); ok {
		if sub, ok := mar.ResourceForAPI(api); ok {
			return sub
		}
	}
	return res
}

// APIsOf returns the set of APIs a resource handle serves. For a composite (multi-API) resource it
// returns every API it serves, in a stable order; for an ordinary resource it returns the single API
// of its Name. It lets a consumer discover a handle's capabilities without a MultiAPIResource type
// assertion or trial-and-error AsType, and is the runtime counterpart to APIsForModel (which answers
// the same question from a model, before construction).
func APIsOf(res Resource) []API {
	if mar, ok := res.(MultiAPIResource); ok {
		return mar.APIs()
	}
	return []API{res.Name().API}
}

// NamedFromProvider resolves a bare (API-less) resource name to its single resource via any Provider,
// superseding lookups that require a fully-qualified Name+API. For a composite it returns the one
// handle serving every API; extract a specific API from it with AsType.
func NamedFromProvider(provider Provider, name string) (Resource, error) {
	return provider.GetResource(SimpleName(name))
}
