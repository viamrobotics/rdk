package resource

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// Two co-equal test APIs plus a third the composite does not serve.
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

// combo serves both testCam and testSens from one identity.
type combo struct {
	Named
	AlwaysRebuild
	TriviallyCloseable
	closes int
}

func (c *combo) Snap() string                        { return "img" }
func (c *combo) Read() int                            { return 42 }
func (c *combo) Close(ctx context.Context) error      { c.closes++; return nil }

func TestRegisterMultiAPIRecordsSet(t *testing.T) {
	model := NewModel("acme", "test", "combo1")
	RegisterMultiAPI(
		[]API{testCamAPI, testSensAPI}, model,
		Registration[Resource, NoNativeConfig]{
			Constructor: func(_ context.Context, _ Dependencies, conf Config, _ logging.Logger) (Resource, error) {
				return &combo{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer Deregister(testCamAPI, model)
	defer Deregister(testSensAPI, model)

	// the one constructor is registered under each api
	_, ok := LookupRegistration(testCamAPI, model)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = LookupRegistration(testSensAPI, model)
	test.That(t, ok, test.ShouldBeTrue)

	// the full API set is recorded
	apis := APIsForModel(model)
	test.That(t, apis, test.ShouldHaveLength, 2)
	test.That(t, apis, test.ShouldContain, testCamAPI)
	test.That(t, apis, test.ShouldContain, testSensAPI)

	// APIModelsFor expands to one entry per api (for ModularMain)
	test.That(t, APIModelsFor(model), test.ShouldHaveLength, 2)
}

func TestSingleAPIRecordsNoSet(t *testing.T) {
	model := NewModel("acme", "test", "single")
	RegisterMultiAPI(
		[]API{testCamAPI}, model,
		Registration[Resource, NoNativeConfig]{
			Constructor: func(_ context.Context, _ Dependencies, conf Config, _ logging.Logger) (Resource, error) {
				return &combo{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer Deregister(testCamAPI, model)

	// a single-API model records no composite set but is still registered
	test.That(t, APIsForModel(model), test.ShouldBeNil)
	_, ok := LookupRegistration(testCamAPI, model)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestCompositeResourceExtraction(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)

	// APIsOf reports every served API
	test.That(t, APIsOf(composite), test.ShouldHaveLength, 2)

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
	test.That(t, subresourceForAPI(c, testSensAPI), test.ShouldEqual, c)

	// APIsOf on an ordinary resource returns its single API
	test.That(t, APIsOf(c), test.ShouldResemble, []API{testCamAPI})
}

func TestCompositeCloseOnce(t *testing.T) {
	c := &combo{Named: NewName(testCamAPI, "dev").AsNamed()}
	// two APIs map to the same instance; Close must fire exactly once
	composite := NewMultiAPIResource(
		NewName(testCamAPI, "dev"),
		[]API{testCamAPI, testSensAPI},
		map[API]Resource{testCamAPI: c, testSensAPI: c},
	)
	test.That(t, composite.Close(context.Background()), test.ShouldBeNil)
	test.That(t, c.closes, test.ShouldEqual, 1)
}
