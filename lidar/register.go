package lidar

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/viamrobotics/robotcore/usb"
	"go.uber.org/multierr"
)

var registrations = map[DeviceType]DeviceTypeRegistration{}
var registrationsMu sync.Mutex

type DeviceTypeRegistration struct {
	New     func(ctx context.Context, desc DeviceDescription) (Device, error)
	USBInfo *usb.Identifier
}

func RegisterDeviceType(deviceType DeviceType, reg DeviceTypeRegistration) {
	registrationsMu.Lock()
	registrations[deviceType] = reg
	registrationsMu.Unlock()
}

func CreateDevice(ctx context.Context, desc DeviceDescription) (Device, error) {
	reg, ok := registrations[desc.Type]
	if !ok {
		return nil, fmt.Errorf("do not know how to create a %q device", desc.Type)
	}
	return reg.New(ctx, desc)
}

func CreateDevices(ctx context.Context, deviceDescs []DeviceDescription) ([]Device, error) {
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
			dev, err := CreateDevice(ctx, devDesc)
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
		var allErrs error
		for i, err := range errs {
			if err == nil {
				if err := devices[i].Close(ctx); err != nil {
					allErrs = multierr.Append(allErrs, err)
				}
				continue
			}
			allErrs = multierr.Append(allErrs, err)
		}
		return nil, allErrs
	}

	for _, dev := range devices {
		if err := dev.Start(ctx); err != nil {
			return nil, fmt.Errorf("error starting: %w", err)
		}
	}

	return devices, nil
}

func CheckProductDeviceIDs(vendorID, productID int) DeviceType {
	registrationsMu.Lock()
	defer registrationsMu.Unlock()

	for t, reg := range registrations {
		if reg.USBInfo != nil &&
			reg.USBInfo.Vendor == vendorID && reg.USBInfo.Product == productID {
			return t
		}
	}
	return DeviceTypeUnknown
}
