// Command servoreplay replays a benchmark path through the builtin motion service's servo session,
// either over StreamServo or over the unary servo_* DoCommand verbs, and writes a trace JSON for
// clients/python/servo_trace_metrics.py (diff-ik). Prototype measuring instrument; see
// clients/python/servo_replay.py, whose schedule and analysis inputs it mirrors.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

// loadPath reads a path JSON (t, xyz_mm): times shifted to start at 0, offsets scaled about the start.
func loadPath(file string, scale float64) ([]float64, []r3.Vector, error) {
	b, err := os.ReadFile(file) //nolint:gosec // operator-supplied path
	if err != nil {
		return nil, nil, err
	}
	var d struct {
		T     []float64    `json:"t"`
		XYZMM [][3]float64 `json:"xyz_mm"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, nil, err
	}
	if len(d.T) == 0 || len(d.T) != len(d.XYZMM) {
		return nil, nil, fmt.Errorf("%s: need equal-length, non-empty t and xyz_mm", file)
	}
	ts, xyz := make([]float64, len(d.T)), make([]r3.Vector, len(d.T))
	for i := range d.T {
		ts[i] = d.T[i] - d.T[0]
		xyz[i] = r3.Vector{X: d.XYZMM[i][0] * scale, Y: d.XYZMM[i][1] * scale, Z: d.XYZMM[i][2] * scale}
	}
	return ts, xyz, nil
}

// feedForward is the finite-difference velocity (mm/s) from each sample to the next; the last is zero.
func feedForward(ts []float64, xyz []r3.Vector) []r3.Vector {
	ff := make([]r3.Vector, len(ts))
	for i := 0; i+1 < len(ts); i++ {
		ff[i] = xyz[i+1].Sub(xyz[i]).Mul(1 / (ts[i+1] - ts[i]))
	}
	return ff
}

// causalFeedForward estimates each sample's velocity (mm/s) from it and the previous sample only, smoothed by an
// EMA with weight alpha on the newest difference: what a live client can compute. The first is zero.
func causalFeedForward(ts []float64, xyz []r3.Vector, alpha float64) []r3.Vector {
	ff := make([]r3.Vector, len(ts))
	for i := 1; i < len(ts); i++ {
		d := xyz[i].Sub(xyz[i-1]).Mul(1 / (ts[i] - ts[i-1]))
		ff[i] = ff[i-1].Mul(1 - alpha).Add(d.Mul(alpha))
	}
	return ff
}

// networkPath describes the sender connection: direct gRPC, or the nominated ICE pair's candidate types and RTT.
func networkPath(rc *client.RobotClient) string {
	st := rc.WebRTCStats()
	if st == nil {
		return "direct-grpc"
	}
	for _, v := range st {
		p, ok := v.(webrtc.ICECandidatePairStats)
		if !ok || !p.Nominated {
			continue
		}
		l, _ := st[p.LocalCandidateID].(webrtc.ICECandidateStats)
		r, _ := st[p.RemoteCandidateID].(webrtc.ICECandidateStats)
		return fmt.Sprintf("webrtc local=%s remote=%s ice_rtt_ms=%.1f", l.CandidateType, r.CandidateType, p.CurrentRoundTripTime*1000)
	}
	return "webrtc (no nominated pair)"
}

// dueAt is the absolute send time of path time t: an offset from one reference instant, so slips don't accumulate.
func dueAt(ref time.Time, t float64) time.Time {
	return ref.Add(time.Duration(t * float64(time.Second)))
}

// stopLatency finds when motion ended: the first sample at or after kill from which every joint (degrees)
// stays within 0.05 deg (max - min) for the next 200 ms; latency is that sample's time minus kill, in
// seconds. ok is false until the trace extends 200 ms past such a sample.
func stopLatency(t []float64, q [][]float64, kill float64) (latency float64, ok bool) {
	for i := range t {
		if t[i] < kill || t[len(t)-1]-t[i] < 0.2 {
			continue
		}
		lo, hi := append([]float64(nil), q[i]...), append([]float64(nil), q[i]...)
		still := true
		for k := i + 1; k < len(t) && t[k]-t[i] <= 0.2 && still; k++ {
			for j, v := range q[k] {
				lo[j], hi[j] = math.Min(lo[j], v), math.Max(hi[j], v)
				still = still && hi[j]-lo[j] <= 0.05
			}
		}
		if still {
			return t[i] - kill, true
		}
	}
	return 0, false
}

type trace struct {
	Path          string         `json:"path"`
	Transport     string         `json:"transport"`
	FF            bool           `json:"ff"`
	Scale         float64        `json:"scale"`
	StartJoints   []float64      `json:"start_joints_deg"`
	Leader        xyzTrace       `json:"leader"`
	Follower      xyzTrace       `json:"follower"`
	Joints        jointTrace     `json:"joints"`
	Send          sendTrace      `json:"send"`
	Acks          ackTrace       `json:"acks"`
	StatusPolls   []statusPoll   `json:"status_polls"`
	FinalStatus   map[string]any `json:"final_status"`
	Errors        int            `json:"errors"`
	StopLatencyMs *float64       `json:"stop_latency_ms"`
	KillAt        float64        `json:"kill_at"`
	Network       string         `json:"network"` // the sender connection's path, from WebRTC stats
}

type xyzTrace struct {
	T   []float64    `json:"t"`
	XYZ [][3]float64 `json:"xyz"`
}
type jointTrace struct {
	T    []float64   `json:"t"`
	QDeg [][]float64 `json:"q_deg"`
}
type sendTrace struct {
	LateMs []float64 `json:"late_ms"`
	RttMs  []float64 `json:"rtt_ms"`
}
type ackTrace struct {
	T         []float64  `json:"t"`
	Index     []uint64   `json:"index"`
	State     []string   `json:"state"`
	Rejected  []string   `json:"rejected"`
	LimitedBy [][]string `json:"limited_by"`
}
type statusPoll struct {
	T      float64        `json:"t"`
	Status map[string]any `json:"status"`
}

var (
	stateNames = []string{"unspecified", "running", "converged", "stalled", "stopped", "stopped_timeout", "faulted"}
	limitNames = []string{"unspecified", "joint_position", "joint_speed", "tcp_speed", "singularity"}
)

type opts struct {
	addr, arm, service, path, transport, out string
	scale, vel, acc, killAt                  float64
	ff, cloud, relay                         bool
	ffCausal                                 float64 // > 0: causal EMA estimate with this alpha instead of the lookahead difference
	timeoutMs                                uint
	startJoints                              string
}

func main() {
	var o opts
	flag.StringVar(&o.addr, "addr", "localhost:8088", "machine address (insecure, direct gRPC)")
	flag.StringVar(&o.arm, "arm", "arm", "arm name")
	flag.StringVar(&o.service, "service", "builtin", "motion service name")
	flag.StringVar(&o.path, "path", "", "path JSON (t, xyz_mm)")
	flag.Float64Var(&o.scale, "scale", 1, "xyz scale about the path start")
	flag.StringVar(&o.transport, "transport", "stream", "stream or unary")
	flag.BoolVar(&o.ff, "ff", false, "stream only: send the finite-difference feed-forward (reference frame <arm>_origin)")
	flag.Float64Var(&o.vel, "vel", 60, "limits.max_vel_degs_per_sec")
	flag.Float64Var(&o.acc, "acc", 240, "limits.max_acc_degs_per_sec2")
	flag.UintVar(&o.timeoutMs, "timeout-ms", 2000, "dead-man timeout")
	flag.StringVar(&o.startJoints, "start-joints", "", "comma-separated degrees; MoveToJointPositions first")
	flag.Float64Var(&o.killAt, "kill-at", 0, "seconds after the first target: stop sending abruptly and time the arm's stop (0 = off)")
	flag.StringVar(&o.out, "out", "", "trace JSON path")
	flag.Float64Var(&o.ffCausal, "ff-causal", 0,
		"with -ff: causal feed-forward estimate (previous sample, EMA alpha in (0,1]) instead of the lookahead difference")
	flag.BoolVar(&o.cloud, "cloud", false, "dial -addr (a machine FQDN) through app.viam.com with VIAM_API_KEY_ID/VIAM_API_KEY; mDNS off")
	flag.BoolVar(&o.relay, "relay", false, "with -cloud: force WebRTC through a TURN relay")
	flag.Parse()
	if o.path == "" || o.out == "" || (o.transport != "stream" && o.transport != "unary") ||
		(o.ff && o.transport != "stream") || (o.relay && !o.cloud) || (o.ffCausal != 0 && (!o.ff || o.ffCausal < 0 || o.ffCausal > 1)) {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(context.Background(), o); err != nil {
		fmt.Fprintln(os.Stderr, "servoreplay:", err)
		os.Exit(1)
	}
}

func deg(rad []float64) []float64 {
	out := make([]float64, len(rad))
	for i, r := range rad {
		out[i] = math.Round(r*180/math.Pi*1000) / 1000
	}
	return out
}

func run(ctx context.Context, o opts) error {
	logger := logging.NewLogger("servoreplay")
	dial := func() (*client.RobotClient, error) {
		if !o.cloud {
			return client.New(ctx, o.addr, logger, client.WithDialOptions(client.WithInsecure(), client.WithForceDirectGRPC()))
		}
		do := []rpc.DialOption{
			rpc.WithEntityCredentials(os.Getenv("VIAM_API_KEY_ID"),
				rpc.Credentials{Type: rpc.CredentialsTypeAPIKey, Payload: os.Getenv("VIAM_API_KEY")}),
			rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{Disable: true}),
		}
		if o.relay {
			do = append(do, rpc.WithForceRelay())
		}
		return client.New(ctx, o.addr, logger, client.WithDialOptions(do...))
	}
	r1, err := dial()
	if err != nil {
		return err
	}
	defer r1.Close(ctx) //nolint:errcheck
	r2, err := dial()   // polls get their own connection so they don't queue behind the sender
	if err != nil {
		return err
	}
	defer r2.Close(ctx) //nolint:errcheck
	svc, err := motion.FromProvider(r1, o.service)
	if err != nil {
		return err
	}
	svc2, err := motion.FromProvider(r2, o.service)
	if err != nil {
		return err
	}
	arm1, err := arm.FromProvider(r1, o.arm)
	if err != nil {
		return err
	}
	arm2, err := arm.FromProvider(r2, o.arm)
	if err != nil {
		return err
	}

	tr := &trace{Path: o.path, Transport: o.transport, FF: o.ff, Scale: o.scale, StartJoints: []float64{}}
	if o.startJoints != "" {
		var rad []float64
		for _, s := range strings.Split(o.startJoints, ",") {
			d, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return err
			}
			tr.StartJoints = append(tr.StartJoints, d)
			rad = append(rad, d*math.Pi/180)
		}
		if err := arm1.MoveToJointPositions(ctx, rad, nil); err != nil {
			return err
		}
	}
	ts, xyz, err := loadPath(o.path, o.scale)
	if err != nil {
		return err
	}
	ffs := feedForward(ts, xyz)
	if o.ffCausal > 0 {
		ffs = causalFeedForward(ts, xyz, o.ffCausal)
	}
	anchor, err := arm1.EndPosition(ctx, nil)
	if err != nil {
		return err
	}
	ov := anchor.Orientation().OrientationVectorDegrees()
	base := o.arm + "_origin"
	targetAt := func(i int) spatialmath.Pose { // anchor + offset, orientation held
		return spatialmath.NewPose(anchor.Point().Add(xyz[i]), ov)
	}

	var mu sync.Mutex // guards tr's polled and ack slices
	t0 := time.Now()
	since := func() float64 { return time.Since(t0).Seconds() }
	stop := make(chan struct{})
	var wg sync.WaitGroup
	status := func() map[string]any {
		pc, pcancel := context.WithTimeout(ctx, 2*time.Second)
		defer pcancel()
		s, err := svc2.DoCommand(pc, map[string]any{"servo_status": map[string]any{"component_name": o.arm}})
		if err != nil {
			return nil
		}
		return s
	}
	loop := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					f()
				}
			}
		}()
	}
	loop(func() {
		s := since()
		pc, pcancel := context.WithTimeout(ctx, 2*time.Second)
		p, err := arm2.EndPosition(pc, nil)
		pcancel()
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			return
		}
		m := (s + since()) / 2
		pt := p.Point()
		mu.Lock()
		tr.Follower.T, tr.Follower.XYZ = append(tr.Follower.T, m), append(tr.Follower.XYZ, [3]float64{pt.X, pt.Y, pt.Z})
		mu.Unlock()
	})
	loop(func() {
		s := since()
		pc, pcancel := context.WithTimeout(ctx, 2*time.Second)
		j, err := arm2.JointPositions(pc, nil)
		pcancel()
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			return
		}
		m := (s + since()) / 2
		mu.Lock()
		tr.Joints.T, tr.Joints.QDeg = append(tr.Joints.T, m), append(tr.Joints.QDeg, deg(j))
		mu.Unlock()
	})
	loop(func() {
		s := since()
		if st := status(); st != nil {
			mu.Lock()
			tr.StatusPolls = append(tr.StatusPolls, statusPoll{T: s, Status: st})
			mu.Unlock()
		}
		time.Sleep(200 * time.Millisecond)
	})
	time.Sleep(300 * time.Millisecond) // follower samples before the first target

	// send abstracts the transport: it sends target i, returns false when the session is gone.
	var send func(i, seq int) (sent, cont bool)
	var finish func()
	var kill func()
	sendT := make([]time.Time, len(ts)+1) // by 1-based seq; guarded by mu
	sessCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	switch o.transport {
	case "stream":
		updates := make(chan motion.ServoUpdate)
		responses := make(chan motion.ServoResponse, 64)
		done := make(chan struct{})
		var streamErr error
		recvDone := make(chan struct{})
		started := make(chan struct{})
		go func() {
			defer close(recvDone)
			for r := range responses {
				if r.Started != nil {
					close(started)
				}
				if r.Status == nil || r.Status.Ack == nil {
					continue
				}
				now := time.Now()
				a := *r.Status.Ack
				mu.Lock()
				if a >= 1 && int(a) < len(sendT) && !sendT[a].IsZero() {
					tr.Send.RttMs = append(tr.Send.RttMs, float64(now.Sub(sendT[a]).Microseconds())/1000)
				}
				rej := ""
				if r.Status.Rejected != nil {
					rej = *r.Status.Rejected
				}
				lim := []string{}
				for _, l := range r.Status.LimitedBy {
					lim = append(lim, limitNames[l])
				}
				tr.Acks.T, tr.Acks.Index = append(tr.Acks.T, now.Sub(t0).Seconds()), append(tr.Acks.Index, a)
				tr.Acks.State, tr.Acks.Rejected = append(tr.Acks.State, stateNames[r.Status.State]), append(tr.Acks.Rejected, rej)
				tr.Acks.LimitedBy = append(tr.Acks.LimitedBy, lim)
				mu.Unlock()
			}
		}()
		vel, acc, tmo := o.vel*math.Pi/180, o.acc*math.Pi/180, uint32(o.timeoutMs)
		cfg := motion.ServoConfig{
			Mode: motion.ServoModePose, TimeoutMs: &tmo,
			Limits: &arm.MoveOptions{MaxVelRads: vel, MaxAccRads: acc},
		}
		go func() {
			streamErr = svc.StreamServo(sessCtx, o.arm, cfg, updates, responses, nil)
			close(done)
		}()
		send = func(i, seq int) (sent, cont bool) {
			u := motion.ServoUpdate{Target: referenceframe.NewPoseInFrame(base, targetAt(i))}
			if o.ff {
				u.FeedForward = &motion.ServoTwist{LinearMMPerSec: ffs[i], ReferenceFrame: base}
			}
			mu.Lock()
			sendT[seq] = time.Now()
			mu.Unlock()
			select {
			case updates <- u:
				return true, true
			case <-done:
				return false, false
			}
		}
		kill = cancel // abrupt: no CloseSend
		finish = func() {
			if sessCtx.Err() == nil {
				close(updates) // the server ramps to a stop
			}
			<-done
			close(responses)
			<-recvDone
			if streamErr != nil && sessCtx.Err() == nil {
				fmt.Fprintln(os.Stderr, "StreamServo:", streamErr)
				tr.Errors++
			}
		}
		select { // the schedule starts only once the session is live
		case <-started:
		case <-done:
			return fmt.Errorf("StreamServo ended before Started: %w", streamErr)
		}
	default:
		start := map[string]any{
			"component_name": o.arm, "mode": "pose", "timeout_ms": float64(o.timeoutMs),
			"limits": map[string]any{"max_vel_degs_per_sec": o.vel, "max_acc_degs_per_sec2": o.acc},
		}
		reply, err := svc.DoCommand(ctx, map[string]any{"servo_start": start})
		if err != nil {
			return err
		}
		sid, _ := reply["session_id"].(string)
		send = func(i, seq int) (sent, cont bool) {
			pt := targetAt(i)
			p, ovd := pt.Point(), pt.Orientation().OrientationVectorDegrees()
			cmd := map[string]any{"servo_target": map[string]any{"session_id": sid, "seq": float64(seq), "target": map[string]any{
				"reference_frame": base, "pose": map[string]any{
					"x": p.X, "y": p.Y, "z": p.Z,
					"o_x": ovd.OX, "o_y": ovd.OY, "o_z": ovd.OZ, "theta": ovd.Theta,
				},
			}}}
			s := time.Now()
			_, err := svc.DoCommand(ctx, cmd)
			mu.Lock()
			tr.Send.RttMs = append(tr.Send.RttMs, float64(time.Since(s).Microseconds())/1000)
			mu.Unlock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "servo_target %d: %v\n", seq, err)
				tr.Errors++
				return false, tr.Errors <= 5
			}
			return true, true
		}
		kill = func() {} // unary: just stop calling
		finish = func() {
			if st, err := svc.DoCommand(ctx, map[string]any{"servo_status": map[string]any{"session_id": sid}}); err == nil {
				tr.FinalStatus = st
			}
			if _, err := svc.DoCommand(ctx, map[string]any{"servo_stop": map[string]any{"session_id": sid}}); err != nil {
				fmt.Fprintln(os.Stderr, "servo_stop:", err)
			}
		}
	}

	if o.killAt > ts[len(ts)-1] {
		fmt.Fprintf(os.Stderr, "-kill-at %v is past the path end (%v s); no kill will happen\n", o.killAt, ts[len(ts)-1])
	}
	ref := time.Now()
	killed := false
	for i := range ts {
		due := dueAt(ref, ts[i])
		time.Sleep(time.Until(due))
		if o.killAt > 0 && ts[i] >= o.killAt {
			tr.KillAt = since()
			kill()
			killed = true
			break
		}
		now := time.Now()
		tr.Send.LateMs = append(tr.Send.LateMs, float64(now.Sub(due).Microseconds())/1000)
		s := since()
		sent, cont := send(i, i+1)
		if !cont {
			break
		}
		if !sent {
			continue
		}
		tr.Leader.T = append(tr.Leader.T, s)
		pt := anchor.Point().Add(xyz[i])
		tr.Leader.XYZ = append(tr.Leader.XYZ, [3]float64{pt.X, pt.Y, pt.Z})
	}
	if killed {
		// wait until the arm has been still for 200 ms (or 15 s)
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			mu.Lock()
			l, ok := stopLatency(tr.Joints.T, tr.Joints.QDeg, tr.KillAt)
			mu.Unlock()
			if ok {
				ms := math.Round(l*1e4) / 10
				tr.StopLatencyMs = &ms
				break
			}
		}
		if tr.StopLatencyMs == nil {
			fmt.Fprintln(os.Stderr, "arm did not settle within 15 s of the kill")
		}
	} else {
		time.Sleep(time.Second) // let the arm settle on the last target
	}
	tr.FinalStatus = status()
	finish()
	close(stop)
	wg.Wait()
	if tr.FinalStatus == nil {
		tr.FinalStatus = map[string]any{}
	}

	tr.Network = networkPath(r1)
	b, err := json.MarshalIndent(tr, "", " ")
	if err != nil {
		return err
	}
	logger.Infof("sent %d targets, %d acks, %d errors, stop_latency_ms=%v\n", len(tr.Leader.T), len(tr.Acks.T), tr.Errors, tr.StopLatencyMs)
	return os.WriteFile(o.out, b, 0o600)
}
