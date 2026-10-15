package resource

import (
	"context"
	"sort"

	"github.com/pkg/errors"
)

// MultiAPIResource is a resource that serves more than one co-equal API from a single identity (a
// "composite"). AsType (or an API package's FromProvider helper) returns the sub-resource for a given API.
type MultiAPIResource interface {
	Resource

	// ResourceForAPI returns the sub-resource serving the given API, and whether this composite
	// serves it. The returned sub-resource is the concrete typed client/instance for that API.
	ResourceForAPI(api API) (Resource, bool)

	// APIs returns the set of APIs this composite serves, in a stable order whose first entry is the
	// canonical API.
	APIs() []API
}

// compositeResource is the default MultiAPIResource: one name and a typed sub-resource per API it
// serves — apis in canonical-first order, byAPI mapping each API to its sub-resource.
type compositeResource struct {
	name  Name
	apis  []API
	byAPI map[API]Resource
}

// NewMultiAPIResource assembles a composite from a name and a per-API set of sub-resources. apis
// gives the stable order; the first entry is the canonical API that DoCommand, Status, and Close
// route to. Every api in apis must have an entry in byAPI; violating that is a programmer error and
// panics (callers should build byAPI to cover apis, as Compose and the robot client do). Both apis
// and byAPI are copied, so the caller may reuse or mutate them afterward without affecting the
// composite.
func NewMultiAPIResource(name Name, apis []API, byAPI map[API]Resource) MultiAPIResource {
	if len(apis) == 0 {
		panic("NewMultiAPIResource requires at least one api")
	}
	subs := make(map[API]Resource, len(apis))
	for _, api := range apis {
		sub, ok := byAPI[api]
		if !ok {
			panic(errors.Errorf("NewMultiAPIResource: no sub-resource for api: %q", api))
		}
		subs[api] = sub
	}
	return &compositeResource{name: name, apis: append([]API(nil), apis...), byAPI: subs}
}

// Sub is a sub-resource tagged with the API it serves within a composite. It is the input to Compose:
// each Sub pairs one API with the implementation that carries that API's methods.
type Sub struct {
	API API
	Res Resource
}

// AsSub tags res as the sub-resource serving api, checking at compile time (via type parameter T) that
// res implements the API's interface, so a facade wired to the wrong API fails to compile.
func AsSub[T Resource](api API, res T) Sub {
	return Sub{API: api, Res: res}
}

// Compose assembles a composite MultiAPIResource from per-API sub-resources. The served APIs are
// sorted by API string (matching RegisterMultiAPISet) so the first is a deterministic canonical API.
// It errors if no subs are given, if two subs share an
// API, or if the subs disagree on their name. Compose is the ergonomic, type-checked front door to
// NewMultiAPIResource (pair it with AsSub).
func Compose(subs ...Sub) (MultiAPIResource, error) {
	if len(subs) == 0 {
		return nil, errors.New("Compose requires at least one sub-resource")
	}
	name := subs[0].Res.Name()
	apis := make([]API, 0, len(subs))
	byAPI := make(map[API]Resource, len(subs))
	for _, sub := range subs {
		if sub.Res.Name() != name {
			return nil, errors.Errorf("Compose given sub-resources with mismatched names: %q and %q", name, sub.Res.Name())
		}
		if _, dup := byAPI[sub.API]; dup {
			return nil, errors.Errorf("Compose given duplicate sub-resources for api: %q", sub.API)
		}
		byAPI[sub.API] = sub.Res
		apis = append(apis, sub.API)
	}
	sort.Slice(apis, func(i, j int) bool { return apis[i].String() < apis[j].String() })
	return NewMultiAPIResource(name, apis, byAPI), nil
}

func (c *compositeResource) Name() Name { return c.name }

// APIs returns a copy of the composite's served APIs in canonical-first order.
func (c *compositeResource) APIs() []API { return append([]API(nil), c.apis...) }

func (c *compositeResource) ResourceForAPI(api API) (Resource, bool) {
	sub, ok := c.byAPI[api]
	return sub, ok
}

// canonicalSub returns the sub-resource for the canonical API (apis[0]). NewMultiAPIResource
// validates at construction that the canonical API has a byAPI entry, so this is always non-nil for a
// composite built through the constructor.
func (c *compositeResource) canonicalSub() Resource {
	return c.byAPI[c.apis[0]]
}

// DoCommand routes the command to the canonical sub-resource.
func (c *compositeResource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return c.canonicalSub().DoCommand(ctx, cmd)
}

// Status routes to the canonical sub-resource.
func (c *compositeResource) Status(ctx context.Context) (map[string]interface{}, error) {
	return c.canonicalSub().Status(ctx)
}

// Close closes the composite once, through its canonical sub-resource. Its subs are either one shared
// impl (closing each would double-close it) or per-API views the creator owns, so one Close is enough.
func (c *compositeResource) Close(ctx context.Context) error {
	return c.canonicalSub().Close(ctx)
}
