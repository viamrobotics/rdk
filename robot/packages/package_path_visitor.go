package packages

import (
	"path"
	"reflect"

	"go.viam.com/rdk/config"
)

// PackagePathVisitor is a visitor that replaces strings containing with references to package names
// with the path containing the package files on the robot.
type PackagePathVisitor struct {
	packageManager Manager
}

// NewPackagePathVisitor creates a new PackagePathVisitor.
func NewPackagePathVisitor(packageManager Manager) *PackagePathVisitor {
	return &PackagePathVisitor{
		packageManager: packageManager,
	}
}

// Visit implements config.Visitor.
func (v *PackagePathVisitor) Visit(data interface{}) (interface{}, error) {
	t := reflect.TypeOf(data)

	var s string
	switch {
	case t.Kind() == reflect.String:
		s = data.(string)
	case t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.String:
		s = *data.(*string)
	default:
		return data, nil
	}

	ref := config.GetPackageReference(s)
	if ref != nil {
		packagePath, err := v.packageManager.PackagePath(PackageName(ref.Package))
		if err != nil {
			return nil, err
		}
		s = path.Join(packagePath, ref.PathInPackage)
	}
	return s, nil
}
