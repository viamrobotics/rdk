package data

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/api/app/datasync/v1"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/resource"
)

// BuildCaptureMetadata builds a DataCaptureMetadata object.
func BuildCaptureMetadata(
	compAPI resource.API,
	compName string,
	method string,
	additionalParams map[string]string,
	methodParams map[string]*anypb.Any,
	tags []string,
) *v1.DataCaptureMetadata {
	dataType := getDataType(method)
	return &v1.DataCaptureMetadata{
		ComponentType:    compAPI.String(),
		ComponentName:    compName,
		MethodName:       method,
		Type:             dataType,
		MethodParameters: methodParams,
		FileExtension:    GetFileExt(dataType, method, additionalParams),
		Tags:             tags,
	}
}

// ProgFile is the data structure containing data captured by collectors.
// It is backed by a file on disk containing
// length delimited protobuf messages, where the first message is the CaptureMetadata for the file, and ensuing
// messages contain the captured data.
type ProgFile struct {
	path        string
	lock        sync.Mutex
	file        *os.File
	writer      *bufio.Writer
	size        int64
	writeOffset int64
}

// NewProgFile creates a new *ProgFile with the specified md in the specified directory.
func NewProgFile(dir string, md *v1.DataCaptureMetadata) (*ProgFile, error) {
	fileName := FilePathWithReplacedReservedChars(
		filepath.Join(dir, getFileTimestampName()) + InProgressCaptureFileExt)
	//nolint:gosec
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	// Then write first metadata message to the file.
	n, err := pbutil.WriteDelimited(f, md)
	if err != nil {
		return nil, err
	}
	return &ProgFile{
		path:        f.Name(),
		writer:      bufio.NewWriter(f),
		file:        f,
		size:        int64(n),
		writeOffset: int64(n),
	}, nil
}

// WriteNext writes the next SensorData reading.
func (f *ProgFile) WriteNext(data *v1.SensorData) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if _, err := f.file.Seek(f.writeOffset, 0); err != nil {
		return err
	}
	n, err := pbutil.WriteDelimited(f.writer, data)
	if err != nil {
		return err
	}
	f.size += int64(n)
	f.writeOffset += int64(n)
	return nil
}

// Size returns the size of the file.
func (f *ProgFile) Size() int64 {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.size
}

// Close closes the file.
func (f *ProgFile) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.writer.Flush(); err != nil {
		return err
	}

	// Rename file to indicate that it is done being written.
	if err := os.Rename(f.file.Name(), captureFilePath(f.file.Name())); err != nil {
		return err
	}
	return f.file.Close()
}

func captureFilePath(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name)) + CompletedCaptureFileExt
}
