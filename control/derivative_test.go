package control

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestDerivativeConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	for _, c := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "Derive1",
				Type: "derivative",
				Attribute: utils.AttributeMap{
					"derive_type": "backward1st1",
				},
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			BlockConfig{
				Name: "Derive1",
				Type: "derivative",
				Attribute: utils.AttributeMap{
					"derive_type": "backward5st1",
				},
				DependsOn: []string{"A"},
			},
			"unsupported derive_type backward5st1 for block Derive1",
		},
		{
			BlockConfig{
				Name: "Derive1",
				Type: "derivative",
				Attribute: utils.AttributeMap{
					"derive_type": "backward2nd1",
				},
				DependsOn: []string{"A", "B"},
			},
			"derive block Derive1 only supports one input got 2",
		},
		{
			BlockConfig{
				Name: "Derive1",
				Type: "derivative",
				Attribute: utils.AttributeMap{
					"derive_type2": "backward2nd1",
				},
				DependsOn: []string{"A"},
			},
			"derive block Derive1 doesn't have a derive_type field",
		},
	} {
		b, err := newDerivative(c.conf, logger)
		if c.err == "" {
			d := b.(*derivative)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(d.y), test.ShouldEqual, 1)
			test.That(t, len(d.y[0].signal), test.ShouldEqual, 1)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldResemble, c.err)
		}
	}
}

func TestDerivativeNext(t *testing.T) {
	const iter int64 = 3000
	const tenMs = 10 * int64(time.Millisecond)
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg := BlockConfig{
		Name: "Derive1",
		Type: "derivative",
		Attribute: utils.AttributeMap{
			"derive_type": "backward2nd2",
		},
		DependsOn: []string{"A"},
	}
	b, err := newDerivative(cfg, logger)
	d := b.(*derivative)
	test.That(t, err, test.ShouldBeNil)
	var sin []float64
	for i := int64(0); i < iter; i++ {
		sin = append(sin, math.Sin(time.Duration(i*tenMs).Seconds()))
	}
	sig := &Signal{
		name:      "A",
		signal:    make([]float64, 1),
		time:      make([]int, 1),
		dimension: 1,
	}
	for i := int64(0); i < iter; i++ {
		sig.SetSignalValueAt(0, sin[i])
		out, ok := d.Next(ctx, []*Signal{sig}, (10 * time.Millisecond))
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual,
				-math.Sin((time.Duration(i * tenMs).Seconds())), 0.01)
		}
	}
	cfg = BlockConfig{
		Name: "Derive1",
		Type: "derivative",
		Attribute: utils.AttributeMap{
			"derive_type": "backward1st2",
		},
		DependsOn: []string{"A"},
	}
	err = d.UpdateConfig(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	for i := int64(0); i < iter; i++ {
		sig.SetSignalValueAt(0, sin[i])
		out, ok := d.Next(ctx, []*Signal{sig}, (10 * time.Millisecond))
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual,
				math.Cos((time.Duration(i * tenMs).Seconds())), 0.01)
		}
	}
	cfg = BlockConfig{
		Name: "Derive1",
		Type: "derivative",
		Attribute: utils.AttributeMap{
			"derive_type": "backward1st3",
		},
		DependsOn: []string{"A"},
	}
	err = d.UpdateConfig(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	sin = nil
	for i := int64(0); i < iter; i++ {
		sin = append(sin, math.Sin(2*math.Pi*(time.Duration(i*tenMs).Seconds())))
	}
	for i := int64(0); i < iter; i++ {
		sig.SetSignalValueAt(0, sin[i])
		out, ok := d.Next(ctx, []*Signal{sig}, 10*time.Millisecond)
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].GetSignalValueAt(0), test.ShouldAlmostEqual,
				2*math.Pi*math.Cos(2*math.Pi*(time.Duration(i*tenMs).Seconds())), 0.01)
		}
	}
}
