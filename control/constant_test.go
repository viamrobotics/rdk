package control

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestConstantConfig(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: config.AttributeMap{
					"constant_val": 1.89345,
				},
				DependsOn: []string{},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: config.AttributeMap{
					"constant_S": 1.89345,
				},
				DependsOn: []string{},
			},
			"constant block constant1 doesn't have a constant_val field",
		},
		{
			ControlBlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: config.AttributeMap{
					"constant_val": 1.89345,
				},
				DependsOn: []string{"A", "B"},
			},
			"invalid number of inputs for constant block constant1 expected 0 got 2",
		},
	} {
		var s constant

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

func TestConstantNext(t *testing.T) {
	ctx := context.Background()
	c := ControlBlockConfig{
		Name: "constant1",
		Type: "constant",
		Attribute: config.AttributeMap{
			"constant_val": 1.89345,
		},
		DependsOn: []string{},
	}
	var s constant
	err := s.Configure(ctx, c)

	test.That(t, err, test.ShouldBeNil)
	out, ok := s.Next(ctx, []Signal{}, (time.Millisecond * 1))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 1.89345)
}
