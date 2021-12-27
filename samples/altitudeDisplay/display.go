package main

import (
	"context"
	"math"
	"time"

	"go.viam.com/rdk/component/board"
)

/*	Values from the original arduino C library that I did not use, if you need them
 * 	sh110xBLACK                   = 0    ///< Draw 'off' pixels
	sh110xWHITE                   = 1    ///< Draw 'on' pixels
	sh110xINVERSE                 = 2    ///< Invert pixels
	sh110xCOLUMNADDR         byte = 0x21 ///< See datasheet
	sh110xPAGEADDR           byte = 0x22 ///< See datasheet
	sh110xCHARGEPUMP         byte = 0x8D ///< See datasheet
	sh110xDISPLAYALLON       byte = 0xA5 ///< Not currently used
	sh110xINVERTDISPLAY      byte = 0xA7 ///< See datasheet
	sh110xDISPLAYON          byte = 0xAF ///< See datasheet
	sh110xSETPAGEADDR        byte = 0xB0 ///< Specify page address to load display RAM data to page address
	sh110xCOMSCANDEC         byte = 0xC8 ///< See datasheet
	sh110xSETCOMPINS         byte = 0xDA ///< See datasheet
	sh110xSETLOWCOLUMN       byte = 0x00 ///< Not currently used
	sh110xSETHIGHCOLUMN      byte = 0x10 ///< Not currently used
	sh110xSETSTARTLINE       byte = 0x40 ///< See datasheet
*/
const (
	sh110xMEMORYMODE         byte = 0x20 ///< See datasheet
	sh110xSETCONTRAST        byte = 0x81 ///< See datasheet
	sh110xSEGREMAP           byte = 0xA0 ///< See datasheet
	sh110xDISPLAYALLONRESUME byte = 0xA4 ///< See datasheet
	sh110xNORMALDISPLAY      byte = 0xA6 ///< See datasheet
	sh110xSETMULTIPLEX       byte = 0xA8 ///< See datasheet
	sh110xDCDC               byte = 0xAD ///< See datasheet
	sh110xDISPLAYOFF         byte = 0xAE ///< See datasheet
	sh110xCOMSCANINC         byte = 0xC0 ///< Not currently used
	sh110xSETDISPLAYOFFSET   byte = 0xD3 ///< See datasheet
	sh110xSETDISPLAYCLOCKDIV byte = 0xD5 ///< See datasheet
	sh110xSETPRECHARGE       byte = 0xD9 ///< See datasheet
	sh110xSETVCOMDETECT      byte = 0xDB ///< See datasheet
	sh110xSETDISPSTARTLINE   byte = 0xDC ///< Specify Column address to determine the initial display line or < COM0.
)

func initDisp(ctx context.Context, handle board.I2CHandle, startup bool) {
	init := []byte{
		0x00,
		sh110xDISPLAYOFF,               // 0xAE
		sh110xSETDISPLAYCLOCKDIV, 0x51, // 0xd5, 0x51,
		sh110xMEMORYMODE,        // 0x20
		sh110xSETCONTRAST, 0x4F, // 0x81, 0x4F
		sh110xDCDC, 0x8A, // 0xAD, 0x8A
		sh110xSEGREMAP,              // 0xA0
		sh110xCOMSCANINC,            // 0xC0
		sh110xSETDISPSTARTLINE, 0x0, // 0xDC 0x00
		sh110xSETDISPLAYOFFSET, 0x60, // 0xd3, 0x60,
		sh110xSETPRECHARGE, 0x22, // 0xd9, 0x22,
		sh110xSETVCOMDETECT, 0x35, // 0xdb, 0x35,
		sh110xSETMULTIPLEX, 0x3F, // 0xa8, 0x3f,
		sh110xDISPLAYALLONRESUME, // 0xa4
		sh110xNORMALDISPLAY,      // 0xa6
	}

	handle.Write(ctx, init)

	time.Sleep(100 * time.Millisecond)

	// turn on
	handle.Write(ctx, []byte{0x00, 0xAF})
	if startup {
		initAnimation(ctx, handle)
	}
}

func checkInit(ctx context.Context, handle board.I2CHandle) {
	buffer, _ := handle.Read(ctx, 1)
	if buffer[0] == 71 {
		initDisp(ctx, handle, false)
	}
}

func initAnimation(ctx context.Context, handle board.I2CHandle) {
	for i := 1; i < 15; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		buf := blank()
		buf = writeFillRect(0, 20, i*8, 24, buf)
		writeBuf(ctx, buf, handle)
	}
}

func writeAlt(ctx context.Context, feet, meters string, handle board.I2CHandle) {
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
	writeBuf(ctx, buf, handle)
}

func blank() []byte {
	return make([]byte, 1024)
}

// This actually writes the buffered bytes to the display.
func writeBuf(ctx context.Context, buf []byte, handle board.I2CHandle) {
	checkInit(ctx, handle)

	var reg byte
	iter := 0
	for reg = 0xB0; reg <= 0xBF; reg++ {
		someBytes := []byte{0, reg, 0x10, 0}
		handle.Write(ctx, someBytes)

		someBytes = append([]byte{0x40}, buf[0+iter*64:31+iter*64]...)
		handle.Write(ctx, someBytes)
		someBytes = append([]byte{0x40}, buf[31+iter*64:62+iter*64]...)
		handle.Write(ctx, someBytes)

		someBytes = []byte{0x40, buf[62+iter*64], buf[63+iter*64]}
		handle.Write(ctx, someBytes)

		iter++
	}
}

func writePixel(x, y int, buf []byte) []byte {
	x, y = y, x

	WIDTH := 64
	x = WIDTH - x

	buf[x+(y/8)*WIDTH] |= (1 << (y & 7))
	return buf
}

// Write a line.  Bresenham's algorithm.
func writeLine(x0, y0, x1, y1 int, buf []byte) []byte {
	steep := math.Abs(float64(y1-y0)) > math.Abs(float64(x1-x0))
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}

	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}

	dx := x1 - x0
	dy := y1 - y0
	if dy < 0 {
		dy *= -1
	}

	err := dx / 2
	ystep := -1

	if y0 < y1 {
		ystep = 1
	}

	for x0 <= x1 {
		if steep {
			buf = writePixel(y0, x0, buf)
		} else {
			buf = writePixel(x0, y0, buf)
		}
		err -= dy
		if err < 0 {
			y0 += ystep
			err += dx
		}
		x0++
	}
	return buf
}

func writeFillRect(x, y, w, h int, buf []byte) []byte {
	for i := x; i < x+w; i++ {
		buf = writeLine(i, y, i, y+h, buf)
	}
	return buf
}

func writeString(x, y int, char string, buf []byte) []byte {
	charBytes := []byte(char)

	for _, cb := range charBytes {
		charIdx := cb - 0x20
		if cb < 0x20 || charIdx >= 95 {
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
				if bit&7 == 0 {
					bits = freemono[bo]
					bo++
				}
				bit++
				if (bits & 0x80) > 0 {
					buf = writePixel(x+xo+xx, y+yo+yy, buf)
				}
				bits <<= 1
			}
		}
		x += adv
	}
	return buf
}
