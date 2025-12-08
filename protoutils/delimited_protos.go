package protoutils

import (
	"bufio"
	"encoding/binary"
	"io"
	"iter"
	"math"

	"google.golang.org/protobuf/proto"
)

// DelimitedProtoWriter writes proto messages to an [io.Writer]. Each message is
// prefixed by its size in bytes so individual messages can later be retrieved.
// See also: [DelimitedProtoReader].
type DelimitedProtoWriter[M proto.Message] struct {
	writer io.Writer
}

// RawDelimitedProtoReader reads proto messages from an [io.Reader] containing
// contents created by [DelimitedProtoWriter] and returns the encoded messages
// as byte slices. To automatically unmarshal the messages during the iteration
// use a [DelimitedProtoReader].
type RawDelimitedProtoReader struct {
	reader io.Reader
}

// DelimitedProtoReader iterates over proto messages from an [io.Reader] with
// contents created by [DelimitedProtoWriter]. It automatically unmarshals the
// messages at each step of the iteration. To iterate over the raw bytes use
// [RawDelimitedProtoReader].
type DelimitedProtoReader[T any, M interface {
	*T
	proto.Message
}] struct {
	RawDelimitedProtoReader
}

// NewDelimitedProtoWriter creates a [DelimitedProtoWriter].
func NewDelimitedProtoWriter[M proto.Message](writer io.Writer) *DelimitedProtoWriter[M] {
	return &DelimitedProtoWriter[M]{writer}
}

// NewRawDelimitedProtoReader creates a [RawDelimitedProtoReader].
func NewRawDelimitedProtoReader(reader io.Reader) *RawDelimitedProtoReader {
	return &RawDelimitedProtoReader{reader}
}

// NewDelimitedProtoReader creates a [DelimitedProtoReader].
func NewDelimitedProtoReader[T any, M interface {
	*T
	proto.Message
}](reader io.Reader) *DelimitedProtoReader[T, M] {
	return &DelimitedProtoReader[T, M]{RawDelimitedProtoReader{reader}}
}

// Close will close the underlying writer if it is a [io.Closer]. Otherwise it
// is a noop.
func (o *DelimitedProtoWriter[_]) Close() error {
	if closer, ok := o.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Append marshals the provided message and writes it to the underlying
// [io.Writer].
func (o *DelimitedProtoWriter[M]) Append(message M) error {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	messageLen := uint32(len(messageBytes))
	messageLenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(messageLenBytes, messageLen)
	for _, buffer := range [][]byte{messageLenBytes, messageBytes} {
		_, err := o.writer.Write(buffer)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close will close the underlying reader if it is a [io.Closer]. Otherwise it
// is a noop.
func (o *RawDelimitedProtoReader) Close() error {
	if closer, ok := o.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// All returns an [iter.Seq] that opens the underlying file and iterates over
// the individual messages while automatically unmarshaling them to T. Each
// iteration will incur an allocation so the returned *T can be owned by the
// caller. If you only need the value inside the iteration, you can use [TODO]
// to reuse memory across iterations.
func (o *DelimitedProtoReader[T, M]) All() iter.Seq[M] {
	return o.allWithMessageProvider(func() M {
		return new(T)
	})
}

// AllWithMemory returns an [iter.Seq] that opens the underlying file and
// iterates over the individual messages while automatically unmarshalling them
// into message, which must be a non-nil pointer. Use this instead of
// [DelimitedProtoReader.All] to reduce allocations when you only need the
// message within the loop.
func (o *DelimitedProtoReader[T, M]) AllWithMemory(message M) iter.Seq[M] {
	return o.allWithMessageProvider(func() M {
		return message
	})
}

func (o *DelimitedProtoReader[T, M]) allWithMessageProvider(getMessage func() M) iter.Seq[M] {
	return func(yield func(M) bool) {
		o.RawDelimitedProtoReader.All()(func(messageBytes []byte) bool {
			message := getMessage()
			err := proto.Unmarshal(messageBytes, message)
			if err != nil {
				panic(err)
			}
			return yield(message)
		})
	}
}

// All returns an [iter.Seq] that reads from the underlying
// [io.Reader] and iterates over the individual messages inside. The []byte
// yielded may be overwritten on subsequent iterations. If you need to use the
// yieded []byte outside an iteration you must copy it somewhere else.
func (o *RawDelimitedProtoReader) All() iter.Seq[[]byte] {
	// 2 GiB, as defined by the protobuf spec
	const protoMaxBytes = 1024 * 1024 * 1024 * 2
	// Max message size + 4 bytes for the length header
	const bufferMaxSize = protoMaxBytes + 4
	// Fall back to max int size if necessary so the 32-bit tests pass.
	const realMaxSize = min(bufferMaxSize, math.MaxInt)
	return func(yield func([]byte) bool) {
		scanner := bufio.NewScanner(o.reader)
		// Start with no buffer and let bufio figure out the initial allocation +
		// when it needs to be resized.
		scanner.Buffer(nil, realMaxSize)
		scanner.Split(splitMessages)

		for scanner.Scan() {
			if !yield(scanner.Bytes()) {
				break
			}
		}
	}
}

func splitMessages(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) < 5 {
		// Not enough data to contain a full message + its length.
		return 0, nil, bufio.ErrFinalToken
	}
	messageSize := binary.LittleEndian.Uint32(data[:4])
	messageBytes := data[4:]
	if len(messageBytes) < int(messageSize) {
		if atEOF {
			// Don't have enough bytes but also reached EOF; invalid state.
			return 0, nil, bufio.ErrFinalToken
		}
		// Don't have the entire message in the buffer, request bufio read more in
		// and try again.
		return 0, nil, nil
	}
	messageBytes = messageBytes[:messageSize]
	return int(messageSize) + 4, messageBytes, nil
}
