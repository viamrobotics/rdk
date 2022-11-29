package referenceframe

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestConvertURDF(t *testing.T) {
	u, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "ur5")

	fmt.Println(u.Name())
}
