package utils

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// TryReserveRandomPort attempts to "reserve" a random port for later use.
// It works by listening on a TCP port and immediately closing that listener.
// In most contexts this is reliable if the port is immediately used after and
// there is not much port churn. Typically an OS will monotonically increase the
// port numbers it assigns.
func TryReserveRandomPort() (port int, err error) {
	//nolint:gosec
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
// It only returns IPv4 addresses, and tries not to return any loopback addresses.
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

// ErrInsufficientX509KeyPair is returned when an incomplete X509 key pair is used.
var ErrInsufficientX509KeyPair = errors.New("must provide both cert and key of an X509 key pair, not just one part")

const defaultListenAddress = "localhost:"

// NewPossiblySecureTCPListenerFromFile returns a TCP listener at the given address that is
// either insecure or TLS based listener depending on presence of the tlsCertFile and tlsKeyFile
// which are expected to be an X509 key pair. If no address is specified, the listener will bind
// to localhost IPV4 on a random port.
func NewPossiblySecureTCPListenerFromFile(address, tlsCertFile, tlsKeyFile string) (net.Listener, bool, error) {
	if (tlsCertFile == "") != (tlsKeyFile == "") {
		return nil, false, ErrInsufficientX509KeyPair
	}

	if address == "" {
		address = defaultListenAddress
	}
	if tlsCertFile == "" || tlsKeyFile == "" {
		insecureListener, err := net.Listen("tcp", address)
		if err != nil {
			return nil, false, err
		}
		return insecureListener, false, nil
	}
	cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
	if err != nil {
		return nil, false, err
	}
	return newTLSListener(address, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})
}

// NewPossiblySecureTCPListenerFromMemory returns a TCP listener at the given address that is
// either insecure or TLS based listener depending on presence of the tlsCertPEM and tlsKeyPEM
// which are expected to be an X509 key pair. If no address is specified, the listener will bind
// to localhost IPV4 on a random port.
func NewPossiblySecureTCPListenerFromMemory(address string, tlsCertPEM, tlsKeyPEM []byte) (net.Listener, bool, error) {
	if (len(tlsCertPEM) == 0) != (len(tlsKeyPEM) == 0) {
		return nil, false, ErrInsufficientX509KeyPair
	}

	if address == "" {
		address = defaultListenAddress
	}
	if len(tlsCertPEM) == 0 || len(tlsKeyPEM) == 0 {
		insecureListener, err := net.Listen("tcp", address)
		if err != nil {
			return nil, false, err
		}
		return insecureListener, false, nil
	}
	cert, err := tls.X509KeyPair(tlsCertPEM, tlsKeyPEM)
	if err != nil {
		return nil, false, err
	}
	return newTLSListener(address, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})
}

// NewPossiblySecureTCPListenerFromConfig returns a TCP listener at the given address that is
// either insecure or TLS based listener depending on presence of certificates in the given
// TLS Config. If no address is specified, the listener will bind to localhost IPV4 on a random port.
func NewPossiblySecureTCPListenerFromConfig(address string, tlsConfig *tls.Config) (net.Listener, bool, error) {
	if address == "" {
		address = defaultListenAddress
	}
	if len(tlsConfig.Certificates) == 0 {
		// try getting it a different way
		if _, err := tlsConfig.GetCertificate(&tls.ClientHelloInfo{}); err != nil {
			insecureListener, err := net.Listen("tcp", address)
			if err != nil {
				return nil, false, err
			}
			return insecureListener, false, nil
		}
	}
	return newTLSListener(address, tlsConfig)
}

func newTLSListener(address string, config *tls.Config) (net.Listener, bool, error) {
	cloned := config.Clone()
	if cloned.MinVersion == 0 {
		cloned.MinVersion = tls.VersionTLS12
	}
	secureListener, err := tls.Listen("tcp", address, cloned)
	if err != nil {
		return nil, false, err
	}
	return secureListener, true, nil
}
