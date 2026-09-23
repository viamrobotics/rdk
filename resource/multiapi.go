package resource

import (
	"context"
	"sort"

	"github.com/pkg/errors"
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

	// APIs returns the set of APIs this composite serves, in a stable order whose first entry is the
	// canonical API (the one DoCommand and Status route to).
	APIs() []API
}

// compositeResource is the default MultiAPIResource: one identity holding a typed sub-resource per
// API. On the robot/in-process side the sub-resources may all be the same underlying instance; the
// robot client builds one sub-client per advertised API on a shared connection. In that client case
// the sub-clients are owned and closed by the robot client's resource-client lifecycle, not by this
// wrapper — the composite is a non-owning view over them (see Close). Composites always rebuild,
// never reconfigure in place (they are reassembled from their sub-resources), hence AlwaysRebuild.
type compositeResource struct {
	AlwaysRebuild
	name  Name
	apis  []API
	byAPI map[API]Resource
}

// NewMultiAPIResource assembles a composite from a name and a per-API set of sub-resources. apis
// gives the stable order; the first entry is the canonical API that DoCommand, Status, and Close
// route to. Every api in apis must have an entry in byAPI; violating that is a programmer error and
// panics, so the canonical route is always resolvable (callers should build byAPI to cover apis, as
// Compose and the robot client do). Both apis and byAPI are copied, so the caller may reuse or mutate
// them afterward without affecting the composite.
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
// each Sub pairs one API with the (facade) implementation that carries that API's methods.
type Sub struct {
	API API
	Res Resource
}

// AsSub tags res as the sub-resource serving api, checking at compile time that res satisfies the API
// interface T. Authors write AsSub[someAPIInterface](theAPI, impl); the type parameter is what forces
// impl to implement someAPIInterface, so a facade wired to the wrong API fails to compile rather than
// at runtime. Per-API packages wrap this with sugar (e.g. camera.AsSub) in a later change; this
// generic form lives here because resource cannot import component packages.
func AsSub[T Resource](api API, res T) Sub {
	return Sub{API: api, Res: res}
}

// Compose assembles a composite MultiAPIResource from per-API sub-resources. The served APIs are
// sorted by API string (matching RegisterMultiAPISet) so the first is a deterministic canonical API
// that DoCommand, Status, and Close route to. It errors if no subs are given or if two subs share an
// API. Compose is the ergonomic, type-checked front door to NewMultiAPIResource (pair it with AsSub).
func Compose(name Name, subs ...Sub) (MultiAPIResource, error) {
	if len(subs) == 0 {
		return nil, errors.New("Compose requires at least one sub-resource")
	}
	apis := make([]API, 0, len(subs))
	byAPI := make(map[API]Resource, len(subs))
	for _, sub := range subs {
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

func (c *compositeResource) APIs() []API { return c.apis }

func (c *compositeResource) ResourceForAPI(api API) (Resource, bool) {
	sub, ok := c.byAPI[api]
	return sub, ok
}

// canonicalSub returns the sub-resource for the canonical (first-declared) API. NewMultiAPIResource
// validates at construction that the canonical API has a byAPI entry, so this is always non-nil for a
// composite built through the constructor.
func (c *compositeResource) canonicalSub() Resource {
	return c.byAPI[c.apis[0]]
}

// DoCommand routes a bare command to the canonical (first-declared) sub-resource. On the server
// every API of a composite resolves to one instance with one DoCommand, so the canonical route is
// representative; access a specific API via AsType to target its sub-resource directly.
func (c *compositeResource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return c.canonicalSub().DoCommand(ctx, cmd)
}

// Status routes to the canonical (first-declared) sub-resource. As with DoCommand, all APIs of a
// composite resolve to one instance on the server, so the canonical route reports the same status.
func (c *compositeResource) Status(ctx context.Context) (map[string]interface{}, error) {
	return c.canonicalSub().Status(ctx)
}

// Close closes the composite once, via its canonical (first-declared) sub-resource — mirroring how
// DoCommand and Status route. A composite is one device with one lifecycle. On the authoring/server
// side its per-API facades embed one shared underlying impl, so closing every facade would
// double-close that impl; the single canonical Close tears the whole device down exactly once. On
// the client side the per-API sub-clients are owned and closed by the robot client's resource-client
// lifecycle, and this wrapper is a non-owning view that is not the close target there.
func (c *compositeResource) Close(ctx context.Context) error {
	return c.canonicalSub().Close(ctx)
}

// SubresourceForAPI unwraps a composite to the sub-resource serving api. If res is not a composite
// (or does not serve api) it is returned unchanged. This is the general, open-world access path used
// by FromDependencies, FromProvider, and the web/gRPC layer, which resolves resources by API and must
// forward each call to a composite's per-API sub-resource.
func SubresourceForAPI(res Resource, api API) Resource {
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
