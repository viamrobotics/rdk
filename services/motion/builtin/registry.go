package builtin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/services/motion/builtin/servo"
)

type holderKind int

const (
	holderServo holderKind = iota + 1
	holderTeleop
	holderStream
)

// holder is what owns an arm: a servo session, the teleop pipeline or a TempStream session.
type holder struct {
	kind   holderKind
	servo  *servoEntry // session, cancel, done, arm, model config
	stream *stream     // today's payload, unchanged
}

// servoEntry is one running servo session as the registry and the verbs see it.
type servoEntry struct {
	id      string
	session *servo.Session
	// cancel ends the arm stream abruptly, with no ramp. Prefer session.BeginStop.
	cancel context.CancelFunc
	// done is closed LAST by the session goroutine: after the loop is terminal, arm.Stop (on FAULTED),
	// the streamer closed and the registry released. Move, Stop, Close and Reconfigure wait on this,
	// never on session.Done(), so they never race the cleanup.
	done       chan struct{}
	arm        string
	ramp       time.Duration
	stopBudget time.Duration // how long waitEntry waits: ramp + arm.Stop + stream close + margin
	config     []byte        // the arm's kinematics config at start, for Reconfigure's same-model check
	gen        uint64        // registry generation at claim; publish refuses if it has moved
	streamed   bool          // owned by a StreamServo call: servo_target / servo_velocity refuse it
}

// registry maps arm name to its holder. The zero value is ready to use.
type registry struct {
	mu sync.Mutex
	m  map[string]holder
	// last is each arm's most recently ended servo session, so a client can still read its terminal
	// state. ponytail: one per arm, dropped when the next session on that arm ends; no TTL.
	last map[string]*servoEntry
	// closed is set by Close; publish refuses afterwards, so a start still in flight can't outlive the service.
	closed bool
	// gen counts Reconfigures and stop-alls. A start claims at one gen and publish refuses if it has moved,
	// since all(holderServo) can't see (so can't stop or re-check) a start still in flight.
	gen uint64
}

func newRegistry() *registry { return &registry{} }

// claim records h as arm's holder, or errors naming the current one. A stream holder keeps today's
// "a stream is already running" wording; teleop re-claiming its own arm is a no-op.
func (r *registry) claim(arm string, h holder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("motion service is closed")
	}
	if cur, ok := r.m[arm]; ok {
		if cur.kind == holderTeleop && h.kind == holderTeleop {
			return nil
		}
		return heldBy(arm, cur)
	}
	if r.m == nil {
		r.m = make(map[string]holder)
	}
	if h.kind == holderServo {
		h.servo.gen = r.gen
	}
	r.m[arm] = h
	return nil
}

// heldBy is the error for an arm cur already holds. A stream keeps today's "a stream is already running" wording.
func heldBy(arm string, cur holder) error {
	switch cur.kind {
	case holderStream:
		return fmt.Errorf("a stream is already running for arm %q", arm)
	case holderServo:
		return fmt.Errorf("arm %q is held by a servo session", arm)
	case holderTeleop:
		return fmt.Errorf("arm %q is held by teleop", arm)
	default:
		return fmt.Errorf("arm %q is held", arm)
	}
}

// release frees arm if kind holds it; a no-op otherwise.
func (r *registry) release(arm string, kind holderKind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.m[arm]; ok && cur.kind == kind {
		delete(r.m, arm)
		if kind == holderServo && cur.servo.session != nil && !r.closed { // a start's unpublished window leaves nothing to report
			if r.last == nil {
				r.last = make(map[string]*servoEntry)
			}
			r.last[arm] = cur.servo
		}
	}
}

func (r *registry) get(arm string) (holder, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.m[arm]
	return h, ok
}

// publish fills in a claimed servo entry once its session exists. Until then verbs don't see it. It refuses
// once the registry is closed.
func (r *registry) publish(e *servoEntry, v servoEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("motion service is closed")
	}
	if e.gen != r.gen {
		return errors.New("arm reconfigured or stopped during servo_start")
	}
	v.gen = e.gen
	*e = v
	return nil
}

// bumpGen invalidates every start claimed so far that has not published.
func (r *registry) bumpGen() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gen++
}

// close marks the registry closed; later publishes fail.
func (r *registry) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
}

// drop releases arm if e still holds it, without recording e as ended (for a start that failed).
func (r *registry) drop(arm string, e *servoEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.m[arm]; ok && cur.kind == holderServo && cur.servo == e {
		delete(r.m, arm)
	}
}

// all returns a copy of the holders of one kind. Servo entries not yet published are skipped.
func (r *registry) all(kind holderKind) map[string]holder {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]holder)
	for a, h := range r.m {
		if h.kind == kind && (kind != holderServo || h.servo.session != nil) {
			out[a] = h
		}
	}
	return out
}

// ended returns the servo sessions that have finished, one per arm.
func (r *registry) ended() map[string]*servoEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]*servoEntry, len(r.last))
	for a, e := range r.last {
		out[a] = e
	}
	return out
}

// releaseKind frees every arm held by kind (the teleop pipeline stopping).
func (r *registry) releaseKind(kind holderKind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for a, h := range r.m {
		if h.kind == kind {
			delete(r.m, a)
		}
	}
}

// forget drops the ended session recorded for arm, so a removed arm's handle and frame system can be collected.
func (r *registry) forget(arm string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.last, arm)
}

// forgetAll drops every recorded ended session.
func (r *registry) forgetAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = nil
}

// busy reports whether any servo or stream entry exists, pending ones included.
func (r *registry) busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.m {
		if h.kind != holderTeleop {
			return true
		}
	}
	return false
}

// getServo returns a copy of arm's servo entry, read under the lock so it can't race publish. A copy with a
// nil session is a start in progress.
func (r *registry) getServo(arm string) (servoEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.m[arm]; ok && h.kind == holderServo {
		return *h.servo, true
	}
	return servoEntry{}, false
}
