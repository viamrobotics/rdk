//go:build linux
// +build linux

package usb

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.viam.com/utils"
)

// SysPaths are where we search for devices. This can be changed for tests.
var SysPaths = []string{"/sys/bus/usb-serial/devices", "/sys/bus/usb/drivers/cdc_acm"}

// SearchFilter does not do anything for linux.
type SearchFilter struct{}

// Search uses linux device APIs to find all applicable USB devices.
func Search(filter SearchFilter, includeDevice func(vendorID, productID int) bool) []Description {
	if includeDevice == nil {
		return nil
	}
	searchPath := func(sysPath string) []Description {
		devicesDir, err := os.Open(sysPath)
		if err != nil {
			return nil
		}
		defer utils.UncheckedErrorFunc(devicesDir.Close)
		devices, err := devicesDir.Readdir(0)
		if err != nil {
			return nil
		}
		var results []Description
		for _, device := range devices {
			linkedFile, err := os.Readlink(filepath.Join(sysPath, device.Name()))
			if err != nil {
				continue
			}
			if !filepath.IsAbs(linkedFile) {
				linkedFile = filepath.Join(sysPath, linkedFile)
			}
			ueventFile, err := os.Open(filepath.Join(linkedFile, "../uevent"))
			if err != nil {
				continue
			}
			defer utils.UncheckedErrorFunc(ueventFile.Close)
			ttyFile, err := os.Open(filepath.Join(linkedFile, "./tty"))
			if err != nil {
				continue
			}
			defer utils.UncheckedErrorFunc(ttyFile.Close)
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
				if !includeDevice(int(vendorID), int(productID)) {
					continue
				}
				results = append(results, Description{
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
	var allDevices []Description
	for _, sysPath := range SysPaths {
		allDevices = append(allDevices, searchPath(sysPath)...)
	}
	return allDevices
}
