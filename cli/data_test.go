package cli

import (
	"testing"

	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
)

func TestFilenameForDownload(t *testing.T) {
	const expectedUTC = "1970-01-01T00_00_00Z"
	noFilename := filenameForDownload(&datapb.BinaryMetadata{Id: "my-id"})
	test.That(t, noFilename, test.ShouldEqual, expectedUTC+"_my-id")

	normalExt := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.txt"})
	test.That(t, normalExt, test.ShouldEqual, expectedUTC+"_whatever.txt")

	inFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.txt"})
	test.That(t, inFolder, test.ShouldEqual, "dir/whatever.txt")

	gzAtRoot := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.gz"})
	test.That(t, gzAtRoot, test.ShouldEqual, expectedUTC+"_whatever")

	gzInFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.gz"})
	test.That(t, gzInFolder, test.ShouldEqual, "dir/whatever")
}
