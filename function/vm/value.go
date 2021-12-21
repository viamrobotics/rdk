package functionvm

import (
	"fmt"
	"math"

	"github.com/pkg/errors"
)

// A Value represents some scalar/array/map type.
type Value interface {
	Type() ValueType

	Interface() interface{}

	Bool() (bool, error)
	MustBool() bool

	String() (string, error)
	MustString() string

	Float() (float64, error)
	MustFloat() float64

	Number() (float64, error)
	MustNumber() float64

	Int() (int64, error)
	MustInt() int64

	Undefined() (Undefined, error)
	MustUndefined() Undefined

	Stringer() string
}

// A ValueType denotes the type of value this is.
type ValueType int

// The set of ValueTypes
const (
	ValueTypeString = ValueType(iota)
	ValueTypeBool
	ValueTypeFloat
	ValueTypeInt
	ValueTypeUndefined
	ValueTypeUnknown
)

// Undefined represents a JavaScript like non-null, but not defined value.
type Undefined struct{}

func (vt ValueType) String() string {
	switch vt {
	case ValueTypeString:
		return "string"
	case ValueTypeBool:
		return "bool"
	case ValueTypeFloat:
		return "float"
	case ValueTypeInt:
		return "int"
	case ValueTypeUndefined:
		return "undefined"
	case ValueTypeUnknown:
		fallthrough
	default:
		return "unknown"
	}
}

type primitiveValue struct {
	valType ValueType
	val     interface{}
}

func (pv *primitiveValue) Type() ValueType {
	return pv.valType
}

func (pv *primitiveValue) Interface() interface{} {
	return pv.val
}

func (pv *primitiveValue) String() (string, error) {
	if pv.valType != ValueTypeString {
		return "", errors.Errorf("expected type %q but got %q", ValueTypeString, pv.valType)
	}
	return pv.val.(string), nil
}

func (pv *primitiveValue) MustString() string {
	val, err := pv.String()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Bool() (bool, error) {
	if pv.valType != ValueTypeBool {
		return false, errors.Errorf("expected type %q but got %q", ValueTypeBool, pv.valType)
	}
	return pv.val.(bool), nil
}

func (pv *primitiveValue) MustBool() bool {
	val, err := pv.Bool()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Float() (float64, error) {
	if pv.valType != ValueTypeFloat {
		return math.NaN(), errors.Errorf("expected type %q but got %q", ValueTypeFloat, pv.valType)
	}
	return pv.val.(float64), nil
}

func (pv *primitiveValue) MustFloat() float64 {
	val, err := pv.Float()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Int() (int64, error) {
	if pv.valType != ValueTypeInt {
		return 0, errors.Errorf("expected type %q but got %q", ValueTypeInt, pv.valType)
	}
	return pv.val.(int64), nil
}

func (pv *primitiveValue) MustInt() int64 {
	val, err := pv.Int()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Number() (float64, error) {
	switch pv.valType {
	case ValueTypeInt:
		return float64(pv.val.(int64)), nil
	case ValueTypeFloat:
		return pv.val.(float64), nil
	default:
		return 0, errors.Errorf("expected type [%q, %q] but got %q", ValueTypeInt, ValueTypeFloat, pv.valType)
	}
}

func (pv *primitiveValue) MustNumber() float64 {
	val, err := pv.Number()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Undefined() (Undefined, error) {
	if pv.valType != ValueTypeUndefined {
		return Undefined{}, errors.Errorf("expected type %q but got %q", ValueTypeUndefined, pv.valType)
	}
	return pv.val.(Undefined), nil
}

func (pv *primitiveValue) MustUndefined() Undefined {
	val, err := pv.Undefined()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) Stringer() string {
	if pv.valType == ValueTypeUndefined {
		return "<undefined>"
	}
	return fmt.Sprintf("%v", pv.val)
}

// NewString returns a new string based value.
func NewString(val string) Value {
	return &primitiveValue{
		valType: ValueTypeString,
		val:     val,
	}
}

// NewBool returns a new bool based value.
func NewBool(val bool) Value {
	return &primitiveValue{
		valType: ValueTypeBool,
		val:     val,
	}
}

// NewFloat returns a new float based value.
func NewFloat(val float64) Value {
	return &primitiveValue{
		valType: ValueTypeFloat,
		val:     val,
	}
}

// NewInt returns a new integer based value.
func NewInt(val int64) Value {
	return &primitiveValue{
		valType: ValueTypeInt,
		val:     val,
	}
}

// NewUndefined returns a new undefined based value.
func NewUndefined() Value {
	return &primitiveValue{
		valType: ValueTypeUndefined,
		val:     Undefined{},
	}
}
