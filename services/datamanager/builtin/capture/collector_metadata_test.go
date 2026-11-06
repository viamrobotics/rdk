package capture

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
)

func TestCollectorMetadata(t *testing.T) {
	t.Run("newCollectorMetadata()", func(t *testing.T) {
		armName := arm.Named("arm1")
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
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
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
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
		apis := []resource.API{fakeAPI}
		test.That(t, captureAPIForMethod(apis, "GetReadings", fakeAPI), test.ShouldResemble, fakeAPI)
		// Even a method with no registered collector resolves to the single API.
		test.That(t, captureAPIForMethod(apis, "Nonexistent", fakeAPI), test.ShouldResemble, fakeAPI)
	})

	t.Run("composite resolves each method to its owning API", func(t *testing.T) {
		apis := []resource.API{capAlphaAPI, capBetaAPI}

		// A method owned by exactly one served API resolves to that API, even when it is not the
		// resource's configured API.
		test.That(t, captureAPIForMethod(apis, "AlphaOnly", capAlphaAPI), test.ShouldResemble, capAlphaAPI)
		test.That(t, captureAPIForMethod(apis, "BetaOnly", capAlphaAPI), test.ShouldResemble, capBetaAPI)

		// A method several served APIs share stays under the configured API.
		test.That(t, captureAPIForMethod(apis, "Shared", capAlphaAPI), test.ShouldResemble, capAlphaAPI)

		// A method no served API implements falls back to the configured API.
		test.That(t, captureAPIForMethod(apis, "Nonexistent", capAlphaAPI), test.ShouldResemble, capAlphaAPI)
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

	// "combo" is a composite serving camera + movement_sensor; deps hold its two per-API subs.
	camSub := &fakeResource{name: resource.NewName(capCamAPI, "combo")}
	msSub := &fakeResource{name: resource.NewName(capMSAPI, "combo")}
	deps := resource.Dependencies{camSub.Name(): camSub, msSub.Name(): msSub}

	captureDir := t.TempDir()
	cfg := datamanager.DataCaptureConfig{
		Name:               resource.NewName(capCamAPI, "combo"),
		Method:             "Readings",
		CaptureFrequencyHz: 1.0,
		CaptureDirectory:   captureDir,
	}

	// Resolution (upstream of newCollectors) picks the movement_sensor sub that owns Readings.
	owningRes, owningAPI, err := OwningResource(deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, owningRes, test.ShouldEqual, msSub)
	// Pin the config to the owning API, as resolution does upstream of newCollectors.
	cfg.Name = cfg.Name.AsAPI(owningAPI)

	c := newTestCapture(t, CollectorConfigsByResource{owningRes: {cfg}}, nil, deps, nil)
	got := c.newCollectors(CollectorConfigsByResource{owningRes: {cfg}}, Config{CaptureDir: captureDir}, nil)
	test.That(t, len(got), test.ShouldEqual, 1)

	// The collector constructor received the movement_sensor sub, tagged + laid out under that API.
	test.That(t, compositeReadingsCapture.sub, test.ShouldEqual, msSub)
	test.That(t, compositeReadingsCapture.componentType, test.ShouldEqual, capMSAPI.String())
	_, statErr := os.Stat(filepath.Join(captureDir, "rdk_component_movement_sensor", "combo", "Readings"))
	test.That(t, statErr, test.ShouldBeNil)
}

func TestUnresolvedCaptureMethodError(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	compositeAPIs := []resource.API{capAlphaAPI, capBetaAPI}

	// A single-API resource gets no special error (the generic lookup error stands).
	test.That(t, unresolvedCaptureMethodError([]resource.API{fakeAPI}, datamanager.DataCaptureConfig{
		Name: resource.NewName(fakeAPI, "x"), Method: "Nonexistent",
	}), test.ShouldBeNil)

	// A method several served APIs own is ambiguous: name the owners and how to fix it.
	err := unresolvedCaptureMethodError(compositeAPIs, datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Shared",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "multiple APIs")
	test.That(t, err.Error(), test.ShouldContainSubstring, capAlphaAPI.String())
	test.That(t, err.Error(), test.ShouldContainSubstring, capBetaAPI.String())

	// A method no served API implements.
	err = unresolvedCaptureMethodError(compositeAPIs, datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Nonexistent",
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "serves no API that implements")
}

// TestNewCollectorsPinnedAPIDisambiguatesCollidingMethod drives newCollectors with a composite whose two
// served APIs both implement the same capture method (the colliding case). Pinning the capture config
// name's API to each owner builds a separate collector laid out under that API.
func TestNewCollectorsPinnedAPIDisambiguatesCollidingMethod(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	alphaSub := &fakeResource{name: resource.NewName(capAlphaAPI, "combo")}
	betaSub := &fakeResource{name: resource.NewName(capBetaAPI, "combo")}
	deps := resource.Dependencies{alphaSub.Name(): alphaSub, betaSub.Name(): betaSub}

	captureDir := t.TempDir()
	// Same method ("Shared", owned by both APIs), each capture config name pinned to a different owner.
	cfgAlpha := datamanager.DataCaptureConfig{
		Name: resource.NewName(capAlphaAPI, "combo"), Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}
	cfgBeta := datamanager.DataCaptureConfig{
		Name: resource.NewName(capBetaAPI, "combo"), Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}

	// A pinned name resolves to that owner's sub, even though the method is served by both APIs.
	subAlpha, _, err := OwningResource(deps, cfgAlpha)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subAlpha, test.ShouldEqual, alphaSub)
	subBeta, _, err := OwningResource(deps, cfgBeta)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subBeta, test.ShouldEqual, betaSub)

	c := newTestCapture(t, nil, nil, deps, nil)
	got := c.newCollectors(CollectorConfigsByResource{subAlpha: {cfgAlpha}, subBeta: {cfgBeta}}, Config{CaptureDir: captureDir}, nil)

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

	alphaSub := &fakeResource{name: resource.NewName(capAlphaAPI, "combo")}
	betaSub := &fakeResource{name: resource.NewName(capBetaAPI, "combo")}
	camSub := &fakeResource{name: resource.NewName(capCamAPI, "combo")}
	deps := resource.Dependencies{alphaSub.Name(): alphaSub, betaSub.Name(): betaSub, camSub.Name(): camSub}

	// Name carries the configured, non-owning API (camera), standing in for an unpinned method whose API
	// resolveAssociatedName filled from the resource's configured API. "Shared" is owned by alpha and beta,
	// neither of which is the configured API, so it can't resolve to one.
	cfg := datamanager.DataCaptureConfig{
		Name: resource.NewName(capCamAPI, "combo"), Method: "Shared",
	}

	_, _, err := OwningResource(deps, cfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "multiple APIs")
}

// TestOwningResourceRejectsUnservedAPI checks that pinning a capture config name to an API the composite does not
// serve fails with a clear "does not serve" error rather than a bare resource-not-found from the lookup.
func TestOwningResourceRejectsUnservedAPI(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	// Composite serves alpha + beta (both own "Shared"); the config pins camera, which it does not serve.
	alphaSub := &fakeResource{name: resource.NewName(capAlphaAPI, "combo")}
	betaSub := &fakeResource{name: resource.NewName(capBetaAPI, "combo")}
	deps := resource.Dependencies{alphaSub.Name(): alphaSub, betaSub.Name(): betaSub}

	cfg := datamanager.DataCaptureConfig{
		Name: resource.NewName(capCamAPI, "combo"), Method: "Shared",
	}
	_, _, err := OwningResource(deps, cfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not serve")
}
