//go:build linux

// Package adxl345 is for an ADXL345 accelerometer. This file is for the interrupt-based
// functionality on the chip.
package adxl345

// addresses relevant to interrupts.
const (
	// an unsigned time value representing the maximum time that an event must be above the
	// THRESH_TAP threshold to qualify as a tap event [625 µs/LSB].
	DurAddr byte = 0x21
	// info on which interrupts have been enabled.
	IntEnableAddr byte = 0x2E
	// info on which interrupt pin to send each interrupt.
	IntMapAddr byte = 0x2F
	// info on which interrupt has gone off since the last time this address has been read from.
	IntSourceAddr byte = 0x30
	// an unsigned time value representing the wait time from the detection of a tap event to the
	// start of the time window [1.25 ms/LSB].
	LatentAddr byte = 0x22
	// info on which axes have been turned on for taps (X, Y, Z are bits 2, 1, 0 respectively).
	TapAxesAddr byte = 0x2A
	// an unsigned threshold value for tap interrupts [62.5 mg/LSB ].
	ThreshTapAddr byte = 0x1D
	// che threshold value, in unsigned format, for free-fall detection.
	ThreshFfAddr byte = 0x28
	// the  minimum time that the value of all axes must be less than THRESH_FF to generate a free-fall interrupt.
	TimeFfAddr byte = 0x29
)

// InterruptID is a type of interrupts available on ADXL345.
type InterruptID = uint8

const (
	// SingleTap is a key value used to find various needs associated with this interrupt.
	SingleTap InterruptID = iota
	// FreeFall is a key value used to find various needs associated with this interrupt.
	FreeFall InterruptID = iota
)

var interruptBitPosition = map[InterruptID]byte{
	SingleTap: 1 << 6,
	FreeFall:  1 << 2,
}

/*
From the data sheet:

	In general, a good starting point is to set the Dur register to a value greater
	than 0x10 (10 ms), the Latent register to a value greater than 0x10 (20 ms), the
	Window register to a value greater than 0x40 (80 ms), and the ThreshTap register
	to a value greater than 0x30 (3 g).
*/
var defaultRegisterValues = map[byte]byte{
	// Interrupt Enabled
	IntEnableAddr: 0x00,
	IntMapAddr:    0x00,

	// Single Tap & Double Tap
	TapAxesAddr:   0x07,
	ThreshTapAddr: 0x30,
	DurAddr:       0x10,
	LatentAddr:    0x10,

	// Free Fall
	TimeFfAddr:   0x20, // 0x14 - 0x46 are recommended
	ThreshFfAddr: 0x07, // 0x05 - 0x09 are recommended
}

const (
	// ThreshTapScaleFactor is the scale factor for THRESH_TAP register.
	ThreshTapScaleFactor float32 = 62.5
	// DurScaleFactor is the scale factor for DUR register.
	DurScaleFactor float32 = 625
	// TimeFfScaleFactor is the scale factor for TIME_FF register.
	TimeFfScaleFactor float32 = .5
	// ThreshFfScaleFactor is the scale factor for THRESH_FF register.
	ThreshFfScaleFactor float32 = 62.5
)

const (
	xBit byte = 1 << 0
	yBit      = 1 << 1
	zBit      = 1 << 2
)

func getAxes(excludeX, excludeY, excludeZ bool) byte {
	var tapAxes byte
	if !excludeX {
		tapAxes |= xBit
	}
	if !excludeY {
		tapAxes |= yBit
	}
	if !excludeZ {
		tapAxes |= zBit
	}
	return tapAxes
}
