package packages_test

import (
	"fmt"
	"testing"

	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot/packages"
)

func TestPackageTypeConversion(t *testing.T) {
	emptyType := config.PackageType("")
	converted, err := packages.PackageTypeToProto(emptyType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, converted, test.ShouldResemble, pb.PackageType_PACKAGE_TYPE_ML_MODEL.Enum())

	moduleType := config.PackageType("module")
	converted, err = packages.PackageTypeToProto(moduleType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, converted, test.ShouldResemble, pb.PackageType_PACKAGE_TYPE_MODULE.Enum())

	badType := config.PackageType("invalid-package-type")
	converted, err = packages.PackageTypeToProto(badType)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "invalid-package-type")
	test.That(t, converted, test.ShouldResemble, pb.PackageType_PACKAGE_TYPE_UNSPECIFIED.Enum())
}
