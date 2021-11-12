package control

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestTrapezoidVelocityProfileConfig(t *testing.T) {
	ctx := context.Background()

	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name:      "Trap1",
				Type:      "TrapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"MaxAcc": 1000.0,
					"MaxVel": 100.0,
				},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name:      "Trap1",
				Type:      "TrapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"MaxAcc": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs MaxVel field",
		},
		{
			ControlBlockConfig{
				Name:      "Trap1",
				Type:      "TrapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"MaxVel": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs MaxAcc field",
		},
	} {
		var s trapezoidVelocityGenerator
		err := s.Configure(ctx, c.conf)
		if c.err == "" {
			test.That(t, err, test.ShouldBeNil)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldEqual, c.err)
		}
	}
}
func TestTrapezoidVelocityProfileGenerator(t *testing.T) {
	ctx := context.Background()
	cfg := ControlBlockConfig{
		Name:      "Trap1",
		Type:      "TrapezoidalVelocityProfile",
		DependsOn: []string{},
		Attribute: config.AttributeMap{
			"MaxAcc": 1000.0,
			"MaxVel": 100.0,
		},
	}
	var s trapezoidVelocityGenerator
	err := s.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)

	ins := []Signal{
		{
			name:   "SetPoint",
			time:   []int{},
			signal: []float64{100.0},
		},
		{
			name:   "CurrentPosition",
			time:   []int{},
			signal: []float64{0.0},
		},
	}

	_, ok := s.Next(ctx, ins, time.Duration(0))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, accelPhase)
	i := 0
	for {
		i++
		y, ok := s.Next(ctx, []Signal{}, (10 * time.Millisecond))
		if ok == false {
			test.That(t, y[2].signal[0], test.ShouldEqual, 100.0)
			break
		}
	}
	ins[0].signal[0] = 3
	_, ok = s.Next(ctx, ins, time.Duration(0))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, accelPhase)
	i = 0
	for {
		i++
		y, ok := s.Next(ctx, []Signal{}, (10 * time.Millisecond))
		if ok == false {
			test.That(t, y[2].signal[0], test.ShouldEqual, 103.0)
			break
		}
	}
}
