package tensor

import (
	"fmt"
	"math"
	"reflect"
	"unsafe"

	"github.com/chewxy/hm"
	"github.com/pkg/errors"
)

// Dtype represents a data type of a Tensor. Concretely it's implemented as an embedded reflect.Type
// which allows for easy reflection operations. It also implements hm.Type, for type inference in Gorgonia
type Dtype struct {
	reflect.Type
}

// note: the Name() and String() methods are already defined in reflect.Type. Might as well use the composed methods

func (dt Dtype) Apply(hm.Subs) hm.Substitutable                { return dt }
func (dt Dtype) FreeTypeVar() hm.TypeVarSet                    { return nil }
func (dt Dtype) Normalize(k, v hm.TypeVarSet) (hm.Type, error) { return dt, nil }
func (dt Dtype) Types() hm.Types                               { return nil }
func (dt Dtype) Format(s fmt.State, c rune)                    { fmt.Fprintf(s, "%s", dt.Name()) }
func (dt Dtype) Eq(other hm.Type) bool                         { return other == dt }

var numpyDtypes map[Dtype]string
var reverseNumpyDtypes map[string]Dtype

func init() {
	numpyDtypes = map[Dtype]string{
		Bool:       "b1",
		Int:        fmt.Sprintf("i%d", Int.Size()),
		Int8:       "i1",
		Int16:      "i2",
		Int32:      "i4",
		Int64:      "i8",
		Uint:       fmt.Sprintf("u%d", Uint.Size()),
		Uint8:      "u1",
		Uint16:     "u2",
		Uint32:     "u4",
		Uint64:     "u8",
		Float32:    "f4",
		Float64:    "f8",
		Complex64:  "c8",
		Complex128: "c16",
	}

	reverseNumpyDtypes = map[string]Dtype{
		"b1":  Bool,
		"i1":  Int8,
		"i2":  Int16,
		"i4":  Int32,
		"i8":  Int64,
		"u1":  Uint8,
		"u2":  Uint16,
		"u4":  Uint32,
		"u8":  Uint64,
		"f4":  Float32,
		"f8":  Float64,
		"c8":  Complex64,
		"c16": Complex128,
	}
}

// NumpyDtype returns the Numpy's Dtype equivalent. This is predominantly used in converting a Tensor to a Numpy ndarray,
// however, not all Dtypes are supported
func (dt Dtype) numpyDtype() (string, error) {
	retVal, ok := numpyDtypes[dt]
	if !ok {
		return "v", errors.Errorf("Unsupported Dtype conversion to Numpy Dtype: %v", dt)
	}
	return retVal, nil
}

func fromNumpyDtype(t string) (Dtype, error) {
	retVal, ok := reverseNumpyDtypes[t]
	if !ok {
		return Dtype{}, errors.Errorf("Unsupported Dtype conversion from %q to Dtype", t)
	}
	if t == "i4" && Int.Size() == 4 {
		return Int, nil
	}
	if t == "i8" && Int.Size() == 8 {
		return Int, nil
	}
	if t == "u4" && Uint.Size() == 4 {
		return Uint, nil
	}
	if t == "u8" && Uint.Size() == 8 {
		return Uint, nil
	}
	return retVal, nil
}

type typeclass struct {
	name string
	set  []Dtype
}

var parameterizedKinds = [...]reflect.Kind{
	reflect.Array,
	reflect.Chan,
	reflect.Func,
	reflect.Interface,
	reflect.Map,
	reflect.Ptr,
	reflect.Slice,
	reflect.Struct,
}

func isParameterizedKind(k reflect.Kind) bool {
	for _, v := range parameterizedKinds {
		if v == k {
			return true
		}
	}
	return false
}

// oh how nice it'd be if I could make them immutable
var (
	Bool       = Dtype{reflect.TypeOf(true)}
	Int        = Dtype{reflect.TypeOf(int(1))}
	Int8       = Dtype{reflect.TypeOf(int8(1))}
	Int16      = Dtype{reflect.TypeOf(int16(1))}
	Int32      = Dtype{reflect.TypeOf(int32(1))}
	Int64      = Dtype{reflect.TypeOf(int64(1))}
	Uint       = Dtype{reflect.TypeOf(uint(1))}
	Uint8      = Dtype{reflect.TypeOf(uint8(1))}
	Uint16     = Dtype{reflect.TypeOf(uint16(1))}
	Uint32     = Dtype{reflect.TypeOf(uint32(1))}
	Uint64     = Dtype{reflect.TypeOf(uint64(1))}
	Float32    = Dtype{reflect.TypeOf(float32(1))}
	Float64    = Dtype{reflect.TypeOf(float64(1))}
	Complex64  = Dtype{reflect.TypeOf(complex64(1))}
	Complex128 = Dtype{reflect.TypeOf(complex128(1))}
	String     = Dtype{reflect.TypeOf("")}

	// aliases
	Byte = Uint8

	// extras
	Uintptr       = Dtype{reflect.TypeOf(uintptr(0))}
	UnsafePointer = Dtype{reflect.TypeOf(unsafe.Pointer(&Uintptr))}
)

// allTypes for indexing
var allTypes = &typeclass{
	name: "τ",
	set: []Dtype{
		Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String, Uintptr, UnsafePointer,
	},
}

// specialized types indicate that there are specialized code generated for these types
var specializedTypes = &typeclass{
	name: "Specialized",
	set: []Dtype{
		Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String,
	},
}

var addableTypes = &typeclass{
	name: "Addable",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String,
	},
}

var numberTypes = &typeclass{
	name: "Number",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Complex64, Complex128,
	},
}

var ordTypes = &typeclass{
	name: "Ord",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, String,
	},
}

var eqTypes = &typeclass{
	name: "Eq",
	set: []Dtype{
		Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String, Uintptr, UnsafePointer,
	},
}

var unsignedTypes = &typeclass{
	name: "Unsigned",
	set:  []Dtype{Uint, Uint8, Uint16, Uint32, Uint64},
}

var signedTypes = &typeclass{
	name: "Signed",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Float32, Float64, Complex64, Complex128,
	},
}

// this typeclass is ever only used by Sub tests
var signedNonComplexTypes = &typeclass{
	name: "Signed NonComplex",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Float32, Float64,
	},
}

var floatTypes = &typeclass{
	name: "Float",
	set: []Dtype{
		Float32, Float64,
	},
}

var complexTypes = &typeclass{
	name: "Complex Numbers",
	set:  []Dtype{Complex64, Complex128},
}

var floatcmplxTypes = &typeclass{
	name: "Real",
	set: []Dtype{
		Float32, Float64, Complex64, Complex128,
	},
}

var nonComplexNumberTypes = &typeclass{
	name: "Non complex numbers",
	set: []Dtype{
		Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64,
	},
}

// this typeclass is ever only used by Pow tests
var generatableTypes = &typeclass{
	name: "Generatable types",
	set: []Dtype{
		Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Float32, Float64, String,
	},
}

func isFloat(dt Dtype) bool {
	return dt == Float64 || dt == Float32
}

func typeclassCheck(a Dtype, tc *typeclass) error {
	if tc == nil {
		return nil
	}
	for _, s := range tc.set {
		if s == a {
			return nil
		}
	}
	return errors.Errorf("Type %v is not a member of %v", a, tc.name)
}

// RegisterNumber is a function required to register a new numerical Dtype.
// This package provides the following Dtype:
//		Int
//		Int8
//		Int16
//		Int32
//		Int64
//		Uint
//		Uint8
//		Uint16
//		Uint32
//		Uint64
//		Float32
//		Float64
//		Complex64
//		Complex128
//
// If a Dtype that is registered already exists on the list, it will not be added to the list.
func RegisterNumber(a Dtype) {
	for _, dt := range numberTypes.set {
		if dt == a {
			return
		}
	}
	numberTypes.set = append(numberTypes.set, a)
	RegisterEq(a)
}

func RegisterFloat(a Dtype) {
	for _, dt := range floatTypes.set {
		if dt == a {
			return
		}
	}
	floatTypes.set = append(floatTypes.set, a)
	RegisterNumber(a)
	RegisterOrd(a)
}

// RegisterOrd registers a dtype as a type that can be typed
func RegisterOrd(a Dtype) {
	for _, dt := range ordTypes.set {
		if dt == a {
			return
		}
	}
	ordTypes.set = append(ordTypes.set, a)
	RegisterEq(a)
}

// RegisterEq registers a dtype as a type that can be compared for equality
func RegisterEq(a Dtype) {
	for _, dt := range eqTypes.set {
		if dt == a {
			return
		}
	}
	eqTypes.set = append(eqTypes.set, a)
	Register(a)
}

// Register registers a new Dtype
func Register(a Dtype) {
	for _, dt := range allTypes.set {
		if a == dt {
			return
		}
	}
	allTypes.set = append(allTypes.set, a)
}

func dtypeID(a Dtype) int {
	for i, v := range allTypes.set {
		if a == v {
			return i
		}
	}
	return -1
}

// NormOrder represents the order of the norm. Ideally, we'd only represent norms with a uint/byte.
// But there are norm types that are outside numerical types, such as nuclear norm and fobenius norm.
// So it is internally represented by a float. If Go could use NaN and Inf as consts, it would have been best,
// Instead, we use constructors. Both Nuclear and Frobenius norm types are represented as NaNs
//
// The using of NaN and Inf as "special" Norm types lead to the need for IsInf() and IsFrobenius() and IsNuclear() method
type NormOrder float64

func Norm(ord int) NormOrder   { return NormOrder(float64(ord)) }
func InfNorm() NormOrder       { return NormOrder(math.Inf(1)) }
func NegInfNorm() NormOrder    { return NormOrder(math.Inf(-1)) }
func UnorderedNorm() NormOrder { return NormOrder(math.Float64frombits(0x7ff8000000000001)) }
func FrobeniusNorm() NormOrder { return NormOrder(math.Float64frombits(0x7ff8000000000002)) }
func NuclearNorm() NormOrder   { return NormOrder(math.Float64frombits(0x7ff8000000000003)) }

// Valid() is a helper method that deterines if the norm order is valid. A valid norm order is
// one where the fraction component is 0
func (n NormOrder) Valid() bool {
	switch {
	case math.IsNaN(float64(n)):
		nb := math.Float64bits(float64(n))
		if math.Float64bits(float64(UnorderedNorm())) == nb || math.Float64bits(float64(FrobeniusNorm())) == nb || math.Float64bits(float64(NuclearNorm())) == nb {
			return true
		}
	case math.IsInf(float64(n), 0):
		return true
	default:
		if _, frac := math.Modf(float64(n)); frac == 0.0 {
			return true
		}
	}
	return false
}

// IsUnordered returns true if the NormOrder is not an ordered norm
func (n NormOrder) IsUnordered() bool {
	return math.Float64bits(float64(n)) == math.Float64bits(float64(UnorderedNorm()))
}

// IsFrobenius returns true if the NormOrder is a Frobenius norm
func (n NormOrder) IsFrobenius() bool {
	return math.Float64bits(float64(n)) == math.Float64bits(float64(FrobeniusNorm()))
}

// IsNuclear returns true if the NormOrder is a nuclear norm
func (n NormOrder) IsNuclear() bool {
	return math.Float64bits(float64(n)) == math.Float64bits(float64(NuclearNorm()))
}

func (n NormOrder) IsInf(sign int) bool {
	return math.IsInf(float64(n), sign)
}

func (n NormOrder) String() string {
	switch {
	case n.IsUnordered():
		return "Unordered"
	case n.IsFrobenius():
		return "Frobenius"
	case n.IsNuclear():
		return "Nuclear"
	case n.IsInf(1):
		return "+Inf"
	case n.IsInf(-1):
		return "-Inf"
	default:
		return fmt.Sprintf("Norm %v", float64(n))
	}
	panic("unreachable")
}

// FuncOpt are optionals for calling Tensor function.
type FuncOpt func(*OpOpt)

// WithIncr passes in a Tensor to be incremented.
func WithIncr(incr Tensor) FuncOpt {
	f := func(opt *OpOpt) {
		opt.incr = incr
	}
	return f
}

// WithReuse passes in a Tensor to be reused.
func WithReuse(reuse Tensor) FuncOpt {
	f := func(opt *OpOpt) {
		opt.reuse = reuse
	}
	return f
}

// UseSafe ensures that the operation is a safe operation (copies data, does not clobber). This is the default option for most methods and functions
func UseSafe() FuncOpt {
	f := func(opt *OpOpt) {
		opt.unsafe = false
	}
	return f
}

// UseUnsafe ensures that the operation is an unsafe operation - data will be clobbered, and operations performed inplace
func UseUnsafe() FuncOpt {
	f := func(opt *OpOpt) {
		opt.unsafe = true
	}
	return f
}

// AsSameType makes sure that the return Tensor is the same type as input Tensors.
func AsSameType() FuncOpt {
	f := func(opt *OpOpt) {
		opt.same = true
	}
	return f
}

// As makes sure that the the return Tensor is of the type specified. Currently only works for FromMat64
func As(t Dtype) FuncOpt {
	f := func(opt *OpOpt) {
		opt.t = t
	}
	return f
}
