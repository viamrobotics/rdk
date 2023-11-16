//go:build linux

// These tests will only run on Linux! Viam's automated build system on Github uses Linux, though,
// so they should run on every PR. We made the tests Linux-only because this entire package is
// Linux-only, and building non-Linux support solely for the test meant that the code tested might
// not be the production code.
package genericlinux

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/logging"
)

func TestGenericLinux(t *testing.T) {
	ctx := context.Background()

	b := &Board{
		logger: logging.NewTestLogger(t),
	}

	t.Run("test empty sysfs board", func(t *testing.T) {
		_, err := b.GPIOPinByName("10")
		test.That(t, err, test.ShouldNotBeNil)
	})

	boardSPIs := map[string]*spiBus{
		"closed": {
			openHandle: &spiHandle{bus: &spiBus{}, isClosed: true},
		},
		"open": {
			openHandle: &spiHandle{bus: &spiBus{}, isClosed: false},
		},
	}
	oneStr := "1"
	twoStr := "1"
	boardSPIs["closed"].bus.Store(&oneStr)
	boardSPIs["closed"].openHandle.bus.bus.Store(&oneStr)
	boardSPIs["open"].bus.Store(&twoStr)
	boardSPIs["open"].openHandle.bus.bus.Store(&twoStr)

	b = &Board{
		Named:         board.Named("foo").AsNamed(),
		gpioMappings:  nil,
		spis:          boardSPIs,
		analogReaders: map[string]*wrappedAnalogReader{"an": {}},
		logger:        logging.NewTestLogger(t),
		cancelCtx:     ctx,
		cancelFunc: func() {
		},
	}

	t.Run("test analog-readers spis i2cs digital-interrupts and gpio names", func(t *testing.T) {
		ans := b.AnalogReaderNames()
		test.That(t, ans, test.ShouldResemble, []string{"an"})

		an1, ok := b.AnalogReaderByName("an")
		test.That(t, an1, test.ShouldHaveSameTypeAs, &wrappedAnalogReader{})
		test.That(t, ok, test.ShouldBeTrue)

		an2, ok := b.AnalogReaderByName("missing")
		test.That(t, an2, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		dns := b.DigitalInterruptNames()
		test.That(t, dns, test.ShouldBeNil)

		dn1, ok := b.DigitalInterruptByName("dn")
		test.That(t, dn1, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		gn1, err := b.GPIOPinByName("10")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, gn1, test.ShouldBeNil)
	})

	t.Run("test spi functionality", func(t *testing.T) {
		spi1 := b.spis["closed"]
		spi2 := b.spis["open"]
		sph1, err := spi1.OpenHandle()
		test.That(t, sph1, test.ShouldHaveSameTypeAs, &spiHandle{})
		test.That(t, err, test.ShouldBeNil)
		sph2, err := spi2.OpenHandle()
		test.That(t, sph2, test.ShouldHaveSameTypeAs, &spiHandle{})
		test.That(t, err, test.ShouldBeNil)

		err = sph2.Close()
		test.That(t, err, test.ShouldBeNil)
		rx, err := sph2.Xfer(ctx, 1, "1", 1, []byte{})
		test.That(t, err.Error(), test.ShouldContainSubstring, "closed")
		test.That(t, rx, test.ShouldBeNil)
	})
}

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	validConfig.AnalogReaders = []mcp3008helper.MCP3008AnalogConfig{{}}
	_, err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.AnalogReaders = []mcp3008helper.MCP3008AnalogConfig{{Name: "bar"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"pin" is required`)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "bar", Pin: "3"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}
