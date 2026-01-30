package sync

import (
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestInferTagsAndDatasetIDsFromFilename(t *testing.T) {
	t.Run("no tag/dataset segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/path/to/file/fooFile.bar"
		gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, gotTags, test.ShouldBeNil)
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("only tag segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/tag=tag2/path/to/file/foo.ex1"
		gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotTags, test.ShouldResemble, []string{"tag2", "tag1"})
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("tag and dataset segments", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/tag=tag2/dataset=1023/tag=tagC/dataset=1024/path/to/file/foo.ex1"
		gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotTags, test.ShouldResemble, []string{"tagC", "tag2", "tag1"})
		test.That(t, gotDatasetIDs, test.ShouldResemble, []string{"1024", "1023"})
	})

	t.Run("ignores base filename", func(t *testing.T) {
		path := "/home/alice/.viam/capture/tag=tag1/dataset=ds1/tag=tag2/path/to/file/tag=tag3.jpg"
		gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotTags, test.ShouldResemble, []string{"tag2", "tag1"})
		test.That(t, gotDatasetIDs, test.ShouldResemble, []string{"ds1"})
	})

	t.Run("windows style separators", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows-style paths are only meaningful on Windows")
		}
		path := `C:\Users\alice\.viam\capture\tag=tag1\dataset=1023\path\to\file\foo.ex1`
		gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromPath(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotTags, test.ShouldResemble, []string{"tag1"})
		test.That(t, gotDatasetIDs, test.ShouldResemble, []string{"1023"})
	})
}
