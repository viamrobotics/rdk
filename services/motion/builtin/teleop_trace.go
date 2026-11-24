package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// teleopTraceSampleInterval is how often a traced arm's measured joint positions are read.
const teleopTraceSampleInterval = 20 * time.Millisecond

// teleopTraceRecord is one line of a teleop trace file. Kind is "goal" (a pose a client asked a
// component to reach), "cmd" (the joint positions the executor commanded an arm to), or "act"
// (an arm's measured joint positions).
type teleopTraceRecord struct {
	TimestampMs float64   `json:"t"`
	Kind        string    `json:"k"`
	Name        string    `json:"n"`
	Pose        []float64 `json:"pose,omitempty"`
	JointDeg    []float64 `json:"q,omitempty"`
}

// teleopTrace records, for one teleop pipeline, what was asked for, what was commanded, and what
// each arm did, as JSON lines. It measures the pipeline the same way whichever executor drives
// the arm, so the two can be compared. A nil *teleopTrace records nothing.
type teleopTrace struct {
	logger logging.Logger
	path   string

	mu     sync.Mutex
	file   *os.File
	w      *bufio.Writer
	enc    *json.Encoder
	arms   map[string]arm.Arm
	closed bool
	// writeFailed is set once a write has failed, so the failure is logged only once.
	writeFailed bool

	cancel context.CancelFunc
	done   chan struct{}
}

// newTeleopTrace starts a trace file under dir, or returns nil if dir is empty or the file
// cannot be created.
func newTeleopTrace(logger logging.Logger, dir, executor string) *teleopTrace {
	if dir == "" {
		return nil
	}
	outDir := filepath.Join(dir, "tag=teleop-trace")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		logger.Warnf("couldn't create teleop trace directory: %v", err)
		return nil
	}
	path := filepath.Join(outDir, fmt.Sprintf("teleop-trace-%s-%s.jsonl", executor, time.Now().Format("2006-01-02T15-04-05.000")))
	//nolint:gosec // path is built from the operator's configured diagnostics directory
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		logger.Warnf("couldn't create teleop trace file: %v", err)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := bufio.NewWriterSize(f, 1<<16)
	tr := &teleopTrace{
		logger: logger,
		path:   path,
		file:   f,
		w:      w,
		enc:    json.NewEncoder(w),
		arms:   map[string]arm.Arm{},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go tr.sample(ctx)
	logger.Infof("writing teleop trace to %s", path)
	return tr
}

func (tr *teleopTrace) write(rec teleopTraceRecord) {
	rec.TimestampMs = float64(time.Now().UnixMicro()) / 1000
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.closed {
		return
	}
	// A failed trace write must not disturb teleop, so it is reported once and otherwise dropped.
	if err := tr.enc.Encode(rec); err != nil && !tr.writeFailed {
		tr.writeFailed = true
		tr.logger.Warnf("couldn't write to teleop trace: %v", err)
	}
}

func inputsToDeg(inputs []referenceframe.Input) []float64 {
	degs := make([]float64, len(inputs))
	for i, v := range inputs {
		degs[i] = v * 180 / 3.141592653589793
	}
	return degs
}

// goal records a pose a client asked the named component to reach.
func (tr *teleopTrace) goal(name string, pif *referenceframe.PoseInFrame) {
	if tr == nil || pif == nil {
		return
	}
	pose := pif.Pose()
	pt := pose.Point()
	ov := pose.Orientation().OrientationVectorDegrees()
	tr.write(teleopTraceRecord{Kind: "goal", Name: name, Pose: []float64{pt.X, pt.Y, pt.Z, ov.OX, ov.OY, ov.OZ, ov.Theta}})
}

// cmd records the joint positions the executor commanded the named arm to, and starts sampling
// that arm's measured positions if it is not already being sampled.
func (tr *teleopTrace) cmd(name string, a arm.Arm, joints []referenceframe.Input) {
	if tr == nil {
		return
	}
	tr.mu.Lock()
	if _, ok := tr.arms[name]; !ok && a != nil {
		tr.arms[name] = a
	}
	tr.mu.Unlock()
	tr.write(teleopTraceRecord{Kind: "cmd", Name: name, JointDeg: inputsToDeg(joints)})
}

func (tr *teleopTrace) sample(ctx context.Context) {
	defer close(tr.done)
	ticker := time.NewTicker(teleopTraceSampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		tr.mu.Lock()
		arms := make(map[string]arm.Arm, len(tr.arms))
		for name, a := range tr.arms {
			arms[name] = a
		}
		tr.mu.Unlock()
		for name, a := range arms {
			readCtx, cancel := context.WithTimeout(ctx, teleopTraceSampleInterval)
			joints, err := a.JointPositions(readCtx, nil)
			cancel()
			if err != nil {
				continue
			}
			tr.write(teleopTraceRecord{Kind: "act", Name: name, JointDeg: inputsToDeg(joints)})
		}
	}
}

// close stops sampling and flushes the file.
func (tr *teleopTrace) close() {
	if tr == nil {
		return
	}
	tr.cancel()
	<-tr.done
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.closed {
		return
	}
	tr.closed = true
	if err := tr.w.Flush(); err != nil {
		tr.logger.Warnf("couldn't flush teleop trace: %v", err)
	}
	if err := tr.file.Close(); err != nil {
		tr.logger.Warnf("couldn't close teleop trace: %v", err)
	}
}
