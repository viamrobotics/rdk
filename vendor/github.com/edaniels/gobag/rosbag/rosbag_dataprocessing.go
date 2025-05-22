package rosbag

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	"github.com/edaniels/gobag/bread"
	"github.com/edaniels/gobag/msgpiler"

	"github.com/pierrec/lz4"
)

func linkMessage(rootmessage *msgpiler.MessageFormat, mf *msgpiler.MessageFormat) error {
	// link all base types to reading functions
	for i := range mf.Fields {
		if mf.Fields[i].IsBaseType {
			if !mf.Fields[i].IsArray {
				switch mf.Fields[i].VariableType {
				case "bool":
					mf.Fields[i].BinaryRead = bread.Bool
				case "int8":
					mf.Fields[i].BinaryRead = bread.Int8
				case "byte":
					fallthrough
				case "uint8":
					mf.Fields[i].BinaryRead = bread.UInt8
				case "int16":
					mf.Fields[i].BinaryRead = bread.Int16
				case "uint16":
					mf.Fields[i].BinaryRead = bread.UInt16
				case "int32":
					mf.Fields[i].BinaryRead = bread.Int32
				case "uint32":
					mf.Fields[i].BinaryRead = bread.UInt32
				case "int64":
					mf.Fields[i].BinaryRead = bread.Int64
				case "uint64":
					mf.Fields[i].BinaryRead = bread.Int64
				case "float32":
					mf.Fields[i].BinaryRead = bread.Float32
				case "float64":
					mf.Fields[i].BinaryRead = bread.Float64
				case "time":
					mf.Fields[i].BinaryRead = bread.Time
				case "duration":
					mf.Fields[i].BinaryRead = bread.Duration
				case "string":
					mf.Fields[i].BinaryRead = bread.String
				default:
					err := fmt.Errorf("Unknown field type %v", mf.Fields[i].VariableType)
					log.Error("Unknown field type", zap.Error(err))
					return err
				}
			} else if mf.Fields[i].ArrayLength == 0 {
				switch mf.Fields[i].VariableType {
				case "bool":
					mf.Fields[i].BinaryRead = bread.BoolA
				case "int8":
					mf.Fields[i].BinaryRead = bread.Int8A
				case "byte":
					fallthrough
				case "uint8":
					mf.Fields[i].BinaryRead = bread.UInt8A
				case "int16":
					mf.Fields[i].BinaryRead = bread.Int16A
				case "uint16":
					mf.Fields[i].BinaryRead = bread.UInt16A
				case "int32":
					mf.Fields[i].BinaryRead = bread.Int32A
				case "uint32":
					mf.Fields[i].BinaryRead = bread.UInt32A
				case "int64":
					mf.Fields[i].BinaryRead = bread.Int64A
				case "uint64":
					mf.Fields[i].BinaryRead = bread.Int64A
				case "float32":
					mf.Fields[i].BinaryRead = bread.Float32A
				case "float64":
					mf.Fields[i].BinaryRead = bread.Float64A
				case "time":
					mf.Fields[i].BinaryRead = bread.TimeA
				case "duration":
					mf.Fields[i].BinaryRead = bread.DurationA
				case "string":
					mf.Fields[i].BinaryRead = bread.StringA
				default:
					err := fmt.Errorf("Unknown field type %v", mf.Fields[i].VariableType)
					log.Error("Unknown field type", zap.Error(err))
					return err
				}
			} else if mf.Fields[i].ArrayLength > 0 {
				switch mf.Fields[i].VariableType {
				case "bool":
					mf.Fields[i].BinaryRead = bread.BoolFA
				case "int8":
					mf.Fields[i].BinaryRead = bread.Int8FA
				case "byte":
					fallthrough
				case "uint8":
					mf.Fields[i].BinaryRead = bread.UInt8FA
				case "int16":
					mf.Fields[i].BinaryRead = bread.Int16FA
				case "uint16":
					mf.Fields[i].BinaryRead = bread.UInt16FA
				case "int32":
					mf.Fields[i].BinaryRead = bread.Int32FA
				case "uint32":
					mf.Fields[i].BinaryRead = bread.UInt32FA
				case "int64":
					mf.Fields[i].BinaryRead = bread.Int64FA
				case "uint64":
					mf.Fields[i].BinaryRead = bread.Int64FA
				case "float32":
					mf.Fields[i].BinaryRead = bread.Float32FA
				case "float64":
					mf.Fields[i].BinaryRead = bread.Float64FA
				case "time":
					mf.Fields[i].BinaryRead = bread.TimeFA
				case "duration":
					mf.Fields[i].BinaryRead = bread.DurationFA
				case "string":
					mf.Fields[i].BinaryRead = bread.StringFA
				default:
					err := fmt.Errorf("Unknown field type %v", mf.Fields[i].VariableType)
					log.Error("Unknown field type", zap.Error(err))
					return err
				}
			}
		} else if mf.Fields[i].IsComplexType {
			// Find best match for type
			var matchingNames []msgpiler.MessageFormat
			log.Debug("Linking complex type", zap.String("type", mf.Fields[i].VariableType))
			for _, m := range rootmessage.MessageFormats {
				log.Debug("Defined type", zap.String("type", m.Name))
				if mf.Fields[i].VariableType == m.Name {
					matchingNames = append(matchingNames, m)
				}
			}
			log.Debug("Matched type", zap.String("types", fmt.Sprintf("%v", matchingNames)))
			if len(matchingNames) < 1 {
				err := fmt.Errorf("Unable to compile message format, unknown complex type")
				log.Error("Error while linking message", zap.Error(err), zap.String("Variable type", mf.Fields[i].VariableType))
				return err
			} else if len(matchingNames) == 1 {
				mf.Fields[i].ComplexTypeFormat = &matchingNames[0]
			} else if len(matchingNames) > 1 {
				err := fmt.Errorf("Unable to compile message format, multiple type matches")
				log.Error("Namespace matchin is not implemented yet", zap.Error(err), zap.String("Variable type", mf.Fields[i].VariableType))
				// Select best match based on rules
				// no name-space = no-namespace
				// root namespace = namespace
				// namespace = namespace
				// Once parsing is done return Format
				return err
			}
		}
	}
	return nil
}

// readMessage reads message from stream according to format given
func (r *RosBag) readMessage(offset int32, mf *msgpiler.MessageFormat) int32 {
	var size int32
	// Start of the JSON message
	r.ob.WriteByte('{')
	fieldsLength := len(mf.Fields) - 1
	for fieldIndex, field := range mf.Fields {
		if field.IsBaseType {
			// result[field.VariableName] = variableValue
			r.ob.WriteByte('"')
			r.ob.WriteString(strings.ToLower(field.VariableName))
			r.ob.WriteString("\":")
			valueSize := field.BinaryRead(r.ob, r.uncompressedChunk, offset+size, field.ArrayLength)
			size = size + valueSize
		} else if field.IsComplexType && !field.IsArray {
			// log.Debugf("Calling complex type %v with offset %v", field.ComplexTypeFormat, offset+size)
			r.ob.WriteByte('"')
			r.ob.WriteString(strings.ToLower(field.VariableName))
			r.ob.WriteString("\":")
			valueSize := r.readMessage(offset+size, field.ComplexTypeFormat)
			// result[field.VariableName] = variableValue
			size = size + valueSize
		} else {
			// Processing complex type array
			var arrayLength int32
			if field.ArrayLength > 0 {
				arrayLength = field.ArrayLength
			} else {
				arrayLength = *(*int32)(unsafe.Pointer(&r.uncompressedChunk[offset+size]))
				size = size + 4
			}
			r.ob.WriteByte('"')
			r.ob.WriteString(strings.ToLower(field.VariableName))
			r.ob.WriteString("\":[")
			for i := int32(0); i < arrayLength; i++ {
				valueSize := r.readMessage(offset+size, field.ComplexTypeFormat)
				size = size + valueSize
				if i < arrayLength-1 {
					r.ob.WriteByte(',')
				}
			}
			r.ob.WriteByte(']')
		}
		// Add separating comma if we are not in last element
		if fieldIndex < fieldsLength {
			r.ob.WriteByte(',')
		}
	}

	r.ob.WriteByte('}')
	return size
}

func (r *RosBag) uncompressChunk(c RosChunk) error {
	if int64(cap(r.uncompressedChunk)) < c.uncompressedSize {
		r.uncompressedChunk = make([]byte, c.uncompressedSize)
	} else {
		r.uncompressedChunk = r.uncompressedChunk[:cap(r.uncompressedChunk)]
	}
	chunkReader := bytes.NewReader(r.rawBytes[c.startOffset : c.startOffset+c.size])
	switch strings.ToLower(c.compressionType) {
	case "lz4":
		lz4Reader := lz4.NewReader(chunkReader)
		for {
			uncompressResultSize, err := lz4Reader.Read(r.uncompressedChunk)
			if err != nil {
				if !(err == io.EOF) {
					log.Error("Error while uncompressing chunk lz4",
						zap.Int("uncompressResultSize", uncompressResultSize),
						zap.Int64("c.uncompressedSize", c.uncompressedSize))
					return err
				}
			}
			if uncompressResultSize == 0 {
				break
			}
		}
	case "bz2":
		bz2Reader := bzip2.NewReader(chunkReader)
		uncompressResultSize, err := bz2Reader.Read(r.uncompressedChunk)
		if err != nil {
			if !(err == io.EOF && int64(uncompressResultSize) == c.uncompressedSize) {
				log.Error("Error while uncompressing chunk bz2",
					zap.Int("uncompressResultSize", uncompressResultSize),
					zap.Int64("c.uncompressedSize", c.uncompressedSize))
				return err
			}
		}
	case "none":
		uncompressResultSize, err := chunkReader.Read(r.uncompressedChunk)
		if err != nil {
			if !(err == io.EOF && int64(uncompressResultSize) == c.uncompressedSize) {
				log.Error("Error while uncompressing chunk none",
					zap.Int("uncompressResultSize", uncompressResultSize),
					zap.Int64("c.uncompressedSize", c.uncompressedSize))
				return err
			}
		}
	default:
		err := fmt.Errorf("Unknown compression type %s", c.compressionType)
		log.Error("Error on uncompressing chunk", zap.Error(err), zap.String("compressiontype", c.compressionType))
		return err
	}
	return nil
}
