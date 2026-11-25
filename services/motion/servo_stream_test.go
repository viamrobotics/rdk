package motion_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	motionpb "go.viam.com/api/service/motion/v1"
	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	injectmotion "go.viam.com/rdk/testutils/inject/motion"
)

// driveServo plays the caller side of StreamServo: it feeds updates (closing the channel after them
// only if closeUpdates), collects every response, and returns them with the wrapper's terminal error.
func driveServo(
	ctx context.Context,
	client motion.Service,
	cfg motion.ServoConfig,
	updates []motion.ServoUpdate,
	closeUpdates bool,
) ([]motion.ServoResponse, error) {
	updatesCh := make(chan motion.ServoUpdate)
	responsesCh := make(chan motion.ServoResponse)
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.StreamServo(ctx, testStreamedArmName, cfg, updatesCh, responsesCh, nil)
	}()

	var got []motion.ServoResponse
	drained := make(chan struct{})
	go func() {
		for r := range responsesCh {
			got = append(got, r)
		}
		close(drained)
	}()

	stopFeeding := make(chan struct{})
	fed := make(chan struct{})
	go func() {
		defer close(fed)
		for _, u := range updates {
			select {
			case updatesCh <- u:
			case <-stopFeeding:
				return
			}
		}
		if closeUpdates {
			close(updatesCh)
		}
	}()

	err := <-errCh
	close(stopFeeding)
	<-fed
	close(responsesCh)
	<-drained
	return got, err
}

func servoTarget(x float64) motion.ServoUpdate {
	return motion.ServoUpdate{Target: referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: x}))}
}

func TestClientServoStream(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := motion.ServoConfig{Mode: motion.ServoModePose}
	ackStatus := func(ack uint64) motion.ServoResponse {
		return motion.ServoResponse{Status: &motion.ServoStatus{State: motion.ServoStateRunning, Ack: &ack}}
	}

	t.Run("happy path", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, name string, gotCfg motion.ServoConfig,
			updates <-chan motion.ServoUpdate, responses chan<- motion.ServoResponse, extra map[string]interface{},
		) error {
			if name != testStreamedArmName || gotCfg.Mode != motion.ServoModePose {
				return status.Error(codes.Internal, "wrong name or mode")
			}
			responses <- motion.ServoResponse{Started: &motion.ServoStarted{RateHz: 50}}
			var n uint64
			for range updates {
				n++
				responses <- ackStatus(n)
			}
			return nil
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		got, err := driveServo(context.Background(), client, cfg,
			[]motion.ServoUpdate{servoTarget(1), servoTarget(2), servoTarget(3)}, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(got), test.ShouldEqual, 4)
		test.That(t, got[0].Started, test.ShouldNotBeNil)
		test.That(t, got[0].Started.RateHz, test.ShouldEqual, 50)
		for i := 1; i <= 3; i++ {
			test.That(t, got[i].Status, test.ShouldNotBeNil)
			test.That(t, *got[i].Status.Ack, test.ShouldEqual, i)
		}
	})

	// Raw-protocol faults use the generated client to send sequences the typed wrapper never produces.
	rawStream := func(t *testing.T, injectMS *injectmotion.MotionService) motionpb.MotionService_StreamServoClient {
		t.Helper()
		raw := motionpb.NewMotionServiceClient(setupStreamedServer(t, logger, injectMS))
		stream, err := raw.StreamServo(context.Background())
		test.That(t, err, test.ShouldBeNil)
		return stream
	}
	initReq := func(init *motionpb.StreamServoRequest_Init) *motionpb.StreamServoRequest {
		return &motionpb.StreamServoRequest{
			Name:    testMotionServiceName.Name,
			Message: &motionpb.StreamServoRequest_Init_{Init: init},
		}
	}

	t.Run("invalid Init is rejected without calling the impl", func(t *testing.T) {
		updateReq := &motionpb.StreamServoRequest{
			Name: testMotionServiceName.Name,
			Message: &motionpb.StreamServoRequest_Update_{
				Update: &motionpb.StreamServoRequest_Update{},
			},
		}
		for name, req := range map[string]*motionpb.StreamServoRequest{
			"first message not Init": updateReq,
			"empty component name": initReq(&motionpb.StreamServoRequest_Init{
				Mode: motionpb.ServoMode_SERVO_MODE_POSE,
			}),
			"unspecified mode": initReq(&motionpb.StreamServoRequest_Init{ComponentName: testStreamedArmName}),
		} {
			t.Run(name, func(t *testing.T) {
				called := make(chan struct{}, 1)
				injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
				injectMS.StreamServoFunc = func(
					context.Context, string, motion.ServoConfig,
					<-chan motion.ServoUpdate, chan<- motion.ServoResponse, map[string]interface{},
				) error {
					called <- struct{}{}
					return nil
				}
				stream := rawStream(t, injectMS)
				test.That(t, stream.Send(req), test.ShouldBeNil)
				test.That(t, stream.CloseSend(), test.ShouldBeNil)
				_, err := stream.Recv()
				test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
				test.That(t, len(called), test.ShouldEqual, 0)
			})
		}
	})

	t.Run("Update with no command is InvalidArgument and cancels the impl", func(t *testing.T) {
		implCtxDone := make(chan struct{})
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			_ <-chan motion.ServoUpdate, _ chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			<-ctx.Done()
			close(implCtxDone)
			return ctx.Err()
		}
		stream := rawStream(t, injectMS)
		test.That(t, stream.Send(initReq(&motionpb.StreamServoRequest_Init{
			ComponentName: testStreamedArmName, Mode: motionpb.ServoMode_SERVO_MODE_POSE,
		})), test.ShouldBeNil)
		test.That(t, stream.Send(&motionpb.StreamServoRequest{
			Message: &motionpb.StreamServoRequest_Update_{Update: &motionpb.StreamServoRequest_Update{}},
		}), test.ShouldBeNil)
		_, err := stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
		select {
		case <-implCtxDone:
		case <-time.After(5 * time.Second):
			t.Fatal("impl context was not canceled")
		}
	})

	t.Run("second Init is InvalidArgument", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			_ <-chan motion.ServoUpdate, _ chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			<-ctx.Done()
			return ctx.Err()
		}
		stream := rawStream(t, injectMS)
		init := initReq(&motionpb.StreamServoRequest_Init{
			ComponentName: testStreamedArmName, Mode: motionpb.ServoMode_SERVO_MODE_POSE,
		})
		test.That(t, stream.Send(init), test.ShouldBeNil)
		test.That(t, stream.Send(init), test.ShouldBeNil)
		_, err := stream.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	})

	t.Run("empty response is an Internal error", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			_ <-chan motion.ServoUpdate, responses chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			select {
			case responses <- motion.ServoResponse{}:
			case <-ctx.Done():
			}
			<-ctx.Done()
			return ctx.Err()
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)
		_, err = driveServo(context.Background(), client, cfg, nil, false)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Internal)
	})

	t.Run("impl error after Started reaches the client", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			_ <-chan motion.ServoUpdate, responses chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			responses <- motion.ServoResponse{Started: &motion.ServoStarted{}}
			return status.Error(codes.Internal, "arm exploded")
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		got, err := driveServo(context.Background(), client, cfg, nil, false)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "arm exploded")
		test.That(t, len(got), test.ShouldEqual, 1)
		test.That(t, got[0].Started, test.ShouldNotBeNil)
	})

	t.Run("client cancel cancels the impl", func(t *testing.T) {
		started := make(chan struct{})
		implCtxDone := make(chan struct{})
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			_ <-chan motion.ServoUpdate, responses chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			responses <- motion.ServoResponse{Started: &motion.ServoStarted{}}
			close(started)
			<-ctx.Done()
			close(implCtxDone)
			return ctx.Err()
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-started
			cancel()
		}()
		_, err = driveServo(ctx, client, cfg, nil, false)
		test.That(t, err, test.ShouldBeError, context.Canceled)
		select {
		case <-implCtxDone:
		case <-time.After(5 * time.Second):
			t.Fatal("impl context was not canceled")
		}
	})

	t.Run("session ending on its own with updates open returns nil", func(t *testing.T) {
		injectMS := injectmotion.NewMotionService(testMotionServiceName.Name)
		injectMS.StreamServoFunc = func(
			ctx context.Context, _ string, _ motion.ServoConfig,
			updates <-chan motion.ServoUpdate, responses chan<- motion.ServoResponse, _ map[string]interface{},
		) error {
			responses <- motion.ServoResponse{Started: &motion.ServoStarted{}}
			<-updates
			responses <- ackStatus(1)
			return nil // e.g. dead-man; updates is still open
		}
		conn := setupStreamedServer(t, logger, injectMS)
		client, err := motion.NewClientFromConn(context.Background(), conn, "", testMotionServiceName, logger)
		test.That(t, err, test.ShouldBeNil)

		got, err := driveServo(context.Background(), client, cfg, []motion.ServoUpdate{servoTarget(1)}, false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(got), test.ShouldEqual, 2)
	})
}
