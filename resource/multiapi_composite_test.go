package resource

import (
	"context"
	"encoding/json"
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
	test.That(t, g.FindBySimpleName("combo"), test.ShouldHaveLength, 1)

	// two genuinely distinct nodes sharing a simple name back different graph nodes and still
	// collide (two matches) — name-uniqueness detection must not regress.
	camDup := NewName(testCamAPI, "dup")
	sensDup := NewName(testSensAPI, "dup")
	test.That(t, g.AddNode(camDup, NewConfiguredGraphNode(
		Config{Name: "dup", API: testCamAPI}, &combo{Named: camDup.AsNamed()}, Model{})), test.ShouldBeNil)
	test.That(t, g.AddNode(sensDup, NewConfiguredGraphNode(
		Config{Name: "dup", API: testSensAPI}, &combo{Named: sensDup.AsNamed()}, Model{})), test.ShouldBeNil)
	test.That(t, g.FindBySimpleName("dup"), test.ShouldHaveLength, 2)
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

	// subresourceForAPI unwraps to the sub-resource for a served API and passes through otherwise
	test.That(t, subresourceForAPI(composite, testSensAPI), test.ShouldEqual, c)
	test.That(t, subresourceForAPI(composite, testMotorAPI), test.ShouldEqual, composite)
	test.That(t, subresourceForAPI(c, testSensAPI), test.ShouldEqual, c)
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

	// distinct sub-resources are each closed exactly once
	cam := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	sens := &combo{Named: NewName(testSensAPI, "dev").AsNamed()}
	composite2 := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: cam, testSensAPI: sens},
	)
	test.That(t, composite2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, cam.closes, test.ShouldEqual, 1)
	test.That(t, sens.closes, test.ShouldEqual, 1)
}

func TestNewMultiAPIResourceDefensiveCopy(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	apis := []API{testCamAPI, testSensAPI}
	composite := NewMultiAPIResource(NewName(testCamAPI, "dev"), apis, map[API]Resource{testCamAPI: c, testSensAPI: c})

	// mutating the caller's slice must not change the composite's stable API order/canonical api
	apis[0] = testMotorAPI
	test.That(t, composite.APIs(), test.ShouldResemble, []API{testCamAPI, testSensAPI})
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

func TestConfigCompositeRoundTrip(t *testing.T) {
	conf := Config{Name: "combo", API: testCamAPI, Model: NewModel("acme", "test", "combo"), Composite: true}
	data, err := json.Marshal(conf)
	test.That(t, err, test.ShouldBeNil)

	var got Config
	test.That(t, json.Unmarshal(data, &got), test.ShouldBeNil)
	test.That(t, got.Composite, test.ShouldBeTrue)

	// the flag is omitted from JSON when false and defaults to false on decode
	data, err = json.Marshal(Config{Name: "plain", API: testCamAPI, Model: NewModel("acme", "test", "plain")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(data), test.ShouldNotContainSubstring, "composite")
	var plain Config
	test.That(t, json.Unmarshal(data, &plain), test.ShouldBeNil)
	test.That(t, plain.Composite, test.ShouldBeFalse)
}
