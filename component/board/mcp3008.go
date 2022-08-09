package board

import (
	"context"

	"go.uber.org/multierr"
)

// MCP3008AnalogReader implements a board.AnalogReader using an MCP3008 ADC via SPI.
type MCP3008AnalogReader struct {
	Channel int
	Bus     SPI
	Chip    string
}

func (mar *MCP3008AnalogReader) Read(ctx context.Context, extra map[string]interface{}) (value int, err error) {
	var tx [3]byte
	tx[0] = 1                            // start bit
	tx[1] = byte((8 + mar.Channel) << 4) // single-ended
	tx[2] = 0                            // extra clocks to receive full 10 bits of data

	bus, err := mar.Bus.OpenHandle()
	if err != nil {
		return 0, err
	}
	defer func() {
		err = multierr.Combine(err, bus.Close())
	}()

	rx, err := bus.Xfer(ctx, 1000000, mar.Chip, 0, tx[:])
	if err != nil {
		return 0, err
	}
	val := (int(rx[1]) << 8) | int(rx[2]) // reassemble 10 bit value

	return val, nil
}
