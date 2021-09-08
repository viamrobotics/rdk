package functionvm

import (
	"fmt"

	"github.com/go-errors/errors"
)

// A Value represents some scalar/array/map type.
type Value interface {
	String() (string, error)
	MustString() string
	Stringer() string
}

// A ValueType denotes the type of value this is.
type ValueType int

// The set of ValueTypes.s
const (
	ValueTypeString = ValueType(iota)
	ValueTypeUnknown
)

func (vt ValueType) String() string {
	switch vt {
	case ValueTypeString:
		return "string"
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

func (pv *primitiveValue) MustString() string {
	val, err := pv.String()
	if err != nil {
		panic(err)
	}
	return val
}

func (pv *primitiveValue) String() (string, error) {
	if pv.valType != ValueTypeString {
		return "", errors.Errorf("expected type %q but got %q", ValueTypeString, pv.valType)
	}
	return pv.val.(string), nil
}

func (pv *primitiveValue) Stringer() string {
	return fmt.Sprintf("%v", pv.val)
}

// NewString returns a new string based value.
func NewString(val string) Value {
	return &primitiveValue{
		valType: ValueTypeString,
		val:     val,
	}
}
