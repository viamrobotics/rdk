package base_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/base"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func setupWorkingBase(
	workingBase *inject.Base,
	argsReceived map[string][]interface{},
	expectedFeatures base.Properties,
	geometries []spatialmath.Geometry,
) {
	workingBase.MoveStraightFunc = func(
		_ context.Context, distanceMm int,
		mmPerSec float64,
		extra map[string]interface{},
	) error {
		argsReceived["MoveStraight"] = []interface{}{distanceMm, mmPerSec, extra}
		return nil
	}

	workingBase.SpinFunc = func(
		_ context.Context, angleDeg, degsPerSec float64, extra map[string]interface{},
	) error {
		argsReceived["Spin"] = []interface{}{angleDeg, degsPerSec, extra}
		return nil
	}

	workingBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return nil
	}

	workingBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
		return expectedFeatures, nil
	}

	workingBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return geometries, nil
	}
}

func setupBrokenBase(brokenBase *inject.Base) {
	brokenBase.MoveStraightFunc = func(
		ctx context.Context,
		distanceMm int, mmPerSec float64,
		extra map[string]interface{},
	) error {
		return errMoveStraight
	}
	brokenBase.SpinFunc = func(
		ctx context.Context,
		angleDeg, degsPerSec float64,
		extra map[string]interface{},
	) error {
		return errSpinFailed
	}
	brokenBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopFailed
	}

	brokenBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
		return base.Properties{}, errPropertiesFailed
	}
}

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	argsReceived := map[string][]interface{}{}

	workingBase := &inject.Base{}
	expectedFeatures := base.Properties{
		TurningRadiusMeters: 1.2,
		WidthMeters:         float64(100) * 0.001,
	}
	expectedGeometries := []spatialmath.Geometry{spatialmath.NewPoint(r3.Vector{1, 2, 3}, "")}
	setupWorkingBase(workingBase, argsReceived, expectedFeatures, expectedGeometries)

	brokenBase := &inject.Base{}
	setupBrokenBase(brokenBase)

	resMap := map[resource.Name]base.Base{
		base.Named(testBaseName): workingBase,
		base.Named(failBaseName): brokenBase,
	}

	baseSvc, err := resource.NewAPIResourceCollection(base.API, resMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[base.Base](base.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, baseSvc), test.ShouldBeNil)

	workingBase.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})
	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingBaseClient, err := base.NewClientFromConn(context.Background(), conn, "", base.Named(testBaseName), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, workingBaseClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()

	t.Run("working base client", func(t *testing.T) {
		expectedExtra := map[string]interface{}{"foo": "bar"}

		t.Run("working MoveStraight", func(t *testing.T) {
			distance := 42
			mmPerSec := 42.0
			err = workingBaseClient.MoveStraight(
				context.Background(),
				distance,
				mmPerSec,
				map[string]interface{}{"foo": "bar"},
			)
			test.That(t, err, test.ShouldBeNil)
			expectedArgs := []interface{}{distance, mmPerSec, expectedExtra}
			test.That(t, argsReceived["MoveStraight"], test.ShouldResemble, expectedArgs)
		})

		t.Run("working DoCommand", func(t *testing.T) {
			resp, err := workingBaseClient.DoCommand(context.Background(), testutils.TestCommand)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
			test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])
		})

		t.Run("working Spin", func(t *testing.T) {
			angleDeg := 90.0
			degsPerSec := 30.0
			err = workingBaseClient.Spin(
				context.Background(),
				angleDeg,
				degsPerSec,
				map[string]interface{}{"foo": "bar"})
			test.That(t, err, test.ShouldBeNil)
			expectedArgs := []interface{}{angleDeg, degsPerSec, expectedExtra}
			test.That(t, argsReceived["Spin"], test.ShouldResemble, expectedArgs)
		})

		t.Run("working Properties", func(t *testing.T) {
			features, err := workingBaseClient.Properties(context.Background(), expectedExtra)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, features, test.ShouldResemble, expectedFeatures)
		})

		t.Run("working Stop", func(t *testing.T) {
			err = workingBaseClient.Stop(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
		})

		t.Run("working Geometries", func(t *testing.T) {
			geometries, err := workingBaseClient.Geometries(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			for i, geometry := range geometries {
				test.That(t, geometry.AlmostEqual(expectedGeometries[i]), test.ShouldBeTrue)
			}
		})
	})

	t.Run("working base client by dialing", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", base.Named(testBaseName), logger)
		test.That(t, err, test.ShouldBeNil)

		degsPerSec := 42.0
		angleDeg := 30.0

		err = client.Spin(context.Background(), angleDeg, degsPerSec, nil)
		test.That(t, err, test.ShouldBeNil)
		expectedArgs := []interface{}{angleDeg, degsPerSec, map[string]interface{}{}}
		test.That(t, argsReceived["Spin"], test.ShouldResemble, expectedArgs)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("failing base client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingBaseClient, err := base.NewClientFromConn(context.Background(), conn, "", base.Named(failBaseName), logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)

		err = failingBaseClient.MoveStraight(context.Background(), 42, 42.0, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveStraight.Error())

		err = failingBaseClient.Spin(context.Background(), 42.0, 42.0, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errSpinFailed.Error())

		_, err = failingBaseClient.Properties(context.Background(), nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPropertiesFailed.Error())

		err = failingBaseClient.Stop(context.Background(), nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopFailed.Error())

		test.That(t, failingBaseClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
