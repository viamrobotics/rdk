package artifact

import (
	"io/ioutil"
	"strings"
	"testing"

	"go.viam.com/test"
)

type unknownConfig struct {
}

func (uc unknownConfig) Type() StoreType {
	return "unknown"
}

func TestNewStore(t *testing.T) {
	_, err := NewStore(unknownConfig{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown store type")
}

func testStore(t *testing.T, store Store, readOnly bool) {
	content1 := "mycoolcontent"
	content2 := "myothercoolcontent"

	hashVal1, err := computeHash([]byte(content1))
	test.That(t, err, test.ShouldBeNil)
	hashVal2, err := computeHash([]byte(content2))
	test.That(t, err, test.ShouldBeNil)

	if !readOnly {
		err = store.Contains(hashVal1)
		test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
		test.That(t, err, test.ShouldResemble, &errArtifactNotFound{hash: &hashVal1})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
		test.That(t, err.Error(), test.ShouldContainSubstring, hashVal1)

		_, err = store.Load(hashVal1)
		test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
		test.That(t, err, test.ShouldResemble, &errArtifactNotFound{hash: &hashVal1})

		err = store.Store(hashVal1, strings.NewReader(content1))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, store.Contains(hashVal1), test.ShouldBeNil)
		test.That(t, store.Contains(hashVal2), test.ShouldNotBeNil)

		err = store.Store(hashVal2, strings.NewReader(content2))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, store.Contains(hashVal1), test.ShouldBeNil)
		test.That(t, store.Contains(hashVal2), test.ShouldBeNil)
	}

	reader, err := store.Load(hashVal1)
	test.That(t, err, test.ShouldBeNil)
	rd, err := ioutil.ReadAll(reader)
	test.That(t, reader.Close(), test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content1)

	reader, err = store.Load(hashVal2)
	test.That(t, err, test.ShouldBeNil)
	rd, err = ioutil.ReadAll(reader)
	test.That(t, reader.Close(), test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content2)

	unknownHash := "foo"
	err = store.Contains(unknownHash)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	test.That(t, err, test.ShouldResemble, &errArtifactNotFound{hash: &unknownHash})
	_, err = store.Load(unknownHash)
	test.That(t, IsErrArtifactNotFound(err), test.ShouldBeTrue)
	test.That(t, err, test.ShouldResemble, &errArtifactNotFound{hash: &unknownHash})
}
