// Package main tests out all the custom models in the multiplemodules.
package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/samber/lo"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	otlpv1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	gtestutils "go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func TestMultipleModules(t *testing.T) {
	logger, observer := logging.NewObservedTestLogger(t)
	testViamHome := t.TempDir()
	var port int
	success := false
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		// Modify the example config to run directly, without compiling the module first.
		cfgFilename, portLocal, err := modifyCfg(t, utils.ResolveFile("examples/customresources/demos/multiplemodules/module.json"), logger)
		port = portLocal
		test.That(t, err, test.ShouldBeNil)

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger,
			robottestutils.WithViamHome(testViamHome))

		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if robottestutils.WaitForServing(observer, port) {
			success = true
			defer func() {
				test.That(t, server.Stop(), test.ShouldBeNil)
			}()
			break
		}
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc, err := connect(port, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, rc.Close(context.Background()), test.ShouldBeNil)
	}()

	// Gizmo is a custom component model and API.
	t.Run("Test Gizmo", func(t *testing.T) {
		res, err := rc.ResourceByName(gizmoapi.Named("gizmo1"))
		test.That(t, err, test.ShouldBeNil)

		giz := res.(gizmoapi.Gizmo)
		retDoTwo, err := giz.DoTwo(context.Background(), true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retDoTwo, test.ShouldEqual, "sum=4")

		// Test that spans from both modules are sent to viam-server and eventually
		// exported to its traces file on disk.
		gtestutils.WaitForAssertionWithSleep(t, time.Millisecond*500, 20, func(t testing.TB) {
			checkTraceContents(t, testViamHome,
				spanExpectation{
					service:      "rdk",
					kind:         otlpv1.Span_SPAN_KIND_SERVER,
					rpcService:   "acme.component.gizmo.v1.GizmoService",
					rpcMethod:    "DoTwo",
					resourceName: "gizmo1",
				},
				spanExpectation{
					service:    "rdk",
					kind:       otlpv1.Span_SPAN_KIND_CLIENT,
					rpcService: "acme.component.gizmo.v1.GizmoService",
					rpcMethod:  "DoTwo",
				},
				spanExpectation{
					service:      "GizmoModule",
					kind:         otlpv1.Span_SPAN_KIND_SERVER,
					rpcService:   "acme.component.gizmo.v1.GizmoService",
					rpcMethod:    "DoTwo",
					resourceName: "gizmo1",
				},
				spanExpectation{
					service:    "GizmoModule",
					kind:       otlpv1.Span_SPAN_KIND_CLIENT,
					rpcService: "acme.service.summation.v1.SummationService",
					rpcMethod:  "Sum",
				},
				spanExpectation{
					service:      "rdk",
					kind:         otlpv1.Span_SPAN_KIND_SERVER,
					rpcService:   "acme.service.summation.v1.SummationService",
					rpcMethod:    "Sum",
					resourceName: "adder",
				},
				spanExpectation{
					service:    "rdk",
					kind:       otlpv1.Span_SPAN_KIND_CLIENT,
					rpcService: "acme.service.summation.v1.SummationService",
					rpcMethod:  "Sum",
				},
				spanExpectation{
					service:      "SummationModule",
					kind:         otlpv1.Span_SPAN_KIND_SERVER,
					rpcService:   "acme.service.summation.v1.SummationService",
					rpcMethod:    "Sum",
					resourceName: "adder",
				},
			)
		})

		retDoTwo, err = giz.DoTwo(context.Background(), false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retDoTwo, test.ShouldEqual, "sum=5")

		retDoOne, err := giz.DoOne(context.Background(), "1.0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retDoOne, test.ShouldBeTrue)

		// also tests that the ForeignServiceHandler does not drop the first message
		retClientStream, err := giz.DoOneClientStream(context.Background(), []string{"1.0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retClientStream, test.ShouldBeFalse)

		retClientStream, err = giz.DoOneClientStream(context.Background(), []string{"0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retClientStream, test.ShouldBeTrue)

		retServerStream, err := giz.DoOneServerStream(context.Background(), "1.0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retServerStream, test.ShouldResemble, []bool{true, false, true, false})

		retBiDiStream, err := giz.DoOneBiDiStream(context.Background(), []string{"1.0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retBiDiStream, test.ShouldResemble, []bool{true, true, true})
	})

	// Summation is a custom service model and API.
	t.Run("Test Summation", func(t *testing.T) {
		res, err := rc.ResourceByName(summationapi.Named("adder"))
		test.That(t, err, test.ShouldBeNil)
		add := res.(summationapi.Summation)
		nums := []float64{10, 0.5, 12}
		retAdd, err := add.Sum(context.Background(), nums)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retAdd, test.ShouldEqual, 22.5)
	})
}

// TestWebRTCSpans verifies that gRPC calls made over WebRTC connections generate
// spans.
func TestWebRTCSpans(t *testing.T) {
	logger, observer := logging.NewObservedTestLogger(t)
	testViamHome := t.TempDir()
	var port int
	success := false
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		portLocal, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		port = portLocal

		cfgJSON := fmt.Sprintf(`{
			"components": [{"name": "motor-1", "type": "motor", "model": "fake"}],
			"network": {"bind_address": "localhost:%d"},
			"tracing": {"enabled": true, "disk": true}
		}`, port)
		cfgFile, err := os.CreateTemp(t.TempDir(), "viam-test-config-*")
		test.That(t, err, test.ShouldBeNil)
		_, err = cfgFile.WriteString(cfgJSON)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfgFile.Close(), test.ShouldBeNil)

		server := robottestutils.ServerAsSeparateProcess(t, cfgFile.Name(), logger,
			robottestutils.WithViamHome(testViamHome))

		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if robottestutils.WaitForServing(observer, port) {
			success = true
			defer func() {
				test.That(t, server.Stop(), test.ShouldBeNil)
			}()
			break
		}
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc, err := connect(port, logger, rpc.WithDisableDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, rc.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("Test WebRTC spans", func(t *testing.T) {
		m, err := motor.FromProvider(rc, "motor-1")
		test.That(t, err, test.ShouldBeNil)
		_, err = m.IsMoving(context.Background())
		test.That(t, err, test.ShouldBeNil)

		gtestutils.WaitForAssertionWithSleep(t, time.Millisecond*500, 20, func(t testing.TB) {
			checkTraceContents(t, testViamHome,
				spanExpectation{
					service:      "rdk",
					kind:         otlpv1.Span_SPAN_KIND_SERVER,
					rpcService:   "viam.component.motor.v1.MotorService",
					rpcMethod:    "IsMoving",
					resourceName: "motor-1",
				},
			)
		})
	})
}

func connect(port int, logger logging.Logger, dialOpts ...rpc.DialOption) (robot.Robot, error) {
	connectCtx, cancelConn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelConn()
	for {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		rc, err := client.New(dialCtx, fmt.Sprintf("localhost:%d", port), logger,
			client.WithDialOptions(dialOpts...),
			client.WithDisableSessions(), // TODO(PRODUCT-343): add session support to modules
		)
		dialCancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			return rc, err
		}
		select {
		case <-connectCtx.Done():
			return nil, connectCtx.Err()
		default:
		}
	}
}

func modifyCfg(t *testing.T, cfgIn string, logger logging.Logger) (string, int, error) {
	gizmoModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/gizmomodule")
	summationModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/summationmodule")

	port, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", 0, err
	}

	cfg, err := config.Read(context.Background(), cfgIn, logger, nil)
	if err != nil {
		return "", 0, err
	}
	cfg.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	cfg.Modules[0].ExePath = gizmoModPath
	cfg.Modules[1].ExePath = summationModPath
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", 0, err
	}
	file, err := os.CreateTemp(t.TempDir(), "viam-test-config-*")
	if err != nil {
		return "", 0, err
	}
	cfgFilename := file.Name()
	_, err = file.Write(output)
	if err != nil {
		return "", 0, err
	}
	return cfgFilename, port, file.Close()
}

type spanExpectation struct {
	service      string
	rpcService   string
	rpcMethod    string
	kind         otlpv1.Span_SpanKind
	resourceName string // viam.resource.name attribute, if expected
}

// Check that all the specified services exist in the viam trace file
func checkTraceContents(t testing.TB, viamHome string, expectations ...spanExpectation) {
	tracesPath := filepath.Join(viamHome, ".viam", "trace", "local-config", "traces")
	tracesFile, err := os.Open(tracesPath)
	test.That(t, err, test.ShouldBeNil)
	defer tracesFile.Close()

	spansByService := map[string][]*otlpv1.Span{}

	protosReader := protoutils.NewDelimitedProtoReader[otlpv1.ResourceSpans](tracesFile)
	for resourceSpan := range protosReader.All() {
		var serviceName string
		for _, attr := range resourceSpan.Resource.Attributes {
			keyStr := attr.Key
			if keyStr == string(semconv.ServiceNameKey) {
				serviceName = attr.Value.GetStringValue()
				break
			}
		}
		if serviceName == "" {
			t.Error("Failed to find service name in trace tags")
			continue
		}
		spans := lo.Flatten(
			lo.Map(resourceSpan.ScopeSpans, func(span *otlpv1.ScopeSpans, _ int) []*otlpv1.Span {
				return span.Spans
			}),
		)
		spansByService[serviceName] = append(spansByService[serviceName], spans...)
	}

	if len(spansByService) < 1 {
		t.Fail()
		return
	}
	for _, exp := range expectations {
		spans, ok := spansByService[exp.service]
		test.That(t, ok, test.ShouldBeTrue)
		matchingSpans := lo.Filter(spans, func(span *otlpv1.Span, _ int) bool {
			if span.Kind != exp.kind {
				return false
			}
			attrs := lo.SliceToMap(span.Attributes, func(item *otlpcommonv1.KeyValue) (string, string) {
				return item.Key, item.Value.GetStringValue()
			})
			if exp.rpcMethod != attrs[string(semconv.RPCMethodKey)] {
				return false
			}
			if exp.rpcService != attrs[string(semconv.RPCServiceKey)] {
				return false
			}
			if exp.resourceName != "" && exp.resourceName != attrs["viam.resource.name"] {
				return false
			}
			return true
		})
		test.That(t, matchingSpans, test.ShouldNotBeEmpty)
	}
}
