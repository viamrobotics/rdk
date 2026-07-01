package generic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errDoFailed         = errors.New("do failed")
	errGetStatusFailed  = errors.New("can't get status")
	errGeometriesFailed = errors.New("can't get geometries")
)

// nonShapedGeneric is a minimal generic resource that does not implement [resource.Shaped].
// It is used to verify that the server safely handles generic components which have not opted in
// to providing geometries.
type nonShapedGeneric struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

func (n *nonShapedGeneric) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func newServer(logger logging.Logger) (genericpb.GenericServiceServer, *inject.GenericComponent, *inject.GenericComponent, error) {
	injectGeneric := &inject.GenericComponent{}
	injectGeneric2 := &inject.GenericComponent{}
	resourceMap := map[resource.Name]resource.Resource{
		generic.Named(testGenericName): injectGeneric,
		generic.Named(failGenericName): injectGeneric2,
	}
	injectSvc, err := resource.NewAPIResourceCollection(generic.API, resourceMap)
	if err != nil {
		return nil, nil, nil, errtrace.Wrap(err)
	}
	return generic.NewRPCServiceServer(injectSvc, logger).(genericpb.GenericServiceServer), injectGeneric, injectGeneric2, nil
}

func TestGenericDo(t *testing.T) {
	genericServer, workingGeneric, failingGeneric, err := newServer(logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	workingGeneric.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return cmd, nil
	}
	failingGeneric.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return nil, errtrace.Wrap(errDoFailed)
	}

	commandStruct, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)

	req := commonpb.DoCommandRequest{Name: testGenericName, Command: commandStruct}
	resp, err := genericServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp.Result.AsMap()["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
	test.That(t, resp.Result.AsMap()["data"], test.ShouldEqual, testutils.TestCommand["data"])

	req = commonpb.DoCommandRequest{Name: failGenericName, Command: commandStruct}
	resp, err = genericServer.DoCommand(context.Background(), &req)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())
	test.That(t, resp, test.ShouldBeNil)
}

func TestGenericGetStatus(t *testing.T) {
	genericServer, workingGeneric, _, err := newServer(logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	_, err = genericServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: "missingGeneric"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	resp, err := genericServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testGenericName})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Result.AsMap(), test.ShouldBeEmpty)

	expectedStatus := map[string]interface{}{"key": "value", "count": float64(42)}
	workingGeneric.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return expectedStatus, nil
	}
	resp, err = genericServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testGenericName})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Result.AsMap(), test.ShouldResemble, expectedStatus)

	workingGeneric.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return nil, errtrace.Wrap(errGetStatusFailed)
	}
	_, err = genericServer.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: testGenericName})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errGetStatusFailed.Error())
	workingGeneric.StatusFunc = nil
}

func TestGenericGetGeometries(t *testing.T) {
	const nonShapedName = "nonShapedGeneric"
	logger := logging.NewTestLogger(t)
	injectGeneric := &inject.GenericComponent{}
	injectGeneric2 := &inject.GenericComponent{}
	resourceMap := map[resource.Name]resource.Resource{
		generic.Named(testGenericName): injectGeneric,
		generic.Named(failGenericName): injectGeneric2,
		generic.Named(nonShapedName):   &nonShapedGeneric{Named: generic.Named(nonShapedName).AsNamed()},
	}
	injectSvc, err := resource.NewAPIResourceCollection(generic.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	genericServer := generic.NewRPCServiceServer(injectSvc, logger).(genericpb.GenericServiceServer)

	expectedGeometries := []spatialmath.Geometry{
		spatialmath.NewPoint(r3.Vector{X: 1, Y: 2, Z: 3}, "pt1"),
		spatialmath.NewPoint(r3.Vector{X: 4, Y: 5, Z: 6}, "pt2"),
	}
	var receivedExtra map[string]interface{}
	injectGeneric.GeometriesFunc = func(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
		receivedExtra = extra
		return expectedGeometries, nil
	}
	injectGeneric2.GeometriesFunc = func(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
		return nil, errtrace.Wrap(errGeometriesFailed)
	}

	t.Run("missing resource returns error", func(t *testing.T) {
		_, err := genericServer.GetGeometries(context.Background(), &commonpb.GetGeometriesRequest{Name: "missingGeneric"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	})

	t.Run("resource implementing Shaped returns geometries and forwards extra", func(t *testing.T) {
		extra := map[string]interface{}{"foo": "Geometries"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		resp, err := genericServer.GetGeometries(
			context.Background(),
			&commonpb.GetGeometriesRequest{Name: testGenericName, Extra: ext},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedExtra, test.ShouldResemble, extra)
		got, err := referenceframe.NewGeometriesFromProto(resp.GetGeometries())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldHaveLength, len(expectedGeometries))
		for i, g := range got {
			test.That(t, spatialmath.GeometriesAlmostEqual(expectedGeometries[i], g), test.ShouldBeTrue)
		}
	})

	t.Run("error from Geometries propagates", func(t *testing.T) {
		_, err := genericServer.GetGeometries(context.Background(), &commonpb.GetGeometriesRequest{Name: failGenericName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGeometriesFailed.Error())
	})

	t.Run("resource not implementing Shaped returns empty response", func(t *testing.T) {
		resp, err := genericServer.GetGeometries(context.Background(), &commonpb.GetGeometriesRequest{Name: nonShapedName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetGeometries(), test.ShouldBeEmpty)
	})
}
