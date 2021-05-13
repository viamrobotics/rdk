package usb

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestSearch(t *testing.T) {
	goodFilter := NewSearchFilter("IOUserSerial", "usbserial-")
	goodInclude := func(vendorID, productID int) bool {
		return true
	}
	for i, tc := range []struct {
		Filter        SearchFilter
		IncludeDevice func(vendorID, productID int) bool
		Output        string
		Expected      []Description
	}{
		{SearchFilter{}, nil, "", nil},
		{SearchFilter{}, nil, "text", nil},
		{SearchFilter{}, goodInclude, "text", nil},
		{goodFilter, nil, out1, nil},
		{NewSearchFilter("IOUserSerial", "usbserial-2"), nil, out1, nil},
		{goodFilter, func(vendorID, productID int) bool {
			return true
		}, out1, []Description{
			{ID: Identifier{Vendor: 4292, Product: 60000}, Path: "/dev/tty.usbserial-0001"}},
		},
		{goodFilter, func(vendorID, productID int) bool {
			return vendorID == 4292 && productID == 60000
		}, out1, []Description{
			{ID: Identifier{Vendor: 4292, Product: 60000}, Path: "/dev/tty.usbserial-0001"}},
		},
		{goodFilter, func(vendorID, productID int) bool {
			return false
		}, out1, nil},
		{NewSearchFilter("IOUserSerial", "usbserial-2"), goodInclude, out1, nil},
		{goodFilter, goodInclude, out2, nil},
		{goodFilter, goodInclude, out3, nil},
		{goodFilter, goodInclude, out4, nil},
		{goodFilter, goodInclude, out5, nil},
		{goodFilter, goodInclude, out6, nil},
		{goodFilter, goodInclude, out7, nil},
		{goodFilter, goodInclude, out8, nil},
		{goodFilter, goodInclude, out9, nil},
		{goodFilter, goodInclude, out10, nil},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			prevSearchCmd := SearchCmd
			defer func() {
				SearchCmd = prevSearchCmd
			}()
			SearchCmd = func(ioObjectClass string) []byte {
				test.That(t, ioObjectClass, test.ShouldEqual, tc.Filter.ioObjectClass)
				return []byte(tc.Output)
			}
			result := Search(tc.Filter, tc.IncludeDevice)
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
			<integer>60000</integer>
			<key>idVendor</key>
			<integer>4292</integer>
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
				<string>usbserial-</string>
				<key>IOTTYDevice</key>
				<string>usbserial-0001</string>
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
		<string>usbserial-</string>
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
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
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
		<integer>4292</integer>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
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
		<string>usbserial-</string>
		<key>idProduct</key>
		<string>usbserial-</string>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out5 = `<plist version="1.0">
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
		<string>usbserial-</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<string>usbserial-</string>
	</dict>
</array>
</plist>
`

	out6 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<string>usbserial-</string>
		<key>IOTTYBaseName</key>
		<string>usbserial-</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out7 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>IODialinDevice</key>
				<string></string>
			</dict>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbserial-</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out8 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<dict>
				<key>IODialinDevice</key>
				<integer>4292</integer>
			</dict>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbserial-</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out9 = `<plist version="1.0">
<array>
	<dict>
		<key>IORegistryEntryChildren</key>
		<array>
			<integer>4292</integer>
		</array>
		<key>IOTTYBaseName</key>
		<string>usbserial-</string>
		<key>idProduct</key>
		<integer>60000</integer>
		<key>idVendor</key>
		<integer>4292</integer>
	</dict>
</array>
</plist>
`

	out10 = `<plist version="1.0">`
)
