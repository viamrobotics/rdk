package board

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/robotcore/rlog"
)

func TestFakeRegistry(t *testing.T) {
	b, err := NewBoard(context.Background(), Config{Model: "fake"}, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	_, ok := b.(*FakeBoard)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestFakeBoard(t *testing.T) {
	cfg := Config{
		Analogs: []AnalogConfig{{Name: "blue", Pin: "0"}},
		Servos: []ServoConfig{
			{Name: "s1", Pin: "16"},
			{Name: "s2", Pin: "29"},
		},
		DigitalInterrupts: []DigitalInterruptConfig{
			{Name: "i1", Pin: "35"},
			{Name: "i2", Pin: "31", Type: "servo"},
			{Name: "hall-a", Pin: "38"},
			{Name: "hall-b", Pin: "40"},
		},
		Motors: []MotorConfig{
			{
				Name:             "m",
				Pins:             map[string]string{"a": "11", "b": "13", "pwm": "15"},
				Encoder:          "hall-a",
				EncoderB:         "hall-b",
				TicksPerRotation: 100,
			},
		},
	}

	b, err := NewFakeBoard(context.Background(), cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, b.Servo("s1").Move(context.Background(), 15), test.ShouldBeNil)

	status, err := b.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, int(status.Servos["s1"].Angle), test.ShouldEqual, 15)

}
