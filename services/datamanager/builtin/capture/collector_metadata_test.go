package capture

import (
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
		arm := arm.Named("arm1")
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
			Name:             arm,
			Method:           "JointPositions",
			AdditionalParams: map[string]interface{}{"some": "input"},
		}, arm.API)
		test.That(t, md, test.ShouldResemble, collectorMetadata{
			ResourceName: "arm1",
			MethodParams: "map[some:input]",
			MethodMetadata: data.MethodMetadata{
				API:        arm.API,
				MethodName: "JointPositions",
			},
		})
	})

	t.Run("String()", func(t *testing.T) {
		md := newCollectorMetadata(datamanager.DataCaptureConfig{
			Name:             arm.Named("arm1"),
			Method:           "JointPositions",
			AdditionalParams: map[string]interface{}{"some": "input"},
		}, arm.Named("arm1").API)
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
