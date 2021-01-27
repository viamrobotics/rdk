// +build !linux,!darwin

package support

func DetectLidarDevices() []DetectLidarDevicePaths {
	return nil
}
