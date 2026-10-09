package web

import (
	"context"
	"strings"
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.viam.com/test"

	"go.viam.com/rdk/internal/otelmetrics"
)

func registerTestMetrics(t *testing.T, svc Service) *otelmetrics.Metrics {
	t.Helper()
	metrics, err := otelmetrics.New(context.Background(), otelmetrics.Config{FTDC: true})
	test.That(t, err, test.ShouldBeNil)
	reg, err := svc.RequestCounter().RegisterMetrics(metrics)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, reg.Unregister(), test.ShouldBeNil)
		test.That(t, metrics.Shutdown(context.Background()), test.ShouldBeNil)
	})
	return metrics
}

// requestStats returns the FTDC keys under "web." with that prefix removed.
func requestStats(metrics *otelmetrics.Metrics) map[string]int64 {
	ret := map[string]int64{}
	for key, value := range metrics.FTDCStatser().Stats().(map[string]float64) {
		if rest, ok := strings.CutPrefix(key, "web."); ok {
			ret[rest] = int64(value)
		}
	}
	return ret
}

func TestDurationFTDCErrors(t *testing.T) {
	const unary = "viam.component.motor.v1.MotorService/IsPowered"
	const stream = "viam.component.inputcontroller.v1.InputControllerService/StreamEvents"
	rc := &RequestCounter{}
	rc.streamMethods.Store(stream, struct{}{})

	stats := map[string]float64{}
	add := func(key string, value float64) { stats[key] += value }
	record := func(method, code string) {
		rc.durationFTDC(otelmetrics.FTDCPoint{Count: 2, Attrs: attribute.NewSet(
			semconv.RPCMethod(method),
			semconv.RPCResponseStatusCode(code),
			otelmetrics.ResourceNameKey.String("r"),
		)}, add)
	}
	record(unary, "CANCELLED")
	record(stream, "CANCELLED")
	record(stream, "UNAVAILABLE")

	test.That(t, stats["web.r.MotorService/IsPowered.errorCnt"], test.ShouldEqual, 2.)
	test.That(t, stats["web.r.InputControllerService/StreamEvents"], test.ShouldEqual, 4.)
	test.That(t, stats["web.r.InputControllerService/StreamEvents.errorCnt"], test.ShouldEqual, 2.)
}

func TestLimitKeyRoundTrip(t *testing.T) {
	for _, key := range []string{
		"arm1.viam.component.arm.v1.ArmService",
		"remote1:arm1.viam.component.arm.v1.ArmService",
		"viam.robot.v1.RobotService",
		"viam.viam.component.arm.v1.ArmService",
	} {
		stats := map[string]float64{}
		limitedFTDC(otelmetrics.FTDCPoint{Value: 3, Attrs: attribute.NewSet(limitKeyAttrs(key)...)},
			func(k string, v float64) { stats[k] += v })
		test.That(t, stats, test.ShouldResemble, map[string]float64{"web." + key + ".inFlightRequests": 3})
	}
	set := attribute.NewSet(limitKeyAttrs("arm1.viam.component.arm.v1.ArmService")...)
	name, ok := set.Value(otelmetrics.ResourceNameKey)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, name.AsString(), test.ShouldEqual, "arm1")
}

func TestLessDirect(t *testing.T) {
	test.That(t, lessDirect(webrtc.ICECandidateTypeHost, webrtc.ICECandidateTypeHost), test.ShouldEqual, "host")
	test.That(t, lessDirect(webrtc.ICECandidateTypeHost, webrtc.ICECandidateTypeSrflx), test.ShouldEqual, "srflx")
	test.That(t, lessDirect(webrtc.ICECandidateTypeRelay, webrtc.ICECandidateTypeSrflx), test.ShouldEqual, "relay")
	test.That(t, lessDirect(webrtc.ICECandidateTypeHost, webrtc.ICECandidateType(0)), test.ShouldEqual, "unknown")
}
