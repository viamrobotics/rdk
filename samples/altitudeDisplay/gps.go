package main

import (
	"context"
	"errors"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/board"

	"github.com/adrianmo/go-nmea"
)

// GpsAlt asks the gps to get a lock, and then returns the first good altitude it gets
func GpsAlt(ctx context.Context, handle board.I2CHandle) (float64, error) {
	var err error

	cmd314 := addChk([]byte("PMTK314,0,1,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

	handle.Write(context.Background(), gpsAddr, cmd314)
	handle.Write(context.Background(), gpsAddr, cmd220)

	strBuf := ""

	var alt float64
	clearBuffer(ctx, handle)

	for i := 0; i <= 20; i++ {
		select {
		case <-ctx.Done():
			return 0, nil
		default:
		}
		buffer, err := handle.Read(context.Background(), gpsAddr, 32)
		if err != nil {
			logger.Error(err)
			continue
		}

		for _, b := range buffer {
			if b == 0x0D {
				if strBuf != "" {
					alt, err = parseGPS(strBuf)
					if err == nil {
						return alt, nil
					}
				}
				strBuf = ""
			} else if b != 0x0A {
				strBuf += string(b)
			}
		}
		utils.SelectContextOrWait(ctx, 150*time.Millisecond)
	}
	return alt, err
}

func parseGPS(line string) (float64, error) {
	s, err := nmea.Parse(line)
	if err != nil {
		logger.Debugf("can't parse nmea %s : %s", line, err)
	}
	gga, ok := s.(nmea.GGA)
	if ok {
		if gga.FixQuality == "0" || (gga.Longitude == 0 && gga.Latitude == 0) {
			return -9999, nil
		}
		return gga.Altitude, nil
	}
	return 0, errors.New("not a valid GGA string")
}

func addChk(data []byte) []byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

// Reads from the buffer until there is nothing left and we get several newlines in a row
func clearBuffer(ctx context.Context, handle board.I2CHandle) {
	emptyBuf := 0
	// Read until we get empty 5x in a row
	for emptyBuf < 5 {
		select {
		case <-ctx.Done():
			return
		default:
		}
		buffer, err := handle.Read(ctx, gpsAddr, 32)
		if err != nil {
			logger.Error(err)
			continue
		}
		emptyBuf++
		for _, b := range buffer {
			if b != 10 {
				emptyBuf = 0
			}
		}
	}
}
