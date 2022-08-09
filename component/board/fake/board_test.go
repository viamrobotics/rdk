package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rlog"
)

func TestFakeBoard(t *testing.T) {
	boardConfig := board.Config{
		I2Cs: []board.I2CConfig{
			{Name: "main", Bus: "0"},
		},
		SPIs: []board.SPIConfig{
			{Name: "aux", BusSelect: "1"},
		},
		Analogs: []board.AnalogConfig{
			{Name: "blue", Pin: "0"},
		},
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

	_, ok := b.I2CByName("main")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = b.SPIByName("aux")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = b.AnalogReaderByName("blue")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = b.DigitalInterruptByName("i1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = b.DigitalInterruptByName("i2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = b.DigitalInterruptByName("hall-a")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = b.DigitalInterruptByName("hall-b")
	test.That(t, ok, test.ShouldBeTrue)

	status, err := b.Status(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, int(status.Analogs["blue"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["i1"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["i2"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["hall-a"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["hall-b"].Value), test.ShouldEqual, 0)
}
