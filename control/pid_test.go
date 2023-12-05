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
			_, err := newPID(tc.conf, logger)
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
	b, err := newPID(cfg, logger)
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
			test.That(t, pid.sat, test.ShouldEqual, 1)
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			s[0].SetSignalValueAt(0, 0.0)
			out, ok := pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.sat, test.ShouldEqual, 1)
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 0.0)
			s[0].SetSignalValueAt(0, -1.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.sat, test.ShouldEqual, 0)
			test.That(t, pid.int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual, 88.8778)
			break
		}
	}
	err = pid.Reset(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.sat, test.ShouldEqual, 0)
	test.That(t, pid.int, test.ShouldEqual, 0)
	test.That(t, pid.error, test.ShouldEqual, 0)
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
	b, err := newPID(cfg, logger)
	pid := b.(*basicPID)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.tuning, test.ShouldBeTrue)
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
