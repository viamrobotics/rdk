package utils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// ValidNameRegex is the pattern that matches to a valid name.
// The name must begin with a letter or number i.e. [a-zA-Z0-9],
// and can only contain up to 60, letters, numbers, dashes, and underscores i.e. [-\w]*.
var ValidNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([-\w]){0,59}$`)

// ErrInvalidName returns a human-readable error for when ValidNameRegex doesn't match.
func ErrInvalidName(name string) error {
	if len(name) > 60 {
		// this is broken out to improve readability of the error msg
		return errors.Errorf("name %q must be 60 characters or fewer", name)
	}
	return errors.Errorf("name %q must start with a letter or number and must only contain letters, numbers, dashes, and underscores", name)
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
