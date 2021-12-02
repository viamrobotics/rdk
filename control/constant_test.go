package control

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestConstantConfig(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name: "Constant1",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 1.89345,
				},
				DependsOn: []string{},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name: "Constant1",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantS": 1.89345,
				},
				DependsOn: []string{},
			},
			"constant block Constant1 doesn't have a ConstantVal field",
		},
		{
			ControlBlockConfig{
				Name: "Constant1",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 1.89345,
				},
				DependsOn: []string{"A", "B"},
			},
			"invalid number of inputs for constant block Constant1 expected 0 got 2",
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
		Name: "Constant1",
		Type: "Constant",
		Attribute: config.AttributeMap{
			"ConstantVal": 1.89345,
		},
		DependsOn: []string{},
	}
	var s constant
	err := s.Configure(ctx, c)

	test.That(t, err, test.ShouldBeNil)
	out, ok := s.Next(ctx, []Signal{}, (time.Millisecond * 1))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].signal[0], test.ShouldEqual, 1.89345)
}
