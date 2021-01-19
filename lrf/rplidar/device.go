package rplidar

import (
	"errors"
	"fmt"
	"math"

	"github.com/echolabsinc/robotcore/lrf"
	rplidargen "github.com/echolabsinc/robotcore/lrf/rplidar/gen"
)

func isResultOk(result uint) bool {
	return result&uint(rplidargen.RESULT_FAIL_BIT) == 0
}

func NewRPLidar(devicePath string) (*RPLidar, error) {
	driver := rplidargen.RPlidarDriverCreateDriver(uint(rplidargen.DRIVER_TYPE_SERIALPORT))
	if !isResultOk(driver.Connect(devicePath, uint(115200))) {
		return nil, errors.New("failed to connect")
	}

	devInfo := rplidargen.NewRplidar_response_device_info_t()
	defer rplidargen.DeleteRplidar_response_device_info_t(devInfo)
	if !isResultOk(driver.GetDeviceInfo(devInfo, uint(5000))) {
		return nil, errors.New("failed to get device info")
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

// TODO(erd): configured based on device
func (rpl *RPLidar) Range() int {
	switch rpl.model {
	case 24: // A1
		return 12
	default:
		panic(fmt.Errorf("range unknown for model %d", rpl.model))
	}
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

func (rpl *RPLidar) Scan() (lrf.Measurements, error) {
	if !rpl.started {
		rpl.Start()
		rpl.started = true
	}

	nodeCount := int64(rpl.nodeSize)
	result := rpl.driver.GrabScanDataHq(rpl.nodes, &nodeCount)
	if !isResultOk(result) {
		return nil, errors.New("bad scan")
	}
	measurements := make(lrf.Measurements, 0, nodeCount)
	rpl.driver.AscendScanData(rpl.nodes, nodeCount)

	for pos := 0; pos < int(nodeCount); pos++ {
		node := rplidargen.MeasurementNodeHqArray_getitem(rpl.nodes, pos)
		if node.GetDist_mm_q2() == 0 {
			continue // TODO(erd): okay to skip?
		}

		nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
		nodeAngle = nodeAngle * math.Pi / 180
		nodeDistance := float64(node.GetDist_mm_q2()) / 4
		measurements = append(measurements, lrf.NewMeasurement(nodeAngle, nodeDistance/1000))
	}

	return measurements, nil
}
