package protoutils

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
)

func TestRecordMessageSize(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	SetLargeMessageLogger(logger)
	stats := func() LargeMessageStats {
		return LargeMessageStatser{}.Stats().(map[string]LargeMessageStats)[string(SiteTabularCapture)]
	}
	// Strings stand in for large messages without byte lists.
	str := func(n int) *structpb.Struct {
		return &structpb.Struct{Fields: map[string]*structpb.Value{"s": structpb.NewStringValue(string(make([]byte, n)))}}
	}
	bytes := func(n int) *structpb.Struct {
		s, err := protoutils.StructToStructPb(map[string]interface{}{"b": make([]byte, n)})
		test.That(t, err, test.ShouldBeNil)
		return s
	}
	before := stats()

	RecordMessageSize(SiteTabularCapture, "str", str(1<<10))
	RecordMessageSize(SiteTabularCapture, "str", str(1<<20))
	RecordMessageSize(SiteTabularCapture, "str", str(5<<20))
	// ~11 wire bytes per byte, so 200 KiB and 600 KiB cross 1 MiB and 5 MiB.
	RecordMessageSize(SiteTabularCapture, "bytes", bytes(200<<10))
	RecordMessageSize(SiteTabularCapture, "bytes", bytes(200<<10))
	RecordMessageSize(SiteTabularCapture, "bytes", bytes(600<<10))

	after := stats()
	test.That(t, after.Over1MB-before.Over1MB, test.ShouldEqual, 5)
	test.That(t, after.Over5MB-before.Over5MB, test.ShouldEqual, 2)
	test.That(t, after.ByteListsOver1MB-before.ByteListsOver1MB, test.ShouldEqual, 3)
	test.That(t, after.ByteListsOver5MB-before.ByteListsOver5MB, test.ShouldEqual, 1)
	// One warning per name and threshold crossed, not per message.
	test.That(t, logs.FilterMessage("large message").Len(), test.ShouldEqual, 4)
}

func TestCountByteLists(t *testing.T) {
	s, err := protoutils.StructToStructPb(map[string]interface{}{
		"bytes":  make([]byte, minByteListLen),
		"short":  make([]byte, minByteListLen-1),
		"nested": map[string]interface{}{"b": []interface{}{make([]byte, minByteListLen)}},
		"floats": []float64{0.5, 1},
		"big":    append(make([]int, minByteListLen), 256),
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, countByteLists(s), test.ShouldEqual, 2*minByteListLen)
}
