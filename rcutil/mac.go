package rcutil

import (
	"fmt"
	"strings"
)

type USBSerialDevice struct {
	Name string
	Port string
}

func GetUSBSerialDevices() ([]USBSerialDevice, error) {
	// TODO(erh): finish
	return MacGetUSBSerialDevices()
}

func FindUSBSerialDevice(filter string) (USBSerialDevice, error) {

	all, err := GetUSBSerialDevices()
	if err != nil {
		return USBSerialDevice{}, err
	}

	// TODO(erh): should we error if there are multiple matching
	for _, d := range all {
		if strings.Contains(d.Name, filter) {
			return d, nil
		}
	}

	return USBSerialDevice{}, fmt.Errorf("cannot find a usb serial devices with filter: %s", filter)
}

// ----

func MacGetUSBSerialDevices() ([]USBSerialDevice, error) {

	lines, err := ExecuteShellCommand("/usr/sbin/ioreg", "-r", "-c", "IOUSBHostDevice", "-l")
	if err != nil {
		return nil, fmt.Errorf("cannot call ioreg %w", err)
	}

	devices := ioregSplit(lines)

	ports := []USBSerialDevice{}

	for _, d := range devices {
		name := strings.Split(d[0][3:], "<")[0]

		for _, l := range d {
			magic := "/dev/tty.usbmodem"
			res := strings.Split(l, magic)
			if len(res) == 1 {
				continue
			}

			ports = append(ports, USBSerialDevice{name, magic + strings.Split(res[1], "\"")[0]})
		}

	}

	return ports, nil
}

func ioregSplit(lines []string) [][]string {
	res := [][]string{}

	things := []int{}

	for idx, s := range lines {
		if strings.HasPrefix(s, "+-o") {
			things = append(things, idx)
		}
	}

	if len(things) == 0 {
		panic("bad ioreg output")
	}

	if len(things) == 1 {
		res = append(res, lines)
		return res
	}

	if things[0] != 0 {
		panic("bad ioreg output 2")
	}

	for idx, i := range things {
		if idx == len(things)-1 {
			res = append(res, lines[i:])
		} else {
			j := things[idx+1]
			res = append(res, lines[i:j])
		}
	}

	return res
}
