//go:build !linux

package genericlinux

type gpioPin struct {
	// This struct is implemented in the Linux version. We have a dummy struct here just to get
	// things to compile on non-Linux environments.
}

func gpioInitialize(gpioMappings map[int]GPIOBoardMapping) map[string]*gpioPin {
	// Don't even log anything here: if someone is running in a non-Linux environment, things
	// should work fine as long as they don't try using these pins, and the log would be an
	// unnecessary warning.
	return map[string]*gpioPin{}
}
