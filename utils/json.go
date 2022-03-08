package utils

import (
	"reflect"
	"strings"
)

// JSONTags returns a slice of strings of the variable names in the JSON tags of a struct.
func JSONTags(s interface{}) []string {
	tags := []string{}
	val := reflect.ValueOf(s)
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		fieldName := t.Name

		switch jsonTag := t.Tag.Get("json"); jsonTag {
		case "-":
		case "":
			tags = append(tags, fieldName) // if json tag doesn't exist, just use field name
		default:
			parts := strings.Split(jsonTag, ",")
			name := parts[0]
			if name == "" {
				name = fieldName
			}
			tags = append(tags, name)
		}
	}
	return tags
}
