package utils

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"
)

type StringFlags []string

func (sf *StringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

func (sf *StringFlags) String() string {
	return fmt.Sprint([]string(*sf))
}

// ParseFlags parses arguments derived from and into the given into struct.
func ParseFlags(args []string, into interface{}) error {
	cmdLine := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var buf bytes.Buffer
	cmdLine.SetOutput(&buf)

	if err := extractFlags(cmdLine, into); err != nil {
		return err
	}
	if err := cmdLine.Parse(args[1:]); err != nil {
		return err
	}

	if err := UnmarshalFlags(cmdLine, into); err != nil {
		if err == errRequiredFlagUnspecified {
			cmdLine.Usage()
			return errors.New(buf.String())
		}
		return err
	}
	return nil
}

var errRequiredFlagUnspecified = errors.New("flag required but not specified")

// UnmarshalFlags unmarshals parsed flags into the given value.
func UnmarshalFlags(flagSet *flag.FlagSet, into interface{}) error {
	v := reflect.ValueOf(into)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagStr, ok := field.Tag.Lookup("flag")
		if !ok {
			continue
		}
		info, err := parseFlagInfo(field.Name, flagStr)
		if err != nil {
			return err
		}
		flagVal := flagSet.Lookup(info.Name)
		if flagVal == nil {
			if info.Required {
				return errRequiredFlagUnspecified
			}
			continue
		}
		val := flagVal.Value.(flag.Getter).Get()
		if info.Required && (val == nil || reflect.ValueOf(val).IsZero()) {
			return errRequiredFlagUnspecified
		}
		v.Field(i).Set(reflect.ValueOf(val))
	}
	return nil
}

type flagInfo struct {
	Name     string
	Default  string
	Usage    string
	Required bool
}

func parseFlagInfo(fieldName, val string) (flagInfo, error) {
	valParts := strings.Split(val, ",")
	if len(valParts) == 0 {
		return flagInfo{Name: fieldName}, nil
	}
	var info flagInfo
	info.Name = valParts[0]
	if info.Name == "" {
		info.Name = fieldName
	}
	for _, part := range valParts[1:] {
		parted := strings.SplitN(part, "=", 2)
		switch parted[0] {
		case "required":
			info.Required = true
		case "default":
			if len(parted) != 2 {
				return flagInfo{}, errors.New("default must have value")
			}
			info.Default = parted[1]
		case "usage":
			if len(parted) != 2 {
				return flagInfo{}, errors.New("usage must have usage string")
			}
			info.Usage = parted[1]
		}
	}
	return info, nil
}

func extractFlags(flagSet *flag.FlagSet, from interface{}) error {
	t := reflect.TypeOf(from)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagStr, ok := field.Tag.Lookup("flag")
		if !ok {
			continue
		}
		info, err := parseFlagInfo(field.Name, flagStr)
		if err != nil {
			return err
		}
		switch field.Type.Kind() {
		case reflect.String:
			flagSet.String(info.Name, info.Default, info.Usage)
		default:
			return fmt.Errorf("unsupported flag kind %q", field.Type.Kind())
		}
	}
	return nil
}
