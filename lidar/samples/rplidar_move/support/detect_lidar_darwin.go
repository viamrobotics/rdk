// +build darwin

package support

import (
	"os/exec"

	"howett.net/plist"
)

func DetectLidarDevices() []LidarDeviceDescription {
	cmd := exec.Command("ioreg", "-r", "-c", "IOUserSerial", "-a", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var data []map[string]interface{}
	if _, err := plist.Unmarshal(out, &data); err != nil {
		return nil
	}
	var results []LidarDeviceDescription
	for _, device := range data {
		if device["IOTTYBaseName"] != "usbserial-" {
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
		lidarType := checkProductLidarDevice(int(idVendor), int(idProduct))
		if lidarType == LidarTypeUnknown {
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
			results = append(results, LidarDeviceDescription{lidarType, dialinDevice})
		}
	}
	return results
}
