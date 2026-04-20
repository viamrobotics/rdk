package data

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/resource"
)

// TODO Data-343: Reorganize this into a more standard interface/package, and add tests.

const (
	// InProgressCaptureFileExt defines the file extension for Viam data capture files
	// which are currently being written to.
	InProgressCaptureFileExt = ".prog"
	// CompletedCaptureFileExt defines the file extension for Viam data capture files
	// which are no longer being written to.
	CompletedCaptureFileExt = ".capture"
	readImage               = "ReadImage"
	getAudio                = "GetAudio"
	// GetImages is used for getting simultaneous images from different imagers.
	GetImages            = "GetImages"
	nextPointCloud       = "NextPointCloud"
	pointCloudMap        = "PointCloudMap"
	captureAllFromCamera = "CaptureAllFromCamera"
	// Non-exhaustive list of characters to strip from file paths, since not allowed
	// on certain file systems.
	filePathReservedChars = ":"
)

// CaptureFile is the data structure containing data captured by collectors. It is backed by a file on disk containing
// length delimited protobuf messages, where the first message is the CaptureMetadata for the file, and ensuing
// messages contain the captured data.
type CaptureFile struct {
	path     string
	lock     sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	size     int64
	metadata *v1.DataCaptureMetadata

	initialReadOffset int64
	readOffset        int64
	writeOffset       int64
}

// ReadCaptureFile creates a File struct from a passed os.File previously constructed using NewFile.
func ReadCaptureFile(f *os.File) (*CaptureFile, error) {
	if !IsDataCaptureFile(f) {
		return nil, errors.Errorf("%s is not a data capture file", f.Name())
	}
	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	md := &v1.DataCaptureMetadata{}
	initOffset, err := pbutil.ReadDelimited(f, md)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read DataCaptureMetadata from %s", f.Name())
	}

	ret := CaptureFile{
		path:              f.Name(),
		file:              f,
		writer:            bufio.NewWriter(f),
		size:              finfo.Size(),
		metadata:          md,
		initialReadOffset: int64(initOffset),
		readOffset:        int64(initOffset),
		writeOffset:       int64(initOffset),
	}

	return &ret, nil
}

// NewCaptureFile creates a new *CaptureFile with the specified md in the specified directory.
func NewCaptureFile(dir string, md *v1.DataCaptureMetadata) (*CaptureFile, error) {
	fileName := CaptureFilePathWithReplacedReservedChars(
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
	return &CaptureFile{
		path:              f.Name(),
		writer:            bufio.NewWriter(f),
		file:              f,
		size:              int64(n),
		initialReadOffset: int64(n),
		readOffset:        int64(n),
		writeOffset:       int64(n),
	}, nil
}

// ReadMetadata reads and returns the metadata in f.
func (f *CaptureFile) ReadMetadata() *v1.DataCaptureMetadata {
	return f.metadata
}

// ReadNext returns the next SensorData reading.
func (f *CaptureFile) ReadNext() (*v1.SensorData, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if err := f.writer.Flush(); err != nil {
		return nil, err
	}

	if _, err := f.file.Seek(f.readOffset, io.SeekStart); err != nil {
		return nil, err
	}
	r := v1.SensorData{}
	read, err := pbutil.ReadDelimited(f.file, &r)
	if err != nil {
		return nil, err
	}
	f.readOffset += int64(read)

	return &r, nil
}

// WriteNext writes the next SensorData reading.
func (f *CaptureFile) WriteNext(data *v1.SensorData) error {
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

// Flush flushes any buffered writes to disk.
func (f *CaptureFile) Flush() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.writer.Flush()
}

// Reset resets the read pointer of f.
func (f *CaptureFile) Reset() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.readOffset = f.initialReadOffset
}

// Size returns the size of the file.
func (f *CaptureFile) Size() int64 {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.size
}

// GetPath returns the path of the underlying os.File.
func (f *CaptureFile) GetPath() string {
	return f.path
}

// Close closes the file.
func (f *CaptureFile) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.writer.Flush(); err != nil {
		return err
	}

	// Rename file to indicate that it is done being written.
	withoutExt := strings.TrimSuffix(f.file.Name(), filepath.Ext(f.file.Name()))
	newName := withoutExt + CompletedCaptureFileExt
	if err := f.file.Close(); err != nil {
		return err
	}
	return os.Rename(f.file.Name(), newName)
}

// Delete deletes the file.
func (f *CaptureFile) Delete() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if err := f.file.Close(); err != nil {
		return err
	}
	return os.Remove(f.GetPath())
}

// BuildCaptureMetadata builds a DataCaptureMetadata object and returns error if
// additionalParams fails to convert to anypb map.
func BuildCaptureMetadata(
	api resource.API,
	name string,
	method string,
	additionalParams map[string]interface{},
	methodParams map[string]*anypb.Any,
	tags []string,
) (*v1.DataCaptureMetadata, CaptureType) {
	dataType := MethodToCaptureType(method)
	return &v1.DataCaptureMetadata{
		ComponentType:    api.String(),
		ComponentName:    name,
		MethodName:       method,
		Type:             dataType.ToProto(),
		MethodParameters: methodParams,
		FileExtension:    getFileExt(dataType, method, additionalParams),
		Tags:             tags,
	}, dataType
}

// IsDataCaptureFile returns whether or not f is a data capture file.
func IsDataCaptureFile(f *os.File) bool {
	return filepath.Ext(f.Name()) == CompletedCaptureFileExt || filepath.Ext(f.Name()) == InProgressCaptureFileExt
}

// Create a filename based on the current time.
func getFileTimestampName() string {
	return time.Now().Format("2006-01-02T15:04:05.000000Z07:00")
}

// SensorDataFromCaptureFilePath returns all readings in the file at filePath.
// NOTE: (Nick S) At time of writing this is only used in tests.
func SensorDataFromCaptureFilePath(filePath string) ([]*v1.SensorData, error) {
	//nolint:gosec
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	dcFile, err := ReadCaptureFile(f)
	if err != nil {
		return nil, err
	}

	return SensorDataFromCaptureFile(dcFile)
}

// SensorDataFromCaptureFile returns all readings in f.
func SensorDataFromCaptureFile(f *CaptureFile) ([]*v1.SensorData, error) {
	f.Reset()
	var ret []*v1.SensorData
	for {
		next, err := f.ReadNext()
		if err != nil {
			// TODO: This swallows errors if the capture file has invalid proto in it
			// https://viam.atlassian.net/browse/DATA-3068
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}
		ret = append(ret, next)
	}
	return ret, nil
}

// BinaryPayloadReader parses the next SensorData message in f without loading the binary
// payload into memory. It returns the SensorMetadata, the exact byte length of the binary
// payload, and an io.Reader that streams the raw binary bytes directly from the file.
//
// BinaryPayloadReader advances the read position on each call, so it can be called in a
// loop to iterate over all SensorData messages in the file. Call f.Reset() before the
// loop to start from the beginning.
//
// The returned io.Reader is backed by an io.SectionReader over the underlying os.File, so
// it remains valid after BinaryPayloadReader returns and does not require the caller to hold
// any lock. It returns io.EOF when there are no more messages.
//
// f must be a DATA_TYPE_BINARY_SENSOR capture file whose SensorData contains a binary oneof
// field (field 3). It is not suitable for legacy camera.GetImages files, which store tabular
// data despite the binary metadata type.
func (f *CaptureFile) BinaryPayloadReader() (*v1.SensorMetadata, int64, io.Reader, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if err := f.writer.Flush(); err != nil {
		return nil, 0, nil, err
	}

	seekOffset := f.readOffset
	if _, err := f.file.Seek(seekOffset, io.SeekStart); err != nil {
		return nil, 0, nil, err
	}

	cr := &countingByteReader{r: f.file}

	// Read the outer length-prefix varint (total SensorData message byte count).
	// We use the value to advance readOffset past the full message so the next
	// call picks up where this one left off.
	outerLen, err := binary.ReadUvarint(cr)
	if err != nil {
		return nil, nil, err // io.EOF means no more messages
	}
	// Advance readOffset past this full outer-delimited message.
	f.readOffset = seekOffset + cr.count + int64(outerLen)

	var sensorMeta *v1.SensorMetadata
	// binaryPayloadOffset/Len are set when we encounter field 3 before field 1.
	// Proto spec does not guarantee field ordering, so we handle both orderings.
	binaryPayloadOffset := int64(-1)
	binaryPayloadLen := int64(0)

	for {
		tagVal, err := binary.ReadUvarint(cr)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, nil, fmt.Errorf("reading SensorData field tag: %w", err)
		}

		fieldNum := protowire.Number(tagVal >> 3)
		wireType := protowire.Type(tagVal & 0x7)

		// Skip non-bytes wire type fields to remain forward-compatible with fields
		// added by future server versions.
		if wireType != protowire.BytesType {
			var skipErr error
			switch wireType {
			case protowire.VarintType:
				_, skipErr = binary.ReadUvarint(cr)
			case protowire.Fixed32Type:
				_, skipErr = io.CopyN(io.Discard, cr, 4)
			case protowire.Fixed64Type:
				_, skipErr = io.CopyN(io.Discard, cr, 8)
			default:
				return nil, 0, nil, fmt.Errorf("unsupported wire type %d for field %d in SensorData", wireType, fieldNum)
			}
			if skipErr != nil {
				return nil, 0, nil, fmt.Errorf("skipping field %d (wire type %d): %w", fieldNum, wireType, skipErr)
			}
			continue
		}

		fieldLen, err := binary.ReadUvarint(cr)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("reading field length for SensorData field %d: %w", fieldNum, err)
		}

		switch fieldNum {
		case 1: // SensorMetadata
			metaBytes := make([]byte, fieldLen)
			if _, err := io.ReadFull(cr, metaBytes); err != nil {
				return nil, 0, nil, fmt.Errorf("reading SensorMetadata bytes: %w", err)
			}
			sensorMeta = &v1.SensorMetadata{}
			if err := proto.Unmarshal(metaBytes, sensorMeta); err != nil {
				return nil, 0, nil, fmt.Errorf("unmarshaling SensorMetadata: %w", err)
			}
			// If we already encountered the binary field (field 3 before field 1),
			// we now have everything we need.
			if binaryPayloadOffset >= 0 {
				return sensorMeta, binaryPayloadLen, io.NewSectionReader(f.file, binaryPayloadOffset, binaryPayloadLen), nil
			}
		case 3: // binary payload (SensorData.binary oneof field)
			// cr.count is the number of bytes consumed since we seeked to seekOffset.
			// The binary payload starts at this absolute file offset.
			binaryPayloadOffset = seekOffset + cr.count
			binaryPayloadLen = int64(fieldLen)
			if sensorMeta != nil {
				// Common case: metadata was field 1, came first.
				return sensorMeta, binaryPayloadLen, io.NewSectionReader(f.file, binaryPayloadOffset, binaryPayloadLen), nil
			}
			// Metadata not yet seen; skip past the binary bytes and keep parsing.
			if _, err := io.CopyN(io.Discard, cr, int64(fieldLen)); err != nil {
				return nil, 0, nil, fmt.Errorf("skipping binary payload while seeking SensorMetadata: %w", err)
			}
		default:
			// Skip any unrecognised fields (e.g. struct oneof, future fields).
			if _, err := io.CopyN(io.Discard, cr, int64(fieldLen)); err != nil {
				return nil, 0, nil, fmt.Errorf("skipping SensorData field %d: %w", fieldNum, err)
			}
		}
	}

	// Return whatever we found. If binaryPayloadOffset is still -1, no binary field
	// exists (e.g. this is a tabular or legacy GetImages file).
	if binaryPayloadOffset >= 0 {
		return sensorMeta, binaryPayloadLen, io.NewSectionReader(f.file, binaryPayloadOffset, binaryPayloadLen), nil
	}
	return nil, 0, nil, errors.New("binary payload field not found in capture file")
}

// countingByteReader wraps an io.Reader and tracks the total number of bytes read.
// It implements io.ByteReader so it can be passed to binary.ReadUvarint.
type countingByteReader struct {
	r     io.Reader
	count int64
}

func (c *countingByteReader) ReadByte() (byte, error) {
	var b [1]byte
	n, err := c.r.Read(b[:])
	c.count += int64(n)
	if n == 1 {
		return b[0], nil
	}
	return 0, err
}

func (c *countingByteReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.count += int64(n)
	return n, err
}

// CaptureFilePathWithReplacedReservedChars returns the filepath with substitutions
// for reserved characters.
func CaptureFilePathWithReplacedReservedChars(filepath string) string {
	// Handle Windows drive letters by preserving them and replacing other colons.
	if isWindowsAbsolutePath(filepath) {
		return filepath[:2] + strings.ReplaceAll(filepath[2:], filePathReservedChars, "_")
	}
	return strings.ReplaceAll(filepath, filePathReservedChars, "_")
}

// isWindowsAbsolutePath returns true if the path is a Windows absolute path. Ex: C:\path\to\file.txt.
func isWindowsAbsolutePath(path string) bool {
	driveLetter := path[0] | 32 // convert to lowercase.
	return len(path) >= 2 && path[1] == ':' && driveLetter >= 'a' && driveLetter <= 'z'
}
