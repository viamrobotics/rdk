package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestFormatBytes(t *testing.T) {
	for _, tc := range []struct {
		in  uint64
		exp string
	}{
		{0, "0 Bytes"},
		{512, "512 Bytes"},
		{1000, "1.00 KB"},        // exact power: must not render as "1000.00 Bytes"
		{1000 * 1000, "1.00 MB"}, // exact power: must not render as "1000.00 KB"
		{1000 * 1000 * 1000, "1.00 GB"},
		{1000 * 1000 * 1000 * 1000, "1.00 TB"},
		{1500 * 1000, "1.50 MB"},
	} {
		test.That(t, FormatBytes(tc.in), test.ShouldEqual, tc.exp)
	}
}

func TestFormatBytesI64(t *testing.T) {
	test.That(t, FormatBytesI64(0), test.ShouldEqual, "0 Bytes")
	test.That(t, FormatBytesI64(1500*1000), test.ShouldEqual, "1.50 MB")
	// Negative values keep the sign rather than wrapping around uint64.
	test.That(t, FormatBytesI64(-1500*1000), test.ShouldEqual, "-1.50 MB")
}
