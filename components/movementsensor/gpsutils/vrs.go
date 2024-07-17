// Package gpsutils implements functions that are used in the gpsrtkserial and gpsrtkpmtk.
package gpsutils

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"

	"go.viam.com/rdk/logging"
)

// ConnectToVirtualBase is responsible for establishing a connection to
// a virtual base station using the NTRIP protocol with enhanced error handling and retries.
func ConnectToVirtualBase(ntripInfo *NtripInfo, logger logging.Logger) (*bufio.ReadWriter, net.Conn, error) {
	mp := "/" + ntripInfo.MountPoint
	credentials := ntripInfo.username + ":" + ntripInfo.password
	credentialsBase64 := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Process the server URL
	serverAddr, err := url.Parse(ntripInfo.url)
	if err != nil {
		return nil, nil, err
	}

	conn, err := net.Dial("tcp", serverAddr.Host)
	if err != nil {
		logger.Errorf("Failed to connect to server %s: %v", serverAddr, err)

		return nil, nil, err
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	// Construct HTTP headers with CRLF line endings
	httpHeaders := "GET " + mp + " HTTP/1.1\r\n" +
		"Host: " + serverAddr.Host + "\r\n" +
		"Authorization: Basic " + credentialsBase64 + "\r\n" +
		"Accept: */*\r\n" +
		"Ntrip-Version: Ntrip/2.0\r\n" +
		"User-Agent: NTRIP viam\r\n\r\n"

	// Send HTTP headers over the TCP connection
	_, err = rw.Write([]byte(httpHeaders))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send HTTP headers: %w %w", err, conn.Close())
	}
	err = rw.Flush()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write to buffer: %w  %w", err, conn.Close())
	}

	logger.Debugf("request header: %v\n", httpHeaders)
	logger.Debug("HTTP headers sent successfully.")
	return rw, conn, nil
}

// HasVRSStream returns the NMEA field associated with the given mountpoint
// and whether it is a Virtual Reference Station.
func HasVRSStream(sourceTable *Sourcetable, mountPoint string) (bool, error) {
	stream, isFound := sourceTable.HasStream(mountPoint)

	if !isFound {
		return false, fmt.Errorf("can not find mountpoint %s in sourcetable", mountPoint)
	}

	return stream.Nmea, nil
}
