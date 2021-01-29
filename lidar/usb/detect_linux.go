// +build linux

package usb

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/echolabsinc/robotcore/lidar"
)

const sysPath = "/sys/bus/usb-serial/devices"

func DetectDevices() []lidar.DeviceDescription {
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
		ueventFile, err := os.Open(filepath.Join(sysPath, linkedFile, "../uevent"))
		if err != nil {
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
			lidarType := checkUSBProductDevice(int(vendorID), int(productID))
			if lidarType == TypeUnknown {
				continue
			}
			results = append(results, DeviceDescription{
				lidarType, filepath.Join("/dev", device.Name())})
		}
	}
	return results
}
