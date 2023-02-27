package adxl345

const (
	INT_ENABLE byte = 0x2E
	INT_MAP         = 0x2F
	INT_SOURCE      = 0x30
	TAP_AXES        = 0x2A
	THRESH_TAP      = 0x1D
	DUR             = 0x21
	LATENT          = 0x22
	WINDOW          = 0x23
)

var interruptRegisters []byte = []byte{INT_ENABLE, INT_MAP, INT_SOURCE, TAP_AXES, THRESH_TAP, DUR, LATENT, WINDOW}

var interruptRegisterNames = map[byte]string{
	INT_ENABLE: "INT_ENABLE",
	INT_MAP:    "INT_MAP",
	INT_SOURCE: "INT_SOURCE",
	TAP_AXES:   "TAP_AXES",
	THRESH_TAP: "THRESH_TAP",
	DUR:        "DUR",
	LATENT:     "LATENT",
	WINDOW:     "WINDOW",
}

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

var interruptTypes []string = []string{DATA_READY, SINGLE_TAP, DOUBLE_TAP, Activity, Inactivity, FREE_FALL, WATERMARK, OVERRUN}

var interruptBitMap = map[string]byte{
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
var defaultTapRelatedRegisterValues = map[byte]byte{
	TAP_AXES:   0x07, //enables x, y, z respectively
	THRESH_TAP: 0x30,
	DUR:        0x10,
	LATENT:     0x10,
	WINDOW:     0x40,
}
