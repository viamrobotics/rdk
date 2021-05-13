// +build darwin

package usb

import (
	"os/exec"

	"howett.net/plist"
)

// SearchFilter describes a specific kind of USB device to look for as it
// pertains to macOS.
type SearchFilter struct {
	ioObjectClass string
	ioTTYBaseName string
}

// NewSearchFilter creates a new search filter with the given IOObjectClass and
// IOTTYBaseName.
func NewSearchFilter(ioObjectClass, ioTTYBaseName string) SearchFilter {
	return SearchFilter{
		ioObjectClass: ioObjectClass,
		ioTTYBaseName: ioTTYBaseName,
	}
}

// SearchCmd is the actual system command to run; it is normally ioreg.
var SearchCmd = func(ioObjectClass string) []byte {
	cmd := exec.Command("ioreg", "-r", "-c", ioObjectClass, "-a", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

// Search uses macOS io device APIs to find all applicable USB devices.
func Search(filter SearchFilter, includeDevice func(vendorID, productID int) bool) []Description {
	if includeDevice == nil {
		return nil
	}
	out := SearchCmd(filter.ioObjectClass)
	if len(out) == 0 {
		return nil
	}
	var data []map[string]interface{}
	if _, err := plist.Unmarshal(out, &data); err != nil {
		return nil
	}
	var results []Description
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
		if !includeDevice(vendorID, productID) {
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
			results = append(results, Description{
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
