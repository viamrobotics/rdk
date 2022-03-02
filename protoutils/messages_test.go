package protoutils_test

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStructToMap(t *testing.T) {
	mockV := mockVector{X: 1.1, Y: 2.2, Z: 3.3}
	t.Run("simple struct", func(t *testing.T) {
		map1, err := protoutils.StructToMap(mockV)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, map1, test.ShouldResemble, map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3})

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap()["x"], test.ShouldResemble, 1.1)

		convMap := mockVector{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, convMap, test.ShouldResemble, mockV)
	})

	mockD := mockDegrees{Degrees: []float64{1.1, 2.2, 3.3}}
	t.Run("simple struct with slice", func(t *testing.T) {
		map1, err := protoutils.StructToMap(mockD)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, map1, test.ShouldResemble, map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}})

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap()["degrees"], test.ShouldResemble, []interface{}{1.1, 2.2, 3.3})

		convMap := mockDegrees{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, convMap, test.ShouldResemble, mockD)
	})

	mockS := mockStruct{MockVector: mockV, MockDegrees: mockD}
	t.Run("nested struct", func(t *testing.T) {
		map1, err := protoutils.StructToMap(mockS)
		test.That(t, err, test.ShouldBeNil)
		test.That(
			t,
			map1,
			test.ShouldResemble,
			map[string]interface{}{
				"mock_degrees": map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
				"mock_vector":  map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3},
			},
		)

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap()["mock_degrees"], test.ShouldResemble, map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}})

		convMap := mockStruct{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, convMap, test.ShouldResemble, mockS)
	})

	noT := noTags{MockVector: mockV, MockDegrees: mockD}
	t.Run("nested struct with no tags", func(t *testing.T) {
		map1, err := protoutils.StructToMap(noT)
		test.That(t, err, test.ShouldBeNil)
		test.That(
			t,
			map1,
			test.ShouldResemble,
			map[string]interface{}{
				"mock_degrees": map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
				"MockVector":   map[string]interface{}{"x": 1.1, "y": 2.2, "z": 3.3},
			},
		)

		newStruct, err := structpb.NewStruct(map1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newStruct.AsMap()["mock_degrees"], test.ShouldResemble, map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}})

		convMap := noTags{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(newStruct.AsMap())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, convMap, test.ShouldResemble, noT)
	})
}
func TestMarshalSlice(t *testing.T) {
	t.Run("not a list", func(t *testing.T) {
		_, err := protoutils.MarshalSlice(1)
		test.That(t, err, test.ShouldBeError, errors.New("input is not a slice"))
	})

	degs := []float64{1.1, 2.2, 3.3}
	t.Run("simple list", func(t *testing.T) {
		marshalled, err := protoutils.MarshalSlice(degs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 3)
		test.That(t, marshalled, test.ShouldResemble, []interface{}{1.1, 2.2, 3.3})
	})

	matrix := [][]float64{degs}
	t.Run("list of simple lists", func(t *testing.T) {
		marshalled, err := protoutils.MarshalSlice(matrix)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 1)
		test.That(t, marshalled, test.ShouldResemble, []interface{}{[]interface{}{1.1, 2.2, 3.3}})
	})

	t.Run("list of list of simple lists", func(t *testing.T) {
		embeddedMatrix := [][][]float64{matrix}
		marshalled, err := protoutils.MarshalSlice(embeddedMatrix)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 1)
		test.That(t, marshalled, test.ShouldResemble, []interface{}{[]interface{}{[]interface{}{1.1, 2.2, 3.3}}})
	})

	objects := []mockDegrees{{Degrees: degs}}
	t.Run("list of objects", func(t *testing.T) {
		marshalled, err := protoutils.MarshalSlice(objects)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 1)
		test.That(t, marshalled, test.ShouldResemble, []interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}})
	})

	t.Run("list of lists of objects", func(t *testing.T) {
		objectList := [][]mockDegrees{objects}
		marshalled, err := protoutils.MarshalSlice(objectList)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 1)
		test.That(
			t,
			marshalled,
			test.ShouldResemble,
			[]interface{}{[]interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}}},
		)
	})

	t.Run("list of mixed objects", func(t *testing.T) {
		mixed := []interface{}{degs, matrix, objects}
		marshalled, err := protoutils.MarshalSlice(mixed)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(marshalled), test.ShouldEqual, 3)
		test.That(
			t,
			marshalled,
			test.ShouldResemble,
			[]interface{}{
				[]interface{}{1.1, 2.2, 3.3},
				[]interface{}{[]interface{}{1.1, 2.2, 3.3}},
				[]interface{}{map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}}},
			},
		)
	})
}

type mockVector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type mockDegrees struct {
	Degrees []float64 `json:"degrees"`
}

type mockStruct struct {
	MockVector  mockVector  `json:"mock_vector"`
	MockDegrees mockDegrees `json:"mock_degrees"`
}

type noTags struct {
	MockVector  mockVector
	MockDegrees mockDegrees `json:"mock_degrees"`
}
