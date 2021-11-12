package control

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestGainConfig(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name: "Gain1",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": 1.89345,
				},
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name: "Gain1",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"GainS": 1.89345,
				},
				DependsOn: []string{"A"},
			},
			"gain block Gain1 doesn't have a Gain field",
		},
		{
			ControlBlockConfig{
				Name: "Gain1",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": 1.89345,
				},
				DependsOn: []string{"A", "B"},
			},
			"invalid number of inputs for gain block Gain1 expected 1 got 2",
		},
	} {
		var s gain

		err := s.Configure(ctx, c.conf)
		if c.err == "" {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(s.y), test.ShouldEqual, 1)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldResemble, c.err)
		}
	}
}

func TestGainNext(t *testing.T) {
	ctx := context.Background()
	c := ControlBlockConfig{
		Name: "Gain1",
		Type: "Gain",
		Attribute: config.AttributeMap{
			"Gain": 1.89345,
		},
		DependsOn: []string{"A"},
	}
	var s gain
	err := s.Configure(ctx, c)

	test.That(t, err, test.ShouldBeNil)

	signals := []Signal{
		{name: "A",
			signal:    []float64{1.0},
			time:      []int{1},
			dimension: 1,
		},
	}
	out, ok := s.Next(ctx, signals, (time.Millisecond * 1))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].signal[0], test.ShouldEqual, 1.89345)
}
