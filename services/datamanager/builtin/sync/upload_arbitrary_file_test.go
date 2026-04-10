package sync

import (
	"runtime"
	"testing"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
)

func TestInferTagsAndDatasetIDsFromFilename(t *testing.T) {
	t.Run("no tag/dataset segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/path/to/file/fooFile.bar"
		gotTags, gotDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, gotTags, test.ShouldBeNil)
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("only tag segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/tag=tag2/path/to/file/foo.ex1"
		gotTags, gotDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, goutils.NewStringSet(gotTags...), test.ShouldResemble, goutils.NewStringSet("tag1", "tag2"))
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("tag and dataset segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/tag=tag2/dataset=1023/tag=tagC/dataset=1024/path/to/file/foo.ex1"
		gotTags, gotDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, goutils.NewStringSet(gotTags...), test.ShouldResemble, goutils.NewStringSet("tag1", "tag2", "tagC"))
		test.That(t, goutils.NewStringSet(gotDatasetIDs...), test.ShouldResemble, goutils.NewStringSet("1023", "1024"))
	})

	t.Run("ignores base filename", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/dataset=ds1/tag=tag2/path/to/file/tag=tag3.jpg"
		gotTags, gotDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, goutils.NewStringSet(gotTags...), test.ShouldResemble, goutils.NewStringSet("tag1", "tag2"))
		test.That(t, goutils.NewStringSet(gotDatasetIDs...), test.ShouldResemble, goutils.NewStringSet("ds1"))
	})

	t.Run("windows style separators", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows-style paths are only meaningful on Windows")
		}
		path := `C:\Users\alice\.viam\capture\tag=tag1\dataset=1023\path\to\file\foo.ex1`
		gotTags, gotDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, goutils.NewStringSet(gotTags...), test.ShouldResemble, goutils.NewStringSet("tag1"))
		test.That(t, goutils.NewStringSet(gotDatasetIDs...), test.ShouldResemble, goutils.NewStringSet("1023"))
	})
}
