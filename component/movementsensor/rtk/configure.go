package rtk

import (
	"fmt"
	"log"

	"github.com/jacobsa/go-serial/serial"
	"go.viam.com/rdk/config"
)

const (
	ubxSynch1           = 0xB5
	ubxSynch2           = 0x62
	ubxRtcm1001         = 0x01
	ubxRtcm1002         = 0x02
	ubxRtcm1005         = 0x05 // Stationary RTK reference ARP
	ubxRtcm1074         = 0x4A // GPS MSM4
	ubxRtcm1077         = 0x4D // GPS MSM7
	ubxRtcm1084         = 0x54 // GLONASS MSM4
	ubxRtcm1087         = 0x57 // GLONASS MSM7
	ubxRtcm1094         = 0x5E // Galileo MSM4
	ubxRtcm1097         = 0x61 // Galileo MSM7
	ubxRtcm1124         = 0x7C // BeiDou MSM4
	ubxRtcm1127         = 0x7F // BeiDou MSM7
	ubxRtcm1230         = 0xE6 // GLONASS code-phase biases, set to once every 10 seconds
	uart2               = 2
	usb                 = 3
	ubxRtcmMsb          = 0xF5
	ubxClassCfg         = 0x06
	ubxCfgMsg           = 0x01
	ubxCfgTmode3        = 0x71
	maxPayloadSize      = 256
	ubxCfgPrt           = 0x00
	ubxCfgCfg           = 0x09
	valCfgSubsecIoPort  = 0x00000001 // ioPort - communications port settings (causes IO system reset!)
	valCfgSubsecMsgconf = 0x00000002 // msgConf - message configuration

	ubxNmeaMsb = 0xF0 // All NMEA enable commands have 0xF0 as MSB. Equal to UBX_CLASS_NMEA
	ubxNmeaDtm = 0x0A // GxDTM (datum reference)
	ubxNmeaGaq = 0x45 // GxGAQ (poll a standard message (if the current talker ID is GA))
	ubxNmeaGbq = 0x44 // GxGBQ (poll a standard message (if the current Talker ID is GB))
	ubxNmeaGbs = 0x09 // GxGBS (GNSS satellite fault detection)
	ubxNmeaGga = 0x00 // GxGGA (Global positioning system fix data)
	ubxNmeaGll = 0x01 // GxGLL (latitude and long, whith time of position fix and status)
	ubxNmeaGlq = 0x43 // GxGLQ (poll a standard message (if the current Talker ID is GL))
	ubxNmeaGnq = 0x42 // GxGNQ (poll a standard message (if the current Talker ID is GN))
	ubxNmeaGns = 0x0D // GxGNS (GNSS fix data)
	ubxNmeaGpq = 0x40 // GxGPQ (poll a standard message (if the current Talker ID is GP))
	ubxNmeaGqq = 0x47 // GxGQQ (poll a standard message (if the current Talker ID is GQ))
	ubxNmeaGrs = 0x06 // GxGRS (GNSS range residuals)
	ubxNmeaGsa = 0x02 // GxGSA (GNSS DOP and Active satellites)
	ubxNmeaGst = 0x07 // GxGST (GNSS Pseudo Range Error Statistics)
	ubxNmeaGsv = 0x03 // GxGSV (GNSS satellites in view)
	ubxNmeaRlm = 0x0B // GxRMC (Return link message (RLM))
	ubxNmeaRmc = 0x04 // GxRMC (Recommended minimum data)
	ubxNmeaTxt = 0x41 // GxTXT (text transmission)
	ubxNmeaVlw = 0x0F // GxVLW (dual ground/water distance)
	ubxNmeaVtg = 0x05 // GxVTG (course over ground and Ground speed)
	ubxNmeaZda = 0x08 // GxZDA (Time and Date)

	svinModeEnable  = 0x01
	svinModeDisable = 0x00

	// configuration constants
	requiredAccuracyConfig = "loc_accuracy"
	observationTimeConfig  = "time_accuracy"
	timeMode               = "time"
	svinConfig             = "svin"

	rtcmMsgs = map[int]int{
		ubxRtcm1005: 1,
		ubxRtcm1074: 1,
		ubxRtcm1084: 1,
		ubxRtcm1094: 1,
		ubxRtcm1124: 1,
		ubxRtcm1230: 5,
	}
	nmeaMsgs = map[int]int{
		ubxNmeaGll: 1,
		ubxNmeaGsa: 1,
		ubxNmeaGsv: 1,
		ubxNmeaRmc: 1,
		ubxNmeaVtg: 1,
		ubxNmeaGga: 1,
	}
)

type configCommand struct {
	correctionType string
	portName       string
	baudRate       uint
	surveyIn       string

	requiredAcc     float64
	observationTime int

	msgsToEnable  map[int]int
	msgsToDisable map[int]int

	portId int
}

// configure an RTKStation with time mode
func ConfigureBaseRTKStation(config config.Component) (*configCommand, error) {
	correctionType := config.Attributes.String(correctionSourceName)

	surveyIn := config.Attributes.String(svinConfig)
	requiredAcc := config.Attributes.Float64(requiredAccuracyConfig, 10)
	observationTime := config.Attributes.Int(observationTimeConfig, 60)

	// already configured
	if surveyIn != timeMode {
		return nil, nil
	}

	c := &configCommand{
		correctionType:  correctionType,
		requiredAcc:     requiredAcc,
		observationTime: observationTime,
		msgsToEnable:    rtcmMsgs, // defaults
		msgsToDisable:   nmeaMsgs, // defaults
	}

	switch correctionType {
	case "serial":
		portName := config.Attributes.String("correction_path")
		if portName == "" {
			return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
		}
		c.portName = portName

		baudRate := config.Attributes.Int("correction_baud", 0)
		if baudRate == 0 {
			baudRate = 9600
		}
		c.baudRate = uint(baudRate)
		c.portId = uart2
		return c, nil
	default:
		return nil, nil
	}

	c.enableAll(ubxRtcmMsb)
	c.disableAll(ubxNmeaMsb)
	c.enableSVIN()

	return nil, nil
}

// configure to default rover settings
func ConfigureRoverDefault(config config.Component) (*configCommand, error) {
	correctionType := config.Attributes.String(correctionSourceName)

	c := &configCommand{
		correctionType: correctionType,
		msgsToEnable:   nmeaMsgs, // defaults
		msgsToDisable:  rtcmMsgs, // defaults
	}

	switch correctionType {
	case "serial":
		portName := config.Attributes.String("correction_path")
		if portName == "" {
			return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
		}
		c.portName = portName

		baudRate := config.Attributes.Int("correction_baud", 0)
		if baudRate == 0 {
			baudRate = 9600
		}
		c.baudRate = uint(baudRate)
		c.portId = uart2
		return c, nil
	default:
		return nil, nil
	}

	c.enableAll(ubxNmeaMsb)
	c.disableAll(ubxRtcmMsb)

	return nil, nil
}

func (c *configCommand) disableAll(msb int) {
	for msg, _ := range c.msgsToDisable {
		c.disableMessageCommand(msb, msg, c.portId)
	}
	c.saveAllConfigs()
}

func (c *configCommand) enableAll(msb int) {
	for msg, sendRate := range c.msgsToEnable {
		c.enableMessageCommand(msb, msg, c.portId, sendRate)
	}
	c.saveAllConfigs()
}

func (c *configCommand) getSurveyMode() []byte {
	cls := ubxClassCfg
	id := ubxCfgTmode3
	payloadCfg := make([]byte, 40)
	return c.sendCommand(cls, id, 0, payloadCfg) // set payloadcfg
}

func (c *configCommand) enableSVIN() {
	c.setSurveyMode(svinModeEnable, c.requiredAcc, c.observationTime)
	c.saveAllConfigs()
}

func (c *configCommand) disableSVIN() {
	c.setSurveyMode(svinModeDisable, 0, 0)
	c.saveAllConfigs()
}

func (c *configCommand) setSurveyMode(mode int, requiredAccuracy float64, observationTime int) bool {
	// payloadCfg := getSurveyMode() // get current configs
	payloadCfg := make([]byte, 40)
	if len(payloadCfg) == 0 {
		return false
	}

	cls := ubxClassCfg
	id := ubxCfgTmode3
	msg_len := 40

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

	c.sendCommand(cls, id, msg_len, payloadCfg)

	return true
}

func (c *configCommand) setStaticPosition(ecefXOrLat int, ecefXOrLatHP int, ecefYOrLon int, ecefYOrLonHP int, ecefZOrAlt int, ecefZOrAltHP int, latLong bool) {
	cls := ubxClassCfg
	id := ubxCfgTmode3
	msg_len := 40

	payloadCfg := make([]byte, maxPayloadSize)
	payloadCfg[2] = byte(2)

	if latLong == true {
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
	c.sendCommand(cls, id, msg_len, payloadCfg)
}

func (c *configCommand) disableMessageCommand(msgClass int, messageNumber int, portId int) {
	c.enableMessageCommand(msgClass, messageNumber, portId, 0)
}

func (c *configCommand) enableMessageCommand(msgClass int, messageNumber int, portId int, sendRate int) {
	//dont use current port settings actually
	payloadCfg := make([]byte, maxPayloadSize)

	cls := ubxClassCfg
	id := ubxCfgMsg
	msg_len := 8

	payloadCfg[0] = byte(msgClass)
	payloadCfg[1] = byte(messageNumber)
	payloadCfg[2+portId] = byte(sendRate)
	//default to enable usb on with same sendRate
	payloadCfg[2+usb] = byte(sendRate)

	c.sendCommand(cls, id, msg_len, payloadCfg)
}

func (c *configCommand) sendCommand(cls int, id int, msg_len int, payloadCfg []byte) ([]byte, error) {
	switch c.correctionType {
	case "serial":
		msg, err := c.sendCommandSerial(cls, id, msg_len, payloadCfg)
		if err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, nil
	}
}

func (c *configCommand) sendCommandSerial(cls int, id int, msg_len int, payloadCfg []byte) ([]byte, error) {
	checksumA, checksumB := calcChecksum(cls, id, msg_len, payloadCfg)
	options := serial.OpenOptions{
		PortName:        c.portName,
		BaudRate:        c.baudRate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	// Open the port.
	writePort, err := serial.Open(options)
	if err != nil {
		return nil, err
	}
	defer writePort.Close()

	//build packet to send over serial
	byteSize := msg_len + 8 //header+checksum+payload
	packet := make([]byte, byteSize)

	//header bytes
	packet[0] = byte(ubxSynch1)
	packet[1] = byte(ubxSynch2)
	packet[2] = byte(cls)
	packet[3] = byte(id)
	packet[4] = byte(msg_len & 0xFF) //LSB
	packet[5] = byte(msg_len >> 8)   //MSB

	ind := 6
	for i := 0; i < msg_len; i++ {
		packet[ind+i] = payloadCfg[i]
	}
	packet[len(packet)-1] = byte(checksumB)
	packet[len(packet)-2] = byte(checksumA)

	writePort.Write(packet)

	//then wait to capture a byte
	buf := make([]byte, maxPayloadSize)
	n, err := writePort.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	return buf[:n], nil
}

func (c *configCommand) saveAllConfigs() {
	cls := ubxClassCfg
	id := ubxCfgCfg
	msg_len := 12

	payloadCfg := make([]byte, maxPayloadSize)

	payloadCfg[4] = 0xFF
	payloadCfg[5] = 0xFF

	c.sendCommand(cls, id, msg_len, payloadCfg)

}

func calcChecksum(cls int, id int, msg_len int, payload []byte) (checksumA int, checksumB int) {
	checksumA = 0
	checksumB = 0

	checksumA += cls
	checksumB += checksumA

	checksumA += id
	checksumB += checksumA

	checksumA += (msg_len & 0xFF)
	checksumB += checksumA

	checksumA += (msg_len >> 8)
	checksumB += checksumA

	for i := 0; i < msg_len; i++ {
		checksumA += int(payload[i])
		checksumB += checksumA
	}
	return checksumA, checksumB
}
