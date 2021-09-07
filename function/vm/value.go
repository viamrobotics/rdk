package functionvm

import "github.com/go-errors/errors"

// A Value represents some scalar/array/map type.
type Value interface {
	MustString() string
}

type valueType int

const (
	valueTypeString = valueType(iota)
	valueTypeUnknown
)

func (vt valueType) String() string {
	switch vt {
	case valueTypeString:
		return "string"
	case valueTypeUnknown:
		fallthrough
	default:
		return "unknown"
	}
}

type primitiveValue struct {
	valType valueType
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
	if pv.valType != valueTypeString {
		return "", errors.Errorf("expected type %q but got %q", valueTypeString, pv.valType)
	}
	return pv.val.(string), nil
}

// NewString returns a new string based value.
func NewString(val string) Value {
	return &primitiveValue{
		valType: valueTypeString,
		val:     val,
	}
}
