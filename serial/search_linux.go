// +build linux

package serial

import (
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/core/usb"
	"go.viam.com/core/utils"
)

func searchUSB(filter SearchFilter) []Description {
	usbDevices := usb.Search(
		usb.SearchFilter{},
		func(vendorID, productID int) bool {
			return checkProductDeviceIDs(vendorID, productID) != TypeUnknown
		})
	serialDeviceDescs := make([]Description, 0, len(usbDevices))
	for _, dev := range usbDevices {
		devType := checkProductDeviceIDs(dev.ID.Vendor, dev.ID.Product)
		if filter.Type != "" && filter.Type != devType {
			continue
		}
		serialDeviceDescs = append(serialDeviceDescs, Description{
			Type: devType,
			Path: dev.Path,
		})
	}
	return serialDeviceDescs
}

var devPath = "/dev"

// Search uses linux device APIs to find all applicable serial devices.
// It's a variable in case you need to override it during tests.
var Search = func(filter SearchFilter) []Description {
	serialDeviceDescs := searchUSB(filter)
	if filter.Type != "" && filter.Type != TypeJetson {
		return serialDeviceDescs
	}
	devicesDir, err := os.Open(devPath)
	if err != nil {
		return serialDeviceDescs
	}
	defer utils.UncheckedError(devicesDir.Close())
	devices, err := devicesDir.Readdir(0)
	if err != nil {
		return serialDeviceDescs
	}
	for _, dev := range devices {
		if strings.HasPrefix(dev.Name(), "ttyTHS") {
			serialDeviceDescs = append(serialDeviceDescs, Description{
				Type: TypeJetson,
				Path: filepath.Join(devPath, dev.Name()),
			})
		}
	}
	return serialDeviceDescs
}
