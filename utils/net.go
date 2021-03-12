package utils

import (
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
