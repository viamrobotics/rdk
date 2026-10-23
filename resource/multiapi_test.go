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

// superSensT's interface is a superset of testSens (it also has Snap), so a sub serving superSensT also
// satisfies testSens. combo implements both (it has Read and Snap).
type superSensT interface {
	Resource
	Read() int
	Snap() string
}

func TestAsTypeResolvesExactAPISub(t *testing.T) {
	// A composite serving a superset API (superSensT, which also satisfies testSens) plus testSens. The
	// superset API is named to sort first, so a naive first-match scan would return its sub for
	// AsType[testSens]. Because the API interfaces are registered, AsType must resolve testSens's own sub.
	superAPI := APINamespaceRDK.WithComponentType("asupersensor") // sorts before ztestsensor
	baseAPI := APINamespaceRDK.WithComponentType("ztestsensor")
	RegisterAPI[superSensT](superAPI, APIRegistration[superSensT]{})
	RegisterAPI[testSens](baseAPI, APIRegistration[testSens]{})
	defer DeregisterAPI(superAPI)
	defer DeregisterAPI(baseAPI)

	superSub := &combo{Named: NewName(superAPI, "dev").AsNamed()}
	baseSub := &combo{Named: NewName(baseAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(superAPI, "dev"), // configured under the canonical (sorted-first) super API
		map[API]Resource{superAPI: superSub, baseAPI: baseSub},
	)

	// AsType[testSens] must return testSens's own sub, not the superset sub that also satisfies testSens.
	got, err := AsType[testSens](composite)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, baseSub)

	// AsType[superSensT] returns the super sub.
	gotSuper, err := AsType[superSensT](composite)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotSuper, test.ShouldEqual, superSub)

	// An API the composite does not serve is a TypeError, even though the subs (combo) would satisfy its
	// interface (testCam needs only Snap). AsType returns T's own API's sub or nothing, never a
	// different API's sub that merely satisfies T.
	camAPI := APINamespaceRDK.WithComponentType("mtestcam")
	RegisterAPI[testCam](camAPI, APIRegistration[testCam]{})
	defer DeregisterAPI(camAPI)
	_, err = AsType[testCam](composite)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestCompositeRoutingToCanonical(t *testing.T) {
	// distinct sub-resources per API: DoCommand and Status must route to the canonical (apis[0])
	// sub-resource only, running exactly once and never touching the non-canonical sub.
	cam := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	sens := &combo{Named: NewName(testSensAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
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

func TestCompositeCloseClosesEachDistinctSub(t *testing.T) {
	// Close closes every distinct sub once. Distinct per-API facade values (camFacade{c}/imuFacade{c})
	// are separate objects, so each is closed; the shared core c is reached under both and so closed
	// twice, which is safe because the Resource contract requires Close to be idempotent.
	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}
	composite, err := Compose(
		AsSub(propCamAPI, camFacade{c}),
		AsSub(propIMUAPI, imuFacade{c}),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, composite.Close(context.Background()), test.ShouldBeNil)
	test.That(t, c.closes, test.ShouldEqual, 2)

	// Distinct per-API subs are each closed, not just the canonical one.
	cam := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	sens := &combo{Named: NewName(testSensAPI, "dev").AsNamed()}
	composite2 := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		map[API]Resource{testCamAPI: cam, testSensAPI: sens},
	)
	test.That(t, composite2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, cam.closes, test.ShouldEqual, 1)
	test.That(t, sens.closes, test.ShouldEqual, 1)
}

func TestNewMultiAPIResourceDefensiveCopy(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	byAPI := map[API]Resource{testCamAPI: c, testSensAPI: c}
	composite := NewMultiAPIResource(NewName(testCamAPI, "dev"), byAPI)

	// mutating the caller's map must not rewire the composite's routing
	byAPI[testCamAPI] = &combo{Named: NewName(testMotorAPI, "other").AsNamed()}
	got, ok := composite.ResourceForAPI(testCamAPI)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, got, test.ShouldEqual, c)

	// mutating the slice returned by APIs() must not corrupt the composite either.
	returned := composite.APIs()
	returned[0] = testMotorAPI
	test.That(t, composite.APIs(), test.ShouldResemble, []API{testCamAPI, testSensAPI})
}

func TestNewMultiAPIResourceValidates(t *testing.T) {
	// an empty byAPI is a programmer error and panics.
	test.That(t, func() {
		NewMultiAPIResource(NewName(testCamAPI, "dev"), map[API]Resource{})
	}, test.ShouldPanic)
}

func TestComposeSortsAPIs(t *testing.T) {
	// Pass the subs in reverse-sorted API order; Compose must still sort so the canonical (sorted-first)
	// api is propcam, not whichever was passed first.
	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}
	composite, err := Compose(
		AsSub(propIMUAPI, imuFacade{c}),
		AsSub(propCamAPI, camFacade{c}),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, composite.APIs(), test.ShouldResemble, []API{propCamAPI, propIMUAPI})
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

// Two co-equal APIs whose method sets collide by name. Both declare Properties, with different
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

// comboProps is one shared-state device serving propCam and propIMU. Its two Properties methods collide
// by name with incompatible signatures, so neither lives on comboProps directly; each is carried by a
// thin per-API facade embedding *comboProps.
type comboProps struct {
	Named
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
	// AsType resolves by the requested interface's registered API, as in production; register these.
	RegisterAPI[propCam](propCamAPI, APIRegistration[propCam]{})
	RegisterAPI[propIMU](propIMUAPI, APIRegistration[propIMU]{})
	defer DeregisterAPI(propCamAPI)
	defer DeregisterAPI(propIMUAPI)

	c := &comboProps{Named: NewName(propCamAPI, "dev").AsNamed()}
	composite, err := Compose(
		AsSub(propCamAPI, camFacade{c}),
		AsSub(propIMUAPI, imuFacade{c}),
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
	_, err := Compose()
	test.That(t, err, test.ShouldNotBeNil)

	// two subs sharing an API is an error
	_, err = Compose(
		AsSub(propCamAPI, camFacade{c}),
		AsSub(propCamAPI, camFacade{c}),
	)
	test.That(t, err, test.ShouldNotBeNil)

	// subs that disagree on their name are an error: a composite is one identity, so every sub must
	// report the same Name.
	other := &comboProps{Named: NewName(propIMUAPI, "other").AsNamed()}
	_, err = Compose(
		AsSub(propCamAPI, camFacade{c}),
		AsSub(propIMUAPI, imuFacade{other}),
	)
	test.That(t, err, test.ShouldNotBeNil)
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

func TestCompositeExtractionAndAPIsOf(t *testing.T) {
	// AsType resolves by the requested interface's registered API, as in production; register these.
	RegisterAPI[testCam](testCamAPI, APIRegistration[testCam]{})
	RegisterAPI[testSens](testSensAPI, APIRegistration[testSens]{})
	RegisterAPI[testMotor](testMotorAPI, APIRegistration[testMotor]{})
	defer DeregisterAPI(testCamAPI)
	defer DeregisterAPI(testSensAPI)
	defer DeregisterAPI(testMotorAPI)

	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
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

func TestNamedFromProvider(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "combo"),
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

func TestProviderUnwrapsComposite(t *testing.T) {
	newComposite := func() MultiAPIResource {
		c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
		return NewMultiAPIResource(
			NewName(testCamAPI, "dev"),
			map[API]Resource{testCamAPI: c, testSensAPI: c},
		)
	}
	name := NewName(testSensAPI, "dev")

	// FromDependencies (dependency-map path) must return the typed sub-resource for the requested API,
	// not the composite wrapper.
	sens, err := FromDependencies[testSens](Dependencies{name: newComposite()}, name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sens.Read(), test.ShouldEqual, 42)

	// FromProvider (the Provider path) unwraps identically.
	provider := fakeProvider{byName: map[Name]Resource{name: newComposite()}}
	sens, err = FromProvider[testSens](provider, name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sens.Read(), test.ShouldEqual, 42)
}

func TestSubsByNameAndDistinctSubs(t *testing.T) {
	base := NewName(testCamAPI, "dev")

	// A shared core served under both co-equal APIs.
	core := &combo{Named: base.AsNamed()}
	shared := NewMultiAPIResource(base, map[API]Resource{testCamAPI: core, testSensAPI: core})

	// SubsByName yields one pair per served API, canonical-first; both point at the shared core.
	var names []Name
	var subs []Resource
	for n, r := range SubsByName(base, shared) {
		names = append(names, n)
		subs = append(subs, r)
	}
	test.That(t, names, test.ShouldResemble, []Name{base.AsAPI(testCamAPI), base.AsAPI(testSensAPI)})
	test.That(t, subs, test.ShouldResemble, []Resource{core, core})

	// DistinctSubs dedups by identity: the shared core yields once, under the canonical API.
	var distinctNames []Name
	var distinctSubs []Resource
	for n, r := range DistinctSubs(base, shared) {
		distinctNames = append(distinctNames, n)
		distinctSubs = append(distinctSubs, r)
	}
	test.That(t, distinctNames, test.ShouldResemble, []Name{base.AsAPI(testCamAPI)})
	test.That(t, distinctSubs, test.ShouldResemble, []Resource{core})

	// Distinct per-API objects: both iterators yield each.
	cam := &combo{Named: base.AsNamed()}
	sens := &combo{Named: base.AsNamed()}
	distinct := NewMultiAPIResource(base, map[API]Resource{testCamAPI: cam, testSensAPI: sens})
	var subsByNameCount, distinctCount int
	for range SubsByName(base, distinct) {
		subsByNameCount++
	}
	for range DistinctSubs(base, distinct) {
		distinctCount++
	}
	test.That(t, subsByNameCount, test.ShouldEqual, 2)
	test.That(t, distinctCount, test.ShouldEqual, 2)
}
