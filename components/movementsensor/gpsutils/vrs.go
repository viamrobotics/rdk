// Package gpsutils implements functions that are used in the gpsrtkserial and gpsrtkpmtk.
package gpsutils

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"go.viam.com/rdk/logging"
)

// ConnectToVirtualBase is responsible for establishing a connection to
// a virtual base station using the NTRIP protocol.
func ConnectToVirtualBase(ntripInfo *NtripInfo,
	logger logging.Logger,
) (*bufio.ReadWriter, error) {
	mp := "/" + ntripInfo.MountPoint
	credentials := ntripInfo.username + ":" + ntripInfo.password
	credentialsBase64 := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Process the server URL
	serverAddr, err := url.Parse(ntripInfo.URL)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("tcp", serverAddr.Host)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to send HTTP headers: %w", err)
	}
	err = rw.Flush()
	if err != nil {
		return nil, fmt.Errorf("failed to write to buffer: %w", err)
	}

	logger.Debugf("request header: %v\n", httpHeaders)
	logger.Debug("HTTP headers sent successfully.")
	return rw, nil
}

// GetGGAMessage checks if a GGA message exists in the buffer and returns it.
func GetGGAMessage(correctionWriter io.ReadWriteCloser, logger logging.Logger) ([]byte, error) {
	buffer := make([]byte, 1024)
	var totalBytesRead int

	for {
		n, err := correctionWriter.Read(buffer[totalBytesRead:])
		if err != nil {
			logger.Errorf("Error reading from Ntrip stream: %v", err)
			return nil, err
		}

		totalBytesRead += n

		// Check if the received data contains "GGA"
		if ContainsGGAMessage(buffer[:totalBytesRead]) {
			return buffer[:totalBytesRead], nil
		}

		// If we haven't found the "GGA" message, and we've reached the end of
		// the buffer, return error.
		if totalBytesRead >= len(buffer) {
			return nil, errors.New("GGA message not found in the received data")
		}
	}
}

// ContainsGGAMessage returns true if data contains GGA message.
func ContainsGGAMessage(data []byte) bool {
	dataStr := string(data)
	return strings.Contains(dataStr, "GGA")
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
