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
				Type: "Derivative",
				Attribute: config.AttributeMap{
					"DeriveType": "Backward1st1",
				},
				DependsOn: []string{"A"},
			},
			{
				Name: "C",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "-+",
				},
				DependsOn: []string{"B", "D"},
			},
			{
				Name:      "D",
				Type:      "TrapezoidalVelocityProfile",
				DependsOn: []string{},
				Attribute: config.AttributeMap{
					"MaxAcc": 1000.0,
					"MaxVel": 100.0,
				},
			},
			{
				Name:      "E",
				Attribute: config.AttributeMap{"Kd": 0.11, "Kp": 0.12, "Ki": 0.22},
				Type:      "PID",
				DependsOn: []string{"C"},
			},
		},
		Frequency: 1.0,
	}
	cLoop, err := createControlLoop(ctx, logger, cfg, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cLoop, test.ShouldNotBeNil)
	cLoop.Start(ctx)
	time.Sleep(1200 * time.Millisecond)
	cLoop.Stop(ctx)
}
