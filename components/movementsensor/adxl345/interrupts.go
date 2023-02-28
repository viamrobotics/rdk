package adxl345

// addresses relevant to interrupts.
const (
	IntEnable   byte = 0x2E
	IntMap      byte = 0x2F
	IntSource   byte = 0x30
	TapAxes     byte = 0x2A
	ThreshTap   byte = 0x1D
	Dur         byte = 0x21
	Latent      byte = 0x22
	Window      byte = 0x23
	ThreshFf    byte = 0x28
	TimeFf      byte = 0x29
	ThreshAct   byte = 0x24
	ThreshInact byte = 0x25
	TimeInact   byte = 0x26
	ActInactCtl byte = 0x27
)

// types of interrupts.
const (
	DataReady  string = "DATA_READY"
	SingleTap  string = "SINGLE_TAP"
	DoubleTap  string = "DOUBLE_TAP"
	Activity   string = "Activity"
	Inactivity string = "Inactivity"
	Freefall   string = "FREE_FALL"
	Watermark  string = "WATERMARK"
	Overrun    string = "OVERRUN"
)

var interruptBitPosition = map[string]byte{
	DataReady:  1 << 7,
	SingleTap:  1 << 6,
	DoubleTap:  1 << 5,
	Activity:   1 << 4,
	Inactivity: 1 << 3,
	Freefall:   1 << 2,
	Watermark:  1 << 1,
	Overrun:    1 << 0,
}

/*
From the data sheet:

In general, a good starting point is to set the Dur register to a value greater
than 0x10 (10 ms), the Latent register to a value greater than0x10 (20 ms), the
Window register to a value greater than 0x40(80 ms), and the ThreshTap register
to a value greater than 0x30 (3 g).
*/
var defaultRegisterValues = map[byte]byte{
	// Single Tap & Double Tap

	ThreshTap: 0x30,
	Dur:       0x10,
	Latent:    0x10,
	Window:    0x40,
	// Free Fall

	TimeFf: 0x20, // 0x14 - 0x46 are recommended
	// Activity
	ThreshAct:   0x80,
	ThreshInact: 0x8,
	// Inactivity
	TimeInact:   0x10,
	ActInactCtl: 0x77, // enables x, y, z for activity and inactivity
}

const (
	xBit byte = 1 << 0
	yBit      = 1 << 1
	zBit      = 1 << 2
)
