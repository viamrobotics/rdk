package rtk

import (
	"fmt"
	"io"

	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
)

const (
	ubxSynch1      = 0xB5
	ubxSynch2      = 0x62
	ubxRtcm1005    = 0x05 // Stationary RTK reference ARP
	ubxRtcm1074    = 0x4A // GPS MSM4
	ubxRtcm1084    = 0x54 // GLONASS MSM4
	ubxRtcm1094    = 0x5E // Galileo MSM4
	ubxRtcm1124    = 0x7C // BeiDou MSM4
	ubxRtcm1230    = 0xE6 // GLONASS code-phase biases, set to once every 10 seconds
	uart2          = 2
	usb            = 3
	ubxRtcmMsb     = 0xF5
	ubxClassCfg    = 0x06
	ubxCfgMsg      = 0x01
	ubxCfgTmode3   = 0x71
	maxPayloadSize = 256
	ubxCfgCfg      = 0x09

	ubxNmeaMsb = 0xF0 // All NMEA enable commands have 0xF0 as MSB. Equal to UBX_CLASS_NMEA
	ubxNmeaGga = 0x00 // GxGGA (Global positioning system fix data)
	ubxNmeaGll = 0x01 // GxGLL (latitude and long, with time of position fix and status)
	ubxNmeaGsa = 0x02 // GxGSA (GNSS DOP and Active satellites)
	ubxNmeaGsv = 0x03 // GxGSV (GNSS satellites in view)
	ubxNmeaRmc = 0x04 // GxRMC (Recommended minimum data)
	ubxNmeaVtg = 0x05 // GxVTG (course over ground and Ground speed)

	svinModeEnable  = 0x01
	svinModeDisable = 0x00

	// configuration constants.
	requiredAccuracyConfig = "loc_accuracy"
	observationTimeConfig  = "time_accuracy"
	timeMode               = "time"
	svinConfig             = "svin"
)

var rtcmMsgs = map[int]int{
	ubxRtcm1005: 1,
	ubxRtcm1074: 1,
	ubxRtcm1084: 1,
	ubxRtcm1094: 1,
	ubxRtcm1124: 1,
	ubxRtcm1230: 5,
}

var nmeaMsgs = map[int]int{
	ubxNmeaGll: 1,
	ubxNmeaGsa: 1,
	ubxNmeaGsv: 1,
	ubxNmeaRmc: 1,
	ubxNmeaVtg: 1,
	ubxNmeaGga: 1,
}

type configCommand struct {
	correctionType string
	portName       string
	baudRate       uint
	surveyIn       string

	requiredAcc     float64
	observationTime int

	msgsToEnable  map[int]int
	msgsToDisable map[int]int

	portID    int
	writePort io.ReadWriteCloser
}

// ConfigureBaseRTKStation configures an RTK chip to act as a base station and send correction data.
func ConfigureBaseRTKStation(config config.Component) error {
	correctionType := config.Attributes.String(correctionSourceName)

	surveyIn := config.Attributes.String(svinConfig)
	requiredAcc := config.Attributes.Float64(requiredAccuracyConfig, 10)
	observationTime := config.Attributes.Int(observationTimeConfig, 60)

	c := &configCommand{
		correctionType:  correctionType,
		requiredAcc:     requiredAcc,
		observationTime: observationTime,
		msgsToEnable:    rtcmMsgs, // defaults
		msgsToDisable:   nmeaMsgs, // defaults
		surveyIn:        surveyIn,
	}

	// already configured
	if c.surveyIn != timeMode {
		return nil
	}

	switch c.correctionType {
	case serialStr:
		err := c.serialConfigure(config)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("configuration not supported for %s", correctionType)
	}

	err := c.enableAll(ubxRtcmMsb)
	if err != nil {
		return err
	}

	err = c.disableAll(ubxNmeaMsb)
	if err != nil {
		return err
	}

	err = c.enableSVIN()
	if err != nil {
		return err
	}

	return nil
}

// ConfigureRoverDefault sets up an RTK chip to act as a rover and receive correction data.
func ConfigureRoverDefault(config config.Component) error {
	correctionType := config.Attributes.String(correctionSourceName)

	c := &configCommand{
		correctionType: correctionType,
		msgsToEnable:   nmeaMsgs, // defaults
		msgsToDisable:  rtcmMsgs, // defaults
	}

	switch correctionType {
	case serialStr:
		err := c.serialConfigure(config)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("configuration not supported for %s", correctionType)
	}

	err := c.enableAll(ubxNmeaMsb)
	if err != nil {
		return err
	}

	err = c.disableAll(ubxRtcmMsb)
	if err != nil {
		return err
	}

	err = c.disableSVIN()
	if err != nil {
		return err
	}

	return nil
}

func (c *configCommand) serialConfigure(config config.Component) error {
	portName := config.Attributes.String("correction_path")
	if portName == "" {
		return fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
	}
	c.portName = portName

	baudRate := config.Attributes.Int("correction_baud", 0)
	if baudRate == 0 {
		baudRate = 9600
	}
	c.baudRate = uint(baudRate)
	c.portID = uart2

	options := serial.OpenOptions{
		PortName:        c.portName,
		BaudRate:        c.baudRate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	// Open the port
	writePort, err := serial.Open(options)
	if err != nil {
		return err
	}
	c.writePort = writePort

	return nil
}

func (c *configCommand) sendCommand(cls, id, msgLen int, payloadCfg []byte) error {
	switch c.correctionType {
	case serialStr:
		_, err := c.sendCommandSerial(cls, id, msgLen, payloadCfg)
		return err
	default:
		return errors.Errorf("configuration not supported for %s", c.correctionType)
	}
}

func (c *configCommand) sendCommandSerial(cls, id, msgLen int, payloadCfg []byte) ([]byte, error) {
	checksumA, checksumB := calcChecksum(cls, id, msgLen, payloadCfg)

	// build packet to send over serial
	byteSize := msgLen + 8 // header+checksum+payload
	packet := make([]byte, byteSize)

	// header bytes
	packet[0] = byte(ubxSynch1)
	packet[1] = byte(ubxSynch2)
	packet[2] = byte(cls)
	packet[3] = byte(id)
	packet[4] = byte(msgLen & 0xFF) // LSB
	packet[5] = byte(msgLen >> 8)   // MSB

	ind := 6
	for i := 0; i < msgLen; i++ {
		packet[ind+i] = payloadCfg[i]
	}
	packet[len(packet)-1] = byte(checksumB)
	packet[len(packet)-2] = byte(checksumA)

	_, err := c.writePort.Write(packet)
	if err != nil {
		return nil, err
	}

	// then wait to capture a byte
	buf := make([]byte, maxPayloadSize)
	n, err := c.writePort.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (c *configCommand) disableAll(msb int) error {
	for msg := range c.msgsToDisable {
		err := c.disableMessageCommand(msb, msg, c.portID)
		if err != nil {
			return err
		}
	}
	err := c.saveAllConfigs()
	if err != nil {
		return err
	}
	return nil
}

func (c *configCommand) enableAll(msb int) error {
	for msg, sendRate := range c.msgsToEnable {
		err := c.enableMessageCommand(msb, msg, c.portID, sendRate)
		if err != nil {
			return err
		}
	}
	err := c.saveAllConfigs()
	if err != nil {
		return err
	}
	return nil
}

//nolint:unused
func (c *configCommand) getSurveyMode() error {
	cls := ubxClassCfg
	id := ubxCfgTmode3
	payloadCfg := make([]byte, 40)
	return c.sendCommand(cls, id, 0, payloadCfg) // set payloadcfg
}

func (c *configCommand) enableSVIN() error {
	err := c.setSurveyMode(svinModeEnable, c.requiredAcc, c.observationTime)
	if err != nil {
		return err
	}

	err = c.saveAllConfigs()
	if err != nil {
		return err
	}
	return nil
}

func (c *configCommand) disableSVIN() error {
	err := c.setSurveyMode(svinModeDisable, 0, 0)
	if err != nil {
		return err
	}

	err = c.saveAllConfigs()
	if err != nil {
		return err
	}
	return nil
}

func (c *configCommand) setSurveyMode(mode int, requiredAccuracy float64, observationTime int) error {
	payloadCfg := make([]byte, 40)
	if len(payloadCfg) == 0 {
		return errors.New("must specify payload")
	}

	cls := ubxClassCfg
	id := ubxCfgTmode3
	msgLen := 40

	// payloadCfg should be loaded with poll response. Now modify only the bits we care about
	payloadCfg[2] = byte(mode) // Set mode. Survey-In and Disabled are most common. Use ECEF (not LAT/LON/ALT).

	// svinMinDur is U4 (uint32_t) in seconds
	payloadCfg[24] = byte(observationTime & 0xFF) // svinMinDur in seconds
	payloadCfg[25] = byte((observationTime >> 8) & 0xFF)
	payloadCfg[26] = byte((observationTime >> 16) & 0xFF)
	payloadCfg[27] = byte((observationTime >> 24) & 0xFF)

	// svinAccLimit is U4 (uint32_t) in 0.1mm.
	svinAccLimit := uint32(requiredAccuracy * 10000.0) // Convert m to 0.1mm

	payloadCfg[28] = byte(svinAccLimit & 0xFF) // svinAccLimit in 0.1mm increments
	payloadCfg[29] = byte((svinAccLimit >> 8) & 0xFF)
	payloadCfg[30] = byte((svinAccLimit >> 16) & 0xFF)
	payloadCfg[31] = byte((svinAccLimit >> 24) & 0xFF)

	return c.sendCommand(cls, id, msgLen, payloadCfg)
}

//nolint:lll,unused
func (c *configCommand) setStaticPosition(ecefXOrLat, ecefXOrLatHP, ecefYOrLon, ecefYOrLonHP, ecefZOrAlt, ecefZOrAltHP int, latLong bool) error {
	cls := ubxClassCfg
	id := ubxCfgTmode3
	msgLen := 40

	payloadCfg := make([]byte, maxPayloadSize)
	payloadCfg[2] = byte(2)

	if latLong {
		payloadCfg[3] = (1 << 0) // Set mode to fixed. Use LAT/LON/ALT.
	}

	// Set ECEF X or Lat
	payloadCfg[4] = byte((ecefXOrLat >> 8 * 0) & 0xFF) // LSB
	payloadCfg[5] = byte((ecefXOrLat >> 8 * 1) & 0xFF)
	payloadCfg[6] = byte((ecefXOrLat >> 8 * 2) & 0xFF)
	payloadCfg[7] = byte((ecefXOrLat >> 8 * 3) & 0xFF) // MSB

	// Set ECEF Y or Long
	payloadCfg[8] = byte((ecefYOrLon >> 8 * 0) & 0xFF) // LSB
	payloadCfg[9] = byte((ecefYOrLon >> 8 * 1) & 0xFF)
	payloadCfg[10] = byte((ecefYOrLon >> 8 * 2) & 0xFF)
	payloadCfg[11] = byte((ecefYOrLon >> 8 * 3) & 0xFF) // MSB

	// Set ECEF Z or Altitude
	payloadCfg[12] = byte((ecefZOrAlt >> 8 * 0) & 0xFF) // LSB
	payloadCfg[13] = byte((ecefZOrAlt >> 8 * 1) & 0xFF)
	payloadCfg[14] = byte((ecefZOrAlt >> 8 * 2) & 0xFF)
	payloadCfg[15] = byte((ecefZOrAlt >> 8 * 3) & 0xFF) // MSB

	// Set high precision parts
	payloadCfg[16] = byte(ecefXOrLatHP)
	payloadCfg[17] = byte(ecefYOrLonHP)
	payloadCfg[18] = byte(ecefZOrAltHP)

	return c.sendCommand(cls, id, msgLen, payloadCfg)
}

func (c *configCommand) disableMessageCommand(msgClass, messageNumber, portID int) error {
	err := c.enableMessageCommand(msgClass, messageNumber, portID, 0)
	if err != nil {
		return err
	}
	return nil
}

func (c *configCommand) enableMessageCommand(msgClass, messageNumber, portID, sendRate int) error {
	// dont use current port settings actually
	payloadCfg := make([]byte, maxPayloadSize)

	cls := ubxClassCfg
	id := ubxCfgMsg
	msgLen := 8

	payloadCfg[0] = byte(msgClass)
	payloadCfg[1] = byte(messageNumber)
	payloadCfg[2+portID] = byte(sendRate)
	// default to enable usb on with same sendRate
	payloadCfg[2+usb] = byte(sendRate)

	return c.sendCommand(cls, id, msgLen, payloadCfg)
}

func (c *configCommand) saveAllConfigs() error {
	cls := ubxClassCfg
	id := ubxCfgCfg
	msgLen := 12

	payloadCfg := make([]byte, maxPayloadSize)

	payloadCfg[4] = 0xFF
	payloadCfg[5] = 0xFF

	return c.sendCommand(cls, id, msgLen, payloadCfg)
}

// Close closes all open ports used in configuration.
func (c *configCommand) Close() error {
	// close port reader if serial
	if c.writePort != nil {
		if err := c.writePort.Close(); err != nil {
			return err
		}
		c.writePort = nil
	}
	return nil
}

func calcChecksum(cls, id, msgLen int, payload []byte) (checksumA, checksumB int) {
	checksumA = 0
	checksumB = 0

	checksumA += cls
	checksumB += checksumA

	checksumA += id
	checksumB += checksumA

	checksumA += (msgLen & 0xFF)
	checksumB += checksumA

	checksumA += (msgLen >> 8)
	checksumB += checksumA

	for i := 0; i < msgLen; i++ {
		checksumA += int(payload[i])
		checksumB += checksumA
	}
	return checksumA, checksumB
}
