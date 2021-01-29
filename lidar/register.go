package lidar

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

var registrations = map[DeviceType]DeviceTypeRegistration{}
var registrationsMu sync.Mutex

type DeviceTypeRegistration struct {
	New func(desc DeviceDescription) (Device, error)
}

func RegisterDeviceType(deviceType DeviceType, reg DeviceTypeRegistration) {
	registrationsMu.Lock()
	registrations[deviceType] = reg
	registrationsMu.Unlock()
}

func CreateDevice(desc DeviceDescription) (Device, error) {
	reg, ok := registrations[desc.Type]
	if !ok {
		return nil, fmt.Errorf("do not know how to create a %q device", desc.Type)
	}
	return reg.New(desc)
}

func CreateDevices(deviceDescs []DeviceDescription) ([]Device, error) {
	var wg sync.WaitGroup
	wg.Add(len(deviceDescs))
	devices := make([]Device, len(deviceDescs))
	errs := make([]error, len(deviceDescs))
	var numErrs int32
	for i, devDesc := range deviceDescs {
		savedI, savedDesc := i, devDesc
		go func() {
			defer wg.Done()
			i, devDesc := savedI, savedDesc
			dev, err := CreateDevice(devDesc)
			if err != nil {
				errs[i] = err
				atomic.AddInt32(&numErrs, 1)
				return
			}
			devices[i] = dev
		}()
	}
	wg.Wait()

	if numErrs != 0 {
		var allErrs []interface{}
		for i, err := range errs {
			if err == nil {
				devices[i].Close()
				continue
			}
			allErrs = append(allErrs, err)
		}
		return nil, fmt.Errorf("encountered errors:"+strings.Repeat(" %w", len(allErrs)), allErrs...)
	}

	for _, dev := range devices {
		dev.Start()
	}

	return devices, nil
}
