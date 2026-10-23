package resource

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// MultiAPIResource is a resource that serves more than one co-equal API from a single identity (a
// "composite"). AsType (or an API package's FromProvider helper) returns the sub-resource for a given API.
type MultiAPIResource interface {
	Resource

	// ResourceForAPI returns the sub-resource serving the given API, and whether this composite
	// serves it. The returned sub-resource is the concrete typed client/instance for that API.
	ResourceForAPI(api API) (Resource, bool)

	// APIs returns the set of APIs this composite serves, in a stable order.
	APIs() []API
}

// compositeResource is the default MultiAPIResource: one name and a typed sub-resource per API it
// serves. byAPI's keys are the served APIs; canonical (the sorted-first served API) is derived once at
// construction, and APIs() derives the sorted set on demand.
type compositeResource struct {
	name      Name
	byAPI     map[API]Resource
	canonical API
}

// NewMultiAPIResource assembles a composite from a name and a per-API set of sub-resources. byAPI maps
// each served API to its sub-resource and must be non-empty (empty panics). The canonical API is the
// sorted-first served API. byAPI is copied, so the caller may reuse or mutate it afterward without
// affecting the composite.
func NewMultiAPIResource(name Name, byAPI map[API]Resource) MultiAPIResource {
	if len(byAPI) == 0 {
		panic("NewMultiAPIResource requires at least one api")
	}
	subs := make(map[API]Resource, len(byAPI))
	var canonical API
	first := true
	for api, sub := range byAPI {
		subs[api] = sub
		if first || api.String() < canonical.String() {
			canonical, first = api, false
		}
	}
	return &compositeResource{name: name, byAPI: subs, canonical: canonical}
}

// Sub is a sub-resource tagged with the API it serves within a composite. It is the input to Compose:
// each Sub pairs one API with the implementation that carries that API's methods.
type Sub struct {
	API API
	Res Resource
}

// AsSub tags res as the sub-resource serving api. Pair it with an API package's AsSub wrapper, whose
// typed parameter checks at compile time that res implements that API's interface.
func AsSub(api API, res Resource) Sub {
	return Sub{API: api, Res: res}
}

// LogName returns the logger and FTDC label for a resource serving apis. A composite is labeled
// "composite/<name>"; an ordinary resource uses its full name.
func LogName(name Name, apis []API) string {
	if len(apis) >= 2 {
		return "composite/" + name.ShortName()
	}
	return name.String()
}

// Compose assembles a composite MultiAPIResource from per-API sub-resources. It errors if no subs are
// given, if two subs share an API, or if the subs disagree on their name. Compose is the ergonomic,
// type-checked front door to NewMultiAPIResource (pair it with AsSub).
func Compose(subs ...Sub) (MultiAPIResource, error) {
	if len(subs) == 0 {
		return nil, errors.New("Compose requires at least one sub-resource")
	}
	name := subs[0].Res.Name()
	byAPI := make(map[API]Resource, len(subs))
	for _, sub := range subs {
		if sub.Res.Name() != name {
			return nil, errors.Errorf("Compose given sub-resources with mismatched names: %q and %q", name, sub.Res.Name())
		}
		if _, dup := byAPI[sub.API]; dup {
			return nil, errors.Errorf("Compose given duplicate sub-resources for api: %q", sub.API)
		}
		byAPI[sub.API] = sub.Res
	}
	return NewMultiAPIResource(name, byAPI), nil
}

func (c *compositeResource) Name() Name { return c.name }

func (c *compositeResource) APIs() []API {
	apis := make([]API, 0, len(c.byAPI))
	for api := range c.byAPI {
		apis = append(apis, api)
	}
	SortAPIs(apis)
	return apis
}

func (c *compositeResource) ResourceForAPI(api API) (Resource, bool) {
	sub, ok := c.byAPI[api]
	return sub, ok
}

// canonicalSub returns the sub-resource for the canonical API. byAPI is non-empty for a composite built
// through the constructor, so this is always non-nil.
func (c *compositeResource) canonicalSub() Resource {
	return c.byAPI[c.canonical]
}

func (c *compositeResource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return c.canonicalSub().DoCommand(ctx, cmd)
}

func (c *compositeResource) Status(ctx context.Context) (map[string]interface{}, error) {
	return c.canonicalSub().Status(ctx)
}

// Close closes each distinct sub-resource once. Separate per-API objects each own resources that would
// leak if not closed; a shared impl reached under several APIs is closed once, and Close must be
// idempotent either way.
func (c *compositeResource) Close(ctx context.Context) error {
	seen := make(map[Resource]bool)
	var errs error
	for _, api := range c.APIs() {
		sub := c.byAPI[api]
		if seen[sub] {
			continue
		}
		seen[sub] = true
		errs = multierr.Combine(errs, sub.Close(ctx))
	}
	return errs
}
