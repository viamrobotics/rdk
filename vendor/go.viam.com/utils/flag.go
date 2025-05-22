package utils

import (
	"bytes"
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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
		return fmt.Errorf("error extracing flags: %w", err)
	}
	if err := cmdLine.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errors.New(buf.String())
		}
		return errors.Errorf("%s\n%s", buf.String(), err)
	}

	if err := UnmarshalFlags(cmdLine, into); err != nil {
		if errors.Is(err, errRequiredFlagUnspecified) {
			cmdLine.Usage()
			return errors.Errorf("%s\n%s", err, buf.String())
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
		return errors.Errorf("expected %T to be an addressable struct", into)
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
				return errors.Errorf("found more than one extra field %q; first was %q", field.Name, extraFieldName)
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
							return errors.Wrapf(err, "error parsing positional argument %d", info.Position)
						}
						val = int(conv)
					case reflect.Bool:
						conv, err := strconv.ParseBool(strVal)
						if err != nil {
							return errors.Wrapf(err, "error parsing positional argument %d", info.Position)
						}
						val = conv
					case reflect.Array, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
						reflect.Float64, reflect.Func, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8,
						reflect.Interface, reflect.Invalid, reflect.Map, reflect.Ptr, reflect.Slice, reflect.Struct,
						reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uintptr,
						reflect.UnsafePointer:
						fallthrough
					default:
						return errors.Errorf("error parsing positional argument %d for %s: do not know how to unmarshal a %q",
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
					return errors.Wrapf(errRequiredFlagUnspecified, "error parsing flag %q", info.Name)
				}
				if info.IsFlagVal {
					val = info.Default
				} else {
					val = info.DefaultIfc
				}
			} else {
				if info.IsFlagVal {
					flagValP, ok := flagVal.Value.(*flagValueProxy)
					if !ok {
						panic(errors.Errorf("expected *flagValueProxy but got %T", flagVal.Value))
					}
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
			return errors.Wrapf(errRequiredFlagUnspecified, "error parsing flag %q", info.Name)
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
			if reflect.PointerTo(fieldV.Type()).Implements(flagValueT) {
				fieldV = fieldV.Addr()
			}
			if err := fieldV.Interface().(flag.Value).Set(val.(string)); err != nil { // will always be string
				return errors.Wrapf(err, "error parsing flag %q", info.Name)
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
			return errors.Errorf("unspecified arguments provided: %v", remArgs)
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
	if field.Type.Implements(flagValueT) || reflect.PointerTo(field.Type).Implements(flagValueT) {
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
				return flagInfo{}, errors.Errorf("error parsing flag info for %q: default must have value", fieldName)
			}
			info.Default = parted[1]
			switch field.Type.Kind() {
			case reflect.Bool:
				conv, err := strconv.ParseBool(info.Default)
				if err != nil {
					return flagInfo{}, errors.Wrapf(err, "error parsing flag info default for %q", fieldName)
				}
				info.DefaultIfc = conv
			case reflect.String:
				info.DefaultIfc = info.Default
			case reflect.Int:
				conv, err := strconv.ParseInt(info.Default, 10, 64)
				if err != nil {
					return flagInfo{}, errors.Wrapf(err, "error parsing flag info default for %q", fieldName)
				}
				info.DefaultIfc = int(conv)
			case reflect.Array, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
				reflect.Float64, reflect.Func, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8,
				reflect.Interface, reflect.Invalid, reflect.Map, reflect.Ptr, reflect.Slice, reflect.Struct,
				reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uintptr,
				reflect.UnsafePointer:
				fallthrough
			default:
				return flagInfo{}, errors.Errorf("error parsing flag  infofor %q: unsupported default for flag kind %q", info.Name, field.Type.Kind())
			}
		case "usage":
			if len(parted) != 2 {
				return flagInfo{}, errors.Errorf("error parsing flag info for %q: usage must have usage string", fieldName)
			}
			info.Usage = parted[1]
		case "extra":
			if field.Type != reflect.TypeOf([]string(nil)) {
				return flagInfo{}, errors.Errorf("error parsing flag info for %q: extra field must be []string", fieldName)
			}
			info.Extra = true
		}
	}
	return info, nil
}

var flagValueT = reflect.TypeOf((*flag.Value)(nil)).Elem()

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
		return errors.Errorf("expected %T to be an addressable struct", from)
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
				return errors.Errorf("found more than one extra field %q; first was %q", field.Name, extraFieldName)
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
			if reflect.PointerTo(fieldT).Implements(flagValueT) {
				flagValue = flagValue.Addr()
			}
			flagSet.Var(&flagValueProxy{Value: flagValue.Interface().(flag.Value)}, info.Name, info.Usage)
			continue
		}
		switch fieldT.Kind() {
		case reflect.Bool:
			var defaultVal bool
			if info.Default != "" {
				defaultVal, ok = info.DefaultIfc.(bool)
				if !ok {
					panic(errors.Errorf("expected int but got %T", info.DefaultIfc))
				}
			}
			flagSet.Bool(info.Name, defaultVal, info.Usage)
		case reflect.String:
			flagSet.String(info.Name, info.Default, info.Usage)
		case reflect.Int:
			var defaultVal int
			if info.Default != "" {
				defaultVal, ok = info.DefaultIfc.(int)
				if !ok {
					panic(errors.Errorf("expected int but got %T", info.DefaultIfc))
				}
			}
			flagSet.Int(info.Name, defaultVal, info.Usage)
		case reflect.Slice:
			sliceElem := fieldT.Elem()
			var ctor func(val string) (interface{}, error)
			if sliceElem.Implements(flagValueT) || reflect.PointerTo(sliceElem).Implements(flagValueT) {
				ctor = func(val string) (interface{}, error) {
					newSliceElem := reflect.New(sliceElem)
					if err := newSliceElem.Interface().(flag.Value).Set(val); err != nil {
						return nil, errors.Wrapf(err, "error setting flag for %q", info.Name)
					}
					return newSliceElem.Elem().Interface(), nil
				}
			} else {
				return errors.Errorf("error extracting flag for %q: unsupported slice element type %q", info.Name, sliceElem)
			}
			flagSet.Var(&sliceFlag{
				ctor: ctor,
			}, info.Name, info.Usage)
		case reflect.Array, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
			reflect.Float64, reflect.Func, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8,
			reflect.Interface, reflect.Invalid, reflect.Map, reflect.Ptr, reflect.Struct,
			reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uintptr,
			reflect.UnsafePointer:
			fallthrough
		default:
			return errors.Errorf("error extracting flag for %q: unsupported flag kind %q", info.Name, fieldT.Kind())
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

// String returns the set value.
func (npf *NetPortFlag) String() string {
	return fmt.Sprintf("%v", int(*npf))
}

// Set attempts to set the value as a network port.
func (npf *NetPortFlag) Set(val string) error {
	portParsed, err := strconv.ParseUint(val, 10, 16)
	if err != nil {
		return err
	}
	*npf = NetPortFlag(portParsed)
	return nil
}

// Get returns the value as an integer.
func (npf *NetPortFlag) Get() interface{} {
	return int(*npf)
}
