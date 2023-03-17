package packages

import (
	"path"
	"reflect"

	"go.viam.com/rdk/config"
)

type PackagePathVisitor struct {
	packageManager Manager
}

func NewPackagePathVisitor(packageManager Manager) *PackagePathVisitor {
	return &PackagePathVisitor{
		packageManager: packageManager,
	}
}

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
