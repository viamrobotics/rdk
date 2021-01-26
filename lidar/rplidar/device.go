package rplidar

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/echolabsinc/robotcore/lidar"
	rplidargen "github.com/echolabsinc/robotcore/lidar/rplidar/gen"
)

func isResultOk(result uint) bool {
	return result&uint(rplidargen.RESULT_FAIL_BIT) == 0
}

func NewRPLidar(devicePath string) (*RPLidar, error) {
	var driver rplidargen.RPlidarDriver

	devInfo := rplidargen.NewRplidar_response_device_info_t()
	defer rplidargen.DeleteRplidar_response_device_info_t(devInfo)

	var lastErr error
	baudRates := []uint{256000, 115200}
	for _, rate := range baudRates {
		driver = rplidargen.RPlidarDriverCreateDriver(uint(rplidargen.DRIVER_TYPE_SERIALPORT))
		if !isResultOk(driver.Connect(devicePath, rate)) {
			lastErr = errors.New("failed to connect")
			continue
		}

		if !isResultOk(driver.GetDeviceInfo(devInfo, uint(5000))) {
			lastErr = errors.New("failed to get device info")
			continue
		}

		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, lastErr
	}

	serialNum := devInfo.GetSerialnum()
	var serialNumStr string
	for pos := 0; pos < 16; pos++ {
		serialNumStr += fmt.Sprintf("%02X", rplidargen.ByteArray_getitem(serialNum, pos))
	}

	firmwareVer := fmt.Sprintf("%d.%02d",
		devInfo.GetFirmware_version()>>8,
		devInfo.GetFirmware_version()&0xFF)
	hardwareRev := int(devInfo.GetHardware_version())

	healthInfo := rplidargen.NewRplidar_response_device_health_t()
	defer rplidargen.DeleteRplidar_response_device_health_t(healthInfo)

	if !isResultOk(driver.GetHealth(healthInfo)) {
		return nil, errors.New("failed to get health")
	}

	if int(healthInfo.GetStatus()) == rplidargen.RPLIDAR_STATUS_ERROR {
		return nil, errors.New("bad health")
	}

	return &RPLidar{
		driver:           driver,
		nodeSize:         8192,
		model:            devInfo.GetModel(),
		serialNumber:     serialNumStr,
		firmwareVersion:  firmwareVer,
		hardwareRevision: hardwareRev,
	}, nil
}

type RPLidar struct {
	driver   rplidargen.RPlidarDriver
	nodes    rplidargen.Rplidar_response_measurement_node_hq_t
	nodeSize int
	started  bool
	bounds   *image.Point

	// info
	model            byte
	serialNumber     string
	firmwareVersion  string
	hardwareRevision int
}

func (rpl *RPLidar) SerialNumber() string {
	return rpl.serialNumber
}

func (rpl *RPLidar) FirmwareVersion() string {
	return rpl.firmwareVersion
}

func (rpl *RPLidar) HardwareRevision() int {
	return rpl.hardwareRevision
}

func (rpl *RPLidar) Model() string {
	switch rpl.model {
	case 24:
		return "A1"
	case 49:
		return "A3"
	default:
		return "unknown"
	}
}

func (rpl *RPLidar) Range() int {
	switch rpl.model {
	case 24: // A1
		return 12
	case 49: // A3
		return 25
	default:
		panic(fmt.Errorf("range unknown for model %d", rpl.model))
	}
}

func (rpl *RPLidar) Bounds() (image.Point, error) {
	if rpl.bounds != nil {
		return *rpl.bounds, nil
	}
	devRange := float64(rpl.Range())
	measurements, _ := rpl.Scan()
	for _, m := range measurements {
		if m.Distance() > devRange {
			devRange = m.Distance()
		}
	}
	width := int(math.Ceil(devRange))
	height := width
	bounds := image.Point{width, height}
	rpl.bounds = &bounds
	return bounds, nil
}

func (rpl *RPLidar) Start() {
	rpl.driver.StartMotor()
	rpl.driver.StartScan(false, true)
	rpl.nodes = rplidargen.New_measurementNodeHqArray(rpl.nodeSize)
}

func (rpl *RPLidar) Stop() {
	defer rplidargen.Delete_measurementNodeHqArray(rpl.nodes)
	rpl.driver.Stop()
	rpl.driver.StopMotor()
}

func (rpl *RPLidar) Close() {
	rpl.Stop()
}

func (rpl *RPLidar) Scan() (lidar.Measurements, error) {
	if !rpl.started {
		rpl.Start()
		rpl.started = true
	}

	nodeCount := int64(rpl.nodeSize)
	result := rpl.driver.GrabScanDataHq(rpl.nodes, &nodeCount)
	if !isResultOk(result) {
		return nil, errors.New("bad scan")
	}
	measurements := make(lidar.Measurements, 0, nodeCount)
	rpl.driver.AscendScanData(rpl.nodes, nodeCount)

	for pos := 0; pos < int(nodeCount); pos++ {
		node := rplidargen.MeasurementNodeHqArray_getitem(rpl.nodes, pos)
		if node.GetDist_mm_q2() == 0 {
			continue // TODO(erd): okay to skip?
		}

		nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
		nodeAngle = nodeAngle * math.Pi / 180
		nodeDistance := float64(node.GetDist_mm_q2()) / 4
		measurements = append(measurements, lidar.NewMeasurement(nodeAngle, nodeDistance/1000))
	}

	return measurements, nil
}
