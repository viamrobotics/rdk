package main

import (
	"context"
	"time"
	"math"

	. "go.viam.com/core/board"
	_ "go.viam.com/core/board/detector"

)

var SH110X_BLACK = 0   ///< Draw 'off' pixels
var SH110X_WHITE = 1   ///< Draw 'on' pixels
var SH110X_INVERSE = 2 ///< Invert pixels

var SH110X_MEMORYMODE byte = 0x20          ///< See datasheet
var SH110X_COLUMNADDR byte = 0x21          ///< See datasheet
var SH110X_PAGEADDR byte = 0x22            ///< See datasheet
var SH110X_SETCONTRAST byte = 0x81         ///< See datasheet
var SH110X_CHARGEPUMP byte = 0x8D          ///< See datasheet
var SH110X_SEGREMAP byte = 0xA0            ///< See datasheet
var SH110X_DISPLAYALLON_RESUME byte = 0xA4 ///< See datasheet
var SH110X_DISPLAYALLON byte = 0xA5        ///< Not currently used
var SH110X_NORMALDISPLAY byte = 0xA6       ///< See datasheet
var SH110X_INVERTDISPLAY byte = 0xA7       ///< See datasheet
var SH110X_SETMULTIPLEX byte = 0xA8        ///< See datasheet
var SH110X_DCDC byte = 0xAD                ///< See datasheet
var SH110X_DISPLAYOFF byte = 0xAE          ///< See datasheet
var SH110X_DISPLAYON byte = 0xAF           ///< See datasheet
var SH110X_SETPAGEADDR byte = 0xB0 ///< Specify page address to load display RAM data to page address
var SH110X_COMSCANINC byte = 0xC0         ///< Not currently used
var SH110X_COMSCANDEC byte = 0xC8         ///< See datasheet
var SH110X_SETDISPLAYOFFSET byte = 0xD3   ///< See datasheet
var SH110X_SETDISPLAYCLOCKDIV byte = 0xD5 ///< See datasheet
var SH110X_SETPRECHARGE byte = 0xD9       ///< See datasheet
var SH110X_SETCOMPINS byte = 0xDA         ///< See datasheet
var SH110X_SETVCOMDETECT byte = 0xDB      ///< See datasheet
var SH110X_SETDISPSTARTLINE byte = 0xDC ///< Specify Column address to determine the initial display line or < COM0.

var SH110X_SETLOWCOLUMN byte = 0x00  ///< Not currently used
var SH110X_SETHIGHCOLUMN byte = 0x10 ///< Not currently used
var SH110X_SETSTARTLINE byte = 0x40  ///< See datasheet

func initDisp(ctx context.Context, handle I2CHandle){
	// set contrast
	contrast := []byte{0, 0x81, 0x2F}
	handle.WriteBytes(ctx, dispAddr, contrast)
	
	init := []byte{
		0x00,
		SH110X_DISPLAYOFF,               // 0xAE
		SH110X_SETDISPLAYCLOCKDIV, 0x51, // 0xd5, 0x51,
		SH110X_MEMORYMODE,               // 0x20
		SH110X_SETCONTRAST, 0x4F,        // 0x81, 0x4F
		SH110X_DCDC, 0x8A,               // 0xAD, 0x8A
		SH110X_SEGREMAP,                 // 0xA0
		SH110X_COMSCANINC,               // 0xC0
		SH110X_SETDISPSTARTLINE, 0x0,    // 0xDC 0x00
		SH110X_SETDISPLAYOFFSET, 0x60,   // 0xd3, 0x60,
		SH110X_SETPRECHARGE, 0x22,       // 0xd9, 0x22,
		SH110X_SETVCOMDETECT, 0x35,      // 0xdb, 0x35,
		SH110X_SETMULTIPLEX, 0x3F,       // 0xa8, 0x3f,
		SH110X_DISPLAYALLON_RESUME, // 0xa4
		SH110X_NORMALDISPLAY,       // 0xa6
	}
	
	handle.WriteBytes(ctx, dispAddr, init)
	
	time.Sleep(100 * time.Millisecond)
	
	// turn on
	handle.WriteBytes(ctx, dispAddr, []byte{0x00, 0xAF})
}

func checkInit(ctx context.Context, handle I2CHandle){
	buffer := []byte{}
	buffer, _ = handle.ReadBytes(ctx, dispAddr, 1)
	if buffer[0] == 71 {
		initDisp(ctx, handle)
	}
}

func writeAlt(ctx context.Context, feet, meters string, handle I2CHandle){
	buf := blank()
	
	// account for special messages
	if meters != "error" && meters != "lock" {
		if len(feet) < 5 {
			feet += " "
		}
		feet += "f"
		meters += " m"
	}
	buf = writeString(0, 28, feet, buf)
	buf = writeString(0, 58, meters, buf)
	writeBuf(ctx, buf, dispAddr, handle)
}

func blank() []byte {
	return make([]byte, 1024)
}

// This actually writes the buffered bytes to the display
func writeBuf(ctx context.Context, buf []byte, dispAddr byte, handle I2CHandle){
	
	checkInit(ctx, handle)
	
	var reg byte
	iter := 0
	for reg = 0xB0; reg <= 0xBF; reg++{
		someBytes := []byte{0, reg, 0x10, 0}
		handle.WriteBytes(context.Background(), dispAddr, someBytes)
		
		someBytes = append([]byte{0x40}, buf[0 + iter * 64:31 + iter * 64]...)
		handle.WriteBytes(context.Background(), dispAddr, someBytes)
		someBytes = append([]byte{0x40}, buf[31 + iter * 64:62 + iter * 64]...)
		handle.WriteBytes(context.Background(), dispAddr, someBytes)
		
		someBytes = []byte{0x40, buf[62 + iter * 64], buf[63 + iter * 64]}
		handle.WriteBytes(context.Background(), dispAddr, someBytes)
		
		iter++
	}
}

func writePixel(x, y int, buf []byte) []byte {
	x, y = y, x
	
	WIDTH := 64
	x = WIDTH - x
	
	buf[x + (y / 8) * WIDTH] |= (1 << (y & 7))
	return buf
}

// Write a line.  Bresenham's algorithm
func writeLine(x0, y0, x1, y1 int, buf []byte) []byte {
	steep := math.Abs(float64(y1 - y0)) > math.Abs(float64(x1 - x0))
	if (steep) {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}

	if (x0 > x1) {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}

	dx := x1 - x0
	dy := y1 - y0
	if dy < 0{
		dy *= -1
	}

	err := dx / 2;
	ystep := -1

	if (y0 < y1) {
		ystep = 1;
	}

	for (x0 <= x1) {
		if (steep) {
			buf = writePixel(y0, x0, buf)
		} else {
			buf = writePixel(x0, y0, buf)
		}
		err -= dy
		if (err < 0) {
			y0 += ystep
			err += dx
		}
		x0++
	}
	return buf
}

func writeFillRect(x, y, w, h int, buf []byte) []byte {
	for i := x; i < x + w; i++ {
		buf = writeLine(i, y, i, y + h, buf);
	}
	return buf
}

func writeString(x, y int, char string, buf []byte) []byte {
	
	charBytes := []byte(char)
	
	for _, cb := range charBytes{
		charIdx := cb - 0x20
		if charIdx < 0 || charIdx >=95 {
			continue
		}
		cInfo := chars[charIdx]
		// byte offset
		bo := cInfo[0]
		w := cInfo[1]
		h := cInfo[2]
		adv := cInfo[3]
		xo := cInfo[4]
		yo := cInfo[5]
		
		var bit byte
		var bits byte
		
		for yy := 0; yy < h; yy++ {
			for xx := 0; xx < w; xx++ {
				if bit & 7 == 0 {
					bits = freemono[bo]
					bo++
				}
				bit++
				if (bits & 0x80) > 0 {
					buf = writePixel(x + xo + xx, y + yo + yy, buf);
				}
				bits <<= 1;
			}
		}
		x += adv
	}
	return buf
}
