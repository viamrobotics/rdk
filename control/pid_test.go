package control

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

var loop = Loop{}

func TestPIDMultiConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for i, tc := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "PID1",
				Attribute: utils.AttributeMap{
					"PIDSets": []*PIDConfig{{P: .12, I: .22, D: .11}, {P: .12, I: .22, D: .11}},
				},
				Type:      "PID",
				DependsOn: []string{"A", "B"},
			},
			"",
		},
		{
			BlockConfig{
				Name: "PID1",
				Attribute: utils.AttributeMap{
					"PIDSets": []*PIDConfig{{P: .12, I: .22, D: .11}, {P: .12, I: .22, D: .11}},
				},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"pid block PID1 should have 2 inputs got 1",
		},
		{
			BlockConfig{
				Name:      "PID1",
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"pid block PID1 does not have a PID configured",
		},
	} {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			_, err := loop.newPID(tc.conf, logger)
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldEqual, tc.err)
			}
		})
	}
}

func TestPIDMultiIntegralWindup(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := BlockConfig{
		Name: "PID1",
		Attribute: utils.AttributeMap{
			"PIDSets":        []*PIDConfig{{P: .12, I: .22, D: .11}, {P: .33, I: .33, D: .10}},
			"limit_up":       100.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 100.0,
			"int_sat_lim_lo": 0.0,
		},
		Type:      "PID",
		DependsOn: []string{"A", "B"},
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	s := []*Signal{
		{
			name:   "A",
			signal: make([]float64, 2),
			time:   make([]int, 1),
		},
	}

	for i := 0; i < 50; i++ {
		dt := time.Duration(1000000 * 10)
		s[0].SetSignalValueAt(0, 1000.0)
		s[0].SetSignalValueAt(1, 1000.0)

		out, ok := pid.Next(ctx, s, dt)
		if i < 46 {
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, out[0].signal[0], test.ShouldEqual, 100.0)
			test.That(t, out[0].signal[1], test.ShouldEqual, 100.0)
		} else {
			// Multi Input Signal Testing s[0]
			s[0].SetSignalValueAt(0, 0.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.PIDSets[0].int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 0.0)
			s[0].SetSignalValueAt(0, -1.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.PIDSets[0].int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual, 88.8778)

			// Multi Input Signal Testing s[1]
			s[0].SetSignalValueAt(1, 0.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.PIDSets[1].int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(1), test.ShouldEqual, 0.0)
			s[0].SetSignalValueAt(1, -1.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.PIDSets[1].int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(1), test.ShouldAlmostEqual, 89.6667)

			break
		}
	}
	err = pid.Reset(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, pid.PIDSets[0].int, test.ShouldEqual, 0)
	test.That(t, pid.PIDSets[0].signalErr, test.ShouldEqual, 0)
	test.That(t, pid.PIDSets[0].P, test.ShouldEqual, .12)
	test.That(t, pid.PIDSets[0].I, test.ShouldEqual, .22)
	test.That(t, pid.PIDSets[0].D, test.ShouldEqual, .11)

	test.That(t, pid.PIDSets[1].int, test.ShouldEqual, 0)
	test.That(t, pid.PIDSets[1].signalErr, test.ShouldEqual, 0)
	test.That(t, pid.PIDSets[1].P, test.ShouldEqual, .33)
	test.That(t, pid.PIDSets[1].I, test.ShouldEqual, .33)
	test.That(t, pid.PIDSets[1].D, test.ShouldEqual, .10)
}

func TestPIDMultiTuner(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// define N PID gains to tune
	pidConfigs := []*PIDConfig{{P: .0, I: .0, D: .0}, {P: .0, I: .0, D: .0}, {P: .0, I: .0, D: .0}}
	dependsOnNames := []string{"A", "B", "C"}

	cfg := BlockConfig{
		Name: "3 PID Set",
		Attribute: utils.AttributeMap{
			"PIDSets":        pidConfigs, // N PID Sets defined here
			"limit_up":       255.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 255.0,
			"int_sat_lim_lo": 0.0,
			"tune_ssr_value": 2.0,
			"tune_step_pct":  0.45,
		},
		Type:      "PID",
		DependsOn: dependsOnNames,
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.GetTuning(), test.ShouldBeTrue)
	test.That(t, pid.tuners[0].currentPhase, test.ShouldEqual, begin)
	s := []*Signal{
		{
			name:   "A",
			signal: make([]float64, len(pidConfigs)), // Make N signals here
			time:   make([]int, 1),
		},
	}
	dt := time.Millisecond * 10

	// we want to test the tuning behavior for each signal that we defined above
	for signalIndex := range s[0].signal {
		// This loop tests each PID controller's response to increasing input values,
		// verifying that it reaches a steady state such that the output remains constant.
		for i := 0; i < 22; i++ {
			s[0].SetSignalValueAt(signalIndex, s[0].GetSignalValueAt(signalIndex)+2)
			out, hold := pid.Next(ctx, s, dt)
			test.That(t, out[0].GetSignalValueAt(signalIndex), test.ShouldEqual, 255.0*0.45)
			test.That(t, hold, test.ShouldBeTrue)
		}

		// This loop tests each PID controller's response to constant input values, verifying
		// that it reaches a steady state such that the output remains constant.
		for i := 0; i < 15; i++ {
			// Set the signal to a constant value
			s[0].SetSignalValueAt(signalIndex, 100.0)
			test.That(t, s[0].GetSignalValueAt(signalIndex), test.ShouldEqual, 100)

			out, hold := pid.Next(ctx, s, dt)

			// Verify that each signal remained the correct output value after call to Next()
			test.That(t, out[0].GetSignalValueAt(signalIndex), test.ShouldEqual, 255.0*0.45)
			test.That(t, hold, test.ShouldBeTrue)
		}
		// After reaching steady state, these tests verify that each signal responds correctly to
		// 1 call to Next(). Each Signal should oscillate,
		out, hold := pid.Next(ctx, s, dt)
		test.That(t, out[0].GetSignalValueAt(signalIndex), test.ShouldEqual, 255.0*0.45+0.5*255.0*0.45)
		test.That(t, hold, test.ShouldBeTrue)

		// disable the tuner to test the next signal
		pid.tuners[signalIndex].tuning = false
	}
}

// A MIMO PID where only some sets need auto-tuning leaves the remaining tuners nil. getTuning()
// reports true as soon as any one set is tuning, so Next() walks every index and used to
// dereference the nil tuners belonging to the fully-configured sets.
func TestPIDMixedAutoTuningDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := BlockConfig{
		Name: "PID1",
		Attribute: utils.AttributeMap{
			// first set is fully configured (tuner stays nil), second needs tuning
			"PIDSets": []*PIDConfig{{P: .12, I: .22, D: .11}, {P: 0, I: 0, D: 0}},
		},
		Type:      "PID",
		DependsOn: []string{"A", "B"},
	}
	pid, err := loop.newPID(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	b, ok := pid.(*basicPID)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, b.tuners[0], test.ShouldBeNil)
	test.That(t, b.tuners[1], test.ShouldNotBeNil)
	test.That(t, b.GetTuning(), test.ShouldBeTrue)

	s := makeSignals("A", "endpoint", 2)
	s.SetSignalValueAt(0, 10.0)
	s.SetSignalValueAt(1, 10.0)
	test.That(t, func() {
		pid.Next(ctx, []*Signal{s}, time.Millisecond*10)
	}, test.ShouldNotPanic)
}

func TestPIDTunerComputeGains(t *testing.T) {
	// Ziegler-Nichols: kU = 4d/(pi*a) where d = 0.5*stepPwr and a is the mean oscillation amplitude.
	// With limUp 100 and stepPct 0.5, stepPwr = 50 and d = 25.
	newTuner := func(peaksH, peaksL []float64, tC time.Duration) *pidTuner {
		return &pidTuner{
			limUp: 100, stepPct: 0.5,
			tuneMethod: tuneMethodZiegerNicholsPI,
			pPeakH:     peaksH, pPeakL: peaksL, tC: tC,
		}
	}

	t.Run("amplitude is the mean over the peak pairs", func(t *testing.T) {
		// One pair, peak-to-peak swing of 10, so a = 5.
		p := newTuner([]float64{10}, []float64{0}, time.Second)
		test.That(t, p.computeGains(), test.ShouldBeNil)
		expectedKU := (4 * 25.0) / (math.Pi * 5.0)
		test.That(t, p.kP, test.ShouldAlmostEqual, 0.4545*expectedKU, 1e-9)

		// Two identical pairs must give the same amplitude, and so the same gains. Dividing by
		// nPeaks+1 instead of nPeaks made this depend on the number of peaks observed.
		p2 := newTuner([]float64{10, 10}, []float64{0, 0}, time.Second)
		test.That(t, p2.computeGains(), test.ShouldBeNil)
		test.That(t, p2.kP, test.ShouldAlmostEqual, p.kP, 1e-9)
	})

	t.Run("degenerate relay results are rejected", func(t *testing.T) {
		// No peaks bracketed: previously divided by zero and wrote +Inf gains.
		p := newTuner(nil, nil, time.Second)
		err := p.computeGains()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no oscillation peaks")
		test.That(t, math.IsInf(p.kP, 0), test.ShouldBeFalse)

		// Flat oscillation.
		p = newTuner([]float64{5}, []float64{5}, time.Second)
		test.That(t, p.computeGains().Error(), test.ShouldContainSubstring, "zero-amplitude")

		// tC never established.
		p = newTuner([]float64{10}, []float64{0}, 0)
		test.That(t, p.computeGains().Error(), test.ShouldContainSubstring, "zero oscillation period")
	})

	t.Run("cohen-coons does not require relay quantities", func(t *testing.T) {
		// Cohen-Coons runs before tC is known, so it must not be rejected for a zero tC.
		p := &pidTuner{
			limUp: 100, stepPct: 0.5,
			tuneMethod: tuneMethodCohenCoonsPID,
			avgSpeedSS: 40,
			ccT2:       2 * time.Second,
			ccT3:       3 * time.Second,
		}
		test.That(t, p.computeGains(), test.ShouldBeNil)
		test.That(t, p.kP, test.ShouldNotAlmostEqual, 0.0)
		test.That(t, math.IsInf(p.kP, 0) || math.IsNaN(p.kP), test.ShouldBeFalse)
	})
}
