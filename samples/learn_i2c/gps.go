package main

import (
	"context"
	"errors"
	"time"

	. "go.viam.com/core/board"
	_ "go.viam.com/core/board/detector"

	"github.com/adrianmo/go-nmea"
)


func GpsAlt(ctx context.Context, handle I2CHandle) (float64, error) {
	var err error
	var addr byte
	addr = 0x10
	
	cmd314 := addChk([]byte("PMTK314,0,1,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))
	
	handle.WriteBytes(context.Background(), addr, cmd314)
	handle.WriteBytes(context.Background(), addr, cmd220)

	strBuf := ""
	
	var alt float64
	
	for i := 0; i <= 20; i++{
		select {
		case <-ctx.Done():
			return 0, nil
		default:
		}
		buffer := []byte{}
		buffer, err = handle.ReadBytes(context.Background(), addr, 32)
		if err != nil {
			logger.Error(err)
			continue
		}
		
		for _, b := range buffer{
			if b == 0x0D {
				if strBuf != "" {
					alt, err = parseGPS(strBuf)
					if err == nil{
						return alt, nil
					}
				}
				strBuf = ""
			}else if b != 0x0A{
				strBuf += string(b)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return alt, err
}

func parseGPS(line string) (float64, error){
	s, err := nmea.Parse(line)
	if err != nil {
		logger.Debugf("can't parse nmea %s : %s", line, err)
	}
	gga, ok := s.(nmea.GGA)
	if ok {
		if gga.FixQuality == "0" {
			return -9999, nil
		}
		return gga.Altitude, nil
	}
	return 0, errors.New("Not a valid GGA string")
}

func addChk(data []byte) []byte {
	var chk byte
	for _, b := range data{
		chk ^= b
	}
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}
