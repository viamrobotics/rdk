package rtkutils

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"go.viam.com/rdk/logging"
)

type VirtualBase struct {
	mu          sync.Mutex
	isConnected bool
}

// ConnectToVirtualBase
func (v *VirtualBase) ConnectToVirtualBase(ntripInfo *NtripInfo,
	logger logging.Logger) (*bufio.ReadWriter, bool) {

	v.mu.Lock()
	defer v.mu.Unlock()

	mp := "/" + ntripInfo.MountPoint
	credentials := ntripInfo.Username + ":" + ntripInfo.Password
	credentialsBase64 := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Process the server URL
	serverAddr := ntripInfo.URL
	serverAddr = strings.TrimPrefix(serverAddr, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		v.isConnected = false
		logger.Errorf("Failed to connect to VRS server:", err)
		return nil, v.isConnected
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
		v.isConnected = false
		logger.Error("Failed to send HTTP headers:", err)
		return nil, v.isConnected
	}
	err = rw.Flush()
	if err != nil {
		v.isConnected = false
		logger.Error("failed to write to buffer")
		return nil, v.isConnected
	}

	logger.Debugf("request header: %v\n", httpHeaders)
	logger.Debug("HTTP headers sent successfully.")
	v.isConnected = true
	return rw, v.isConnected
}

// GetGGAMessage checks if a GGA message exists in the buffer and returns it.
func (v *VirtualBase) GetGGAMessage(correctionWriter io.ReadWriteCloser, logger logging.Logger) ([]byte, error) {

	v.mu.Lock()
	defer v.mu.Unlock()

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
