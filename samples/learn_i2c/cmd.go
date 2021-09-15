package main

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	_ "go.viam.com/core/board"
	_ "go.viam.com/core/board/detector"
	webserver "go.viam.com/core/web/server"

	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/adrianmo/go-nmea"
)

var boardName = "gpsBoard"
var END_BYTES = []byte{0x0D, 0x0A}
var logger = golog.NewDevelopmentLogger("gps")

func init() {
	action.RegisterAction("Gps", func(ctx context.Context, r robot.Robot) {
		err := Gps(ctx, r)
		if err != nil {
			logger.Errorf("error Gps: %s", err)
		}
	})
	action.RegisterAction("Disp", func(ctx context.Context, r robot.Robot) {
		err := Disp(ctx, r)
		if err != nil {
			logger.Errorf("error Disp: %s", err)
		}
	})
}

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

func Disp(ctx context.Context, theRobot robot.Robot) error {
	gpsBoard, ok := theRobot.BoardByName(boardName)
	if !ok {
		return fmt.Errorf("failed to find board %s", boardName)
	}
	
	var addr byte
	addr = 0x3C
	
	i2c, _ := gpsBoard.I2CByName("gps")
	handle, err := i2c.OpenHandle()
	defer handle.Close()
	
	if err != nil{
		return err
	}

	// set contrast
	contrast := []byte{0, 0x81, 0x2F}
	handle.WriteBytes(context.Background(), addr, contrast)
	
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
	
	handle.WriteBytes(context.Background(), addr, init)
	
	time.Sleep(200 * time.Millisecond)
	
	// turn on
	handle.WriteBytes(context.Background(), addr, []byte{0x00, 0xAF})
	
	time.Sleep(200 * time.Millisecond)
	
	// blank
	var reg byte
	for reg = 0xB0; reg <= 0xBF; reg++{
		someBytes := []byte{0, reg, 0x10, 0}
		handle.WriteBytes(context.Background(), addr, someBytes)
		
		//~ someBytes = []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		//~ handle.WriteBytes(context.Background(), addr, someBytes)
		
		someBytes = []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,0,0,0,0,0,0,0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		handle.WriteBytes(context.Background(), addr, someBytes)
		someBytes = []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,0,0,0,0,0,0,0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		handle.WriteBytes(context.Background(), addr, someBytes)
		
		someBytes = []byte{0x40, 0x0, 0}
		handle.WriteBytes(context.Background(), addr, someBytes)
		
		//~ someBytes = []byte{0x40, 0x0, 0x0}
		//~ handle.WriteBytes(context.Background(), addr, someBytes)
	}
	
	// Write some stuff to the display?
	for reg = 0xB0; reg < 0xBE; reg++{
		someBytes := []byte{0, reg, 0x10, 0}
		handle.WriteBytes(context.Background(), addr, someBytes)

		someBytes = []byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
		handle.WriteBytes(context.Background(), addr, someBytes)
		someBytes = []byte{0x40, 0x88, 0x88, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
		handle.WriteBytes(context.Background(), addr, someBytes)
		
		someBytes = []byte{0x40, 0x0, 0xFF}
		handle.WriteBytes(context.Background(), addr, someBytes)
		
		//~ someBytes = []byte{0x40, 0x0, 0x0}
		//~ handle.WriteBytes(context.Background(), addr, someBytes)
	}
	return nil
}

func Gps(ctx context.Context, theRobot robot.Robot) error {

	gpsBoard, ok := theRobot.BoardByName(boardName)
	if !ok {
		return fmt.Errorf("failed to find board %s", boardName)
	}
	
	i2c, _ := gpsBoard.I2CByName("gps")
	handle, err := i2c.OpenHandle()
	defer handle.Close()
	
	if err != nil{
		return err
	}
	
	var addr byte
	addr = 0x10
	
	cmd314 := addChk([]byte("PMTK314,0,1,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))
	
	handle.WriteBytes(context.Background(), addr, cmd314)
	handle.WriteBytes(context.Background(), addr, cmd220)

	strBuf := ""
	for i := 0; i <= 1500; i++{
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		buffer, err := handle.ReadBytes(context.Background(), addr, 32)
		if err != nil {
			logger.Fatal(err)
		}
		
		//~ fmt.Println(buffer)
		for _, b := range buffer{
			if b == 0x0D {
				if strBuf != "" {
					parseGPS(strBuf)
				}
				strBuf = ""
			}else if b != 0x0A{
				strBuf += string(b)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func parseGPS(line string){
	s, err := nmea.Parse(line)
	if err != nil {
		logger.Debugf("can't parse nmea %s : %s", line, err)
	}
	var currentLocation nmea.GGA
	gga, ok := s.(nmea.GGA)
	if ok {
		currentLocation = gga
		fmt.Println("alt:", currentLocation.Altitude)
	}
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

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

func blank() [][]byte {
	return [][]byte{
		[]byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,0,0,0,0,0,0,0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		[]byte{0x40, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,0,0,0,0,0,0,0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		[]byte{0x40, 0x0, 0x0},
	}
}
