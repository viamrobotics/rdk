package control

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestPIDConfig(t *testing.T) {
	ctx := context.Background()

	for i, tc := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name:      "PID1",
				Attribute: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
				Type:      "PID",
				DependsOn: []string{"A", "B"},
			},
			"pid block PID1 should have 1 input got 2",
		},
		{
			ControlBlockConfig{
				Name:      "PID1",
				Attribute: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name:      "PID1",
				Attribute: config.AttributeMap{"Kdd": 0.11},
				Type:      "PID",
				DependsOn: []string{"A"},
			},
			"pid block PID1 should have at least one Ki, Kp or Kd field",
		},
	} {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			var p basicPID
			err := p.Configure(ctx, tc.conf)
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
	var pid basicPID
	cfg := ControlBlockConfig{
		Name: "PID1",
		Attribute: config.AttributeMap{
			"Kd":          0.11,
			"Kp":          0.12,
			"Ki":          0.22,
			"LimitUp":     100.0,
			"LimitLo":     0.0,
			"IntSatLimUp": 100.0,
			"IntSatLimLo": 0.0,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	err := pid.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	s := []Signal{
		{
			name:   "A",
			signal: make([]float64, 1),
			time:   make([]int, 1),
		},
	}
	for i := 0; i < 50; i++ {
		dt := time.Duration(1000000 * 10)
		s[0].signal[0] = 1000
		out, ok := pid.Next(ctx, s, dt)
		if i < 47 {
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, out[0].signal[0], test.ShouldEqual, 100.0)
		} else {
			test.That(t, pid.sat, test.ShouldEqual, 1)
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			s[0].signal[0] = 0
			out, ok := pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.sat, test.ShouldEqual, 1)
			test.That(t, pid.int, test.ShouldBeGreaterThanOrEqualTo, 100)
			test.That(t, out[0].signal[0], test.ShouldEqual, 0.0)
			out, ok = pid.Next(ctx, s, dt)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, pid.sat, test.ShouldEqual, 0)
			test.That(t, pid.int, test.ShouldBeLessThanOrEqualTo, 100)
			test.That(t, out[0].signal[0], test.ShouldEqual, 100.0)
			break
		}
	}
	err = pid.Reset(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.sat, test.ShouldEqual, 0)
	test.That(t, pid.int, test.ShouldEqual, 0)
	test.That(t, pid.error, test.ShouldEqual, 0)
}

func TestPIDTunner(t *testing.T) {
	ctx := context.Background()
	var pid basicPID
	cfg := ControlBlockConfig{
		Name: "PID1",
		Attribute: config.AttributeMap{
			"Kd":          0.0,
			"Kp":          0.0,
			"Ki":          0.0,
			"LimitUp":     255.0,
			"LimitLo":     0.0,
			"IntSatLimUp": 255.0,
			"IntSatLimLo": 0.0,
			"TuneRValue":  1.0,
			"TuneStepPct": 0.45,
		},
		Type:      "PID",
		DependsOn: []string{"A"},
	}
	err := pid.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pid.tuning, test.ShouldBeTrue)
	test.That(t, pid.tuner.currentPhase, test.ShouldEqual, begin)
	s := []Signal{
		{
			name:   "A",
			signal: make([]float64, 1),
			time:   make([]int, 1),
		},
	}
	dt := time.Duration(time.Millisecond * 10)
	for i := 0; i < 22; i++ {
		s[0].signal[0] += 2
		out, hold := pid.Next(ctx, s, dt)
		test.That(t, out[0].signal[0], test.ShouldEqual, 255.0*0.45)
		test.That(t, hold, test.ShouldBeTrue)
	}
	for i := 0; i < 15; i++ {
		s[0].signal[0] = 100
		out, hold := pid.Next(ctx, s, dt)
		test.That(t, out[0].signal[0], test.ShouldEqual, 255.0*0.45)
		test.That(t, hold, test.ShouldBeTrue)
	}
	out, hold := pid.Next(ctx, s, dt)
	test.That(t, out[0].signal[0], test.ShouldEqual, 255.0*0.45+0.5*255.0*0.45)
	test.That(t, hold, test.ShouldBeTrue)
}
