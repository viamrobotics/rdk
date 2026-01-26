package sync

import (
	"testing"

	"go.viam.com/test"
)

func TestInferTagsAndDatasetIDsFromFilename(t *testing.T) {
	t.Run("no query suffix", func(t *testing.T) {
		path := "/tmp/fooFile.bar"
		gotName, gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromFilename(path)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, gotName, test.ShouldEqual, path)
		test.That(t, gotTags, test.ShouldBeNil)
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("tags comma separated", func(t *testing.T) {
		path := "/tmp/fooFile.bar?tags=tag1,tag2,tagC"
		gotName, gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromFilename(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotName, test.ShouldEqual, "/tmp/fooFile.bar")
		test.That(t, gotTags, test.ShouldResemble, []string{"tag1", "tag2", "tagC"})
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})

	t.Run("repeated tags and dataset ids", func(t *testing.T) {
		path := "/tmp/fooFile.bar?tags=tag1&tags=tag2%2Ctag3&dataset_ids=ds1,ds2"
		gotName, gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromFilename(path)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, gotName, test.ShouldEqual, "/tmp/fooFile.bar")
		test.That(t, gotTags, test.ShouldResemble, []string{"tag1", "tag2", "tag3"})
		test.That(t, gotDatasetIDs, test.ShouldResemble, []string{"ds1", "ds2"})
	})

	t.Run("unknown keys do not trigger stripping", func(t *testing.T) {
		path := "/tmp/a?b.txt"
		gotName, gotTags, gotDatasetIDs, ok := inferTagsAndDatasetIDsFromFilename(path)
		test.That(t, ok, test.ShouldBeFalse)
		test.That(t, gotName, test.ShouldEqual, path)
		test.That(t, gotTags, test.ShouldBeNil)
		test.That(t, gotDatasetIDs, test.ShouldBeNil)
	})
}
