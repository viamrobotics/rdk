package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestFakeBoard(t *testing.T) {
	logger := logging.NewTestLogger(t)
	boardConfig := Config{
		AnalogReaders: []board.AnalogReaderConfig{
			{Name: "blue", Channel: analogTestPin},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "35"},
			{Name: "i2", Pin: "31"},
			{Name: "a", Pin: "38"},
			{Name: "b", Pin: "40"},
		},
	}

	cfg := resource.Config{Name: "board1", ConvertedAttributes: &boardConfig}
	b, err := NewBoard(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = b.AnalogByName("blue")
	test.That(t, err, test.ShouldBeNil)

	_, err = b.DigitalInterruptByName("i1")
	test.That(t, err, test.ShouldBeNil)
	_, err = b.DigitalInterruptByName("i2")
	test.That(t, err, test.ShouldBeNil)
	_, err = b.DigitalInterruptByName("a")
	test.That(t, err, test.ShouldBeNil)
	_, err = b.DigitalInterruptByName("b")
	test.That(t, err, test.ShouldBeNil)
}

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	validConfig.AnalogReaders = []board.AnalogReaderConfig{{}}
	_, err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")

	validConfig.AnalogReaders = []board.AnalogReaderConfig{{Name: "bar"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "pin")

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar", Pin: "3"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}
