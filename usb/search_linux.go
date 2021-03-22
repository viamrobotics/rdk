// +build linux

package usb

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var sysPaths = []string{"/sys/bus/usb-serial/devices", "/sys/bus/usb/drivers/cdc_acm"}

type SearchFilter struct{}

func SearchDevices(filter SearchFilter, includeDevice func(vendorID, productID int) bool) []DeviceDescription {
	searchPath := func(sysPath string) []DeviceDescription {
		devicesDir, err := os.Open(sysPath)
		if err != nil {
			return nil
		}
		defer devicesDir.Close()
		devices, err := devicesDir.Readdir(0)
		if err != nil {
			return nil
		}
		var results []DeviceDescription
		for _, device := range devices {
			linkedFile, err := os.Readlink(path.Join(sysPath, device.Name()))
			if err != nil {
				continue
			}
			ueventFile, err := os.Open(filepath.Join(linkedFile, "../uevent"))
			if err != nil {
				continue
			}
			defer ueventFile.Close()
			ttyFile, err := os.Open(filepath.Join(linkedFile, "./tty"))
			if err != nil {
				continue
			}
			defer ttyFile.Close()
			ttys, err := ttyFile.Readdir(0)
			if err != nil {
				continue
			}
			if len(ttys) == 0 {
				continue
			}

			reader := bufio.NewReader(ueventFile)
		searchProduct:
			for {
				line, _, err := reader.ReadLine()
				if err != nil {
					break searchProduct
				}
				lineStr := string(line)
				const productPrefix = "PRODUCT="
				if !strings.HasPrefix(lineStr, productPrefix) {
					continue
				}
				productInfo := strings.TrimPrefix(lineStr, productPrefix)
				productInfoParts := strings.Split(productInfo, "/")
				if len(productInfoParts) < 2 {
					continue
				}
				vendorID, err := strconv.ParseInt(productInfoParts[0], 16, 64)
				if err != nil {
					continue
				}
				productID, err := strconv.ParseInt(productInfoParts[1], 16, 64)
				if err != nil {
					continue
				}
				if includeDevice == nil || !includeDevice(int(vendorID), int(productID)) {
					continue
				}
				results = append(results, DeviceDescription{
					ID: Identifier{
						Vendor:  int(vendorID),
						Product: int(productID),
					},
					Path: filepath.Join("/dev", ttys[0].Name()),
				})
			}
		}
		return results
	}
	var allDevices []DeviceDescription
	for _, sysPath := range sysPaths {
		allDevices = append(allDevices, searchPath(sysPath)...)
	}
	return allDevices
}
