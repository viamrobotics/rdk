package utils

import (
	"reflect"
	"strings"
)

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
