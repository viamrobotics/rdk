package functionvm

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestValue(t *testing.T) {
	t.Run("Type", func(t *testing.T) {
		test.That(t, NewString("").Type(), test.ShouldEqual, ValueTypeString)
		test.That(t, NewBool(false).Type(), test.ShouldEqual, ValueTypeBool)
		test.That(t, NewFloat(0).Type(), test.ShouldEqual, ValueTypeFloat)
		test.That(t, NewInt(0).Type(), test.ShouldEqual, ValueTypeInt)
		test.That(t, NewUndefined().Type(), test.ShouldEqual, ValueTypeUndefined)
	})

	t.Run("Interface", func(t *testing.T) {
		test.That(t, NewString("a").Interface(), test.ShouldEqual, "a")
		test.That(t, NewBool(true).Interface(), test.ShouldEqual, true)
		test.That(t, NewFloat(0.1).Interface(), test.ShouldEqual, 0.1)
		test.That(t, NewInt(1).Interface(), test.ShouldEqual, 1)
		test.That(t, NewUndefined().Interface(), test.ShouldResemble, Undefined{})
	})

	allMustExcept := func(t *testing.T, val Value, valType ValueType) {
		t.Helper()
		for _, vt := range []ValueType{
			ValueTypeString,
			ValueTypeBool,
			ValueTypeFloat,
			ValueTypeInt,
			ValueTypeUndefined,
		} {
			if vt == valType {
				continue
			}
			t.Run(fmt.Sprintf("is not a %s", vt), func(t *testing.T) {
				switch vt {
				case ValueTypeString:
					_, err := val.String()
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("expected type %q", vt))
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
					test.That(t, func() { val.MustString() }, test.ShouldPanic)
				case ValueTypeBool:
					_, err := val.Bool()
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("expected type %q", vt))
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
					test.That(t, func() { val.MustBool() }, test.ShouldPanic)
				case ValueTypeFloat:
					_, err := val.Float()
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("expected type %q", vt))
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
					test.That(t, func() { val.MustFloat() }, test.ShouldPanic)
				case ValueTypeInt:
					_, err := val.Int()
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("expected type %q", vt))
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
					test.That(t, func() { val.MustInt() }, test.ShouldPanic)
				case ValueTypeUndefined:
					_, err := val.Undefined()
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("expected type %q", vt))
					test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
					test.That(t, func() { val.MustUndefined() }, test.ShouldPanic)
				}
			})
		}

		switch valType {
		case ValueTypeFloat, ValueTypeInt:
		default:
			_, err := val.Number()
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "expected type")
			test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("but got %q", valType))
			test.That(t, func() { val.MustNumber() }, test.ShouldPanic)
		}
	}

	t.Run("Bool", func(t *testing.T) {
		value := NewBool(true)
		val, err := value.Bool()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldBeTrue)
		test.That(t, value.MustBool(), test.ShouldBeTrue)
		allMustExcept(t, value, value.Type())
		test.That(t, value.Stringer(), test.ShouldEqual, "true")
	})

	t.Run("Float", func(t *testing.T) {
		value := NewFloat(1.2)
		val, err := value.Float()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldEqual, 1.2)
		test.That(t, value.MustFloat(), test.ShouldEqual, 1.2)
		allMustExcept(t, value, value.Type())
		test.That(t, value.Stringer(), test.ShouldEqual, "1.2")

		num, err := value.Number()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, num, test.ShouldEqual, 1.2)
		test.That(t, value.MustNumber(), test.ShouldEqual, 1.2)
	})

	t.Run("Int", func(t *testing.T) {
		value := NewInt(1)
		val, err := value.Int()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldEqual, 1)
		test.That(t, value.MustInt(), test.ShouldEqual, 1)
		allMustExcept(t, value, value.Type())
		test.That(t, value.Stringer(), test.ShouldEqual, "1")

		num, err := value.Number()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, num, test.ShouldEqual, 1)
		test.That(t, value.MustNumber(), test.ShouldEqual, 1)
	})

	t.Run("Undefined", func(t *testing.T) {
		value := NewUndefined()
		val, err := value.Undefined()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldResemble, Undefined{})
		test.That(t, value.MustUndefined(), test.ShouldResemble, Undefined{})
		allMustExcept(t, value, value.Type())
		test.That(t, value.Stringer(), test.ShouldEqual, "<undefined>")
	})
}
