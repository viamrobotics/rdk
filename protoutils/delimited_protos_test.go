package protoutils_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"slices"
	"testing"

	"go.viam.com/test"
	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/protoutils"
)

type countingWriter struct {
	buffer     bytes.Buffer
	writeCalls int
}

// Write implements [io.Writer].
func (c *countingWriter) Write(p []byte) (n int, err error) {
	c.writeCalls++
	return c.buffer.Write(p)
}

func (c *countingWriter) Len() int {
	return c.buffer.Len()
}

func (c *countingWriter) Bytes() []byte {
	return c.buffer.Bytes()
}

func TestDelimitedProtoWriter(t *testing.T) {
	buffer := &countingWriter{}
	messages := [][]byte{}
	delimetedProtos := protoutils.NewDelimitedProtoWriter[*webrtcpb.CallRequest](buffer)
	reqs := []*webrtcpb.CallRequest{{Sdp: "hello"}, {Sdp: "world", DisableTrickle: true}}
	for _, req := range reqs {
		err := delimetedProtos.Append(req)
		test.That(t, err, test.ShouldBeNil)
		reqBytes, err := proto.Marshal(req)
		test.That(t, err, test.ShouldBeNil)
		messages = append(messages, reqBytes)
	}

	// Test that we called write exactly once for each message. Making multiple
	// calls per message could create corrupt trace files if rotation occurs
	// between calls.
	test.That(t, buffer.writeCalls, test.ShouldEqual, len(reqs))

	delimitedBytes := make([]byte, buffer.Len())
	copy(delimitedBytes, buffer.Bytes())
	for _, message := range messages {
		expectedLenBytes := make([]byte, 4)
		messageLen := len(message)
		binary.LittleEndian.PutUint32(expectedLenBytes, uint32(messageLen))
		test.That(t, delimitedBytes[:4], test.ShouldResemble, expectedLenBytes)
		delimitedBytes = delimitedBytes[4:]
		test.That(t, delimitedBytes[:messageLen], test.ShouldResemble, message)
		delimitedBytes = delimitedBytes[messageLen:]
	}
	test.That(t, delimitedBytes, test.ShouldHaveLength, 0)
}

func TestDelimitedProtoReaderAll(t *testing.T) {
	buffer := &bytes.Buffer{}
	delimetedProtos := protoutils.NewDelimitedProtoWriter[*webrtcpb.CallRequest](buffer)
	reqs := []*webrtcpb.CallRequest{{Sdp: "hello"}, {Sdp: "world", DisableTrickle: true}}
	for _, req := range reqs {
		err := delimetedProtos.Append(req)
		test.That(t, err, test.ShouldBeNil)
	}

	protosReader := protoutils.NewDelimitedProtoReader[webrtcpb.CallRequest](buffer)
	roundTrippedMessages := slices.Collect(protosReader.All())
	// Using test.ShouldResemble here causes the test to hang until it times out.
	test.That(t, roundTrippedMessages, test.ShouldHaveLength, len(reqs))
	for i, req := range reqs {
		test.That(t, roundTrippedMessages[i], test.ShouldResembleProto, req)
	}
	test.That(t, buffer.Len(), test.ShouldEqual, 0)
}

func TestDelimitedProtoReaderAllWithMemory(t *testing.T) {
	buffer := &bytes.Buffer{}
	delimetedProtos := protoutils.NewDelimitedProtoWriter[*webrtcpb.CallRequest](buffer)
	reqs := []*webrtcpb.CallRequest{{Sdp: "hello"}, {Sdp: "world", DisableTrickle: true}}
	for _, req := range reqs {
		err := delimetedProtos.Append(req)
		test.That(t, err, test.ShouldBeNil)
	}

	protosReader := protoutils.NewDelimitedProtoReader[webrtcpb.CallRequest](buffer)
	i := 0
	messageBuffer := &webrtcpb.CallRequest{}
	for msg := range protosReader.AllWithMemory(messageBuffer) {
		test.That(t, msg, test.ShouldResembleProto, reqs[i])
		i++
	}
	test.That(t, i, test.ShouldEqual, len(reqs))
	test.That(t, buffer.Len(), test.ShouldEqual, 0)
}

// slowReader wraps an io.Reader and returns at most n bytes per Read call,
// exercising the case where bufio.Scanner receives fewer than 5 bytes at a
// time and splitMessages must request more data rather than stopping early.
type slowReader struct {
	r io.Reader
	n int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(p) > s.n {
		p = p[:s.n]
	}
	return s.r.Read(p)
}

func TestDelimitedProtoReaderShortReads(t *testing.T) {
	buffer := &bytes.Buffer{}
	delimitedProtos := protoutils.NewDelimitedProtoWriter[*webrtcpb.CallRequest](buffer)
	reqs := []*webrtcpb.CallRequest{{Sdp: "hello"}, {Sdp: "world", DisableTrickle: true}}
	for _, req := range reqs {
		err := delimitedProtos.Append(req)
		test.That(t, err, test.ShouldBeNil)
	}

	// Wrap in a reader that returns 1 byte at a time, forcing splitMessages to
	// be called with len(data) < 5 on every read while not at EOF.
	slow := &slowReader{r: buffer, n: 1}
	protosReader := protoutils.NewDelimitedProtoReader[webrtcpb.CallRequest](slow)
	roundTrippedMessages := slices.Collect(protosReader.All())
	test.That(t, roundTrippedMessages, test.ShouldHaveLength, len(reqs))
	for i, req := range reqs {
		test.That(t, roundTrippedMessages[i], test.ShouldResembleProto, req)
	}
}
