// Package dataprocess manages code related to the data-saving process
package dataprocess

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"time"

	pc "go.viam.com/rdk/pointcloud"
)

const (
	// SlamTimeFormat is the timestamp format used in the dataprocess.
	SlamTimeFormat = "2006-01-02T15:04:05.0000Z"
)

// CreateTimestampFilename creates an absolute filename with a primary sensor name and timestamp written
// into the filename.
func CreateTimestampFilename(dataDirectory, primarySensorName, fileType string, timeStamp time.Time) string {
	return filepath.Join(dataDirectory, primarySensorName+"_data_"+timeStamp.UTC().Format(SlamTimeFormat)+fileType)
}

// WritePCDToFile encodes the pointcloud and then saves it to the passed filename.
func WritePCDToFile(pointcloud pc.PointCloud, filename string) error {
	buf := new(bytes.Buffer)
	if err := pc.ToPCD(pointcloud, buf, 1); err != nil {
		return err
	}
	return WriteBytesToFile(buf.Bytes(), filename)
}

// WriteBytesToFile writes the passed bytes to the passed filename.
func WriteBytesToFile(bytes []byte, filename string) error {
	//nolint:gosec
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if _, err := w.Write(bytes); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}
