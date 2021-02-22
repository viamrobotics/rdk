package mti

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/viamrobotics/robotcore/sensor/compass"
	mtigen "github.com/viamrobotics/robotcore/sensor/compass/mti/gen"
)

type Device struct {
	control   mtigen.XsControl
	device    mtigen.XSDevice
	callback  mtigen.CallbackHandler
	heading   atomic.Value
	closeCh   chan struct{}
	closeOnce sync.Once
}

func New(deviceID string, path string, baudRate int) (compass.Device, error) {
	control := mtigen.XsControlConstruct()

	portInfoArray := mtigen.XSScannerScanPorts()
	portInfoArrayPtr := mtigen.SwigcptrXsArrayXsPortInfo(portInfoArray.Swigcptr())
	mtPort := portInfoArrayPtr.First()

	golog.Global.Infow("found device",
		"id", mtPort.DeviceId().ToString().ToStdString(),
		"port", mtPort.PortName().ToStdString(),
		"baudrate", mtPort.Baudrate(),
	)

	var useBaudRate mtigen.XsBaudRate
	switch baudRate {
	case 115200:
		useBaudRate = mtigen.XBR_115k2
	default:
		return nil, fmt.Errorf("unknown baudrate %d", baudRate)
	}

	pathStr := mtigen.NewXSString(path)
	defer mtigen.DeleteXSString(pathStr)
	if !control.OpenPort(pathStr, useBaudRate) {
		defer control.Destruct()
		return nil, errors.New("failed to open port")
	}

	devID := mtigen.NewXSDeviceId()
	defer mtigen.DeleteXSDeviceId(devID)
	devIDStr := mtigen.NewXSString(deviceID)
	defer mtigen.DeleteXSString(devIDStr)
	devID.FromString(devIDStr)

	device := control.Device(devID)
	if device.Swigcptr() == 0 {
		defer control.Destruct()
		return nil, errors.New("expected device")
	}

	callback := mtigen.NewCallbackHandler()
	mtigen.AddCallbackHandler(callback, device)

	if !device.GotoMeasurement() {
		return nil, errors.New("failed to go to measurement mode")
	}

	d := &Device{
		control:  control,
		device:   device,
		callback: callback,
	}
	d.heading.Store(math.NaN())

	d.closeCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-d.closeCh:
				return
			case <-ticker.C:
			}

			if callback.PacketAvailable() {
				packet := callback.GetNextPacket()
				if packet.ContainsOrientation() {
					euler := packet.OrientationEuler()
					if yaw := euler.Yaw(); !math.IsNaN(yaw) {
						d.heading.Store(yaw)
					}
				}
			}

		}
	}()
	return d, nil
}

func (d *Device) StartCalibration() error {
	return nil
}

func (d *Device) StopCalibration() error {
	return nil
}

func (d *Device) Readings() ([]interface{}, error) {
	heading, err := d.Heading()
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (d *Device) Heading() (float64, error) {
	return d.heading.Load().(float64), nil
}

func (d *Device) Close() error {
	d.closeOnce.Do(func() {
		close(d.closeCh)
		defer d.control.Destruct()
		defer mtigen.DeleteCallbackHandler(d.callback)
	})
	return nil
}
