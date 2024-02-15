//go:build linux

// Package adxl345 is for an ADXL345 accelerometer. This file is for the interrupt-based
// functionality on the chip.
package adxl345

// addresses relevant to interrupts.
const (
	// an unsigned time value representing the maximum time that an event must be above the
	// THRESH_TAP threshold to qualify as a tap event [625 µs/LSB].
	durAddr byte = 0x21
	// info on which interrupts have been enabled.
	intEnableAddr byte = 0x2E
	// info on which interrupt pin to send each interrupt.
	intMapAddr byte = 0x2F
	// info on which interrupt has gone off since the last time this address has been read from.
	intSourceAddr byte = 0x30
	// an unsigned time value representing the wait time from the detection of a tap event to the
	// start of the time window [1.25 ms/LSB].
	latentAddr byte = 0x22
	// info on which axes have been turned on for taps (X, Y, Z are bits 2, 1, 0 respectively).
	tapAxesAddr byte = 0x2A
	// an unsigned threshold value for tap interrupts [62.5 mg/LSB ].
	threshTapAddr byte = 0x1D
	// che threshold value, in unsigned format, for free-fall detection.
	threshFfAddr byte = 0x28
	// the  minimum time that the value of all axes must be less than THRESH_FF to generate a free-fall interrupt.
	timeFfAddr byte = 0x29
)

// InterruptID is a type of interrupts available on ADXL345.
type InterruptID = uint8

const (
	// singleTap is a key value used to find various needs associated with this interrupt.
	singleTap InterruptID = iota
	// freeFall is a key value used to find various needs associated with this interrupt.
	freeFall InterruptID = iota
)

var interruptBitPosition = map[InterruptID]byte{
	singleTap: 1 << 6,
	freeFall:  1 << 2,
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
	intEnableAddr: 0x00,
	intMapAddr:    0x00,

	// Single Tap & Double Tap
	tapAxesAddr:   0x07,
	threshTapAddr: 0x30,
	durAddr:       0x10,
	latentAddr:    0x10,

	// Free Fall
	timeFfAddr:   0x20, // 0x14 - 0x46 are recommended
	threshFfAddr: 0x07, // 0x05 - 0x09 are recommended
}

const (
	// threshTapScaleFactor is the scale factor for THRESH_TAP register.
	threshTapScaleFactor float32 = 62.5
	// durScaleFactor is the scale factor for DUR register.
	durScaleFactor float32 = 625
	// timeFfScaleFactor is the scale factor for TIME_FF register.
	timeFfScaleFactor float32 = .5
	// threshFfScaleFactor is the scale factor for THRESH_FF register.
	threshFfScaleFactor float32 = 62.5
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
