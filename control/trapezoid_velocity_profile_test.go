package control

import (
	"context"
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestTrapezoidVelocityProfileConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)

	for _, c := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name:      "Trap1",
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: utils.AttributeMap{
					"max_acc": 1000.0,
					"max_vel": 100.0,
				},
			},
			"",
		},
		{
			BlockConfig{
				Name:      "Trap1",
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: utils.AttributeMap{
					"max_acc": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs max_vel field",
		},
		{
			BlockConfig{
				Name:      "Trap1",
				Type:      "trapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: utils.AttributeMap{
					"max_vel": 1000.0,
				},
			},
			"trapezoidale velocity profile block Trap1 needs max_acc field",
		},
	} {
		_, err := newTrapezoidVelocityProfile(c.conf, logger)
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
	logger := logging.NewTestLogger(t)
	targetPos := 100.0
	posWindow := 10.0
	cfg := BlockConfig{
		Name:      "Trap1",
		Type:      "trapezoidalVelocityProfile",
		DependsOn: []string{},
		Attribute: utils.AttributeMap{
			"max_acc":    1000.0,
			"max_vel":    100.0,
			"pos_window": posWindow,
		},
	}
	b, err := newTrapezoidVelocityProfile(cfg, logger)
	s := b.(*trapezoidVelocityGenerator)
	test.That(t, err, test.ShouldBeNil)

	ins := []*Signal{
		{
			name:   "set_point",
			time:   []int{},
			signal: []float64{targetPos},
		},
		{
			name:   "endpoint",
			time:   []int{},
			signal: []float64{0.0},
		},
	}

	y, ok := s.Next(ctx, ins, (10 * time.Millisecond))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, active)
	test.That(t, y[0].GetSignalValueAt(0), test.ShouldNotBeZeroValue)
	for {
		y, _ := s.Next(ctx, ins, (10 * time.Millisecond))
		if math.Abs(ins[1].GetSignalValueAt(0)-targetPos) > posWindow {
			test.That(t, s.currentPhase, test.ShouldEqual, active)
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldNotBeZeroValue)
		} else {
			test.That(t, s.currentPhase, test.ShouldEqual, rest)
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldBeZeroValue)
			break
		}
		ins[1].SetSignalValueAt(0, ins[1].GetSignalValueAt(0)+(10*time.Millisecond).Seconds()*y[0].GetSignalValueAt(0))
	}
	ins[0].SetSignalValueAt(0, targetPos-4)
	y, ok = s.Next(ctx, ins, (10 * time.Millisecond))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, s.currentPhase, test.ShouldEqual, rest)
	test.That(t, y[0].GetSignalValueAt(0), test.ShouldBeZeroValue)
	ins[1].SetSignalValueAt(0, targetPos*2)
	for {
		y, _ := s.Next(ctx, ins, (10 * time.Millisecond))
		if math.Abs(ins[1].GetSignalValueAt(0)-targetPos+4) > posWindow {
			test.That(t, s.currentPhase, test.ShouldEqual, active)
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldNotBeZeroValue)
		} else {
			test.That(t, s.currentPhase, test.ShouldEqual, rest)
			test.That(t, y[0].GetSignalValueAt(0), test.ShouldBeZeroValue)
			break
		}
		ins[1].SetSignalValueAt(0, ins[1].GetSignalValueAt(0)+(10*time.Millisecond).Seconds()*y[0].GetSignalValueAt(0))
	}
}
