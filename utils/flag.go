package utils

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strconv"
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
		if errors.Is(err, errRequiredFlagUnspecified) {
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
		info, err := parseFlagInfo(field, flagStr)
		if err != nil {
			return err
		}
		var val interface{}
		if info.Positional {
			strVal := flagSet.Arg(info.Position)
			if info.IsFlagVal {
				if strVal == "" {
					val = info.Default
				} else {
					val = strVal
				}
			} else {
				if strVal == "" {
					val = info.DefaultIfc
				} else {
					switch field.Type.Kind() {
					case reflect.String:
						val = strVal
					case reflect.Int:
						conv, err := strconv.ParseInt(strVal, 10, 64)
						if err != nil {
							return fmt.Errorf("error parsing positional argument %d: %w", info.Position, err)
						}
						val = int(conv)
					default:
						return fmt.Errorf("error parsing positional argument %d for %s: do not know how to unmarshal a %q",
							info.Position,
							info.Name,
							field.Type.Kind())
					}
				}
			}
		} else {
			flagVal := flagSet.Lookup(info.Name)
			if flagVal == nil {
				if info.Required {
					return fmt.Errorf("error parsing flag %q: %w", info.Name, errRequiredFlagUnspecified)
				}
				if info.IsFlagVal {
					val = info.Default
				} else {
					val = info.DefaultIfc
				}
			} else {
				if info.IsFlagVal {
					flagValP := flagVal.Value.(*flagValueProxy)
					if flagValP.IsSet {
						// no work to do here
						continue
					}
					val = info.Default
				} else {
					val = flagVal.Value.(flag.Getter).Get()
				}
			}
		}
		if info.Required && (val == nil || reflect.ValueOf(val).IsZero()) {
			return fmt.Errorf("error parsing flag %q: %w", info.Name, errRequiredFlagUnspecified)
		}

		valV := reflect.ValueOf(val)
		fieldV := v.Field(i)
		if info.IsFlagVal {
			if reflect.PtrTo(fieldV.Type()).Implements(flagValueT) {
				fieldV = fieldV.Addr()
			}
			if err := fieldV.Interface().(flag.Value).Set(val.(string)); err != nil { // will always be string
				return fmt.Errorf("error parsing flag %q: %w", info.Name, err)
			}
			continue
		}
		if !valV.IsValid() || valV.IsZero() {
			continue
		}
		if field.Type.Kind() == reflect.Slice {
			fieldSlice := v.Field(i)
			for i := 0; i < valV.Len(); i++ {
				fieldSlice = reflect.Append(fieldSlice, valV.Index(i).Elem())
			}
			fieldV.Set(fieldSlice)
		} else {
			vT := fieldV.Type()
			if valV.Type().AssignableTo(vT) {
				fieldV.Set(valV)
			} else {
				fieldV.Set(valV.Convert(vT))
			}
		}
	}
	return nil
}

type flagInfo struct {
	Name       string
	Default    string
	DefaultIfc interface{}
	Usage      string
	Required   bool
	Positional bool
	Position   int
	IsFlagVal  bool
}

func parseFlagInfo(field reflect.StructField, val string) (flagInfo, error) {
	fieldName := field.Name
	valParts := strings.Split(val, ",")
	if len(valParts) == 0 {
		return flagInfo{Name: fieldName}, nil
	}
	var info flagInfo
	info.Name = valParts[0]
	if info.Name == "" {
		info.Name = fieldName
	}
	if posIdx, err := strconv.ParseInt(info.Name, 10, 64); err == nil {
		info.Positional = true
		info.Position = int(posIdx)
	}
	if field.Type.Implements(flagValueT) || reflect.PtrTo(field.Type).Implements(flagValueT) {
		info.IsFlagVal = true
	}
	for _, part := range valParts[1:] {
		parted := strings.SplitN(part, "=", 2)
		switch parted[0] {
		case "required":
			info.Required = true
		case "default":
			if len(parted) != 2 {
				return flagInfo{}, fmt.Errorf("error parsing flag info for %q: default must have value", fieldName)
			}
			info.Default = parted[1]
			switch field.Type.Kind() {
			case reflect.String:
				info.DefaultIfc = info.Default
			case reflect.Int:
				conv, err := strconv.ParseInt(info.Default, 10, 64)
				if err != nil {
					return flagInfo{}, fmt.Errorf("error parsing flag info default for %q: %w", fieldName, err)
				}
				info.DefaultIfc = int(conv)
			default:
				return flagInfo{}, fmt.Errorf("error parsing flag  infofor %q: unsupported default for flag kind %q", info.Name, field.Type.Kind())
			}
		case "usage":
			if len(parted) != 2 {
				return flagInfo{}, fmt.Errorf("error parsing flag info for %q: usage must have usage string", fieldName)
			}
			info.Usage = parted[1]
		}
	}
	return info, nil
}

type flagUnmarshaler interface {
	UnmarshalFlag(flagName, val string) error
}

var (
	flagUnmarshalerT = reflect.TypeOf((*flagUnmarshaler)(nil)).Elem()
	flagValueT       = reflect.TypeOf((*flag.Value)(nil)).Elem()
)

type flagValueProxy struct {
	flag.Value
	IsSet bool
}

func (fvp *flagValueProxy) String() string {
	if fvp.Value == nil {
		return ""
	}
	return fvp.Value.String()
}

func (fvp *flagValueProxy) Set(val string) error {
	fvp.IsSet = true
	return fvp.Value.Set(val)
}

func (fvp *flagValueProxy) Get() interface{} {
	if fvp.Value == nil {
		return nil
	}
	return fvp.Value.(flag.Getter).Get()
}

func extractFlags(flagSet *flag.FlagSet, from interface{}) error {
	v := reflect.ValueOf(from)
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagStr, ok := field.Tag.Lookup("flag")
		if !ok {
			continue
		}
		info, err := parseFlagInfo(field, flagStr)
		if err != nil {
			return err
		}
		if info.IsFlagVal {
			flagValue := v.Field(i)
			if reflect.PtrTo(field.Type).Implements(flagValueT) {
				flagValue = flagValue.Addr()
			}
			flagSet.Var(&flagValueProxy{Value: flagValue.Interface().(flag.Value)}, info.Name, info.Usage)
			continue
		}
		switch field.Type.Kind() {
		case reflect.String:
			flagSet.String(info.Name, info.Default, info.Usage)
		case reflect.Int:
			var defaultVal int
			if info.Default != "" {
				defaultVal = info.DefaultIfc.(int)
			}
			flagSet.Int(info.Name, defaultVal, info.Usage)
		case reflect.Slice:
			sliceElem := field.Type.Elem()
			var ctor func(val string) (interface{}, error)
			if sliceElem.Implements(flagUnmarshalerT) || reflect.PtrTo(sliceElem).Implements(flagUnmarshalerT) {
				ctor = func(val string) (interface{}, error) {
					newSliceElem := reflect.New(sliceElem)
					if err := newSliceElem.Interface().(flagUnmarshaler).UnmarshalFlag(info.Name, val); err != nil {
						return nil, fmt.Errorf("error unmarshaling flag for %q: %w", info.Name, err)
					}
					return newSliceElem.Elem().Interface(), nil
				}
			} else {
				switch sliceElem.Kind() {
				default:
					return fmt.Errorf("error extracting flag for %q: unsupported slice element type %q", info.Name, sliceElem)
				}
			}
			flagSet.Var(&sliceFlag{
				ctor: ctor,
			}, info.Name, info.Usage)
		default:
			return fmt.Errorf("error extracting flag for %q: unsupported flag kind %q", info.Name, field.Type.Kind())
		}
	}
	return nil
}

type sliceFlag struct {
	values []interface{}
	ctor   func(val string) (interface{}, error)
}

func (sf *sliceFlag) String() string {
	return fmt.Sprintf("%v", sf.values)
}

func (sf *sliceFlag) Set(val string) error {
	newVal, err := sf.ctor(val)
	if err != nil {
		return err
	}
	sf.values = append(sf.values, newVal)
	return nil
}

func (sf *sliceFlag) Get() interface{} {
	return sf.values
}
