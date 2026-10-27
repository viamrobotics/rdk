package capture

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/benbjohnson/clock"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

func TestCollectorMetadata(t *testing.T) {
	t.Run("newCollectorMetadata()", func(t *testing.T) {
		armName := arm.Named("arm1")
		md := newCollectorMetadata(&fakeResource{name: armName}, datamanager.DataCaptureConfig{
			Name:             armName,
			Method:           "JointPositions",
			AdditionalParams: map[string]interface{}{"some": "input"},
		})
		test.That(t, md, test.ShouldResemble, collectorMetadata{
			ResourceName: "arm1",
			MethodParams: "map[some:input]",
			MethodMetadata: data.MethodMetadata{
				API:        armName.API,
				MethodName: "JointPositions",
			},
		})
	})

	t.Run("String()", func(t *testing.T) {
		md := newCollectorMetadata(&fakeResource{name: arm.Named("arm1")}, datamanager.DataCaptureConfig{
			Name:             arm.Named("arm1"),
			Method:           "JointPositions",
			AdditionalParams: map[string]interface{}{"some": "input"},
		})
		exp := "[Resource Name: arm1, API: rdk:component:arm, Method Name: JointPositions, Method Params: map[some:input]]"
		test.That(t, md.String(), test.ShouldResemble, exp)
	})
}

var (
	capAlphaAPI = resource.APINamespaceRDK.WithComponentType("capalpha")
	capBetaAPI  = resource.APINamespaceRDK.WithComponentType("capbeta")

	registerCaptureAPIResolutionCollectorsOnce sync.Once
)

func registerCaptureAPIResolutionCollectors() {
	registerCaptureAPIResolutionCollectorsOnce.Do(func() {
		noop := func(_ interface{}, _ data.CollectorParams) (data.Collector, error) { return &mockCollector{}, nil }
		data.RegisterCollector(data.MethodMetadata{API: capAlphaAPI, MethodName: "AlphaOnly"}, noop)
		data.RegisterCollector(data.MethodMetadata{API: capBetaAPI, MethodName: "BetaOnly"}, noop)
		// A method both served APIs implement — the shared-method case (e.g. DoCommand).
		data.RegisterCollector(data.MethodMetadata{API: capAlphaAPI, MethodName: "Shared"}, noop)
		data.RegisterCollector(data.MethodMetadata{API: capBetaAPI, MethodName: "Shared"}, noop)
	})
}

func TestCaptureAPIForMethod(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	t.Run("single-API resource always yields its configured API", func(t *testing.T) {
		res := newFakeRes("fake-1")
		test.That(t, captureAPIForMethod(res, "GetReadings", fakeAPI), test.ShouldResemble, fakeAPI)
		// Even a method with no registered collector resolves to the single API.
		test.That(t, captureAPIForMethod(res, "Nonexistent", fakeAPI), test.ShouldResemble, fakeAPI)
	})

	t.Run("composite resolves each method to its owning API", func(t *testing.T) {
		composite, err := resource.Compose(
			resource.AsSub(capAlphaAPI, newFakeRes("composite-1")),
			resource.AsSub(capBetaAPI, newFakeRes("composite-1")),
		)
		test.That(t, err, test.ShouldBeNil)

		// A method owned by exactly one served API resolves to that API, even when it is not the
		// resource's configured API — this is the composite mis-tagging fix.
		test.That(t, captureAPIForMethod(composite, "AlphaOnly", capAlphaAPI), test.ShouldResemble, capAlphaAPI)
		test.That(t, captureAPIForMethod(composite, "BetaOnly", capAlphaAPI), test.ShouldResemble, capBetaAPI)

		// A method several served APIs share stays under the configured API.
		test.That(t, captureAPIForMethod(composite, "Shared", capAlphaAPI), test.ShouldResemble, capAlphaAPI)

		// A method no served API implements falls back to the configured API.
		test.That(t, captureAPIForMethod(composite, "Nonexistent", capAlphaAPI), test.ShouldResemble, capAlphaAPI)
	})
}

var (
	capMSAPI  = resource.APINamespaceRDK.WithComponentType("movement_sensor")
	capCamAPI = resource.APINamespaceRDK.WithComponentType("camera")

	compositeReadingsCapture struct {
		sub           interface{}
		componentType string
	}
	registerCompositeReadingsOnce sync.Once
)

func registerCompositeReadingsCollector() {
	registerCompositeReadingsOnce.Do(func() {
		data.RegisterCollector(data.MethodMetadata{API: capMSAPI, MethodName: "Readings"},
			func(sub interface{}, p data.CollectorParams) (data.Collector, error) {
				compositeReadingsCapture.sub = sub
				compositeReadingsCapture.componentType = p.ComponentType
				return &mockCollector{}, nil
			})
	})
}

// TestNewCollectorsTagsCompositeUnderOwningAPI drives newCollectors with a composite configured as a
// camera that also serves movement_sensor, capturing Readings. The collector must be built from the
// movement_sensor sub, not the composite, and tagged and laid out under the movement_sensor API.
func TestNewCollectorsTagsCompositeUnderOwningAPI(t *testing.T) {
	registerCompositeReadingsCollector()
	compositeReadingsCapture.sub = nil
	compositeReadingsCapture.componentType = ""

	// Both subs share one identity (name); AsSub serves each under its own API. They are distinct objects
	// so the test can tell which one the collector constructor received.
	comboName := resource.NewName(capCamAPI, "combo")
	camSub := &fakeResource{name: comboName}
	msSub := &fakeResource{name: comboName}
	composite, err := resource.Compose(resource.AsSub(capCamAPI, camSub), resource.AsSub(capMSAPI, msSub))
	test.That(t, err, test.ShouldBeNil)

	captureDir := t.TempDir()
	cfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(capCamAPI, "combo"),
		Method:             "Readings",
		CaptureFrequencyHz: 1.0,
		CaptureDirectory:   captureDir,
	}
	c := newTestCapture(t, CollectorConfigsByResource{composite: {cfg}}, nil, nil, nil)

	got := c.newCollectors(CollectorConfigsByResource{composite: {cfg}}, Config{CaptureDir: captureDir}, nil)
	test.That(t, len(got), test.ShouldEqual, 1)

	// The collector constructor received the movement_sensor sub, not the composite wrapper.
	test.That(t, compositeReadingsCapture.sub, test.ShouldEqual, msSub)
	test.That(t, compositeReadingsCapture.sub, test.ShouldNotEqual, composite)
	// Tagged under the movement_sensor API, and its capture files land under that API directory.
	test.That(t, compositeReadingsCapture.componentType, test.ShouldEqual, capMSAPI.String())
	_, statErr := os.Stat(filepath.Join(captureDir, "rdk_component_movement_sensor", "combo", "Readings"))
	test.That(t, statErr, test.ShouldBeNil)
}

func TestUnresolvedCaptureMethodError(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	composite, err := resource.Compose(
		resource.AsSub(capAlphaAPI, newFakeRes("combo")),
		resource.AsSub(capBetaAPI, newFakeRes("combo")),
	)
	test.That(t, err, test.ShouldBeNil)

	// A single-API resource gets no special error (the generic lookup error stands).
	test.That(t, unresolvedCaptureMethodError(newFakeRes("x"), datamanager.DataCaptureConfig{
		Name: resource.NewName(fakeAPI, "x"), Method: "Nonexistent",
	}), test.ShouldBeNil)

	// A method several served APIs own is ambiguous: name the owners and how to fix it.
	err = unresolvedCaptureMethodError(composite, datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Shared",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "multiple APIs")
	test.That(t, err.Error(), test.ShouldContainSubstring, capAlphaAPI.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, capBetaAPI.String())

	// A method no served API implements.
	err = unresolvedCaptureMethodError(composite, datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Nonexistent",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no API")
}

// TestNewCollectorsPinnedAPIDisambiguatesCollidingMethod drives newCollectors with a composite whose two
// served APIs both implement the same capture method (the colliding case). Pinning the capture config
// name's API to each owner builds a separate collector laid out under that API.
func TestNewCollectorsPinnedAPIDisambiguatesCollidingMethod(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	comboName := resource.NewName(capAlphaAPI, "combo")
	composite, err := resource.Compose(
		resource.AsSub(capAlphaAPI, &fakeResource{name: comboName}),
		resource.AsSub(capBetaAPI, &fakeResource{name: comboName}),
	)
	test.That(t, err, test.ShouldBeNil)

	captureDir := t.TempDir()
	// Same method ("Shared", owned by both APIs), each capture config name pinned to a different owner.
	cfgAlpha := datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}
	cfgBeta := datamanager.DataCaptureConfig{
		Name: resource.NewName(capBetaAPI, "combo"), Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}

	c := newTestCapture(t, nil, nil, nil, nil)
	got := c.newCollectors(CollectorConfigsByResource{composite: {cfgAlpha, cfgBeta}}, Config{CaptureDir: captureDir}, nil)

	// Two distinct collectors, each tagged and laid out under the API its name pinned.
	test.That(t, len(got), test.ShouldEqual, 2)
	_, statErr := os.Stat(filepath.Join(captureDir, "rdk_component_capalpha", "combo", "Shared"))
	test.That(t, statErr, test.ShouldBeNil)
	_, statErr = os.Stat(filepath.Join(captureDir, "rdk_component_capbeta", "combo", "Shared"))
	test.That(t, statErr, test.ShouldBeNil)
}

// TestNewCollectorsAmbiguousCollidingMethodErrors drives newCollectors with a composite that serves the
// two colliding owners plus a third API it is configured under that does not implement the method. With
// no owner pinned, the method cannot resolve to one API, so no collector is built and the failure names
// the owners and how to fix it.
func TestNewCollectorsAmbiguousCollidingMethodErrors(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	comboName := resource.NewName(capAlphaAPI, "combo")
	composite, err := resource.Compose(
		resource.AsSub(capAlphaAPI, &fakeResource{name: comboName}),
		resource.AsSub(capBetaAPI, &fakeResource{name: comboName}),
		resource.AsSub(capCamAPI, &fakeResource{name: comboName}),
	)
	test.That(t, err, test.ShouldBeNil)

	captureDir := t.TempDir()
	// Name carries the configured, non-owning API (camera), standing in for an unpinned method whose API
	// resolveAssociatedName filled from the resource's configured API.
	cfg := datamanager.DataCaptureConfig{
		Name: resource.NewName(capCamAPI, "combo"), Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}

	logger, logs := logging.NewObservedTestLogger(t)
	c := &Capture{
		logger:               logger,
		clk:                  clock.New(),
		collectors:           make(collectors),
		captureDir:           captureDir,
		maxCaptureFileSize:   256 * 1024,
		resourcesByShortName: map[string]resource.Resource{},
	}
	got := c.newCollectors(CollectorConfigsByResource{composite: {cfg}}, Config{CaptureDir: captureDir}, nil)

	// No collector built, and the logged failure is the actionable ambiguity error.
	test.That(t, len(got), test.ShouldEqual, 0)
	entries := logs.FilterMessageSnippet("failed to initialize or update collector").All()
	test.That(t, len(entries), test.ShouldEqual, 1)
	test.That(t, entries[0].ContextMap()["error"], test.ShouldContainSubstring, "multiple APIs")
}
