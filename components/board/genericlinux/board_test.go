//go:build linux

// These tests will only run on Linux! Viam's automated build system on Github uses Linux, though,
// so they should run on every PR. We made the tests Linux-only because this entire package is
// Linux-only, and building non-Linux support solely for the test meant that the code tested might
// not be the production code.
package genericlinux

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
)

func TestGenericLinux(t *testing.T) {
	ctx := context.Background()

	b := &Board{
		logger: golog.NewTestLogger(t),
	}

	t.Run("test empty sysfs board", func(t *testing.T) {
		test.That(t, b.GPIOPinNames(), test.ShouldBeNil)
		test.That(t, b.SPINames(), test.ShouldBeNil)
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
		analogReaders: map[string]*wrappedAnalogReaders{"an": {}},
		logger:        golog.NewTestLogger(t),
		cancelCtx:     ctx,
		cancelFunc: func() {
		},
	}

	t.Run("test analog-readers spis i2cs digital-interrupts and gpio names", func(t *testing.T) {
		ans := b.AnalogReaderNames()
		test.That(t, ans, test.ShouldResemble, []string{"an"})

		an1, ok := b.AnalogReaderByName("an")
		test.That(t, an1, test.ShouldHaveSameTypeAs, &wrappedAnalogReaders{})
		test.That(t, ok, test.ShouldBeTrue)

		an2, ok := b.AnalogReaderByName("missing")
		test.That(t, an2, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		sns := b.SPINames()
		test.That(t, len(sns), test.ShouldEqual, 2)

		sn1, ok := b.SPIByName("closed")
		test.That(t, sn1, test.ShouldHaveSameTypeAs, &spiBus{})
		test.That(t, ok, test.ShouldBeTrue)

		sn2, ok := b.SPIByName("missing")
		test.That(t, sn2, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		ins := b.I2CNames()
		test.That(t, ins, test.ShouldBeNil)

		in1, ok := b.I2CByName("in")
		test.That(t, in1, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		dns := b.DigitalInterruptNames()
		test.That(t, dns, test.ShouldBeNil)

		dn1, ok := b.DigitalInterruptByName("dn")
		test.That(t, dn1, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeFalse)

		gns := b.GPIOPinNames()
		test.That(t, gns, test.ShouldResemble, []string(nil))

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

	validConfig.AnalogReaders = []board.MCP3008AnalogConfig{{}}
	_, err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.AnalogReaders = []board.MCP3008AnalogConfig{{Name: "bar"}}
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
