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

	if t.Kind() == reflect.String {
		ref := config.GetPackageReference(data.(string))
		if ref != nil {
			packagePath, err := v.packageManager.PackagePath(PackageName(ref.Package))
			if err != nil {
				return nil, err
			}
			data = path.Join(packagePath, ref.PathInPackage)
		}
		return data, nil
	} else if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.String {
		ref := config.GetPackageReference(*data.(*string))
		if ref != nil {
			packagePath, err := v.packageManager.PackagePath(PackageName(ref.Package))
			if err != nil {
				return nil, err
			}
			data = path.Join(packagePath, ref.PathInPackage)
		}
		return &data, nil
	}
	return data, nil
}
