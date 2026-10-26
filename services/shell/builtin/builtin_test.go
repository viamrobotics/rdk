package builtin

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

func TestDoCommand(t *testing.T) {
	svc, err := NewBuiltIn(shell.Named("shell1"), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	resp, err := svc.DoCommand(context.Background(), map[string]interface{}{shell.GetViamHomeCommand: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[shell.ViamHomeKey], test.ShouldEqual, rutils.ViamDotDir)

	_, err = svc.DoCommand(context.Background(), map[string]interface{}{"something_else": true})
	test.That(t, err, test.ShouldBeError, resource.ErrDoUnimplemented)
}

// TestUTF8ChunkerReassembly is the regression for APP-18421: a multibyte rune split across
// pty reads must reassemble losslessly and never yield invalid UTF-8, at any split point.
func TestUTF8ChunkerReassembly(t *testing.T) {
	raw := []byte("\u25cf viam-agent active \u65e5\u672c\u8a9e\n")
	for i := 0; i <= len(raw); i++ {
		var c utf8Chunker
		var got strings.Builder
		got.WriteString(c.push(raw[:i]))
		got.WriteString(c.push(raw[i:]))
		got.WriteString(c.flush())
		test.That(t, utf8.ValidString(got.String()), test.ShouldBeTrue)
		test.That(t, got.String(), test.ShouldEqual, string(raw))
	}
}

// TestUTF8ChunkerProtoSafe pins the exact failure mode: every emitted chunk must marshal into
// a ShellResponse. Previously a split rune produced invalid UTF-8 and killed the shell stream.
func TestUTF8ChunkerProtoSafe(t *testing.T) {
	raw := []byte("systemctl status \u25cf \u65e5\u672c\u8a9e \U0001f916")
	for i := 0; i <= len(raw); i++ {
		var c utf8Chunker
		for _, part := range [][]byte{raw[:i], raw[i:]} {
			_, err := proto.Marshal(&pb.ShellResponse{DataOut: c.push(part)})
			test.That(t, err, test.ShouldBeNil)
		}
		_, err := proto.Marshal(&pb.ShellResponse{DataOut: c.flush()})
		test.That(t, err, test.ShouldBeNil)
	}
}

// TestUTF8ChunkerInvalidBytes checks that genuinely malformed bytes are sanitized rather than
// dropped or allowed to break marshaling.
func TestUTF8ChunkerInvalidBytes(t *testing.T) {
	var c utf8Chunker
	got := c.push([]byte{'a', 0xff, 'b'}) + c.flush()
	test.That(t, utf8.ValidString(got), test.ShouldBeTrue)
	test.That(t, got, test.ShouldEqual, "a�b")
}

func TestSafeUTF8Boundary(t *testing.T) {
	test.That(t, safeUTF8Boundary([]byte("ab")), test.ShouldEqual, 2)
	test.That(t, safeUTF8Boundary([]byte{0xe2, 0x97, 0x8f}), test.ShouldEqual, 3) // whole ●
	test.That(t, safeUTF8Boundary([]byte{'x', 0xe2, 0x97}), test.ShouldEqual, 1)  // hold back partial ●
	test.That(t, safeUTF8Boundary([]byte{0xf0, 0x9f, 0x98}), test.ShouldEqual, 0) // hold back partial 4-byte rune
	test.That(t, safeUTF8Boundary([]byte{0xff}), test.ShouldEqual, 1)             // invalid-but-complete, sanitize later
}
