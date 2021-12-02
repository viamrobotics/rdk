package control

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestControlLoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg := ControlConfig{
		Blocks: []ControlBlockConfig{
			{
				Name: "A",
				Type: "Endpoint",
				Attribute: config.AttributeMap{
					"MotorName": "MotorFake",
				},
				DependsOn: []string{"E"},
			},
			{
				Name: "B",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "+-",
				},
				DependsOn: []string{"A", "S1"},
			},
			{
				Name: "S1",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 3.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "C",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": -2.0,
				},
				DependsOn: []string{"B"},
			},
			{
				Name: "D",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "+-",
				},
				DependsOn: []string{"C", "S2"},
			},
			{
				Name: "S2",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 10.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "E",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": -2.0,
				},
				DependsOn: []string{"D"},
			},
		},
		Frequency: 20.0,
	}
	cLoop, err := createControlLoop(ctx, logger, cfg, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cLoop, test.ShouldNotBeNil)
	cLoop.Start(ctx)
	for i := 200; i > 0; i-- {
		time.Sleep(65 * time.Millisecond)
		b, err := cLoop.OutputAt(ctx, "E")
		test.That(t, b[0].signal[0], test.ShouldEqual, 8.0)
		test.That(t, err, test.ShouldBeNil)
		b, err = cLoop.OutputAt(ctx, "B")
		test.That(t, b[0].signal[0], test.ShouldEqual, -3.0)
		test.That(t, err, test.ShouldBeNil)
	}
	cLoop.Stop(ctx)
}
