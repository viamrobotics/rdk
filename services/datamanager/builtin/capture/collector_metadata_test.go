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

// resolvedCollector builds a ResolvedCollector the way ResolveCollector does, for tests that construct
// CollectorConfigsByResource directly without going through resolution.
func resolvedCollector(cfg datamanager.DataCaptureConfig) ResolvedCollector {
	return ResolvedCollector{Metadata: newCollectorMetadata(cfg), Config: cfg}
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

func TestResolveOwningAPI(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	// cfg builds a capture config whose Name carries api; pinned marks it a real pin via PinnedAPI.
	cfg := func(api resource.API, method string, pinned bool) datamanager.DataCaptureConfig {
		c := datamanager.DataCaptureConfig{Name: resource.NewName(api, "r"), Method: method}
		if pinned {
			c.PinnedAPI = api
		}
		return c
	}
	composite := []resource.API{capAlphaAPI, capBetaAPI}

	t.Run("single-API resource yields its one API", func(t *testing.T) {
		got, err := resolveOwningAPI([]resource.API{fakeAPI}, cfg(fakeAPI, "GetReadings", false))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, fakeAPI)
	})

	t.Run("unpinned: a method one API owns resolves to that API", func(t *testing.T) {
		// Configured alpha, but BetaOnly is owned by beta.
		got, err := resolveOwningAPI(composite, cfg(capAlphaAPI, "BetaOnly", false))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, capBetaAPI)
	})

	t.Run("unpinned: a shared method falls back to the configured API", func(t *testing.T) {
		got, err := resolveOwningAPI(composite, cfg(capAlphaAPI, "Shared", false))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, capAlphaAPI)
	})

	t.Run("unpinned: a method no API owns errors", func(t *testing.T) {
		_, err := resolveOwningAPI(composite, cfg(capAlphaAPI, "Nonexistent", false))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no API that captures")
	})

	t.Run("pinned to an owner resolves to the pin", func(t *testing.T) {
		got, err := resolveOwningAPI(composite, cfg(capBetaAPI, "BetaOnly", true))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, capBetaAPI)
	})

	t.Run("pinned to a served API that does not capture the method errors", func(t *testing.T) {
		_, err := resolveOwningAPI(composite, cfg(capAlphaAPI, "BetaOnly", true))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "does not capture method")
	})

	t.Run("pinned to an unserved API errors", func(t *testing.T) {
		_, err := resolveOwningAPI(composite, cfg(fakeAPI, "AlphaOnly", true))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "does not serve")
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
	resolved, owningRes, err := NewCollectorResolver(deps).Resolve(cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, owningRes, test.ShouldEqual, msSub)

	c := newTestCapture(t, CollectorConfigsByResource{owningRes: {resolved}}, nil, deps, nil)
	got := c.newCollectors(CollectorConfigsByResource{owningRes: {resolved}}, Config{CaptureDir: captureDir}, nil)
	test.That(t, len(got), test.ShouldEqual, 1)

	// The collector constructor received the movement_sensor sub, tagged + laid out under that API.
	test.That(t, compositeReadingsCapture.sub, test.ShouldEqual, msSub)
	test.That(t, compositeReadingsCapture.componentType, test.ShouldEqual, capMSAPI.String())
	_, statErr := os.Stat(filepath.Join(captureDir, "rdk_component_movement_sensor", "combo", "Readings"))
	test.That(t, statErr, test.ShouldBeNil)
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
		Name: resource.NewName(capAlphaAPI, "combo"), PinnedAPI: capAlphaAPI, Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}
	cfgBeta := datamanager.DataCaptureConfig{
		Name: resource.NewName(capBetaAPI, "combo"), PinnedAPI: capBetaAPI, Method: "Shared", CaptureFrequencyHz: 1, CaptureDirectory: captureDir,
	}

	// A pinned name resolves to that owner's sub, even though the method is served by both APIs.
	resolvedAlpha, subAlpha, err := NewCollectorResolver(deps).Resolve(cfgAlpha)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subAlpha, test.ShouldEqual, alphaSub)
	resolvedBeta, subBeta, err := NewCollectorResolver(deps).Resolve(cfgBeta)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subBeta, test.ShouldEqual, betaSub)

	c := newTestCapture(t, nil, nil, deps, nil)
	got := c.newCollectors(CollectorConfigsByResource{subAlpha: {resolvedAlpha}, subBeta: {resolvedBeta}}, Config{CaptureDir: captureDir}, nil)

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

	_, _, err := NewCollectorResolver(deps).Resolve(cfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "multiple APIs")
}

// TestResolveCollectorRejectsUnservedPinSingleAPI checks that a single-API resource whose config pins an API
// it doesn't serve is rejected. Resolution would otherwise ignore the bad pin and fall back to the one served
// API, letting it slip through and fail later with a confusing missing-collector error.
func TestResolveCollectorRejectsUnservedPinSingleAPI(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	// Resource serves only alpha; the config pins beta, which it does not serve.
	alphaSub := &fakeResource{name: resource.NewName(capAlphaAPI, "combo")}
	deps := resource.Dependencies{alphaSub.Name(): alphaSub}

	cfg := datamanager.DataCaptureConfig{
		Name: resource.NewName(capBetaAPI, "combo"), PinnedAPI: capBetaAPI, Method: "Shared",
	}

	_, _, err := NewCollectorResolver(deps).Resolve(cfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not serve")
}

// TestResolveCollectorRejectsUnservedAPI checks that pinning a capture config name to an API the composite does not
// serve fails with a clear "does not serve" error rather than a bare resource-not-found from the lookup.
func TestResolveCollectorRejectsUnservedAPI(t *testing.T) {
	registerCaptureAPIResolutionCollectors()

	// Composite serves alpha + beta (both own "Shared"); the config pins camera, which it does not serve.
	alphaSub := &fakeResource{name: resource.NewName(capAlphaAPI, "combo")}
	betaSub := &fakeResource{name: resource.NewName(capBetaAPI, "combo")}
	deps := resource.Dependencies{alphaSub.Name(): alphaSub, betaSub.Name(): betaSub}

	cfg := datamanager.DataCaptureConfig{
		Name: resource.NewName(capCamAPI, "combo"), PinnedAPI: capCamAPI, Method: "Shared",
	}
	_, _, err := NewCollectorResolver(deps).Resolve(cfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not serve")
}
