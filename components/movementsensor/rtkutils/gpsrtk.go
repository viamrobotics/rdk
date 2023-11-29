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

// SendGGAMessage sends GGA messages to the NTRIP Caster over a TCP connection
// to get the NTRIP steam when the mount point is a Virtual Reference Station.
func SendGGAMessage(correctionWriter io.ReadWriteCloser,
	readerWriter *bufio.ReadWriter, isConnected bool,
	ntripInfo *NtripInfo,
	logger logging.Logger) error {
	if !isConnected {
		readerWriter, _ = ConnectToVirtualBase(ntripInfo.MountPoint, ntripInfo.Username,
			ntripInfo.Password, ntripInfo.URL, logger)
	}

	// read from the socket until we know if a successful connection has been
	// established.
	for {
		if readerWriter.Reader == nil || readerWriter.Writer == nil {
			break
		}
		line, _, err := readerWriter.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.Debug("EOF encountered. sending GGA message again")
				readerWriter = nil
				isConnected = false
				return err
			}
			logger.Error("Failed to read server response:", err)
			return err
		}

		if strings.HasPrefix(string(line), "HTTP/1.1 ") {
			if strings.Contains(string(line), "200 OK") {
				isConnected = true
				break
			} else {
				logger.Error("Bad HTTP response")
				isConnected = false
				return err
			}
		}
	}

	ggaMessage, err := GetGGAMessage(correctionWriter, logger)
	if err != nil {
		logger.Error("Failed to get GGA message")
		return err
	}

	logger.Debugf("Writing GGA message: %v\n", string(ggaMessage))

	_, err = readerWriter.WriteString(string(ggaMessage))
	if err != nil {
		logger.Error("Failed to send NMEA data:", err)
		return err
	}

	err = readerWriter.Flush()
	if err != nil {
		logger.Error("failed to write to buffer: ", err)
		return err
	}

	logger.Debug("GGA message sent successfully.")

	return nil
}

// ConnectToVirtualBase
func ConnectToVirtualBase(mountPoint string, usr string,
	pass string, url string,
	logger logging.Logger) (*bufio.ReadWriter, bool) {

	var isConnected bool

	mp := "/" + mountPoint
	credentials := usr + ":" + pass
	credentialsBase64 := base64.StdEncoding.EncodeToString([]byte(credentials))

	// Process the server URL
	serverAddr := url
	serverAddr = strings.TrimPrefix(serverAddr, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		isConnected = false
		logger.Errorf("Failed to connect to VRS server:", err)
		return nil, isConnected
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
		isConnected = false
		logger.Error("Failed to send HTTP headers:", err)
		return nil, isConnected
	}
	err = rw.Flush()
	if err != nil {
		isConnected = false
		logger.Error("failed to write to buffer")
		return nil, isConnected
	}

	logger.Debugf("request header: %v\n", httpHeaders)
	logger.Debug("HTTP headers sent successfully.")
	isConnected = true
	return rw, isConnected
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
