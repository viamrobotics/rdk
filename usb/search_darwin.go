// +build darwin

package usb

import (
	"os/exec"

	"howett.net/plist"
)

type SearchFilter struct {
	ioObjectClass string
	ioTTYBaseName string
}

func NewSearchFilter(ioObjectClass, ioTTYBaseName string) SearchFilter {
	return SearchFilter{
		ioObjectClass: ioObjectClass,
		ioTTYBaseName: ioTTYBaseName,
	}
}

var SearchCmd = func(ioObjectClass string) []byte {
	cmd := exec.Command("ioreg", "-r", "-c", ioObjectClass, "-a", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

func SearchDevices(filter SearchFilter, includeDevice func(vendorID, productID int) bool) []DeviceDescription {
	out := SearchCmd(filter.ioObjectClass)
	if len(out) == 0 {
		return nil
	}
	var data []map[string]interface{}
	if _, err := plist.Unmarshal(out, &data); err != nil {
		return nil
	}
	var results []DeviceDescription
	for _, device := range data {
		if device["IOTTYBaseName"] != filter.ioTTYBaseName {
			continue
		}
		idVendor, ok := device["idVendor"].(uint64)
		if !ok {
			continue
		}
		idProduct, ok := device["idProduct"].(uint64)
		if !ok {
			continue
		}
		vendorID, productID := int(idVendor), int(idProduct)
		if includeDevice == nil || !includeDevice(vendorID, productID) {
			continue
		}

		children, ok := device["IORegistryEntryChildren"].([]interface{})
		if !ok {
			continue
		}
		var dialinDevice string
		for _, child := range children {
			childM, ok := child.(map[string]interface{})
			if !ok {
				continue
			}
			dialinDevice, ok = childM["IODialinDevice"].(string)
			if !ok {
				continue
			}
			if dialinDevice != "" {
				break
			}
		}
		if dialinDevice != "" {
			results = append(results, DeviceDescription{
				ID: Identifier{
					Vendor:  vendorID,
					Product: productID,
				},
				Path: dialinDevice,
			})
		}
	}
	return results
}
