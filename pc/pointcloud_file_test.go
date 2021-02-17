package pc

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/test"
)

func TestNewPointCloudFromFile(t *testing.T) {
	pc, err := NewPointCloudFromFile("data/test.las")
	test.That(t, err, test.ShouldBeNil)
	numPoints := pc.Size()
	test.That(t, numPoints, test.ShouldEqual, 8413)

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = pc.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextPC, err := NewPointCloudFromFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextPC, test.ShouldResemble, pc)
}
