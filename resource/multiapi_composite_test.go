package resource

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// Two co-equal test APIs plus a third the composite does not serve. testcam sorts before testsens,
// so testCamAPI is the canonical (sorted-first) API of a {cam, sens} composite.
var (
	testCamAPI   = APINamespaceRDK.WithComponentType("testcam")
	testSensAPI  = APINamespaceRDK.WithComponentType("testsens")
	testMotorAPI = APINamespaceRDK.WithComponentType("testmotor")
)

type testCam interface {
	Resource
	Snap() string
}

type testSens interface {
	Resource
	Read() int
}

type testMotor interface {
	Resource
	Spin()
}

// combo serves both testCam and testSens from one identity, and records how often each pass-through
// method runs so routing and close-once behavior can be asserted.
type combo struct {
	Named
	AlwaysRebuild
	closes int
	dos    int
	stats  int
}

func (c *combo) Snap() string { return "img" }

func (c *combo) Read() int { return 42 }

func (c *combo) DoCommand(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	c.dos++
	return map[string]interface{}{"ok": true}, nil
}

func (c *combo) Status(context.Context) (map[string]interface{}, error) {
	c.stats++
	return map[string]interface{}{"live": true}, nil
}

func (c *combo) Close(context.Context) error {
	c.closes++
	return nil
}

func newComboConstructor() Registration[Resource, NoNativeConfig] {
	return Registration[Resource, NoNativeConfig]{
		Constructor: func(_ context.Context, _ Dependencies, conf Config, _ logging.Logger) (Resource, error) {
			return &combo{Named: conf.ResourceName().AsNamed()}, nil
		},
	}
}

func TestRegisterMultiAPIRecordsSet(t *testing.T) {
	model := NewModel("acme", "test", "combo1")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	// the one constructor is registered under each api
	_, ok := LookupRegistration(testCamAPI, model)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = LookupRegistration(testSensAPI, model)
	test.That(t, ok, test.ShouldBeTrue)

	// the full API set is recorded, sorted, with the canonical api first
	apis := APIsForModel(model)
	test.That(t, apis, test.ShouldResemble, []API{testCamAPI, testSensAPI})

	// ExpandModel expands to one APIModel per api (for ModularMain), in the same sorted order
	test.That(t, ExpandModel(model), test.ShouldResemble, []APIModel{
		{API: testCamAPI, Model: model},
		{API: testSensAPI, Model: model},
	})
}

func TestSingleAPIRegisterMultiAPIBehavesLikeRegister(t *testing.T) {
	model := NewModel("acme", "test", "single")
	RegisterMultiAPI([]API{testCamAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)

	// a single-API model records no composite set but is still registered like a plain Register
	test.That(t, APIsForModel(model), test.ShouldBeNil)
	_, ok := LookupRegistration(testCamAPI, model)
	test.That(t, ok, test.ShouldBeTrue)

	// ExpandModel falls back to the registry scan and yields the single {api, model} pair
	test.That(t, ExpandModel(model), test.ShouldResemble, []APIModel{{API: testCamAPI, Model: model}})
}

func TestRegisterMultiAPIEmptyPanics(t *testing.T) {
	model := NewModel("acme", "test", "empty")
	test.That(t, func() { RegisterMultiAPI([]API{}, model, newComboConstructor()) }, test.ShouldPanic)
	test.That(t, APIsForModel(model), test.ShouldBeNil)
}

func TestCanonicalAPISortingDeterministic(t *testing.T) {
	// register with APIs in reverse-sorted order; the recorded set must still be sorted so apis[0]
	// is a deterministic canonical api.
	model := NewModel("acme", "test", "sortme")
	RegisterMultiAPI([]API{testSensAPI, testMotorAPI, testCamAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)
	defer Deregister(testMotorAPI, model)

	want := []API{testCamAPI, testMotorAPI, testSensAPI} // testcam < testmotor < testsens
	test.That(t, APIsForModel(model), test.ShouldResemble, want)
	test.That(t, APIsForModel(model)[0], test.ShouldResemble, testCamAPI)

	// RegisterMultiAPISet on its own (modular path) sorts identically regardless of input order.
	model2 := NewModel("acme", "test", "sortme2")
	RegisterMultiAPISet(model2, []API{testSensAPI, testCamAPI, testMotorAPI})
	defer func() {
		registryMu.Lock()
		delete(multiAPIByModel, model2)
		registryMu.Unlock()
	}()
	test.That(t, APIsForModel(model2), test.ShouldResemble, want)
}

func TestRegisterMultiAPISetIgnoresSingle(t *testing.T) {
	model := NewModel("acme", "test", "setsingle")
	RegisterMultiAPISet(model, []API{testCamAPI})
	test.That(t, APIsForModel(model), test.ShouldBeNil)
}

func TestExpandModelUnregistered(t *testing.T) {
	test.That(t, ExpandModel(NewModel("acme", "test", "nope")), test.ShouldBeNil)
}

func TestDeregisterDropsCompositeSet(t *testing.T) {
	model := NewModel("acme", "test", "dropme")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())

	// dropping one api of the set leaves the composite set intact (a constructor still remains)
	Deregister(testCamAPI, model)
	test.That(t, APIsForModel(model), test.ShouldResemble, []API{testCamAPI, testSensAPI})

	// dropping the last api removes the recorded composite set entirely
	Deregister(testSensAPI, model)
	test.That(t, APIsForModel(model), test.ShouldBeNil)
}

func TestCompositeNodeForAPIResolvesEachCoEqualAPI(t *testing.T) {
	model := NewModel("acme", "test", "graphcombo")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	g := NewGraph(logging.NewTestLogger(t))
	canonical := NewName(testCamAPI, "dev")
	res := &combo{Named: canonical.AsNamed()}
	node := NewConfiguredGraphNode(Config{Name: "dev", API: testCamAPI, Model: model}, res, model)
	test.That(t, g.AddNode(canonical, node), test.ShouldBeNil)

	// the node is stored under the canonical api and resolves directly
	got, err := g.FindBySimpleNameAndAPI("dev", testCamAPI)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, node)

	// each co-equal api resolves to the same one node, even though nothing is cached under it
	got, err = g.FindBySimpleNameAndAPI("dev", testSensAPI)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, node)

	// an api the model does not serve is still not found
	_, err = g.FindBySimpleNameAndAPI("dev", testMotorAPI)
	test.That(t, IsNodeNotFoundError(err), test.ShouldBeTrue)
}

func TestCompositeCoequalIndexMaintenance(t *testing.T) {
	// The co-equal index that lets a non-configured API resolve to a composite's one node must be kept
	// in sync as the node is re-prefixed and deleted, or a co-equal lookup would resolve a stale/dead
	// node (or fail after a prefix change).
	model := NewModel("acme", "test", "graphidx")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	g := NewGraph(logging.NewTestLogger(t))
	canonical := NewName(testCamAPI, "dev")
	node := NewConfiguredGraphNode(Config{Name: "dev", API: testCamAPI, Model: model}, &combo{Named: canonical.AsNamed()}, model)
	test.That(t, g.AddNode(canonical, node), test.ShouldBeNil)

	// prefix change moves the co-equal index entry with the node: it resolves under the new prefixed
	// simple name and no longer under the old bare name.
	g.UpdateNodePrefix(canonical, "pfx.")
	got, err := g.FindBySimpleNameAndAPI("pfx.dev", testSensAPI)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, node)
	_, err = g.FindBySimpleNameAndAPI("dev", testSensAPI)
	test.That(t, IsNodeNotFoundError(err), test.ShouldBeTrue)

	// deleting the composite drops BOTH its canonical simpleNameCache entry and its co-equal index
	// entry, so neither api resolves afterward.
	g.nodes.Delete(canonical)
	_, err = g.FindBySimpleNameAndAPI("pfx.dev", testCamAPI)
	test.That(t, IsNodeNotFoundError(err), test.ShouldBeTrue)
	_, err = g.FindBySimpleNameAndAPI("pfx.dev", testSensAPI)
	test.That(t, IsNodeNotFoundError(err), test.ShouldBeTrue)
	test.That(t, g.nodes.compositeByAPI, test.ShouldBeEmpty)
}

func TestCompositeIndexViaPlaceholderReplace(t *testing.T) {
	// A composite depended on before it is configured is first added as an uninitialized placeholder,
	// then replaced by its configured node (addNode's replace path). The co-equal index must be
	// populated there too — the model only becomes known at replace — or the composite would resolve
	// under its configured API but 404 under its other co-equal APIs.
	model := NewModel("acme", "test", "phcombo")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	g := NewGraph(logging.NewTestLogger(t))
	compName := NewName(testCamAPI, "dev")
	test.That(t, g.AddNode(compName, NewUninitializedNode()), test.ShouldBeNil)

	configured := NewConfiguredGraphNode(Config{Name: "dev", API: testCamAPI, Model: model}, &combo{Named: compName.AsNamed()}, model)
	test.That(t, g.AddNode(compName, configured), test.ShouldBeNil)

	_, err := g.FindBySimpleNameAndAPI("dev", testCamAPI)
	test.That(t, err, test.ShouldBeNil)
	_, err = g.FindBySimpleNameAndAPI("dev", testSensAPI)
	test.That(t, err, test.ShouldBeNil)
}

func TestFindBySimpleNameCompositeVsCollision(t *testing.T) {
	model := NewModel("acme", "test", "graphcombo2")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	g := NewGraph(logging.NewTestLogger(t))

	// a composite is a single node; a bare-name lookup finds exactly one match (not a collision).
	comboName := NewName(testCamAPI, "combo")
	comboNode := NewConfiguredGraphNode(
		Config{Name: "combo", API: testCamAPI, Model: model},
		&combo{Named: comboName.AsNamed()}, model)
	test.That(t, g.AddNode(comboName, comboNode), test.ShouldBeNil)
	// The resolver returns the one owner with no error, and the all-matches scan returns a single match.
	_, err := g.FindBySimpleName("combo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.FindAllBySimpleName("combo"), test.ShouldHaveLength, 1)

	// two genuinely distinct nodes sharing a simple name back different graph nodes and still
	// collide — name-uniqueness detection must not regress: the resolver returns a
	// MultipleMatchingNamesError and the all-matches scan returns two matches.
	camDup := NewName(testCamAPI, "dup")
	sensDup := NewName(testSensAPI, "dup")
	test.That(t, g.AddNode(camDup, NewConfiguredGraphNode(
		Config{Name: "dup", API: testCamAPI}, &combo{Named: camDup.AsNamed()}, Model{})), test.ShouldBeNil)
	test.That(t, g.AddNode(sensDup, NewConfiguredGraphNode(
		Config{Name: "dup", API: testSensAPI}, &combo{Named: sensDup.AsNamed()}, Model{})), test.ShouldBeNil)
	_, err = g.FindBySimpleName("dup")
	test.That(t, IsMultipleMatchingNamesError(err), test.ShouldBeTrue)
	test.That(t, g.FindAllBySimpleName("dup"), test.ShouldHaveLength, 2)
}

func TestExpandCompositeNames(t *testing.T) {
	model := NewModel("acme", "test", "graphcombo3")
	RegisterMultiAPI([]API{testCamAPI, testSensAPI}, model, newComboConstructor())
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	g := NewGraph(logging.NewTestLogger(t))
	comboName := NewName(testCamAPI, "combo")
	test.That(t, g.AddNode(comboName, NewConfiguredGraphNode(
		Config{Name: "combo", API: testCamAPI, Model: model},
		&combo{Named: comboName.AsNamed()}, model)), test.ShouldBeNil)

	// a plain single-API node advertises unchanged
	plainName := NewName(testMotorAPI, "plain")
	test.That(t, g.AddNode(plainName, NewConfiguredGraphNode(
		Config{Name: "plain", API: testMotorAPI}, &combo{Named: plainName.AsNamed()}, Model{})), test.ShouldBeNil)

	unknown := NewName(testMotorAPI, "ghost")
	out := g.ExpandCompositeNames([]Name{comboName, plainName, unknown})

	// composite fans out to one name per co-equal api; the others pass through untouched
	test.That(t, out, test.ShouldContain, NewName(testCamAPI, "combo"))
	test.That(t, out, test.ShouldContain, NewName(testSensAPI, "combo"))
	test.That(t, out, test.ShouldContain, plainName)
	test.That(t, out, test.ShouldContain, unknown)
	test.That(t, out, test.ShouldHaveLength, 4)
}

func TestCompositeExtractionAndAPIsOf(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)

	// APIsOf reports every served API for a composite, and the single api for an ordinary resource
	test.That(t, APIsOf(composite), test.ShouldResemble, []API{testCamAPI, testSensAPI})
	test.That(t, APIsOf(c), test.ShouldResemble, []API{testCamAPI})

	// AsType extracts each interface from the one identity
	cam, err := AsType[testCam](composite)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam.Snap(), test.ShouldEqual, "img")

	sens, err := AsType[testSens](composite)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sens.Read(), test.ShouldEqual, 42)

	// AsType errors for an API the composite genuinely does not serve
	_, err = AsType[testMotor](composite)
	test.That(t, err, test.ShouldNotBeNil)

	// SubresourceForAPI unwraps to the sub-resource for a served API and passes through otherwise
	test.That(t, SubresourceForAPI(composite, testSensAPI), test.ShouldEqual, c)
	test.That(t, SubresourceForAPI(composite, testMotorAPI), test.ShouldEqual, composite)
	test.That(t, SubresourceForAPI(c, testSensAPI), test.ShouldEqual, c)
	test.That(t, SubresourceForAPI(composite, testCamAPI), test.ShouldEqual, c)
}

func TestCompositeRoutingToCanonical(t *testing.T) {
	// distinct sub-resources per API: DoCommand and Status must route to the canonical (apis[0])
	// sub-resource only, running exactly once and never touching the non-canonical sub.
	cam := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	sens := &combo{Named: NewName(testSensAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: cam, testSensAPI: sens},
	)

	_, err := composite.DoCommand(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam.dos, test.ShouldEqual, 1)
	test.That(t, sens.dos, test.ShouldEqual, 0)

	_, err = composite.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam.stats, test.ShouldEqual, 1)
	test.That(t, sens.stats, test.ShouldEqual, 0)
}

func TestCompositeCloseOnce(t *testing.T) {
	// two APIs map to the same instance; Close must fire exactly once
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)
	test.That(t, composite.Close(context.Background()), test.ShouldBeNil)
	test.That(t, c.closes, test.ShouldEqual, 1)

	// a composite is one lifecycle: Close routes only to the canonical (apis[0]) sub, so a
	// non-canonical sub value is never independently closed.
	cam := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	sens := &combo{Named: NewName(testSensAPI, "dev").AsNamed()}
	composite2 := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: cam, testSensAPI: sens},
	)
	test.That(t, composite2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, cam.closes, test.ShouldEqual, 1)
	test.That(t, sens.closes, test.ShouldEqual, 0)
}

func TestNewMultiAPIResourceDefensiveCopy(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	apis := []API{testCamAPI, testSensAPI}
	composite := NewMultiAPIResource(NewName(testCamAPI, "dev"), apis, map[API]Resource{testCamAPI: c, testSensAPI: c})

	// mutating the caller's slice must not change the composite's stable API order/canonical api
	apis[0] = testMotorAPI
	test.That(t, composite.APIs(), test.ShouldResemble, []API{testCamAPI, testSensAPI})
}

func TestNewMultiAPIResourceValidates(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}

	// no apis is a programmer error and panics.
	test.That(t, func() {
		NewMultiAPIResource(NewName(testCamAPI, "dev"), nil, map[API]Resource{})
	}, test.ShouldPanic)

	// an api with no byAPI entry panics, so the canonical route is always resolvable.
	test.That(t, func() {
		NewMultiAPIResource(NewName(testCamAPI, "dev"), []API{testCamAPI, testSensAPI}, map[API]Resource{testCamAPI: c})
	}, test.ShouldPanic)
}

func TestNewMultiAPIResourceCopiesByAPI(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	byAPI := map[API]Resource{testCamAPI: c, testSensAPI: c}
	composite := NewMultiAPIResource(NewName(testCamAPI, "dev"), []API{testCamAPI, testSensAPI}, byAPI)

	// mutating the caller's map after construction must not rewire the composite's routing.
	other := &combo{Named: NewName(testMotorAPI, "other").AsNamed()}
	byAPI[testCamAPI] = other
	got, ok := composite.ResourceForAPI(testCamAPI)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, got, test.ShouldEqual, c)
}

func TestNamedFromProvider(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "combo"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)
	// The provider is keyed by the bare (API-less) SimpleName, as NamedFromProvider looks it up.
	provider := fakeProvider{byName: map[Name]Resource{SimpleName("combo"): composite}}

	res, err := NamedFromProvider(provider, "combo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, composite)

	_, err = NamedFromProvider(provider, "nope")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestSimpleNamesWhereRemoteComposite(t *testing.T) {
	// SimpleNamesWhere surfaces every per-API sibling of a remote composite (same remote, one identity)
	// so a client can detect and assemble it, but still hides a genuine machine-wide collision.
	g := NewGraph(logging.NewTestLogger(t))
	newNode := func(n Name) *GraphNode {
		return NewConfiguredGraphNode(Config{Name: n.Name, API: n.API}, &combo{Named: n.AsNamed()}, Model{})
	}
	add := func(n Name) { test.That(t, g.AddNode(n, newNode(n)), test.ShouldBeNil) }

	// A remote composite: two co-equal APIs from the SAME remote under one name -> both surfaced.
	add(Name{API: testCamAPI, Name: "combo", Remote: "r1"})
	add(Name{API: testSensAPI, Name: "combo", Remote: "r1"})
	// A genuine collision: same name across DIFFERENT remotes -> hidden.
	add(Name{API: testCamAPI, Name: "dup", Remote: "r1"})
	add(Name{API: testSensAPI, Name: "dup", Remote: "r2"})
	// An ordinary single remote resource -> surfaced.
	add(Name{API: testCamAPI, Name: "solo", Remote: "r1"})

	got := g.SimpleNamesWhere(func(Name, *GraphNode) bool { return true })
	test.That(t, got, test.ShouldContain, Name{API: testCamAPI, Name: "combo", Remote: "r1"})
	test.That(t, got, test.ShouldContain, Name{API: testSensAPI, Name: "combo", Remote: "r1"})
	test.That(t, got, test.ShouldContain, Name{API: testCamAPI, Name: "solo", Remote: "r1"})

	byName := map[string]int{}
	for _, n := range got {
		byName[n.Name]++
	}
	test.That(t, byName["combo"], test.ShouldEqual, 2) // both composite siblings
	test.That(t, byName["dup"], test.ShouldEqual, 0)   // collision hidden
	test.That(t, byName["solo"], test.ShouldEqual, 1)
}

func TestFromProviderUnwrapsComposite(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)
	provider := fakeProvider{byName: map[Name]Resource{NewName(testSensAPI, "dev"): composite}}

	// FromProvider requests the sensor API and must receive the unwrapped, typed sub-resource
	sens, err := FromProvider[testSens](provider, NewName(testSensAPI, "dev"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sens.Read(), test.ShouldEqual, 42)
}

type fakeProvider struct {
	byName map[Name]Resource
}

func (f fakeProvider) GetResource(name Name) (Resource, error) {
	res, ok := f.byName[name]
	if !ok {
		return nil, NewNotFoundError(name)
	}
	return res, nil
}

func TestSimpleName(t *testing.T) {
	n := SimpleName("combo")
	test.That(t, n.Name, test.ShouldEqual, "combo")
	test.That(t, n.API, test.ShouldResemble, API{})
	test.That(t, n.Remote, test.ShouldEqual, "")
}

// Two co-equal APIs whose method sets collide by name: both declare Properties, with different
// signatures. propcam sorts before propimu, so propCamAPI is the canonical (sorted-first) API.
var (
	propCamAPI = APINamespaceRDK.WithComponentType("propcam")
	propIMUAPI = APINamespaceRDK.WithComponentType("propimu")
)

// CamProps and IMUProps are the deliberately different return types of the two colliding Properties
// methods, so a single Go type cannot carry both.
type (
	CamProps struct{ Width int }
	IMUProps struct{ AngularRateHz float64 }
)

type propCam interface {
	Resource
	Properties(context.Context) (CamProps, error)
}

type propIMU interface {
	Resource
	Properties(context.Context, map[string]interface{}) (*IMUProps, error)
}

// comboProps is one shared-state device serving both propCam and propIMU. Its two Properties methods
// collide by name with incompatible signatures, so neither can live on comboProps directly; each is
// carried by a thin per-API facade embedding *comboProps. All lifecycle state (here, a close counter)
// lives on the one shared comboProps, never on a facade.
type comboProps struct {
	Named
	AlwaysRebuild
	closes int
}

func (c *comboProps) DoCommand(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (c *comboProps) Status(context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (c *comboProps) Close(context.Context) error {
	c.closes++
	return nil
}

type camFacade struct{ *comboProps }

func (f camFacade) Properties(context.Context) (CamProps, error) { return CamProps{Width: 640}, nil }

type imuFacade struct{ *comboProps }

func (f imuFacade) Properties(context.Context, map[string]interface{}) (*IMUProps, error) {
	return &IMUProps{AngularRateHz: 100}, nil
}

func TestComposeFacadesCollidingMethods(t *testing.T) {
	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}
	composite, err := Compose(
		NewName(propCamAPI, "dev"),
		AsSub[propCam](propCamAPI, camFacade{c}),
		AsSub[propIMU](propIMUAPI, imuFacade{c}),
	)
	test.That(t, err, test.ShouldBeNil)

	// each co-equal API resolves to its own facade
	camSub, ok := composite.ResourceForAPI(propCamAPI)
	test.That(t, ok, test.ShouldBeTrue)
	_, isCam := camSub.(camFacade)
	test.That(t, isCam, test.ShouldBeTrue)
	imuSub, ok := composite.ResourceForAPI(propIMUAPI)
	test.That(t, ok, test.ShouldBeTrue)
	_, isIMU := imuSub.(imuFacade)
	test.That(t, isIMU, test.ShouldBeTrue)

	// AsType extracts each colliding interface, and each one's own Properties runs and returns its
	// own type.
	cam, err := AsType[propCam](composite)
	test.That(t, err, test.ShouldBeNil)
	cp, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cp.Width, test.ShouldEqual, 640)

	imu, err := AsType[propIMU](composite)
	test.That(t, err, test.ShouldBeNil)
	ip, err := imu.Properties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ip.AngularRateHz, test.ShouldEqual, 100)

	// APIsOf reports both served APIs, sorted with the canonical (propcam) first
	test.That(t, APIsOf(composite), test.ShouldResemble, []API{propCamAPI, propIMUAPI})
}

func TestComposeErrors(t *testing.T) {
	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}

	// no subs is an error
	_, err := Compose(NewName(propCamAPI, "dev"))
	test.That(t, err, test.ShouldNotBeNil)

	// two subs sharing an API is an error
	_, err = Compose(
		NewName(propCamAPI, "dev"),
		AsSub[propCam](propCamAPI, camFacade{c}),
		AsSub[propCam](propCamAPI, camFacade{c}),
	)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestComposeCloseOnceAcrossFacades(t *testing.T) {
	// camFacade{c} and imuFacade{c} are distinct sub values both delegating to the one shared c;
	// Close must fire the shared Close exactly once, not once per facade.
	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}
	composite, err := Compose(
		NewName(propCamAPI, "dev"),
		AsSub[propCam](propCamAPI, camFacade{c}),
		AsSub[propIMU](propIMUAPI, imuFacade{c}),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, composite.Close(context.Background()), test.ShouldBeNil)
	test.That(t, c.closes, test.ShouldEqual, 1)
}
