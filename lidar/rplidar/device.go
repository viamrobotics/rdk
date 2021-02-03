package rplidar

import (
	"errors"
	"fmt"
	"image"
	"sync"

	"github.com/viamrobotics/robotcore/lidar"
	rplidargen "github.com/viamrobotics/robotcore/lidar/rplidar/gen"
	"github.com/viamrobotics/robotcore/utils"
)

const ModelName = "rplidar"
const DeviceType = lidar.DeviceType("RPLidar")

func init() {
	lidar.RegisterDeviceType(DeviceType, lidar.DeviceTypeRegistration{
		New: func(desc lidar.DeviceDescription) (lidar.Device, error) {
			return NewDevice(desc.Path)
		},
	})
}

type Result uint32
type ResultError struct {
	Result
}

var (
	ResultOk                 = Result(rplidargen.RESULT_OK)
	ResultAlreadyDone        = Result(rplidargen.RESULT_ALREADY_DONE)
	ResultInvalidData        = Result(rplidargen.RESULT_INVALID_DATA)
	ResultOpFail             = Result(rplidargen.RESULT_OPERATION_FAIL)
	ResultOpTimeout          = Result(rplidargen.RESULT_OPERATION_TIMEOUT)
	ResultOpStop             = Result(rplidargen.RESULT_OPERATION_STOP)
	ResultOpNotSupported     = Result(rplidargen.RESULT_OPERATION_NOT_SUPPORT)
	ResultFormatNotSupported = Result(rplidargen.RESULT_FORMAT_NOT_SUPPORT)
	ResultInsufficientMemory = Result(rplidargen.RESULT_INSUFFICIENT_MEMORY)
)

func (r Result) Failed() error {
	if int(r)&rplidargen.RESULT_FAIL_BIT == 0 {
		return nil
	}
	return ResultError{r}
}

func (r Result) String() string {
	switch r {
	case ResultOk:
		return "Ok"
	case ResultAlreadyDone:
		return "AlreadyDone"
	case ResultInvalidData:
		return "InvalidData"
	case ResultOpFail:
		return "OpFail"
	case ResultOpTimeout:
		return "OpTimeout"
	case ResultOpStop:
		return "OpStop"
	case ResultOpNotSupported:
		return "OpNotSupported"
	case ResultFormatNotSupported:
		return "FormatNotSupported"
	case ResultInsufficientMemory:
		return "InsufficientMemory"
	default:
		return "Unknown"
	}
}

func (r ResultError) Error() string {
	return r.String()
}

const defaultTimeout = uint(1000)

func NewDevice(devicePath string) (*RPLidar, error) {
	var driver rplidargen.RPlidarDriver
	devInfo := rplidargen.NewRplidar_response_device_info_t()
	defer rplidargen.DeleteRplidar_response_device_info_t(devInfo)

	var connectErr error
	for _, rate := range []uint{256000, 115200} {
		possibleDriver := rplidargen.RPlidarDriverCreateDriver(uint(rplidargen.DRIVER_TYPE_SERIALPORT))
		if result := possibleDriver.Connect(devicePath, rate); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to connect: %w", Result(result).Failed())
			continue
		}

		if result := possibleDriver.GetDeviceInfo(devInfo, defaultTimeout); Result(result) != ResultOk {
			r := Result(result)
			if r == ResultOpTimeout {
				continue
			}
			connectErr = fmt.Errorf("failed to get device info: %w", Result(result).Failed())
			continue
		}

		driver = possibleDriver
		break
	}
	if driver == nil {
		if connectErr == nil {
			return nil, fmt.Errorf("timed out connecting to %q", devicePath)
		}
		return nil, connectErr
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

	if result := driver.GetHealth(healthInfo, defaultTimeout); Result(result) != ResultOk {
		return nil, fmt.Errorf("failed to get health: %w", Result(result).Failed())
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
	mu       sync.Mutex
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
	width := rpl.Range() * 2
	height := width
	bounds := image.Point{width, height}
	rpl.bounds = &bounds
	return bounds, nil
}

func (rpl *RPLidar) Start() {
	rpl.mu.Lock()
	defer rpl.mu.Unlock()
	rpl.started = true
	rpl.driver.StartMotor()
	rpl.driver.StartScan(false, true)
	rpl.nodes = rplidargen.New_measurementNodeHqArray(rpl.nodeSize)
}

func (rpl *RPLidar) Stop() {
	rpl.mu.Lock()
	defer rpl.mu.Unlock()
	if rpl.nodes != nil {
		defer func() {
			rplidargen.Delete_measurementNodeHqArray(rpl.nodes)
			rpl.nodes = nil
		}()
	}
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
	result := rpl.driver.GrabScanDataHq(rpl.nodes, &nodeCount, defaultTimeout)
	if Result(result) != ResultOk {
		return nil, fmt.Errorf("bad scan: %w", Result(result).Failed())
	}
	measurements := make(lidar.Measurements, 0, nodeCount)
	rpl.driver.AscendScanData(rpl.nodes, nodeCount)

	for pos := 0; pos < int(nodeCount); pos++ {
		node := rplidargen.MeasurementNodeHqArray_getitem(rpl.nodes, pos)
		if node.GetDist_mm_q2() == 0 {
			continue // TODO(erd): okay to skip?
		}

		nodeAngle := (float64(node.GetAngle_z_q14()) * 90 / (1 << 14))
		nodeAngle = utils.DegToRad(nodeAngle)
		nodeDistance := float64(node.GetDist_mm_q2()) / 4
		measurements = append(measurements, lidar.NewMeasurement(nodeAngle, nodeDistance/1000))
	}

	return measurements, nil
}
