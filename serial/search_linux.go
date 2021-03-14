// +build linux

package serial

import (
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/robotcore/usb"
)

func searchUSB(filter SearchFilter) ([]DeviceDescription, error) {
	usbDevices, err := usb.SearchDevices(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != DeviceTypeUnknown
		})
	if err != nil {
		return nil, err
	}
	serialDeviceDecss := make([]DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDecss = append(serialDeviceDecss, DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDecss, nil
}

var SearchDevices = func(filter SearchFilter) ([]DeviceDescription, error) {
	serialDeviceDecss, err := searchUSB(filter)
	if err != nil {
		return nil, err
	}

	if filter.Type != "" && filter.Type != DeviceTypeJetson {
		return serialDeviceDecss, nil
	}
	devicesDir, err := os.Open("/dev")
	if err != nil {
		return nil, err
	}
	defer devicesDir.Close()
	devices, err := devicesDir.Readdir(0)
	if err != nil {
		return nil, err
	}
	for _, dev := range devices {
		if strings.HasPrefix(dev.Name(), "ttyTHS") {
			serialDeviceDecss = append(serialDeviceDecss, DeviceDescription{
				Type: DeviceTypeJetson,
				Path: filepath.Join("/dev", dev.Name()),
			})
		}
	}
	return serialDeviceDecss, nil
}
