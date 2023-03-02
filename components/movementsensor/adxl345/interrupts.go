package adxl345

// addresses relevant to interrupts.
const (
	DurAddr       byte = 0x21 // contains an unsigned time value representing the maximum time that an event must be above the THRESH_TAP threshold to qualify as a tap event [625 µs/LSB]
	IntEnableAddr byte = 0x2E // contains info on which interrupts have been enabled
	IntMapAddr    byte = 0x2F // contains info on which interrupt pin to send each interrupt
	IntSourceAddr byte = 0x30 // contains info on which interrupt has gone off since the last time this address has been read from
	LatentAddr    byte = 0x22 // contains an unsigned time value representing the wait time from the detection of a tap event to the start of the time window [1.25 ms/LSB]
	TapAxesAddr   byte = 0x2A // contains info on which axes have been turned on for taps (X, Y, Z are bits 2, 1, 0 respectivel)
	ThreshTapAddr byte = 0x1D // contains an unsigned threshold value for tap interrupts [62.5 mg/LSB ]
	WindowAddr    byte = 0x23 // contains an unsigned time value representing the amount of time after the expiration of the latency time (determined by the latent register) during which a second valid tap can begin [ 1.25 ms/LSB]
	ThreshFfAddr  byte = 0x28
	TimeFfAddr    byte = 0x29
)

// types of interrupts.
const (
	SingleTap string = "SINGLE_TAP"
	FreeFall  string = "FREE_FALL"
)

var interruptBitPosition = map[string]byte{
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
	WindowAddr:    0x40,

	// Free Fall
	TimeFfAddr:   0x20, // 0x14 - 0x46 are recommended
	ThreshFfAddr: 0x07, // 0x05 - 0x09 are recp,,emded
}

const (
	xBit byte = 1 << 0
	yBit      = 1 << 1
	zBit      = 1 << 2
)

func getAxes(excludeX, excludeY, excludeZ bool) byte {
	var tapAxes byte
	if !excludeX {
		tapAxes += xBit
	}
	if !excludeY {
		tapAxes += yBit
	}
	if !excludeZ {
		tapAxes += zBit
	}
	return tapAxes
}
