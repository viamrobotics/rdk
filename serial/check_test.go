package serial

import (
	"testing"

	"go.viam.com/test"
)

func TestCheckProductDeviceIDs(t *testing.T) {
	test.That(t, checkProductDeviceIDs(0x0, 0x0), test.ShouldEqual, DeviceTypeUnknown)
	test.That(t, checkProductDeviceIDs(0x2341, 0x0), test.ShouldEqual, DeviceTypeUnknown)
	test.That(t, checkProductDeviceIDs(0x0, 0x0043), test.ShouldEqual, DeviceTypeUnknown)
	test.That(t, checkProductDeviceIDs(0x2341, 0x0043), test.ShouldEqual, DeviceTypeArduino)
}
