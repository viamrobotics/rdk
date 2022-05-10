package protoutils

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

type mapTest struct {
	TestName string
	Data     interface{}
	Expected map[string]interface{}
}

var (
	simpleMap    = map[string]bool{"exists": true}
	sliceMap     = map[string][]string{"foo": {"bar"}}
	nestedMap    = map[string]map[string]string{"foo": {"bar": "bar2"}}
	pointerMap   = map[string]interface{}{"foo": &simpleStruct}
	structMap    = map[string]SimpleStruct{"foo": simpleStruct}
	structMapMap = map[string]MapStruct{"foo": mapStruct}
	mapTests     = []mapTest{
		{"simple map", simpleMap, map[string]interface{}{"exists": true}},
		{"slice map", sliceMap, map[string]interface{}{"foo": []interface{}{"bar"}}},
		{"pointer map", pointerMap, map[string]interface{}{"foo": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3}}},
		{"nested map", nestedMap, map[string]interface{}{"foo": map[string]interface{}{"bar": "bar2"}}},
		{"struct map", structMap, map[string]interface{}{"foo": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3}}},
		{"struct map map", structMapMap, map[string]interface{}{"foo": map[string]interface{}{"status": map[string]interface{}{"foo": "bar"}}}},
	}
)

type structTest struct {
	TestName string
	Data     interface{}
	Expected map[string]interface{}
	Return   interface{}
}

var (
	simpleStruct       = SimpleStruct{X: 1.1, Y: 2.2, Z: 3.3}
	typedStringStruct  = TypedStringStruct{TypedString: TypedString("hello")}
	sliceStruct        = SliceStruct{Degrees: []float64{1.1, 2.2, 3.3}}
	mapStruct          = MapStruct{Status: map[string]string{"foo": "bar"}}
	pointerStruct      = PointerStruct{&simpleStruct}
	emptyPointerStruct = EmptyPointerStruct{EmptyStruct: nil}
	nestedMapStruct    = NestedMapStruct{Status: map[string]SimpleStruct{"foo": simpleStruct}}
	nestedStruct       = NestedStruct{SimpleStruct: simpleStruct, SliceStruct: sliceStruct}
	noTagStruct        = NoTagsStruct{SimpleStruct: simpleStruct, SliceStruct: sliceStruct}
	embeddedStruct     = EmbeddedStruct{simpleStruct, sliceStruct}

	singleByteStruct = SingleUintStruct{UintValue: uint16(1)}

	nilPointerResembleVal = EmptyPointerStruct{EmptyStruct: &EmptyStruct{}}

	structTests = []structTest{
		{"simple struct", simpleStruct, map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3}, SimpleStruct{}},
		{"typed string struct", typedStringStruct, map[string]interface{}{"typed_string": "hello"}, TypedStringStruct{}},
		{"omit struct", OmitStruct{}, map[string]interface{}{"x": 0.0}, OmitStruct{}},
		{"ignore struct", IgnoreStruct{X: 1}, map[string]interface{}{}, IgnoreStruct{X: 1}},
		{"slice struct", sliceStruct, map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}, SliceStruct{}},
		{"map struct", mapStruct, map[string]interface{}{"status": map[string]interface{}{"foo": "bar"}}, MapStruct{}},
		{
			"pointer struct",
			pointerStruct,
			map[string]interface{}{"simple_struct": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3}},
			PointerStruct{},
		},
		{
			"nested map struct",
			nestedMapStruct,
			map[string]interface{}{"status": map[string]interface{}{"foo": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3}}},
			NestedMapStruct{},
		},
		{
			"nested struct",
			nestedStruct,
			map[string]interface{}{
				"slice_struct":  map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
				"simple_struct": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3},
			},
			NestedStruct{},
		},
		{
			"nested struct with no tags",
			noTagStruct,
			map[string]interface{}{
				"slice_struct": map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
				"SimpleStruct": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3},
			},
			NoTagsStruct{},
		},
		{
			"embedded struct",
			embeddedStruct,
			map[string]interface{}{
				"SliceStruct":  map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
				"SimpleStruct": map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3},
			},
			EmbeddedStruct{},
		}, {
			"nil pointer struct",
			emptyPointerStruct,
			map[string]interface{}{"empty_struct": map[string]interface{}{}},
			EmptyPointerStruct{},
		}, {
			"struct with uint",
			singleByteStruct,
			map[string]interface{}{"UintValue": uint(1)},
			SingleUintStruct{},
		},
	}

	uintStructTest = structTest{
		"struct with nil value",
		singleByteStruct,
		map[string]interface{}{"UintValue": uint(1)},
		SingleUintStruct{},
	}
)

func TestInterfaceToMap(t *testing.T) {
	t.Run("not a map or struct", func(t *testing.T) {
		_, err := InterfaceToMap("1")
		test.That(t, err, test.ShouldBeError, errors.New("data of type string and kind string not a struct or a map-like object"))

		_, err = InterfaceToMap([]string{"1"})
		test.That(t, err, test.ShouldBeError, errors.New("data of type []string and kind slice not a struct or a map-like object"))
	})

	for _, tc := range mapTests {
		map1, err := InterfaceToMap(tc.Data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, map1, test.ShouldResemble, tc.Expected)

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap(), test.ShouldResemble, tc.Expected)
	}

	for _, tc := range structTests {
		map1, err := InterfaceToMap(tc.Data)
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "struct with uint":
			test.That(t, map1["UintValue"], test.ShouldEqual, 1)
		default:
			test.That(t, map1, test.ShouldResemble, tc.Expected)
		}

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "struct with uint":
			test.That(t, newStruct.AsMap()["UintValue"], test.ShouldEqual, 1)
		default:
			test.That(t, newStruct.AsMap(), test.ShouldResemble, tc.Expected)
		}

		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &tc.Return})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "nil pointer struct":
			test.That(t, tc.Return, test.ShouldResemble, nilPointerResembleVal)
		default:
			test.That(t, tc.Return, test.ShouldResemble, tc.Data)
		}

	}

}

func TestMarshalMap(t *testing.T) {
	t.Run("not a valid map", func(t *testing.T) {
		_, err := marshalMap(simpleStruct)
		test.That(t, err, test.ShouldBeError, errors.New("data of type protoutils.SimpleStruct is not a map"))

		_, err = marshalMap("1")
		test.That(t, err, test.ShouldBeError, errors.New("data of type string is not a map"))

		_, err = marshalMap([]string{"1"})
		test.That(t, err, test.ShouldBeError, errors.New("data of type []string is not a map"))

		_, err = marshalMap(map[int]string{1: "1"})
		test.That(t, err, test.ShouldBeError, errors.New("map keys of type int are not strings"))

		_, err = marshalMap(map[interface{}]string{"1": "1"})
		test.That(t, err, test.ShouldBeError, errors.New("map keys of type interface are not strings"))
	})

	for _, tc := range mapTests {
		map1, err := marshalMap(tc.Data)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, map1, test.ShouldResemble, tc.Expected)

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap(), test.ShouldResemble, tc.Expected)
	}
}

func TestStructToMap(t *testing.T) {
	t.Run("not a struct", func(t *testing.T) {
		_, err := structToMap(map[string]interface{}{"exists": true})
		test.That(t, err, test.ShouldBeError, errors.New("data of type map[string]interface {} is not a struct"))

		_, err = structToMap(1)
		test.That(t, err, test.ShouldBeError, errors.New("data of type int is not a struct"))

		_, err = structToMap([]string{"1"})
		test.That(t, err, test.ShouldBeError, errors.New("data of type []string is not a struct"))
	})

	for _, tc := range structTests {
		map1, err := structToMap(tc.Data)
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "struct with uint":
			test.That(t, map1["UintValue"], test.ShouldEqual, 1)
		default:
			test.That(t, map1, test.ShouldResemble, tc.Expected)
		}

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "struct with uint":
			test.That(t, newStruct.AsMap()["UintValue"], test.ShouldEqual, 1)
		default:
			test.That(t, newStruct.AsMap(), test.ShouldResemble, tc.Expected)
		}

		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &tc.Return})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		switch tc.TestName {
		case "nil pointer struct":
			test.That(t, tc.Return, test.ShouldResemble, nilPointerResembleVal)
		default:
			test.That(t, tc.Return, test.ShouldResemble, tc.Data)
		}
	}
}
func TestMarshalSlice(t *testing.T) {
	t.Run("not a list", func(t *testing.T) {
		_, err := marshalSlice(1)
		test.That(t, err, test.ShouldBeError, errors.New("data of type int is not a slice"))
	})

	degs := []float64{1.1, 2.2, 3.3}
	matrix := [][]float64{degs}
	embeddedMatrix := [][][]float64{matrix}
	objects := []SliceStruct{{Degrees: degs}}
	objectList := [][]SliceStruct{objects}
	maps := []map[string]interface{}{{"hello": "world"}, {"foo": 1.1}}
	mapsOfLists := []map[string][]string{{"hello": {"world"}}, {"foo": {"bar"}}}
	mixed := []interface{}{degs, matrix, objects}

	for _, tc := range []struct {
		TestName string
		Data     interface{}
		Length   int
		Expected []interface{}
	}{
		{"simple list", degs, 3, []interface{}{1.1, 2.2, 3.3}},
		{"list of simple lists", matrix, 1, []interface{}{[]interface{}{1.1, 2.2, 3.3}}},
		{"list of list of simple lists", embeddedMatrix, 1, []interface{}{[]interface{}{[]interface{}{1.1, 2.2, 3.3}}}},
		{"list of objects", objects, 1, []interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}}},
		{"list of lists of objects", objectList, 1, []interface{}{[]interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}}}},
		{"list of maps", maps, 2, []interface{}{map[string]interface{}{"hello": "world"}, map[string]interface{}{"foo": 1.1}}},
		{
			"list of maps of lists",
			mapsOfLists,
			2,
			[]interface{}{map[string]interface{}{"hello": []interface{}{"world"}}, map[string]interface{}{"foo": []interface{}{"bar"}}}},
		{
			"list of mixed objects",
			mixed,
			3,
			[]interface{}{
				[]interface{}{1.1, 2.2, 3.3},
				[]interface{}{[]interface{}{1.1, 2.2, 3.3}},
				[]interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}},
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			marshalled, err := marshalSlice(tc.Data)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(marshalled), test.ShouldEqual, tc.Length)
			test.That(t, marshalled, test.ShouldResemble, tc.Expected)
		})
	}
}

type TypedString string

type SimpleStruct struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type EmptyStruct struct{}
type TypedStringStruct struct {
	TypedString TypedString `json:"typed_string"`
}

type OmitStruct struct {
	X float64 `json:"x"`
	Y float64 `json:"y,omitempty"`
}
type IgnoreStruct struct {
	X float64 `json:"-"`
	Y float64 `json:"y,omitempty"`
}
type SliceStruct struct {
	Degrees []float64 `json:"degrees"`
}

type MapStruct struct {
	Status map[string]string `json:"status"`
}

type NestedMapStruct struct {
	Status map[string]SimpleStruct `json:"status"`
}

type PointerStruct struct {
	SimpleStruct *SimpleStruct `json:"simple_struct"`
}

type EmptyPointerStruct struct {
	EmptyStruct *EmptyStruct `json:"empty_struct"`
}

type NestedStruct struct {
	SimpleStruct SimpleStruct `json:"simple_struct"`
	SliceStruct  SliceStruct  `json:"slice_struct"`
}

type NoTagsStruct struct {
	SimpleStruct SimpleStruct
	SliceStruct  SliceStruct `json:"slice_struct"`
}
type EmbeddedStruct struct {
	SimpleStruct
	SliceStruct
}

type SingleUintStruct struct {
	UintValue uint16
}
