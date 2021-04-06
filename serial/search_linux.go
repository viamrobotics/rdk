// +build linux

package serial

import (
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/robotcore/usb"
)

func searchUSB(filter SearchFilter) []DeviceDescription {
	usbDevices := usb.SearchDevices(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != DeviceTypeUnknown
		})
	serialDeviceDescs := make([]DeviceDescription, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDescs = append(serialDeviceDescs, DeviceDescription{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDescs
}

var devPath = "/dev"

var SearchDevices = func(filter SearchFilter) []DeviceDescription {
	serialDeviceDescs := searchUSB(filter)
	if filter.Type != "" && filter.Type != DeviceTypeJetson {
		return serialDeviceDescs
	}
	devicesDir, err := os.Open(devPath)
	if err != nil {
		return serialDeviceDescs
	}
	defer devicesDir.Close()
	devices, err := devicesDir.Readdir(0)
	if err != nil {
		return serialDeviceDescs
	}
	for _, dev := range devices {
		if strings.HasPrefix(dev.Name(), "ttyTHS") {
			serialDeviceDescs = append(serialDeviceDescs, DeviceDescription{
				Type: DeviceTypeJetson,
				Path: filepath.Join(devPath, dev.Name()),
			})
		}
	}
	return serialDeviceDescs
}
