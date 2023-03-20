package packages

import (
	"reflect"
)

// PackagePathVisitor is a visitor that replaces strings containing references to package names
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

	withReplacedRefs, err := v.packageManager.RefPath(s)
	if err != nil {
		return nil, err
	}

	// If the input was a pointer, return a pointer.
	if t.Kind() == reflect.Ptr {
		return &withReplacedRefs, nil
	}
	return withReplacedRefs, nil
}
