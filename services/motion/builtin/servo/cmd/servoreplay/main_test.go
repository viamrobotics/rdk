package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestLoadPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.json")
	test.That(t, os.WriteFile(f, []byte(`{"t":[10,10.5,11],"xyz_mm":[[0,0,0],[10,0,0],[10,5,0]]}`), 0o600), test.ShouldBeNil)
	ts, xyz, err := loadPath(f, 0.5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ts, test.ShouldResemble, []float64{0, 0.5, 1})        // shifted to start at 0
	test.That(t, xyz[2], test.ShouldResemble, r3.Vector{X: 5, Y: 2.5}) // scaled about the start
}

func TestFeedForward(t *testing.T) {
	ff := feedForward([]float64{0, 0.5, 1}, []r3.Vector{{}, {X: 10}, {X: 10, Y: 5}})
	test.That(t, ff, test.ShouldResemble, []r3.Vector{{X: 20}, {Y: 10}, {}}) // last is zero
}

func TestDueAt(t *testing.T) {
	ref := time.Unix(100, 0)
	test.That(t, dueAt(ref, 1.25).Sub(ref), test.ShouldEqual, 1250*time.Millisecond) // from the reference, not the previous send
}

func TestStopLatency(t *testing.T) {
	ts := []float64{0.9, 1.0, 1.1, 1.2, 1.3, 1.4, 1.5}
	q := [][]float64{{0}, {1}, {2}, {3}, {3}, {3}, {3}}
	_, ok := stopLatency(ts[:5], q[:5], 1.0)
	test.That(t, ok, test.ShouldBeFalse) // only 100 ms still
	l, ok := stopLatency(ts, q, 1.0)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, l, test.ShouldAlmostEqual, 0.2, 1e-9) // last change at 1.2

	// 1 ms samples drifting 0.01 deg/sample for 300 ms, then flat: a consecutive-sample rule would call this still.
	var dt []float64
	var dq [][]float64
	for i := 0; i < 700; i++ {
		dt = append(dt, 1+float64(i)*0.001)
		dq = append(dq, []float64{math.Min(float64(i), 300) * 0.01})
	}
	_, ok = stopLatency(dt[:450], dq[:450], 1.0)
	test.That(t, ok, test.ShouldBeFalse) // drifting, then < 200 ms of stillness
	l, ok = stopLatency(dt, dq, 1.0)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, l, test.ShouldBeBetween, 0.293, 0.301) // within 0.05 deg of the final value from ~5 ms before the drift ends
}

func TestCausalFeedForward(t *testing.T) {
	ts := []float64{0, 0.1, 0.2, 0.3}
	xyz := []r3.Vector{{}, {X: 1}, {X: 2}, {X: 3}} // 10 mm/s
	ff := causalFeedForward(ts, xyz, 1)
	if ff[0].X != 0 || math.Abs(ff[3].X-10) > 1e-9 {
		t.Fatalf("alpha 1 should be the plain backward difference: %v", ff)
	}
	ff = causalFeedForward(ts, xyz, 0.5)
	if math.Abs(ff[1].X-5) > 1e-9 || math.Abs(ff[2].X-7.5) > 1e-9 {
		t.Fatalf("alpha 0.5 EMA: %v", ff)
	}
}
