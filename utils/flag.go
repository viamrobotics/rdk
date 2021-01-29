package utils

import "fmt"

type StringFlags []string

func (sf *StringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

func (sf *StringFlags) String() string {
	return fmt.Sprint([]string(*sf))
}
