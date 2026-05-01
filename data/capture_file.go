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

// ErrNoBinaryField is returned by BinaryPayloadReader when a SensorData message
// contains no binary payload field. This typically indicates a legacy camera.GetImages
// file that stores tabular data in a BINARY_SENSOR-typed capture file.
var ErrNoBinaryField = errors.New("binary payload field not found in capture file")

// ErrSensorMetadataTooLarge is returned by BinaryPayloadReader when the SensorMetadata
// field exceeds the size cap, indicating a corrupt or unexpected file.
var ErrSensorMetadataTooLarge = errors.New("SensorMetadata field exceeds size limit")

// ErrUnparsableBinaryCapture is returned by BinaryPayloadReader when the file cannot
// be streamed due to an unexpected wire format (e.g. unknown wire type).
var ErrUnparsableBinaryCapture = errors.New("capture file cannot be streamed due to unexpected wire format")

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

// BinaryPayloadReader reads the next SensorData message from f without loading
// the binary payload into memory. It returns the SensorMetadata, payload size,
// and an io.Reader for streaming the payload.
//
// Binary capture files contain one SensorData message (WriteBinary
// writes one message per file), so this is typically called once. The readOffset
// is advanced past the message regardless, consistent with ReadNext.
// Returns io.EOF when no messages remain.
//
// Each SensorData entry in the capture file is stored as a length-prefixed
// protobuf message (standard protobuf framing). Rather than unmarshaling the
// full message into memory, this function hand-parses the wire format to locate
// the binary payload field and returns an io.SectionReader directly over that
// region of the file. This lets callers stream large payloads without buffering.
//
// Assumes proto field 1 (SensorMetadata) precedes proto field 3 (binary payload)
// within each SensorData message, which matches the encoding produced by our writers.
func (f *CaptureFile) BinaryPayloadReader() (*v1.SensorMetadata, int64, io.Reader, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	// Seek to where we left off. seekOffset is captured before the seek so we can
	// compute absolute file positions for the SectionReader later.
	seekOffset := f.readOffset
	if _, err := f.file.Seek(seekOffset, io.SeekStart); err != nil {
		return nil, 0, nil, err
	}

	// The capture file is a sequence of length-delimited protobuf records. Each record
	// is: [outerLen varint] [SensorData proto bytes].
	// Read outerLen so we know where this message ends and the next begins.
	varintCR := &countingByteReader{r: f.file}
	outerLen, err := binary.ReadUvarint(varintCR)
	if err != nil {
		return nil, 0, nil, err // io.EOF means no more messages
	}

	// msgStart is the absolute file offset of the first byte of the SensorData fields
	// (i.e., just past the outerLen varint). We use this later to anchor the SectionReader.
	msgStart := seekOffset + varintCR.count

	// Advance readOffset past this entire record so the next call starts at the right place,
	// regardless of how much of the payload the caller actually consumes.
	f.readOffset = msgStart + int64(outerLen)

	// inner is bounded to exactly outerLen bytes so field parsing can never stray into
	// the next record, even if the file is malformed.
	inner := &countingByteReader{r: io.LimitReader(f.file, int64(outerLen))}

	var sensorMeta *v1.SensorMetadata

	// Walk the SensorData fields in wire order. Each field starts with a tag varint that
	// encodes both the field number (upper bits) and the wire type (lower 3 bits), followed
	// by the field value. We only need two fields:
	//   field 1 (BytesType) — SensorMetadata, decoded fully into memory (always small).
	//   field 3 (BytesType) — binary payload, returned as a SectionReader without buffering.
	// All other fields are skipped so we remain forward-compatible with future additions.
	for {
		tagVal, err := binary.ReadUvarint(inner)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, 0, nil, fmt.Errorf("reading SensorData field tag: %w", err)
		}

		// Protobuf tag encoding: upper bits are the field number, lower 3 bits are the wire type.
		fieldNum := protowire.Number(tagVal >> 3)
		wireType := protowire.Type(tagVal & 0x7)

		// Skip any non-length-delimited field (varint, fixed32, fixed64) by consuming its
		// fixed-width value and moving on.
		if wireType != protowire.BytesType {
			var skipErr error
			switch wireType { //nolint:exhaustive
			case protowire.VarintType:
				_, skipErr = binary.ReadUvarint(inner)
			case protowire.Fixed32Type:
				_, skipErr = io.CopyN(io.Discard, inner, 4)
			case protowire.Fixed64Type:
				_, skipErr = io.CopyN(io.Discard, inner, 8)
			default:
				return nil, 0, nil, fmt.Errorf("%w: unsupported wire type %d for field %d", ErrUnparsableBinaryCapture, wireType, fieldNum)
			}
			if skipErr != nil {
				return nil, 0, nil, fmt.Errorf("skipping field %d (wire type %d): %w", fieldNum, wireType, skipErr)
			}
			continue
		}

		// For BytesType fields the value is: [fieldLen varint] [fieldLen bytes].
		fieldLen, err := binary.ReadUvarint(inner)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("reading field length for SensorData field %d: %w", fieldNum, err)
		}

		switch fieldNum { //nolint:exhaustive
		case 1: // SensorMetadata — small proto message, safe to read fully into memory.
			const maxSensorMetaBytes = 1024 * 1024 // 1 MiB; metadata is always tiny in practice
			if fieldLen > maxSensorMetaBytes {
				return nil, 0, nil, fmt.Errorf("%w: %d bytes (limit %d)", ErrSensorMetadataTooLarge, fieldLen, maxSensorMetaBytes)
			}
			metaBytes := make([]byte, fieldLen)
			if _, err := io.ReadFull(inner, metaBytes); err != nil {
				return nil, 0, nil, fmt.Errorf("reading SensorMetadata bytes: %w", err)
			}
			sensorMeta = &v1.SensorMetadata{}
			if err := proto.Unmarshal(metaBytes, sensorMeta); err != nil {
				return nil, 0, nil, fmt.Errorf("unmarshaling SensorMetadata: %w", err)
			}
		case 3: // binary payload (SensorData.binary oneof field)
			if sensorMeta == nil {
				return nil, 0, nil, errors.New("binary payload field appeared before SensorMetadata in capture file")
			}
			// inner.count is the number of bytes consumed from msgStart so far, so
			// msgStart+inner.count is the absolute file offset where the payload bytes begin.
			// SectionReader lets the caller read directly from the file at that window.
			return sensorMeta, int64(fieldLen), io.NewSectionReader(f.file, msgStart+inner.count, int64(fieldLen)), nil
		default:
			// Unknown field — skip the value bytes and continue.
			if _, err := io.CopyN(io.Discard, inner, int64(fieldLen)); err != nil {
				return nil, 0, nil, fmt.Errorf("skipping SensorData field %d: %w", fieldNum, err)
			}
		}
	}

	return nil, 0, nil, ErrNoBinaryField
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
