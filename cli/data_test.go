package cli

import (
	"testing"

	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
)

func TestFilenameForDownload(t *testing.T) {
	const utc0 = "1970-01-01T00:00:00Z"
	const linux = "linux"
	const windows = "windows"
	noFilename := filenameForDownload(&datapb.BinaryMetadata{
		Id: "my-id",
	}, linux)
	test.That(t, noFilename, test.ShouldEqual, utc0+"_my-id")

	normalExt := filenameForDownload(&datapb.BinaryMetadata{
		FileName: "whatever.txt",
	}, linux)
	test.That(t, normalExt, test.ShouldEqual, utc0+"_whatever.txt")

	charReplacedTimestamp := filenameForDownload(&datapb.BinaryMetadata{
		FileName: "whatever.txt",
	}, windows)
	test.That(t, charReplacedTimestamp, test.ShouldEqual, "1970-01-01T00_00_00Z_whatever.txt")

	inFolder := filenameForDownload(&datapb.BinaryMetadata{
		FileName: "dir/whatever.txt",
	}, linux)
	test.That(t, inFolder, test.ShouldEqual, "dir/whatever.txt")

	gzAtRoot := filenameForDownload(&datapb.BinaryMetadata{
		FileName: "whatever.gz",
	}, linux)
	test.That(t, gzAtRoot, test.ShouldEqual, utc0+"_whatever")

	gzInFolder := filenameForDownload(&datapb.BinaryMetadata{
		FileName: "dir/whatever.gz",
	}, linux)
	test.That(t, gzInFolder, test.ShouldEqual, "dir/whatever")
}
