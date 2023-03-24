package utils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// ValidNameRegex is the pattern that matches to a valid name.
// The name must begin with a letter i.e. [a-zA-Z],
// and the body can only contain 0 or more numbers, letters, dashes and underscores i.e. [-\w]*.
var ValidNameRegex = regexp.MustCompile(`^[a-zA-Z][-\w]*$`)

// ErrInvalidName returns a human-readable error for when ValidNameRegex doesn't match.
func ErrInvalidName(name string) error {
	return errors.Errorf("name %q must start with a letter and must only contain letters, numbers, dashes, and underscores", name)
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
