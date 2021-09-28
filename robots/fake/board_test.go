package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/rlog"
)

func TestFakeBoard(t *testing.T) {
	boardConfig := board.Config{
		Analogs: []board.AnalogConfig{{Name: "blue", Pin: "0"}},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "35"},
			{Name: "i2", Pin: "31", Type: "servo"},
			{Name: "hall-a", Pin: "38"},
			{Name: "hall-b", Pin: "40"},
		},
	}

	cfg := config.Component{Name: "board1", ConvertedAttributes: &boardConfig}
	b, err := NewBoard(context.Background(), cfg, rlog.Logger)
	test.That(t, err, test.ShouldBeNil)

	status, err := b.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, int(status.Analogs["blue"].Value), test.ShouldEqual, 0)

}
