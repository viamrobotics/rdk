// Package rtkutils implements functions that are used in the gpsrtkserial and gpsrtkpmtk.
package rtkutils

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/de-bkg/gognss/pkg/ntrip"

	"go.viam.com/rdk/logging"
)

// ConnectToVirtualBase is responsible for establishing a connection to
// a virtual base station using the NTRIP protocol.
func ConnectToVirtualBase(ntripInfo *NtripInfo,
	logger logging.Logger,
) *bufio.ReadWriter {
	mp := "/" + ntripInfo.MountPoint
	credentials := ntripInfo.Username + ":" + ntripInfo.Password
	credentialsBase64 := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Process the server URL
	serverAddr := ntripInfo.URL
	serverAddr = strings.TrimPrefix(serverAddr, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Construct HTTP headers with CRLF line endings
	httpHeaders := "GET " + mp + " HTTP/1.1\r\n" +
		"Host: " + serverAddr + "\r\n" +
		"Authorization: Basic " + credentialsBase64 + "\r\n" +
		"Accept: */*\r\n" +
		"Ntrip-Version: Ntrip/2.0\r\n" +
		"User-Agent: NTRIP viam\r\n\r\n"

	// Send HTTP headers over the TCP connection
	_, err = rw.Write([]byte(httpHeaders))
	if err != nil {
		logger.Error("Failed to send HTTP headers:", err)
		return nil
	}
	err = rw.Flush()
	if err != nil {
		logger.Error("failed to write to buffer")
		return nil
	}

	logger.Debugf("request header: %v\n", httpHeaders)
	logger.Debug("HTTP headers sent successfully.")
	return rw
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

// FindLineWithMountPoint parses the given source-table returns the NMEA field associated with
// the given mountpoint.
func FindLineWithMountPoint(sourceTable *ntrip.Sourcetable, mountPoint string) (bool, error) {
	stream, isFound := sourceTable.HasStream(mountPoint)

	if !isFound {
		return false, fmt.Errorf("can not find mountpoint %s in sourcetable", mountPoint)
	}

	return stream.Nmea, nil
}
