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

// ParseFlags parses arguments derived from and into the given into struct.
func ParseFlags(args []string, into interface{}) error {
	if len(args) == 0 || into == nil {
		return nil
	}
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
			return fmt.Errorf("%w\n%s", err, buf.String())
		}
		return err
	}
	return nil
}

var errRequiredFlagUnspecified = errors.New("flag required but not specified")

// UnmarshalFlags unmarshals parsed flags into the given value.
func UnmarshalFlags(flagSet *flag.FlagSet, into interface{}) error {
	numPositionalsTotal := len(flagSet.Args())
	numPositionalsLeft := make(map[int]struct{}, numPositionalsTotal)
	for i := 0; i < numPositionalsTotal; i++ {
		numPositionalsLeft[i] = struct{}{}
	}

	v := reflect.ValueOf(into)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Type().Kind() != reflect.Struct || !v.CanAddr() {
		return fmt.Errorf("expected %T to be an addressable struct", into)
	}
	var extraField reflect.Value
	var extraFieldName string
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
		if info.Extra {
			if extraField.IsValid() {
				return fmt.Errorf("found more than one extra field %q; first was %q", field.Name, extraFieldName)
			}
			extraField = v.Field(i)
			extraFieldName = field.Name
			continue
		}
		var val interface{}
		var flagValIsSet bool
		if info.Positional {
			strVal := flagSet.Arg(info.Position)
			if info.Position < numPositionalsTotal {
				delete(numPositionalsLeft, info.Position)
			}
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
					case reflect.Bool:
						conv, err := strconv.ParseBool(strVal)
						if err != nil {
							return fmt.Errorf("error parsing positional argument %d: %w", info.Position, err)
						}
						val = conv
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
						flagValIsSet = true
						if field.Type.Kind() != reflect.Ptr {
							// no more work to do here
							continue
						}
						val = flagVal.Value.(flag.Getter).Get()
					} else {
						val = info.Default
					}
				} else {
					val = flagVal.Value.(flag.Getter).Get()
				}
			}
		}
		if info.Required && (val == nil || reflect.ValueOf(val).IsZero()) {
			return fmt.Errorf("error parsing flag %q: %w", info.Name, errRequiredFlagUnspecified)
		}

		valV := reflect.ValueOf(val)
		if !valV.IsValid() {
			continue
		}
		if !info.DefaultSet && valV.IsZero() {
			continue
		}
		fieldV := v.Field(i)
		if info.IsFlagVal && !flagValIsSet {
			if reflect.PtrTo(fieldV.Type()).Implements(flagValueT) {
				fieldV = fieldV.Addr()
			}
			if err := fieldV.Interface().(flag.Value).Set(val.(string)); err != nil { // will always be string
				return fmt.Errorf("error parsing flag %q: %w", info.Name, err)
			}
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
			if vT.Kind() == reflect.Ptr {
				newFieldV := reflect.New(vT.Elem())
				fieldV.Set(newFieldV)
				if valV.Kind() != reflect.Ptr {
					fieldV = fieldV.Elem()
					vT = vT.Elem()
				}
			}
			if valV.Type().AssignableTo(vT) {
				fieldV.Set(valV)
			} else {
				fieldV.Set(valV.Convert(vT))
			}
		}
	}
	if len(numPositionalsLeft) != 0 {
		remArgs := make([]string, 0, len(numPositionalsLeft))
		for idx := range numPositionalsLeft {
			remArgs = append(remArgs, flagSet.Arg(idx))
		}
		if !extraField.IsValid() {
			return fmt.Errorf("unspecified arguments provided: %v", remArgs)
		}
		extraField.Set(reflect.ValueOf(remArgs))
	}
	return nil
}

type flagInfo struct {
	Name       string
	Default    string
	DefaultIfc interface{}
	DefaultSet bool
	Usage      string
	Required   bool
	Positional bool
	Position   int
	IsFlagVal  bool
	Extra      bool
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
		info.Name = fmt.Sprintf("positional_arg_%d", posIdx)
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
			info.DefaultSet = true
			if len(parted) != 2 {
				return flagInfo{}, fmt.Errorf("error parsing flag info for %q: default must have value", fieldName)
			}
			info.Default = parted[1]
			switch field.Type.Kind() {
			case reflect.Bool:
				conv, err := strconv.ParseBool(info.Default)
				if err != nil {
					return flagInfo{}, fmt.Errorf("error parsing flag info default for %q: %w", fieldName, err)
				}
				info.DefaultIfc = conv
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
		case "extra":
			if field.Type != reflect.TypeOf([]string(nil)) {
				return flagInfo{}, fmt.Errorf("error parsing flag info for %q: extra field must be []string", fieldName)
			}
			info.Extra = true
		}
	}
	return info, nil
}

var (
	flagValueT = reflect.TypeOf((*flag.Value)(nil)).Elem()
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
	if t.Kind() != reflect.Struct || !v.CanAddr() {
		return fmt.Errorf("expected %T to be an addressable struct", from)
	}
	var extraField reflect.Value
	var extraFieldName string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue // unexported
		}
		flagStr, ok := field.Tag.Lookup("flag")
		if !ok {
			continue
		}
		info, err := parseFlagInfo(field, flagStr)
		if err != nil {
			return err
		}
		if info.Extra {
			if extraField.IsValid() {
				return fmt.Errorf("found more than one extra field %q; first was %q", field.Name, extraFieldName)
			}
			extraField = v.Field(i)
			extraFieldName = field.Name
			continue
		}
		fieldT := field.Type
		if fieldT.Kind() == reflect.Ptr {
			fieldT = fieldT.Elem()
		}
		if info.IsFlagVal {
			flagValue := v.Field(i)
			if flagValue.Kind() == reflect.Ptr {
				flagValue = reflect.New(fieldT).Elem()
			}
			if reflect.PtrTo(fieldT).Implements(flagValueT) {
				flagValue = flagValue.Addr()
			}
			flagSet.Var(&flagValueProxy{Value: flagValue.Interface().(flag.Value)}, info.Name, info.Usage)
			continue
		}
		switch fieldT.Kind() {
		case reflect.Bool:
			var defaultVal bool
			if info.Default != "" {
				defaultVal = info.DefaultIfc.(bool)
			}
			flagSet.Bool(info.Name, defaultVal, info.Usage)
		case reflect.String:
			flagSet.String(info.Name, info.Default, info.Usage)
		case reflect.Int:
			var defaultVal int
			if info.Default != "" {
				defaultVal = info.DefaultIfc.(int)
			}
			flagSet.Int(info.Name, defaultVal, info.Usage)
		case reflect.Slice:
			sliceElem := fieldT.Elem()
			var ctor func(val string) (interface{}, error)
			if sliceElem.Implements(flagValueT) || reflect.PtrTo(sliceElem).Implements(flagValueT) {
				ctor = func(val string) (interface{}, error) {
					newSliceElem := reflect.New(sliceElem)
					if err := newSliceElem.Interface().(flag.Value).(flag.Value).Set(val); err != nil {
						return nil, fmt.Errorf("error setting flag for %q: %w", info.Name, err)
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
			return fmt.Errorf("error extracting flag for %q: unsupported flag kind %q", info.Name, fieldT.Kind())
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

// NetPortFlag is used to correctly set and validate a
// network port.
type NetPortFlag int

func (npf *NetPortFlag) String() string {
	return fmt.Sprintf("%v", int(*npf))
}

func (npf *NetPortFlag) Set(val string) error {
	portParsed, err := strconv.ParseUint(val, 10, 16)
	if err != nil {
		return err
	}
	*npf = NetPortFlag(portParsed)
	return nil
}

func (npf *NetPortFlag) Get() interface{} {
	return int(*npf)
}
