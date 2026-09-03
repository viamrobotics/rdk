//go:build !linux

package videosource

// initializeDrivers is a no-op on non-Linux platforms. The mediadevices camera package registers
// the platform camera drivers from its init function; only Linux uses RDK's own V4L2 driver
// (see v4l2_driver_linux.go for why).
func initializeDrivers() {}
