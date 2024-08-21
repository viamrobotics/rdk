package control

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

var loop = Loop{}

func TestPIDConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for i, tc := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name:      "PID1",
				Attribute: utils.AttributeMap{"kD": 0.11, "kP": 0.12, "kI": 0.22},
				Type:      "PID",
				DependsOn: []string{"A", "B"},
			},
			"pid block PID1 should have 1 input got 2",
		},
		{
			BlockConfig{
				Name:      "PID1",
				Attribute: utils.AttributeMap{"kD": 0.11, "kP": 0.12, "kI": 0.22},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			BlockConfig{
				Name:      "PID1",
				Attribute: utils.AttributeMap{"Kdd": 0.11},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"pid block PID1 should have at least one kI, kP or kD field",
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

func TestPIDBasicIntegralWindup(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := BlockConfig{
		Name: "PID1",
		Attribute: utils.AttributeMap{
			"kD":             0.11,
			"kP":             0.12,
			"kI":             0.22,
			"limit_up":       100.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 100.0,
			"int_sat_lim_lo": 0.0,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	s := []*Signal{
		{
			name:   "A",
			signal: make([]float64, 1),
			time:   make([]int, 1),
		},
	}
	for i := 0; i < 50; i++ {
		dt := time.Duration(1000000 * 10)
		s[0].SetSignalValueAt(0, 1000.0)
		out, ok := pid.Next(ctx, s, dt)
		if i < 46 {
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 100.0)
		} else {
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			s[0].SetSignalValueAt(0, 0.0)
			out, ok := pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 0.0)
			s[0].SetSignalValueAt(0, -1.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual, 88.8778)
			break
		}
	}
	err = pid.Reset(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.int, test.ShouldEqual, 0)
	test.That(t, pid.error, test.ShouldEqual, 0)
}

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
					"kD":      0.11,
					"kP":      0.12,
					"kI":      0.22,
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
					"kD":      0.11,
					"kP":      0.12,
					"kI":      0.22,
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
				Attribute: utils.AttributeMap{"Kdd": 0.11},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"pid block PID1 should have at least one kI, kP or kD field",
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
			"kP":             0.12,
			"kI":             0.22,
			"kD":             0.11,
			"PIDSets":        []*PIDConfig{{P: .12, I: .22, D: .11}, {P: .33, I: .33, D: .10}},
			"limit_up":       100.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 100.0,
			"int_sat_lim_lo": 0.0,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	pid.useMulti = true
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
	test.That(t, pid.int, test.ShouldEqual, 0)
	test.That(t, pid.error, test.ShouldEqual, 0)

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

func TestPIDTuner(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := BlockConfig{
		Name: "PID1",
		Attribute: utils.AttributeMap{
			"kD":             0.0,
			"kP":             0.0,
			"kI":             0.0,
			"limit_up":       255.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 255.0,
			"int_sat_lim_lo": 0.0,
			"tune_ssr_value": 2.0,
			"tune_step_pct":  0.45,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.GetTuning(), test.ShouldBeTrue)
	test.That(t, pid.tuner.currentPhase, test.ShouldEqual, begin)
	s := []*Signal{
		{
			name:   "A",
			signal: make([]float64, 1),
			time:   make([]int, 1),
		},
	}
	dt := time.Millisecond * 10
	for i := 0; i < 22; i++ {
		s[0].SetSignalValueAt(0, s[0].GetSignalValueAt(0)+2)
		out, hold := pid.Next(ctx, s, dt)
		test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 255.0*0.45)
		test.That(t, hold, test.ShouldBeTrue)
	}
	for i := 0; i < 15; i++ {
		s[0].SetSignalValueAt(0, 100.0)
		out, hold := pid.Next(ctx, s, dt)
		test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 255.0*0.45)
		test.That(t, hold, test.ShouldBeTrue)
	}
	out, hold := pid.Next(ctx, s, dt)
	test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 255.0*0.45+0.5*255.0*0.45)
	test.That(t, hold, test.ShouldBeTrue)
}

func TestPIDMultiTuner(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// define N PID gains to tune
	pidConfigs := []*PIDConfig{{P: .0, I: .0, D: .0}, {P: .0, I: .0, D: .0}, {P: .0, I: .0, D: .0}}
	cfg := BlockConfig{
		Name: "3 PID Set",
		Attribute: utils.AttributeMap{
			"kD":             0.0,
			"kP":             0.0,
			"kI":             0.0,
			"PIDSets":        pidConfigs, // N PID Sets defined here
			"limit_up":       255.0,
			"limit_lo":       0.0,
			"int_sat_lim_up": 255.0,
			"int_sat_lim_lo": 0.0,
			"tune_ssr_value": 2.0,
			"tune_step_pct":  0.45,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	b, err := loop.newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.GetTuning(), test.ShouldBeTrue)
	test.That(t, pid.tuner.currentPhase, test.ShouldEqual, begin)
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

func TestMIMOPIDConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for i, tc := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "PID1",
				Attribute: utils.AttributeMap{
					"kD": 0.11, "kP": 0.12, "kI": 0.22,
					"PIDSets": []*PIDConfig{{P: .12, I: .13, D: .14}, {P: .22, I: .23, D: .24}},
				},
				Type:      "PID",
				DependsOn: []string{"A", "B"},
			},
			"pid block PID1 should have 1 input got 2",
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
