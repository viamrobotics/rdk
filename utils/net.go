package utils

import (
	"fmt"
	"net"

	"go.uber.org/multierr"
)

// TryReserveRandomPort attempts to "reserve" a random port for later use.
// It works by listening on a TCP port and immediately closing that listener.
// In most contexts this is reliable if the port is immediately used after and
// there is not much port churn. Typically an OS will monotonically increase the
// port numbers it assigns.
func TryReserveRandomPort() (port int, err error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func() {
		err = multierr.Combine(err, listener.Close())
	}()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// GetAllLocalIPv4s finds all the local ips from all interfaces
// It only returns IPv4 addresses, and tries not to return any loopback addresses
func GetAllLocalIPv4s() ([]string, error) {

	allInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	all := []string{}

	for _, i := range allInterfaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.IsLoopback() {
					continue
				}

				ones, bits := v.Mask.Size()
				if bits != 32 {
					// this is what limits to ipv4
					continue
				}

				if ones == bits {
					// this means it's a loopback of some sort
					// likely a bridge to parallels or docker or something
					continue
				}

				all = append(all, v.IP.String())
			default:
				return nil, fmt.Errorf("unknow address type: %T", v)
			}
		}

	}

	return all, nil
}
