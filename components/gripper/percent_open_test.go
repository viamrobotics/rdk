package gripper_test

import (
	"context"
	"errors"
	"math"
	"net"
	"testing"

	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/gripper"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

// percentOpenRecorder records which of SetPercentOpen, Open and Grab an inject gripper received.
type percentOpenRecorder struct {
	calls    []string
	position float64
	extra    map[string]interface{}
}

func newPercentOpenGripper(
	rec *percentOpenRecorder,
	props func() (gripper.Properties, error),
	setErr error,
) *inject.Gripper {
	g := &inject.Gripper{}
	g.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (gripper.Properties, error) {
		return props()
	}
	g.SetPercentOpenFunc = func(ctx context.Context, position float64, extra map[string]interface{}) error {
		rec.calls = append(rec.calls, "SetPercentOpen")
		rec.position = position
		rec.extra = extra
		return setErr
	}
	g.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		rec.calls = append(rec.calls, "Open")
		rec.extra = extra
		return nil
	}
	g.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		rec.calls = append(rec.calls, "Grab")
		rec.extra = extra
		return true, nil
	}
	return g
}

func newPercentOpenServer(t *testing.T, g gripper.Gripper) pb.GripperServiceServer {
	t.Helper()
	coll, err := resource.NewAPIResourceCollection(gripper.API, map[resource.Name]gripper.Gripper{
		gripper.Named(testGripperName): g,
	})
	test.That(t, err, test.ShouldBeNil)
	return gripper.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.GripperServiceServer)
}

func supported() (gripper.Properties, error) {
	return gripper.Properties{PercentOpenSupported: true}, nil
}

func notSupported() (gripper.Properties, error) {
	return gripper.Properties{PercentOpenSupported: false}, nil
}

func propsUnimplemented() (gripper.Properties, error) {
	return gripper.Properties{}, rdkgrpc.UnimplementedError
}

func TestServerSetPercentOpen(t *testing.T) {
	ctx := context.Background()
	extra := map[string]interface{}{"foo": "bar"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	t.Run("supported gripper gets SetPercentOpen", func(t *testing.T) {
		rec := &percentOpenRecorder{}
		s := newPercentOpenServer(t, newPercentOpenGripper(rec, supported, nil))
		_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: 0.37, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rec.calls, test.ShouldResemble, []string{"SetPercentOpen"})
		test.That(t, rec.position, test.ShouldEqual, 0.37)
		test.That(t, rec.extra, test.ShouldResemble, extra)
	})

	t.Run("errors from a supported gripper are returned", func(t *testing.T) {
		errJammed := errors.New("jammed")
		rec := &percentOpenRecorder{}
		s := newPercentOpenServer(t, newPercentOpenGripper(rec, supported, errJammed))
		_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: 0.8})
		test.That(t, err, test.ShouldBeError, errJammed)
		test.That(t, rec.calls, test.ShouldResemble, []string{"SetPercentOpen"})
	})

	fallbacks := []struct {
		name   string
		props  func() (gripper.Properties, error)
		setErr error
		first  []string
	}{
		{"properties reports unsupported", notSupported, nil, nil},
		{"properties unimplemented", propsUnimplemented, nil, nil},
		{"SetPercentOpen unimplemented", supported, rdkgrpc.UnimplementedError, []string{"SetPercentOpen"}},
	}
	for _, fb := range fallbacks {
		t.Run("fallback when "+fb.name, func(t *testing.T) {
			for _, tc := range []struct {
				position float64
				want     string
			}{
				{0, "Grab"},
				{0.37, "Grab"},
				{0.4999, "Grab"},
				{0.5, "Open"},
				{0.75, "Open"},
				{1, "Open"},
			} {
				rec := &percentOpenRecorder{}
				s := newPercentOpenServer(t, newPercentOpenGripper(rec, fb.props, fb.setErr))
				_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: tc.position, Extra: ext})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, rec.calls, test.ShouldResemble, append(append([]string{}, fb.first...), tc.want))
				test.That(t, rec.extra, test.ShouldResemble, extra)
			}
		})
	}

	t.Run("fallback returns Open and Grab errors", func(t *testing.T) {
		g := newPercentOpenGripper(&percentOpenRecorder{}, notSupported, nil)
		g.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error { return errCantOpen }
		g.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) { return false, errCantGrab }
		s := newPercentOpenServer(t, g)
		_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: 1})
		test.That(t, err, test.ShouldBeError, errCantOpen)
		_, err = s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: 0})
		test.That(t, err, test.ShouldBeError, errCantGrab)
	})

	t.Run("other Properties errors are returned without moving", func(t *testing.T) {
		errBroken := errors.New("broken")
		rec := &percentOpenRecorder{}
		s := newPercentOpenServer(t, newPercentOpenGripper(rec, func() (gripper.Properties, error) {
			return gripper.Properties{}, errBroken
		}, nil))
		_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: 0.2})
		test.That(t, err, test.ShouldBeError, errBroken)
		test.That(t, rec.calls, test.ShouldBeEmpty)
	})

	t.Run("invalid positions are InvalidArgument and never reach the gripper", func(t *testing.T) {
		for _, p := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
			rec := &percentOpenRecorder{}
			s := newPercentOpenServer(t, newPercentOpenGripper(rec, supported, nil))
			_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: testGripperName, PercentOpen: p})
			test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
			test.That(t, rec.calls, test.ShouldBeEmpty)
		}
	})

	t.Run("missing gripper", func(t *testing.T) {
		s := newPercentOpenServer(t, newPercentOpenGripper(&percentOpenRecorder{}, supported, nil))
		_, err := s.SetPercentOpen(ctx, &pb.SetPercentOpenRequest{Name: missingGripperName, PercentOpen: 0.5})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())
	})
}

func TestServerGetPercentOpenAndProperties(t *testing.T) {
	ctx := context.Background()
	extra := map[string]interface{}{"foo": "bar"}
	ext, err := protoutils.StructToStructPb(extra)
	test.That(t, err, test.ShouldBeNil)

	var gotExtra map[string]interface{}
	g := &inject.Gripper{}
	g.PercentOpenFunc = func(ctx context.Context, e map[string]interface{}) (float64, error) {
		gotExtra = e
		return 0.25, nil
	}
	g.PropertiesFunc = func(ctx context.Context, e map[string]interface{}) (gripper.Properties, error) {
		gotExtra = e
		return gripper.Properties{PercentOpenSupported: true}, nil
	}
	s := newPercentOpenServer(t, g)

	resp, err := s.GetPercentOpen(ctx, &pb.GetPercentOpenRequest{Name: testGripperName, Extra: ext})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetPercentOpen(), test.ShouldEqual, 0.25)
	test.That(t, gotExtra, test.ShouldResemble, extra)

	props, err := s.GetProperties(ctx, &pb.GetPropertiesRequest{Name: testGripperName, Extra: ext})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.GetPercentOpenSupported(), test.ShouldBeTrue)
	test.That(t, gotExtra, test.ShouldResemble, extra)

	// GetPercentOpen has no fallback: an unsupported gripper stays Unimplemented.
	g.PercentOpenFunc = func(ctx context.Context, e map[string]interface{}) (float64, error) {
		return 0, rdkgrpc.UnimplementedError
	}
	_, err = s.GetPercentOpen(ctx, &pb.GetPercentOpenRequest{Name: testGripperName})
	test.That(t, status.Code(err), test.ShouldEqual, codes.Unimplemented)

	_, err = s.GetPercentOpen(ctx, &pb.GetPercentOpenRequest{Name: missingGripperName})
	test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())
	_, err = s.GetProperties(ctx, &pb.GetPropertiesRequest{Name: missingGripperName})
	test.That(t, err.Error(), test.ShouldContainSubstring, errGripperNotFound.Error())
}

func TestClientPercentOpen(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	supportedRec := &percentOpenRecorder{}
	supportedGripper := newPercentOpenGripper(supportedRec, supported, nil)
	supportedGripper.PercentOpenFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0.6, nil
	}
	// legacyGripper stands in for a gripper built before SetPercentOpen existed.
	legacyRec := &percentOpenRecorder{}
	legacyGripper := newPercentOpenGripper(legacyRec, propsUnimplemented, rdkgrpc.UnimplementedError)
	legacyGripper.PercentOpenFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, rdkgrpc.UnimplementedError
	}

	coll, err := resource.NewAPIResourceCollection(gripper.API, map[resource.Name]gripper.Gripper{
		gripper.Named(testGripperName):  supportedGripper,
		gripper.Named(testGripperName2): legacyGripper,
	})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[gripper.Gripper](gripper.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, coll, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	defer rpcServer.Stop()

	conn, err := rdkgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	ctx := context.Background()
	extra := map[string]interface{}{"foo": "bar"}

	t.Run("supported gripper", func(t *testing.T) {
		c, err := gripper.NewClientFromConn(ctx, conn, "", gripper.Named(testGripperName), logger)
		test.That(t, err, test.ShouldBeNil)

		props, err := c.Properties(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, props, test.ShouldResemble, gripper.Properties{PercentOpenSupported: true})

		test.That(t, c.SetPercentOpen(ctx, 0.3, extra), test.ShouldBeNil)
		test.That(t, supportedRec.calls, test.ShouldResemble, []string{"SetPercentOpen"})
		test.That(t, supportedRec.position, test.ShouldEqual, 0.3)
		test.That(t, supportedRec.extra, test.ShouldResemble, extra)

		pos, err := c.PercentOpen(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 0.6)

		err = c.SetPercentOpen(ctx, 1.5, nil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("legacy gripper falls back to Open and Grab", func(t *testing.T) {
		c, err := gripper.NewClientFromConn(ctx, conn, "", gripper.Named(testGripperName2), logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = c.Properties(ctx, nil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unimplemented)

		test.That(t, c.SetPercentOpen(ctx, 0.37, nil), test.ShouldBeNil)
		test.That(t, c.SetPercentOpen(ctx, 0.5, nil), test.ShouldBeNil)
		test.That(t, legacyRec.calls, test.ShouldResemble, []string{"Grab", "Open"})

		_, err = c.PercentOpen(ctx, nil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unimplemented)
	})
}
