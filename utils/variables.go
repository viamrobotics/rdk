package utils

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var (
	// validResourcenameRegex is the pattern that matches to a valid name.
	// The name must begin with a letter or number i.e. [a-zA-Z0-9],
	// and contains only letters, numbers, dashes, and underscores i.e. [-\w]*.
	//
	// Note:
	// This regex cannot be changed to allow for:
	// - colons: used as delimiters for remote names
	// - plus signs: used for WebRTC track names.
	validResourceNameRegex       = regexp.MustCompile(`^[a-zA-Z0-9][-\w]*$`)
	validResourceNameExplanation = "must start with a letter or number and must only contain letters, numbers, dashes, and underscores"
)

// ValidateResourceName validates that the resource follows our naming requirements.
func ValidateResourceName(name string) error {
	if len(name) > 60 {
		return fmt.Errorf("name %q must be 60 characters or fewer", name)
	}
	if !validResourceNameRegex.MatchString(name) {
		return fmt.Errorf("name %q %s", name, validResourceNameExplanation)
	}
	return nil
}

// ValidateModuleName validates that the module follows our naming requirements.
// the module name is used to create the socket path, so if you modify this, ensure that this only
// accepts valid socket paths.
func ValidateModuleName(name string) error {
	if len(name) > 200 {
		return fmt.Errorf("module name %q must be 200 characters or fewer", name)
	}
	if !validResourceNameRegex.MatchString(name) {
		return fmt.Errorf("module name %q %s", name, validResourceNameExplanation)
	}
	return nil
}

// ValidatePackageName validates that the package follows our naming requirements.
func ValidatePackageName(name string) error {
	if len(name) > 200 {
		return fmt.Errorf("package name %q must be 200 characters or fewer", name)
	}
	if !validResourceNameRegex.MatchString(name) {
		return fmt.Errorf("package name %q %s", name, validResourceNameExplanation)
	}
	return nil
}

// ValidateRemoteName validates that the remote follows our naming requirements.
func ValidateRemoteName(name string) error {
	// same as resource name validation for now
	return ValidateResourceName(name)
}

// A TypedName stores both the name and type of the variable.
type TypedName struct {
	Name string
	Type string
}

// JSONTags returns a slice of strings of the variable names in the JSON tags of a struct.
func JSONTags(s interface{}) []TypedName {
	tags := []TypedName{}
	val := reflect.ValueOf(s)
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		fieldName := t.Name

		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-":
		case "":
			tags = append(tags, TypedName{fieldName, t.Type.Name()}) // if json tag doesn't exist, just use field name
		default:
			parts := strings.Split(jsonTag, ",")
			name := parts[0]
			if name == "" {
				name = fieldName
			}
			tags = append(tags, TypedName{name, t.Type.Name()})
		}
	}
	return tags
}
