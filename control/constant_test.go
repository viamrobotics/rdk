package control

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

func TestConstantConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	for _, c := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: utils.AttributeMap{
					"constant_val": 1.89345,
				},
				DependsOn: []string{},
			},
			"",
		},
		{
			BlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: utils.AttributeMap{
					"constant_S": 1.89345,
				},
				DependsOn: []string{},
			},
			"constant block constant1 doesn't have a constant_val field",
		},
		{
			BlockConfig{
				Name: "constant1",
				Type: "constant",
				Attribute: utils.AttributeMap{
					"constant_val": 1.89345,
				},
				DependsOn: []string{"A", "B"},
			},
			"invalid number of inputs for constant block constant1 expected 0 got 2",
		},
	} {
		b, err := newConstant(c.conf, logger)
		if c.err == "" {
			s := b.(*constant)
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
	logger := logging.NewTestLogger(t)
	c := BlockConfig{
		Name: "constant1",
		Type: "constant",
		Attribute: utils.AttributeMap{
			"constant_val": 1.89345,
		},
		DependsOn: []string{},
	}
	s, err := newConstant(c, logger)
	test.That(t, err, test.ShouldBeNil)
	out, ok := s.Next(ctx, []*Signal{}, (time.Millisecond * 1))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, 1.89345)
}
