package control

import (
	"context"
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestDerivativeConfig(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name: "Derive1",
				Type: "Derivative",
				Attribute: config.AttributeMap{
					"DeriveType": "Backward1st1",
				},
				DependsOn: []string{"A"},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name: "Derive1",
				Type: "Derivative",
				Attribute: config.AttributeMap{
					"DeriveType": "Backward5st1",
				},
				DependsOn: []string{"A"},
			},
			"unsupported DeriveType Backward5st1 for block Derive1",
		},
		{
			ControlBlockConfig{
				Name: "Derive1",
				Type: "Derivative",
				Attribute: config.AttributeMap{
					"DeriveType": "Backward2nd1",
				},
				DependsOn: []string{"A", "B"},
			},
			"derive block Derive1 only supports one input got 2",
		},
		{
			ControlBlockConfig{
				Name: "Derive1",
				Type: "Derivative",
				Attribute: config.AttributeMap{
					"DeriveType2": "Backward2nd1",
				},
				DependsOn: []string{"A"},
			},
			"derive block Derive1 doesn't have a DerivType field",
		},
	} {
		var d derivative
		err := d.Configure(ctx, c.conf)
		if c.err == "" {
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
	const iter int = 3000
	ctx := context.Background()
	cfg := ControlBlockConfig{
		Name: "Derive1",
		Type: "Derivative",
		Attribute: config.AttributeMap{
			"DeriveType": "Backward2nd2",
		},
		DependsOn: []string{"A"},
	}
	var d derivative
	err := d.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	var sin []float64
	for i := 0; i < iter; i++ {
		sin = append(sin, math.Sin((time.Duration(10 * i * int(time.Millisecond)).Seconds())))
	}
	sig := Signal{
		name:      "A",
		signal:    make([]float64, 1),
		time:      make([]int, 1),
		dimension: 1,
	}
	for i := 0; i < iter; i++ {
		sig.signal[0] = sin[i]
		out, ok := d.Next(ctx, []Signal{sig}, (10 * time.Millisecond))
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].signal[0], test.ShouldAlmostEqual,
				-math.Sin((time.Duration(10 * i * int(time.Millisecond)).Seconds())), 0.01)
		}
	}
	cfg = ControlBlockConfig{
		Name: "Derive1",
		Type: "Derivative",
		Attribute: config.AttributeMap{
			"DeriveType": "Backward1st2",
		},
		DependsOn: []string{"A"},
	}
	err = d.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < iter; i++ {
		sig.signal[0] = sin[i]
		out, ok := d.Next(ctx, []Signal{sig}, (10 * time.Millisecond))
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].signal[0], test.ShouldAlmostEqual,
				math.Cos((time.Duration(10 * i * int(time.Millisecond)).Seconds())), 0.01)
		}
	}
	cfg = ControlBlockConfig{
		Name: "Derive1",
		Type: "Derivative",
		Attribute: config.AttributeMap{
			"DeriveType": "Backward1st3",
		},
		DependsOn: []string{"A"},
	}
	err = d.Configure(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	sin = nil
	for i := 0; i < iter; i++ {
		sin = append(sin, math.Sin(2*math.Pi*(time.Duration(10*i*int(time.Millisecond)).Seconds())))
	}
	for i := 0; i < iter; i++ {
		sig.signal[0] = sin[i]
		out, ok := d.Next(ctx, []Signal{sig}, 10*time.Millisecond)
		test.That(t, ok, test.ShouldBeTrue)
		if i > 5 {
			test.That(t, out[0].signal[0], test.ShouldAlmostEqual,
				2*math.Pi*math.Cos(2*math.Pi*(time.Duration(10*i*int(time.Millisecond)).Seconds())), 0.01)
		}
	}
}
