package resource

import (
	"context"
	"encoding/json"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// Two ad-hoc co-equal APIs with distinct typed methods, used to exercise multi-API composition
// without any gRPC plumbing.
type testAPIOne interface {
	Resource
	One() string
}

type testAPITwo interface {
	Resource
	Two() string
}

// testAPIThree is served by neither sub-resource, for negative extraction tests.
type testAPIThree interface {
	Resource
	Three() string
}

var (
	apiOne = APINamespace("multiapi").WithComponentType("one")
	apiTwo = APINamespace("multiapi").WithComponentType("two")
)

// oneImpl implements only testAPIOne. Used as a client-side per-API sub-resource.
type oneImpl struct {
	Named
	TriviallyReconfigurable
	TriviallyCloseable
	closed *bool
}

func (o *oneImpl) One() string { return "one" }

func (o *oneImpl) Close(ctx context.Context) error {
	if o.closed != nil {
		*o.closed = true
	}
	return nil
}

// twoImpl implements only testAPITwo, and gives a real DoCommand so DoCommand routing can be tested.
type twoImpl struct {
	Named
	TriviallyReconfigurable
	TriviallyCloseable
	closed *bool
}

func (t *twoImpl) Two() string { return "two" }

func (t *twoImpl) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"served_by": "two"}, nil
}

func (t *twoImpl) Close(ctx context.Context) error {
	if t.closed != nil {
		*t.closed = true
	}
	return nil
}

func TestCompositeFromResourceForAPI(t *testing.T) {
	name := NewName(apiOne, "dev")
	one := &oneImpl{Named: name.AsNamed()}
	two := &twoImpl{Named: NewName(apiTwo, "dev").AsNamed()}

	composite := NewMultiAPIResource(name, []API{apiOne, apiTwo}, map[API]Resource{
		apiOne: one,
		apiTwo: two,
	})

	// The general open-world path: extract each API's typed handle off the one object.
	gotOne, err := FromResourceForAPI[testAPIOne](composite, apiOne)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOne.One(), test.ShouldEqual, "one")

	gotTwo, err := FromResourceForAPI[testAPITwo](composite, apiTwo)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotTwo.Two(), test.ShouldEqual, "two")

	// Extracting a type that no sub-resource implements fails cleanly.
	_, err = FromResourceForAPI[testAPIThree](composite, apiOne)
	test.That(t, err, test.ShouldNotBeNil)

	// APIs() reports the served set in declared order.
	test.That(t, composite.APIs(), test.ShouldResemble, []API{apiOne, apiTwo})
}

func TestFromResourceForAPIPassThrough(t *testing.T) {
	// For an ordinary (non-composite) resource, FromResourceForAPI is just AsType.
	name := NewName(apiOne, "plain")
	one := &oneImpl{Named: name.AsNamed()}
	got, err := FromResourceForAPI[testAPIOne](one, apiOne)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.One(), test.ShouldEqual, "one")
}

func TestAccessorsUnwrapComposite(t *testing.T) {
	// The typed accessors (FromDependencies / FromProvider) transparently unwrap a composite to the
	// sub-resource for the requested API — so an existing arm.FromProvider(m, "dev") keeps working
	// when "dev" is a multi-API resource. This is the back-compat guarantee of the one-object design.
	nameOne := NewName(apiOne, "dev")
	nameTwo := NewName(apiTwo, "dev")
	one := &oneImpl{Named: nameOne.AsNamed()}
	two := &twoImpl{Named: nameTwo.AsNamed()}
	composite := NewMultiAPIResource(nameOne, []API{apiOne, apiTwo}, map[API]Resource{
		apiOne: one, apiTwo: two,
	})

	// The client caches the one composite under each of its API names.
	deps := Dependencies{nameOne: composite, nameTwo: composite}

	gotOne, err := FromDependencies[testAPIOne](deps, nameOne)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOne.One(), test.ShouldEqual, "one")

	gotTwo, err := FromDependencies[testAPITwo](deps, nameTwo)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotTwo.Two(), test.ShouldEqual, "two")

	// FromProvider (used by X.FromRobot) unwraps the same way.
	gotOne2, err := FromProvider[testAPIOne](deps, nameOne)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOne2.One(), test.ShouldEqual, "one")
}

func TestCompositeDoCommandRouting(t *testing.T) {
	name := NewName(apiOne, "dev")
	// oneImpl's DoCommand is the unimplemented default (from Named); twoImpl implements it.
	one := &oneImpl{Named: name.AsNamed()}
	two := &twoImpl{Named: NewName(apiTwo, "dev").AsNamed()}
	composite := NewMultiAPIResource(name, []API{apiOne, apiTwo}, map[API]Resource{
		apiOne: one, apiTwo: two,
	})

	// A bare DoCommand skips the unimplemented sub and routes to the one that implements it.
	resp, err := composite.DoCommand(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["served_by"], test.ShouldEqual, "two")
}

func TestCompositeCloseClosesEachOnce(t *testing.T) {
	name := NewName(apiOne, "dev")
	var oneClosed, twoClosed bool
	one := &oneImpl{Named: name.AsNamed(), closed: &oneClosed}
	two := &twoImpl{Named: NewName(apiTwo, "dev").AsNamed(), closed: &twoClosed}
	composite := NewMultiAPIResource(name, []API{apiOne, apiTwo}, map[API]Resource{
		apiOne: one, apiTwo: two,
	})

	test.That(t, composite.Close(context.Background()), test.ShouldBeNil)
	test.That(t, oneClosed, test.ShouldBeTrue)
	test.That(t, twoClosed, test.ShouldBeTrue)
}

func TestRegisterMultiAPI(t *testing.T) {
	model := DefaultModelFamily.WithModel("multiapi_regmodel")
	ctor := func(context.Context, Dependencies, Config, logging.Logger) (Resource, error) { return nil, nil }

	RegisterMultiAPI([]API{apiOne, apiTwo}, model, Registration[Resource, NoNativeConfig]{Constructor: ctor})
	defer func() {
		Deregister(apiOne, model)
		Deregister(apiTwo, model)
		registryMu.Lock()
		delete(multiAPIByModel, model)
		registryMu.Unlock()
	}()

	// Registered under each API, and the set is discoverable from the model alone.
	_, okOne := LookupRegistration(apiOne, model)
	_, okTwo := LookupRegistration(apiTwo, model)
	test.That(t, okOne, test.ShouldBeTrue)
	test.That(t, okTwo, test.ShouldBeTrue)
	test.That(t, APIsForModel(model), test.ShouldResemble, []API{apiOne, apiTwo})
	// A single-API model has no recorded set.
	test.That(t, APIsForModel(DefaultModelFamily.WithModel("nonexistent")), test.ShouldBeNil)
}

func TestMultiAPIGraphPeerRouting(t *testing.T) {
	// A node whose model serves {apiOne, apiTwo} is cached under BOTH, resolving to one node.
	logger := logging.NewTestLogger(t)
	model := DefaultModelFamily.WithModel("multiapi_graphmodel")
	registryMu.Lock()
	multiAPIByModel[model] = []API{apiOne, apiTwo}
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		delete(multiAPIByModel, model)
		registryMu.Unlock()
	}()

	g := NewGraph(logger)
	node := NewUnconfiguredGraphNode(Config{API: apiOne, Name: "dev", Model: model}, nil)
	test.That(t, g.AddNode(NewName(apiOne, "dev"), node), test.ShouldBeNil)

	gotOne, err := g.FindBySimpleNameAndAPI("dev", apiOne)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOne, test.ShouldEqual, node)
	gotTwo, err := g.FindBySimpleNameAndAPI("dev", apiTwo)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotTwo, test.ShouldEqual, node)

	// Advertised under both APIs.
	names := g.SimpleNamesWhere(func(Name, *GraphNode) bool { return true })
	test.That(t, names, test.ShouldContain, NewName(apiOne, "dev"))
	test.That(t, names, test.ShouldContain, NewName(apiTwo, "dev"))

	// Removal tears down every peer entry.
	g.remove(NewName(apiOne, "dev"))
	_, err = g.FindBySimpleNameAndAPI("dev", apiOne)
	test.That(t, err, test.ShouldNotBeNil)
	_, err = g.FindBySimpleNameAndAPI("dev", apiTwo)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestResolveDependencyOnComposite(t *testing.T) {
	// A resource can depend on a composite by its bare name. The bare name matches every one of the
	// composite's API names, which all back one node — that must resolve as a single dependency
	// rather than being reported as a conflict between distinct names.
	logger := logging.NewTestLogger(t)
	model := DefaultModelFamily.WithModel("multiapi_depmodel")
	registryMu.Lock()
	multiAPIByModel[model] = []API{apiOne, apiTwo}
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		delete(multiAPIByModel, model)
		registryMu.Unlock()
	}()

	g := NewGraph(logger)

	// The composite node, advertised under both apiOne and apiTwo.
	composite := NewUnconfiguredGraphNode(Config{API: apiOne, Name: "dev", Model: model}, nil)
	test.That(t, g.AddNode(NewName(apiOne, "dev"), composite), test.ShouldBeNil)

	// A consumer that depends on the composite by its bare name.
	consumerName := NewName(apiOne, "consumer")
	consumer := NewUnconfiguredGraphNode(Config{API: apiOne, Name: "consumer"}, []string{"dev"})
	test.That(t, g.AddNode(consumerName, consumer), test.ShouldBeNil)

	// Resolution succeeds (one composite, not a conflict) and the edge points at the composite's
	// canonical stored name.
	test.That(t, g.ResolveDependencies(logger), test.ShouldBeNil)
	test.That(t, consumer.UnresolvedDependencies(), test.ShouldBeEmpty)
	test.That(t, g.GetAllParentsOf(consumerName), test.ShouldContain, NewName(apiOne, "dev"))
}

func TestConfigOmitsAPIForMultiAPIModel(t *testing.T) {
	// A composite resource's config may omit `api`; AdjustPartialNames resolves the canonical API
	// from the model's registered set.
	model := DefaultModelFamily.WithModel("multiapi_cfgmodel")
	registryMu.Lock()
	multiAPIByModel[model] = []API{apiOne, apiTwo}
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		delete(multiAPIByModel, model)
		registryMu.Unlock()
	}()

	conf := Config{Name: "dev", Model: model} // no API
	conf.AdjustPartialNames("")
	test.That(t, conf.API, test.ShouldResemble, apiOne)
	test.That(t, conf.ResourceName(), test.ShouldResemble, NewName(apiOne, "dev"))
}

func TestConfigJSONRoundTripOmitsAPI(t *testing.T) {
	// A composite resource's config, parsed from JSON with no "api" field, resolves its API from the
	// model's registered set through the normal Validate path (as config.Read would).
	model := DefaultModelFamily.WithModel("multiapi_jsonmodel")
	registryMu.Lock()
	multiAPIByModel[model] = []API{apiOne, apiTwo}
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		delete(multiAPIByModel, model)
		registryMu.Unlock()
	}()

	var conf Config
	err := json.Unmarshal([]byte(`{"name": "dev", "model": "rdk:builtin:multiapi_jsonmodel"}`), &conf)
	test.That(t, err, test.ShouldBeNil)
	// No api in the JSON.
	test.That(t, conf.API, test.ShouldResemble, API{})

	_, _, err = conf.Validate("test", "component")
	test.That(t, err, test.ShouldBeNil)
	// Resolved to the canonical (first) API of the set.
	test.That(t, conf.API, test.ShouldResemble, apiOne)
}
