package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/config"
)

func TestFakeBoard(t *testing.T) {
	logger := golog.NewTestLogger(t)
	boardConfig := Config{
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
			{Name: "a", Pin: "38"},
			{Name: "b", Pin: "40"},
		},
	}

	cfg := config.Component{Name: "board1", ConvertedAttributes: &boardConfig}
	b, err := NewBoard(context.Background(), cfg, logger)
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
	_, ok = b.DigitalInterruptByName("a")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = b.DigitalInterruptByName("b")
	test.That(t, ok, test.ShouldBeTrue)

	status, err := b.Status(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, int(status.Analogs["blue"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["i1"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["i2"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["a"].Value), test.ShouldEqual, 0)
	test.That(t, int(status.DigitalInterrupts["b"].Value), test.ShouldEqual, 0)
}

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	validConfig.Analogs = []board.AnalogConfig{{}}
	err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Analogs = []board.AnalogConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar"}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"pin" is required`)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar", Pin: "3"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}
