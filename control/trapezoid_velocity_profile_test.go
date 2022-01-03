package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
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
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"max_acc": 1000.0,
					"max_vel": 100.0,
				},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name:      "Trap1",
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"max_acc": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs max_vel field",
		},
		{
			ControlBlockConfig{
				Name:      "Trap1",
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"max_vel": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs max_acc field",
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
		Type:      "trapezoidalVelocityProfile",
		DependsOn: []string{},
		Attribute: config.AttributeMap{
			"max_acc": 1000.0,
			"max_vel": 100.0,
		},
	}
	var s trapezoidVelocityGenerator
	err := s.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)

	ins := []Signal{
		{
			name:   "set_point",
			time:   []int{},
			signal: []float64{100.0},
			mu:     &sync.Mutex{},
		},
		{
			name:   "endpoint",
			time:   []int{},
			signal: []float64{0.0},
			mu:     &sync.Mutex{},
		},
	}

	_, ok := s.Next(ctx, ins, time.Duration(0))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, accelPhase)
	i := 0
	for {
		i++
		y, _ := s.Next(ctx, []Signal{}, (10 * time.Millisecond))
		if i == 102 {
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldEqual, 10.0)
			break
		}
		if i == 87 {
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldEqual, 100.0)
		}
	}
	ins[0].SetSignalValueAt(0, 3)
	_, ok = s.Next(ctx, ins, time.Duration(0))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, accelPhase)
	i = 0
	for {
		i++
		y, _ := s.Next(ctx, []Signal{}, (10 * time.Millisecond))
		time.Sleep(100 * time.Millisecond)
		if i == 5 {
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldEqual, 60.0)
			break
		}
	}
}
