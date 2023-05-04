// Package testhelper implements a slam service definition with additional exported functions for
// the purpose of testing
package testhelper

import (
	"bytes"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
)

// TestComparePointCloudsFromPCDs is a helper function for checking GetPointCloudMapFull response along with associated pcd validity checks.
func TestComparePointCloudsFromPCDs(t *testing.T, pcdInput, pcdOutput []byte) {
	pcInput, err := pointcloud.ReadPCD(bytes.NewReader(pcdInput))
	test.That(t, err, test.ShouldBeNil)
	pcOutput, err := pointcloud.ReadPCD(bytes.NewReader(pcdOutput))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, pcInput.MetaData(), test.ShouldResemble, pcOutput.MetaData())

	pcInput.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		dOutput, ok := pcOutput.At(p.X, p.Y, p.Z)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, dOutput, test.ShouldResemble, d)
		return true
	})
}
