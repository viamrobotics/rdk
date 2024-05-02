package shell

import (
	"testing"

	"go.viam.com/test"
)

func TestCopyFilesSourceTypeProtoRoundTrip(t *testing.T) {
	for _, tc := range []CopyFilesSourceType{
		CopyFilesSourceTypeSingleFile,
		CopyFilesSourceTypeSingleDirectory,
		CopyFilesSourceTypeMultipleFiles,
		CopyFilesSourceTypeMultipleUnknown,
	} {
		t.Run(tc.String(), func(t *testing.T) {
			test.That(t, CopyFilesSourceTypeFromProto(tc.ToProto()), test.ShouldEqual, tc)
		})
	}
}
