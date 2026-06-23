package robot_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/testutils/robottestutils"
)

func TestSessionsMixedClients(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, nil, logger.Sublogger("main"))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// This test sets up two clients to the same motor. Both clients connect to the robot through a WebRTC
	// gRPC connection, which adds an auth entity to the connection implicitly.
	// The test sets up this scenario:
	// 1) Client 1 will first `SetPower` with a session heartbeat. This operation returns with the
	//    motor engaged.
	// 2) Client 2 will override that operation with a very slow `GoFor`, becoming the last caller to the motor in
	//    the process.
	// 3) Client 1 will disconnect. This results in the client ceasing heartbeats. However, as Client 2 is the last caller
	//    and sending heartbeats, the robot's heartbeat thread will not call `motor1.Stop`.
	// 4) Client 2 will disconnect. This results in the client ceasing heartbeats and the robot's heartbeat thread
	//    will call `motor1.Stop`.

	roboClient1, err := client.New(ctx, addr, logger.Sublogger("client1"))
	test.That(t, err, test.ShouldBeNil)
	roboClient2, err := client.New(ctx, addr, logger.Sublogger("client2"))
	test.That(t, err, test.ShouldBeNil)

	motor1Client1, err := motor.FromProvider(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)
	motor1Client2, err := motor.FromProvider(roboClient2, "motor1")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)
	// now client 2 is the last caller
	test.That(t, motor1Client2.GoFor(ctx, 1, 2, nil), test.ShouldBeNil)

	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)

	timer := time.NewTimer(config.DefaultSessionHeartbeatWindow * 2)
	select {
	case <-stopChMotor1:
		panic("unexpected")
	case <-timer.C:
		timer.Stop()
	}

	test.That(t, roboClient2.Close(ctx), test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		select {
		case <-stopChMotor1:
			return
		default:
			tb.Fail()
		}
	})

	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

func TestSessionsMixedOwnersNoAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, nil, logger.Sublogger("main"))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// with no auth turned on, we will have no session owner, meaning mixing sessions technically works, for now
	roboClient1, err := client.New(ctx, addr, logger, client.WithDialOptions(rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	})))
	test.That(t, err, test.ShouldBeNil)

	roboClientConn2, err := grpc.Dial(ctx, addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	}))
	test.That(t, err, test.ShouldBeNil)

	// This test sets up two clients to the same motor. Both clients connect to the robot through a direct
	// gRPC connection, which does not add an auth entity to the connection implicitly.
	// The test sets up this (contrived) scenario:
	// 1) Client 1 will first `SetPower` with a session heartbeat. This operation returns with the
	//    motor engaged.
	// 2) Client 2 will attempt to "override" that operation with a very slow `GoFor` using the same session id
	//    as Client 1. The operation will succeed as the session is not tied to any auth entities.
	// 3) Client 2 will attempt to resume a session with the main robot using the same session id as Client 1.
	//    The robot will accept the session id from Client 2.
	// 4) Client 1 will disconnect. This results in the client ceasing heartbeats for the
	//    `SetPower` operation. The robot's heartbeat thread will call `motor1.Stop`. While Client 2 sent a command
	//    to the motor, it used Client 1's session and never sent heartbeats.
	motor1Client1, err := motor.FromProvider(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)

	// clients made directly with a connection will not send heartbeats.
	motor1Client2, err := motor.NewClientFromConn(ctx, roboClientConn2, "", motor.Named("motor1"), logger.Sublogger("motor1client2"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)

	sessions := r.SessionManager().All()
	test.That(t, sessions, test.ShouldHaveLength, 1)
	sessID := sessions[0].ID().String()

	// now client 2 is the last caller but the sessions are the same
	client2Ctx := metadata.AppendToOutgoingContext(ctx, session.IDMetadataKey, sessID)
	test.That(t, motor1Client2.GoFor(client2Ctx, 1, 2, nil), test.ShouldBeNil)

	// this would just heartbeat client 1's session
	resp, err := robotpb.NewRobotServiceClient(roboClientConn2).StartSession(ctx, &robotpb.StartSessionRequest{
		Resume: sessID,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Id, test.ShouldEqual, sessID)

	// this is the only one heartbeating so we expect a stop
	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		select {
		case <-stopChMotor1:
			return
		default:
			tb.Fail()
		}
	})

	test.That(t, roboClientConn2.Close(), test.ShouldBeNil)
	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

// TODO(RSDK-890): add explicit auth test once entities are actually unique.
func TestSessionsMixedOwnersImplicitAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, nil, logger.Sublogger("main"))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-890): using WebRTC (the default) gives us an implicit auth entity, for now
	roboClient1, err := client.New(ctx, addr, logger.Sublogger("client"))
	test.That(t, err, test.ShouldBeNil)

	// there is no auth entity with a direct connection
	roboClientConn2, err := grpc.Dial(ctx, addr, logger.Sublogger("motor1client2"), rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	}))
	test.That(t, err, test.ShouldBeNil)

	// This test sets up two clients to the same motor. Client 1 connects to the robot through WebRTC, which
	// implicitly adds an auth entity to the connection. Client 2 connects through a direct gRPC connection,
	// which does not.
	// The test sets up this (contrived) scenario:
	// 1) Client 1 will first `SetPower` with a session heartbeat. This operation returns with the
	//    motor engaged.
	// 2) Client 2 will attempt to "override" that operation with a very slow `GoFor` using the same session id
	//    as Client 1. The operation will fail as sessions cannot be shared between different auth entities.
	// 3) Client 2 will attempt to resume a session with the main robot using the same session id as Client 1.
	//    The robot will reject the session id and create a new session for Client 2.
	// 4) Client 1 will disconnect. This results in the client ceasing heartbeats for the
	//    `SetPower` operation. As Client 2 never sent a successful command to the motor (even if it did, Client 2
	//    won't sent heartbeats), the robot's heartbeat thread will call `motor1.Stop`.
	motor1Client1, err := motor.FromProvider(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)

	// clients made directly with a connection will not send heartbeats.
	motor1Client2, err := motor.NewClientFromConn(ctx, roboClientConn2, "", motor.Named("motor1"), logger.Sublogger("motor1client2"))
	test.That(t, err, test.ShouldBeNil)

	// Set the power on client1.
	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)

	sessions := r.SessionManager().All()
	test.That(t, sessions, test.ShouldHaveLength, 1)
	sessID := sessions[0].ID().String()

	// cannot share sessions across different auth entities.
	client2Ctx := metadata.AppendToOutgoingContext(ctx, session.IDMetadataKey, sessID)
	err = motor1Client2.GoFor(client2Ctx, 1, 2, nil)
	test.That(t, err, test.ShouldNotBeNil)
	statusErr, ok := status.FromError(err)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, statusErr.Code(), test.ShouldEqual, session.StatusNoSession.Code())
	test.That(t, statusErr.Message(), test.ShouldEqual, session.StatusNoSession.Message())

	// this should give us a new session since sessions cannot be shared across different auth entities.
	resp, err := robotpb.NewRobotServiceClient(roboClientConn2).StartSession(ctx, &robotpb.StartSessionRequest{
		Resume: sessID,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Id, test.ShouldNotEqual, sessID)
	test.That(t, resp.Id, test.ShouldNotEqual, "")

	// Assert that closing `roboClient1` results in heartbeats stopping that propagates to `Stop`ing
	// the motor operation. We observe this with a message over the `stopChMotor1` channel.
	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		select {
		case <-stopChMotor1:
			return
		default:
			tb.Fail()
		}
	})

	test.That(t, roboClientConn2.Close(), test.ShouldBeNil)
	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

type dummyMotor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	mu     sync.Mutex
	stopCh chan struct{}
}

func (dm *dummyMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 2, nil
}

func (dm *dummyMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	close(dm.stopCh)
	return nil
}

func (dm *dummyMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{}, nil
}

func (dm *dummyMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return false, 0, nil
}

func (dm *dummyMotor) IsMoving(context.Context) (bool, error) {
	return false, nil
}
