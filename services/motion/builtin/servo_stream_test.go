package builtin

import (
	"context"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	fakearm "go.viam.com/rdk/components/arm/fake"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// streamRun is a StreamServo call running in a goroutine, with its responses collected.
type streamRun struct {
	t       *testing.T
	updates chan motion.ServoUpdate
	cancel  context.CancelFunc
	done    chan error

	mu  sync.Mutex
	got []motion.ServoResponse
}

func runStream(t *testing.T, ms *builtIn, cfg motion.ServoConfig) *streamRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &streamRun{t: t, updates: make(chan motion.ServoUpdate, 64), cancel: cancel, done: make(chan error, 1)}
	responses := make(chan motion.ServoResponse, 64)
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		for resp := range responses {
			r.mu.Lock()
			r.got = append(r.got, resp)
			r.mu.Unlock()
		}
	}()
	go func() {
		err := ms.StreamServo(ctx, "arm", cfg, r.updates, responses, map[string]interface{}{"ramp_ms": 100.0})
		close(responses) // the framework closes it after the call returns
		<-collected
		r.done <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-r.done:
			r.done <- err
		case <-time.After(10 * time.Second):
			t.Error("StreamServo did not return after cancel")
		}
	})
	return r
}

func (r *streamRun) responses() []motion.ServoResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]motion.ServoResponse(nil), r.got...)
}

// statuses are the Status responses so far.
func (r *streamRun) statuses() []motion.ServoStatus {
	var out []motion.ServoStatus
	for _, g := range r.responses() {
		if g.Status != nil {
			out = append(out, *g.Status)
		}
	}
	return out
}

// waitStatus waits for a Status matching pred.
func (r *streamRun) waitStatus(pred func(motion.ServoStatus) bool) motion.ServoStatus {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range r.statuses() {
			if pred(s) {
				return s
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.t.Fatalf("no matching Status; got %+v", r.statuses())
	return motion.ServoStatus{}
}

// waitStarted waits for the Started response.
func (r *streamRun) waitStarted() {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for len(r.responses()) == 0 {
		if time.Now().After(deadline) {
			r.t.Fatal("no Started response")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// finish waits for the call to return and gives its final Status and error.
func (r *streamRun) finish() (motion.ServoStatus, error) {
	r.t.Helper()
	var err error
	select {
	case err = <-r.done:
		r.done <- err
	case <-time.After(15 * time.Second):
		r.t.Fatal("StreamServo did not return")
	}
	ss := r.statuses()
	test.That(r.t, len(ss), test.ShouldBeGreaterThan, 0)
	return ss[len(ss)-1], err
}

func poseCfg() motion.ServoConfig {
	to := uint32(8000)
	return motion.ServoConfig{Mode: motion.ServoModePose, TimeoutMs: &to}
}

func endPose(t *testing.T, a arm.Arm) spatialmath.Pose {
	t.Helper()
	p, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	return p
}

// worldTarget is the arm's current end pose moved by d, in world (the arm is mounted at world's origin).
func worldTarget(t *testing.T, a arm.Arm, d r3.Vector) motion.ServoUpdate {
	t.Helper()
	p := endPose(t, a)
	return motion.ServoUpdate{Target: referenceframe.NewPoseInFrame("world", spatialmath.NewPose(p.Point().Add(d), p.Orientation()))}
}

func TestStreamServoStartedAndAcks(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	for _, dx := range []float64{0, 5, 10} {
		r.updates <- worldTarget(t, a, r3.Vector{X: dx})
	}
	r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 3 })
	close(r.updates)
	fin, err := r.finish()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStopped)
	test.That(t, fin.Ack, test.ShouldBeNil)

	got := r.responses()
	test.That(t, got[0].Started, test.ShouldNotBeNil)
	test.That(t, len(got[0].Started.Limits.MaxVelRadsJoints), test.ShouldEqual, 6)
	test.That(t, len(got[0].Started.Limits.MaxAccRadsJoints), test.ShouldEqual, 6)
	test.That(t, got[0].Started.RateHz, test.ShouldBeGreaterThan, 0)
	test.That(t, got[0].Started.TimeoutMs, test.ShouldEqual, 8000)
	test.That(t, got[0].Started.Limits.MaxTCPSpeedMPerSec, test.ShouldBeNil)
	var acks []uint64
	for _, s := range r.statuses() {
		if s.Ack != nil {
			acks = append(acks, *s.Ack)
			test.That(t, s.Rejected, test.ShouldBeNil)
		}
	}
	test.That(t, acks, test.ShouldResemble, []uint64{1, 2, 3})

	// the arm was released
	id := do(t, ms, DoServoStart, map[string]interface{}{"component_name": "arm", "mode": "pose"})["session_id"].(string)
	do(t, ms, DoServoStop, map[string]interface{}{"session_id": id})
}

func TestStreamServoRejectsBigStep(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.updates <- worldTarget(t, a, r3.Vector{X: 500})
	rej := r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 1 })
	test.That(t, rej.Rejected, test.ShouldNotBeNil)
	r.updates <- worldTarget(t, a, r3.Vector{X: 5})
	ok := r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 2 })
	test.That(t, ok.Rejected, test.ShouldBeNil)
	select {
	case err := <-r.done:
		t.Fatalf("stream ended after a rejection: %v", err)
	default:
	}
}

func TestStreamServoWrongModeEndsStream(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.updates <- motion.ServoUpdate{Velocity: &motion.ServoTwist{LinearMMPerSec: r3.Vector{X: 1}}}
	fin, err := r.finish()
	test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStopped)
}

func TestStreamServoMalformedUpdates(t *testing.T) {
	vel := &motion.ServoTwist{LinearMMPerSec: r3.Vector{X: 1}}
	for name, c := range map[string]struct {
		mode motion.ServoMode
		u    motion.ServoUpdate
	}{
		"empty update":               {motion.ServoModePose, motion.ServoUpdate{}},
		"velocity with feed-forward": {motion.ServoModeVelocity, motion.ServoUpdate{Velocity: vel, FeedForward: vel}},
	} {
		t.Run(name, func(t *testing.T) {
			ms, _ := newServoService(t, nil, nil)
			cfg := poseCfg()
			cfg.Mode = c.mode
			r := runStream(t, ms, cfg)
			r.updates <- c.u
			fin, err := r.finish()
			test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
			test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStopped)
		})
	}
}

func TestStreamServoArmStopFaults(t *testing.T) {
	ms, sa := newStoppable(t)
	r := runStream(t, ms, poseCfg())
	r.waitStarted()
	test.That(t, sa.Stop(context.Background(), nil), test.ShouldBeNil)
	fin, err := r.finish()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldNotEqual, context.Canceled)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateFaulted)
	test.That(t, fin.Ack, test.ShouldBeNil)
}

func TestStreamServoVelocityEmptyFrameIsWorld(t *testing.T) {
	mount := spatialmath.NewPose(r3.Vector{}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90})
	ms, a := newServoServiceMounted(t, &fakearm.Config{ArmModel: "ur5e"}, servoHome, nil, nil, mount)
	to := uint32(8000)
	r := runStream(t, ms, motion.ServoConfig{Mode: motion.ServoModeVelocity, TimeoutMs: &to})
	world := func() r3.Vector { return spatialmath.Compose(mount, endPose(t, a)).Point() }
	before := world()
	for i := 0; i < 20; i++ {
		r.updates <- motion.ServoUpdate{Velocity: &motion.ServoTwist{LinearMMPerSec: r3.Vector{X: 10}}}
		time.Sleep(50 * time.Millisecond)
	}
	r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 20 })
	for _, s := range r.statuses() {
		test.That(t, s.Rejected, test.ShouldBeNil)
	}
	d := world().Sub(before)
	test.That(t, d.X, test.ShouldBeGreaterThan, 3)
	test.That(t, math.Abs(d.Y), test.ShouldBeLessThan, 1)
}

func TestStreamServoDeadMan(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	to := uint32(200)
	r := runStream(t, ms, motion.ServoConfig{Mode: motion.ServoModePose, TimeoutMs: &to})
	r.updates <- worldTarget(t, a, r3.Vector{X: 1})
	fin, err := r.finish() // the updates channel stays open
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStoppedTimeout)
}

func TestStreamServoCancel(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.updates <- worldTarget(t, a, r3.Vector{X: 5})
	r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil })
	r.cancel()
	_, err := r.finish()
	test.That(t, err, test.ShouldEqual, context.Canceled)
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"component_name": "arm"})["state"], test.ShouldEqual, "stopped")
}

func TestStreamServoStopPlanEndsSession(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.waitStarted()
	test.That(t, ms.StopPlan(context.Background(), motion.StopPlanReq{ComponentName: "arm"}), test.ShouldBeNil)
	fin, err := r.finish()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStopped)
}

func TestStreamServoBusyArm(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	startServo(t, ms, nil)
	responses := make(chan motion.ServoResponse, 4)
	err := ms.StreamServo(context.Background(), "arm", poseCfg(), make(chan motion.ServoUpdate), responses, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.FailedPrecondition)
	test.That(t, len(responses), test.ShouldEqual, 0)
	err = ms.StreamServo(context.Background(), "nope", poseCfg(), make(chan motion.ServoUpdate), responses, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.NotFound)
}

func TestStreamServoDoCommandRefusesStreamedSession(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.waitStarted()
	err := doErr(t, ms, DoServoTarget, map[string]interface{}{"component_name": "arm", "target": map[string]interface{}{}})
	test.That(t, err.Error(), test.ShouldContainSubstring, "owned by a StreamServo call")
	err = doErr(t, ms, DoServoVelocity, map[string]interface{}{"component_name": "arm"})
	test.That(t, err.Error(), test.ShouldContainSubstring, "owned by a StreamServo call")
	test.That(t, do(t, ms, DoServoStatus, map[string]interface{}{"component_name": "arm"})["state"], test.ShouldNotBeNil)
}

func TestStreamServoStateChangePushed(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	r := runStream(t, ms, poseCfg())
	r.updates <- worldTarget(t, a, r3.Vector{X: 20})
	r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack == nil && s.State == motion.ServoStateConverged })
}

func TestStreamServoFeedForward(t *testing.T) {
	endX := func(ff bool) float64 {
		ms, a := newServoService(t, nil, nil)
		r := runStream(t, ms, poseCfg())
		start := endPose(t, a)
		for i := 1; i <= 3; i++ {
			u := motion.ServoUpdate{Target: referenceframe.NewPoseInFrame("world",
				spatialmath.NewPose(start.Point().Add(r3.Vector{X: float64(i)}), start.Orientation()))}
			if ff {
				u.FeedForward = &motion.ServoTwist{LinearMMPerSec: r3.Vector{X: 20}}
			}
			r.updates <- u
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(150 * time.Millisecond)
		return endPose(t, a).Point().X
	}
	plain, ff := endX(false), endX(true)
	test.That(t, ff-plain, test.ShouldBeGreaterThan, 0.5)
}

func TestStreamServoLimitedBy(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	cfg := poseCfg()
	cfg.Limits = &arm.MoveOptions{MaxTCPSpeedMPerSec: ptr(0.02)}
	r := runStream(t, ms, cfg)
	has := func(s motion.ServoStatus) bool {
		for _, l := range s.LimitedBy {
			if l == motion.ServoLimitTCPSpeed {
				return true
			}
		}
		return false
	}
	r.waitStarted()
	tcp := r.responses()[0].Started.Limits.MaxTCPSpeedMPerSec
	test.That(t, tcp, test.ShouldNotBeNil)
	test.That(t, *tcp, test.ShouldEqual, 0.02)
	u := worldTarget(t, a, r3.Vector{X: 90})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.updates <- u // each acknowledgement reads the loop's latest limiters
		time.Sleep(50 * time.Millisecond)
		for _, s := range r.statuses() {
			if has(s) {
				return
			}
		}
	}
	t.Fatalf("no Status reported the TCP speed limit; got %+v", r.statuses())
}

func TestStreamServoEmptyFrameIsWorld(t *testing.T) {
	mount := spatialmath.NewPoseFromPoint(r3.Vector{Y: 200})
	ms, a := newServoServiceMounted(t, &fakearm.Config{ArmModel: "ur5e"}, servoHome, nil, nil, mount)
	r := runStream(t, ms, poseCfg())
	p := endPose(t, a)
	before := p.Point()
	same := spatialmath.NewPose(before.Add(r3.Vector{Y: 200}), p.Orientation())

	r.updates <- motion.ServoUpdate{Target: referenceframe.NewPoseInFrame("", same)}
	s := r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 1 })
	test.That(t, s.Rejected, test.ShouldBeNil)
	time.Sleep(300 * time.Millisecond)
	test.That(t, endPose(t, a).Point().Sub(before).Norm(), test.ShouldBeLessThan, 1)

	r.updates <- motion.ServoUpdate{Target: referenceframe.NewPoseInFrame("arm_origin", same)}
	s = r.waitStatus(func(s motion.ServoStatus) bool { return s.Ack != nil && *s.Ack == 2 })
	test.That(t, s.Rejected, test.ShouldNotBeNil)
	test.That(t, *s.Rejected, test.ShouldContainSubstring, "from the last applied target")
}

func TestStreamServoRejectsBadLimits(t *testing.T) {
	ms, _ := newServoService(t, nil, nil)
	cfg := poseCfg()
	cfg.Limits = &arm.MoveOptions{MaxVelRads: -1}
	responses := make(chan motion.ServoResponse, 4)
	err := ms.StreamServo(context.Background(), "arm", cfg, make(chan motion.ServoUpdate), responses, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
	test.That(t, err.Error(), test.ShouldContainSubstring, "limits.max_vel_degs_per_sec must be finite and >= 0")
	test.That(t, len(responses), test.ShouldEqual, 0)
	_, held := ms.reg.get("arm")
	test.That(t, held, test.ShouldBeFalse)

	cfg.Limits = &arm.MoveOptions{MaxTCPSpeedMPerSec: ptr(math.NaN())}
	err = ms.StreamServo(context.Background(), "arm", cfg, make(chan motion.ServoUpdate), responses, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.InvalidArgument)
}

func TestStreamServoOverGRPC(t *testing.T) {
	ms, a := newServoService(t, nil, nil)
	logger := ms.logger
	//nolint:noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	coll, err := resource.NewAPIResourceCollection(motion.API, map[resource.Name]motion.Service{ms.Name(): ms})
	test.That(t, err, test.ShouldBeNil)
	reg, ok, err := resource.LookupAPIRegistration[motion.Service](motion.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.RegisterRPCService(context.Background(), rpcServer, coll, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
		test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	})
	client, err := motion.NewClientFromConn(context.Background(), conn, "", ms.Name(), logger)
	test.That(t, err, test.ShouldBeNil)

	updates := make(chan motion.ServoUpdate)
	responses := make(chan motion.ServoResponse)
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.StreamServo(context.Background(), "arm", poseCfg(), updates, responses, map[string]interface{}{"ramp_ms": 100.0})
	}()
	var got []motion.ServoResponse
	drained := make(chan struct{})
	go func() {
		for r := range responses {
			got = append(got, r)
		}
		close(drained)
	}()
	updates <- worldTarget(t, a, r3.Vector{X: 3})
	updates <- worldTarget(t, a, r3.Vector{X: 6})
	close(updates)
	test.That(t, <-errCh, test.ShouldBeNil)
	close(responses)
	<-drained

	test.That(t, got[0].Started, test.ShouldNotBeNil)
	var acks []uint64
	for _, r := range got[1:] {
		test.That(t, r.Status, test.ShouldNotBeNil)
		if r.Status.Ack != nil {
			acks = append(acks, *r.Status.Ack)
		}
	}
	test.That(t, acks, test.ShouldResemble, []uint64{1, 2})
	fin := got[len(got)-1].Status
	test.That(t, fin.Ack, test.ShouldBeNil)
	test.That(t, fin.State, test.ShouldEqual, motion.ServoStateStopped)
}
