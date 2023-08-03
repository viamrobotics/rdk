package packages

import (
	"github.com/pkg/errors"
	pb "go.viam.com/api/app/packages/v1"

	"go.viam.com/rdk/config"
)

// PackageTypeToProto converts a config PackageType to its proto equivalent
// This is required be because app/packages uses a PackageType enum but app/PackageConfig uses a string Type.
func PackageTypeToProto(t config.PackageType) (*pb.PackageType, error) {
	switch t {
	case "":
		// for backwards compatibility
		fallthrough
	case config.PackageTypeMlModel:
		return pb.PackageType_PACKAGE_TYPE_ML_MODEL.Enum(), nil
	case config.PackageTypeModule:
		return pb.PackageType_PACKAGE_TYPE_MODULE.Enum(), nil
	default:
		return pb.PackageType_PACKAGE_TYPE_UNSPECIFIED.Enum(), errors.Errorf("unknown package type %q", t)
	}
}
