package packages

import (
	"fmt"
	"reflect"
	"strings"
)

// PackagePathVisitor is a visitor that replaces strings containing references to package names
// with the path containing the package files on the robot.
type PackagePathVisitor struct {
	packageManager Manager
	packagePaths   map[string]string
}

// NewPackagePathVisitor creates a new PackagePathVisitor.
func NewPackagePathVisitor(packageManager Manager, packagePaths map[string]string) *PackagePathVisitor {
	return &PackagePathVisitor{
		packageManager: packageManager,
		packagePaths:   packagePaths,
	}
}

func (v *PackagePathVisitor) getExpectedFilepathForPackagePlaceholder(placeholder string) (string, error) {
	filepath, ok := v.packagePaths[placeholder]
	if !ok {
		return placeholder, fmt.Errorf("there is no corresponding real path for this valid placeholder %s", placeholder)
	}
	return filepath, nil
}

// VisitAndReplaceString replaces a string with a package path if its a valid package placeholder.
func (v *PackagePathVisitor) VisitAndReplaceString(s string) (string, error) {
	if len(s) > 0 && s[0] != '$' {
		// don't error here as its not a placeholder
		return s, nil
	}

	placeholderRef, err := v.packageManager.PlaceholderPath(s)
	if err != nil {
		return "", err
	}

	// get the expected filepath based on the list of packages in the config
	filepath, err := v.getExpectedFilepathForPackagePlaceholder(placeholderRef.nestedPath)
	if err != nil {
		return "", err
	}

	withReplacedRefs := strings.Replace(s, placeholderRef.matchedPlaceholder, filepath, 1)
	return withReplacedRefs, nil
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

	// if there is an error do not replace the string
	// the reasoning is because this could be any string so we don't want to nullify any string
	withReplacedRefs, err := v.VisitAndReplaceString(s)
	if err != nil {
		return data, err
	}

	// If the input was a pointer, return a pointer.
	if t.Kind() == reflect.Ptr {
		return &withReplacedRefs, nil
	}
	return withReplacedRefs, nil
}
