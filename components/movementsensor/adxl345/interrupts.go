package adxl345

// addresses relevant to interrupts
const (
	INT_ENABLE    byte = 0x2E
	INT_MAP            = 0x2F
	INT_SOURCE         = 0x30
	TAP_AXES           = 0x2A
	THRESH_TAP         = 0x1D
	DUR                = 0x21
	LATENT             = 0x22
	WINDOW             = 0x23
	THRESH_FF          = 0x28
	TIME_FF            = 0x29
	THRESH_ACT         = 0x24
	THRESH_INACT       = 0x25
	TIME_INACT         = 0x26
	ACT_INACT_CTL      = 0x27
)

var interruptRegisterNames = map[byte]string{
	INT_ENABLE:    "INT_ENABLE",
	INT_MAP:       "INT_MAP",
	INT_SOURCE:    "INT_SOURCE",
	TAP_AXES:      "TAP_AXES",
	THRESH_TAP:    "THRESH_TAP",
	DUR:           "DUR",
	LATENT:        "LATENT",
	WINDOW:        "WINDOW",
	THRESH_FF:     "THRESH_FF",
	TIME_FF:       "TIME_FF",
	THRESH_ACT:    "THRESH_ACT",
	THRESH_INACT:  "THRESH_INACT",
	TIME_INACT:    "TIME_INACT",
	ACT_INACT_CTL: "ACT_INACT_CTL",
}

// types of interrupts
const (
	DATA_READY string = "DATA_READY"
	SINGLE_TAP        = "SINGLE_TAP"
	DOUBLE_TAP        = "DOUBLE_TAP"
	Activity          = "Activity"
	Inactivity        = "Inactivity"
	FREE_FALL         = "FREE_FALL"
	WATERMARK         = "WATERMARK"
	OVERRUN           = "OVERRUN"
)

var interruptBitPosition = map[string]byte{
	DATA_READY: 1 << 7,
	SINGLE_TAP: 1 << 6,
	DOUBLE_TAP: 1 << 5,
	Activity:   1 << 4,
	Inactivity: 1 << 3,
	FREE_FALL:  1 << 2,
	WATERMARK:  1 << 1,
	OVERRUN:    1 << 0,
}

/*
From the data sheet:

In general, a good starting point is to set the DUR register to a value greater
than 0x10 (10 ms), the latent register to a value greater than0x10 (20 ms), the
window register to a value greater than 0x40(80 ms), and the THRESH_TAP register
to a value greater than 0x30 (3 g)
*/
var defaultRegisterValues = map[byte]byte{
	// Single Tap & Double Tap
	TAP_AXES:   0x07, //enables x, y, z
	THRESH_TAP: 0x30,
	DUR:        0x10,
	LATENT:     0x10,
	WINDOW:     0x40,
	// Free Fall
	THRESH_FF: 0x07, //0x05 - 0x09 are recommended
	TIME_FF:   0x20, // 0x14 - 0x46 are recommended
	// Activity
	THRESH_ACT:   0x80,
	THRESH_INACT: 0x8,
	// Inactivity
	TIME_INACT:    0x10,
	ACT_INACT_CTL: 0x77, // enables x, y, z for activity and inactivity
}

const (
	X byte = 1 << 0
	Y      = 1 << 1
	Z      = 1 << 2
)

var tapDirections []byte = []byte{X, Y, Z}
