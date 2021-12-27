package serial

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/usb"
)

func TestSearch(t *testing.T) {
	for i, tc := range []struct {
		Filter   SearchFilter
		Output   string
		Expected []Description
	}{
		{SearchFilter{}, "", nil},
		{SearchFilter{}, "text", nil},
		{
			SearchFilter{}, out1, []Description{
				{Type: TypeArduino, Path: "/dev/tty.usbserial-0001"},
			},
		},
		{
			SearchFilter{Type: TypeArduino}, out1, []Description{
				{Type: TypeArduino, Path: "/dev/tty.usbserial-0001"},
			},
		},
		{SearchFilter{Type: TypeJetson}, out1, nil},
		{SearchFilter{}, out2, nil},
		{
			SearchFilter{}, out3, []Description{
				{Type: TypeArduino, Path: "/dev/tty.usbserial-0001"},
			},
		},
		{SearchFilter{}, out4, nil},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			prevSearchCmd := usb.SearchCmd
			defer func() {
				usb.SearchCmd = prevSearchCmd
			}()
			usb.SearchCmd = func(ioObjectClass string) []byte {
				test.That(t, ioObjectClass, test.ShouldEqual, "AppleUSBACMData")
				return []byte(tc.Output)
			}
			result := Search(tc.Filter)
			test.That(t, result, test.ShouldResemble, tc.Expected)
		})
	}
}

const (
	out1 = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<array>
	<dict>
		<key>CFBundleIdentifier</key>
		<string>com.apple.DriverKit-AppleUSBSLCOM</string>
		<key>CFBundleIdentifierKernel</key>
		<string>com.apple.driver.driverkit.serial</string>
		<key>HiddenPort</key>
		<true/>
		<key>IOClass</key>
		<string>IOUserSerial</string>
		<key>IOMatchCategory</key>
		<string>IODefaultMatchCategory</string>
		<key>IOMatchDefer</key>
		<true/>
		<key>IOMatchedPersonality</key>
		<dict>
			<key>CFBundleIdentifier</key>
			<string>com.apple.DriverKit-AppleUSBSLCOM</string>
			<key>CFBundleIdentifierKernel</key>
			<string>com.apple.driver.driverkit.serial</string>
			<key>IOClass</key>
			<string>IOUserSerial</string>
			<key>IOMatchDefer</key>
			<true/>
			<key>IOPersonalityPublisher</key>
			<string>com.apple.DriverKit-AppleUSBSLCOM</string>
			<key>IOProviderClass</key>
			<string>IOUSBHostInterface</string>
			<key>IOUserClass</key>
			<string>AppleUSBSLCOM</string>
			<key>IOUserServerCDHash</key>
			<string>22f46d43db2d7218c847e6697391d7b7ca44d90e</string>
			<key>IOUserServerName</key>
			<string>com.apple.driverkit.AppleUSBSLCOM</string>
			<key>bConfigurationValue</key>
			<integer>1</integer>
			<key>bInterfaceNumber</key>
			<integer>0</integer>
			<key>idProduct</key>
			<integer>67</integer>
			<key>idVendor</key>
			<integer>9025</integer>
		</dict>
		<key>IOObjectClass</key>
		<string>IOUserSerial</string>
		<key>IOObjectRetainCount</key>
		<integer>11</integer>
		<key>IOPersonalityPublisher</key>
		<string>com.apple.DriverKit-AppleUSBSLCOM</string>
		<key>IOPowerManagement</key>
		<dict>
			<key>CapabilityFlags</key>
			<integer>2</integer>
			<key>CurrentPowerState</key>
			<integer>2</integer>
			<key>MaxPowerState</key>
			<integer>2</integer>
		</dict>
		<key>IOProbeScore</key>
		<integer>89999</integer>
		<key>IOProviderClass</key>
		<string>IOUSBHostInterface</string>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>CFBundleIdentifier</key>
				<string>com.apple.iokit.IOSerialFamily</string>
				<key>CFBundleIdentifierKernel</key>
				<string>com.apple.iokit.IOSerialFamily</string>
				<key>IOCalloutDevice</key>
				<string>/dev/cu.usbserial-0001</string>
				<key>IOClass</key>
				<string>IOSerialBSDClient</string>
				<key>IODialinDevice</key>
				<string>/dev/tty.usbserial-0001</string>
				<key>IOMatchCategory</key>
				<string>IODefaultMatchCategory</string>
				<key>IOObjectClass</key>
				<string>IOSerialBSDClient</string>
				<key>IOObjectRetainCount</key>
				<integer>5</integer>
				<key>IOPersonalityPublisher</key>
				<string>com.apple.iokit.IOSerialFamily</string>
				<key>IOProbeScore</key>
				<integer>1000</integer>
				<key>IOProviderClass</key>
				<string>IOSerialStreamSync</string>
				<key>IORegistryEntryID</key>
				<integer>4295056527</integer>
				<key>IORegistryEntryName</key>
				<string>IOSerialBSDClient</string>
				<key>IOResourceMatch</key>
				<string>IOBSD</string>
				<key>IOSerialBSDClientType</key>
				<string>IOSerialStream</string>
				<key>IOServiceBusyState</key>
				<integer>0</integer>
				<key>IOServiceBusyTime</key>
				<integer>681250</integer>
				<key>IOServiceState</key>
				<integer>30</integer>
				<key>IOTTYBaseName</key>
				<string>usbmodem</string>
				<key>IOTTYDevice</key>
				<string>usbmodem-0001</string>
				<key>IOTTYSuffix</key>
				<string>0001</string>
			</dict>
		</array>
		<key>IORegistryEntryID</key>
		<integer>4295056520</integer>
		<key>IORegistryEntryName</key>
		<string>AppleUSBSLCOM</string>
		<key>IOServiceBusyState</key>
		<integer>0</integer>
		<key>IOServiceBusyTime</key>
		<integer>926125</integer>
		<key>IOServiceDEXTEntitlements</key>
		<string>com.apple.developer.driverkit.family.serial</string>
		<key>IOServiceState</key>
		<integer>30</integer>
		<key>IOTTYBaseName</key>
		<string>usbmodem</string>
		<key>IOTTYSuffix</key>
		<string>0001</string>
		<key>IOUserClass</key>
		<string>AppleUSBSLCOM</string>
		<key>IOUserServerCDHash</key>
		<string>22f46d43db2d7218c847e6697391d7b7ca44d90e</string>
		<key>IOUserServerName</key>
		<string>com.apple.driverkit.AppleUSBSLCOM</string>
		<key>bConfigurationValue</key>
		<integer>1</integer>
		<key>bInterfaceNumber</key>
		<integer>0</integer>
		<key>idProduct</key>
		<integer>67</integer>
		<key>idVendor</key>
		<integer>9025</integer>
	</dict>
</array>
</plist>
`

	out2 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>IODialinDevice</key>
				<string>/dev/tty.usbserial-0001</string>
			</dict>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbserial</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out3 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>IODialinDevice</key>
				<string>/dev/tty.usbserial-0001</string>
			</dict>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbmodem</string>
		<key>idProduct</key>
		<integer>68</integer>
		<key>idVendor</key>
		<integer>9025</integer>
	</dict>
</array>
</plist>
`

	out4 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>IODialinDevice</key>
				<string>/dev/tty.usbserial-0001</string>
			</dict>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbmodem</string>
		<key>idProduct</key>
		<integer>67</integer>
		<key>idVendor</key>
		<integer>9026</integer>
	</dict>
</array>
</plist>
`
)
