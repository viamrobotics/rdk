package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestGainConfig(t *testing.T) {
	logger := golog.NewDebugLogger("gain")
	for _, c := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "Gain1",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gain": 1.89345,
				},
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			BlockConfig{
				Name: "Gain1",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gainS": 1.89345,
				},
				DependsOn: []string{"A"},
			},
			"gain block Gain1 doesn't have a gain field",
		},
		{
			BlockConfig{
				Name: "Gain1",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gain": 1.89345,
				},
				DependsOn: []string{"A", "B"},
			},
			"invalid number of inputs for gain block Gain1 expected 1 got 2",
		},
	} {
		b, err := newGain(c.conf, logger)
		if c.err == "" {
			s := b.(*gain)
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
	logger := golog.NewDebugLogger("gain")
	c := BlockConfig{
		Name: "Gain1",
		Type: "gain",
		Attribute: config.AttributeMap{
			"gain": 1.89345,
		},
		DependsOn: []string{"A"},
	}
	s, err := newGain(c, logger)

	test.That(t, err, test.ShouldBeNil)

	signals := []Signal{
		{
			name:      "A",
			signal:    []float64{1.0},
			time:      []int{1},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
	}
	out, ok := s.Next(ctx, signals, (time.Millisecond * 1))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 1.89345)
}
