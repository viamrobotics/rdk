package ftdc

import (
	"bytes"
	"reflect"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

type Statser1 struct {
	Metric1 int
	Metric2 int
	Metric3 float32
}

type Statser2 struct {
	Metric1 int
	Metric2 int
	Metric3 float32
}

type Basic struct {
	Foo int
}

func TestCustomFormatRoundtripBasic(t *testing.T) {
	// This FTDC test will write to this `serializedData`.
	serializedData := bytes.NewBuffer(nil)

	logger := logging.NewTestLogger(t)
	ftdc := NewWithWriter(serializedData, logger.Sublogger("ftdc"))

	// Write two datapoints with "schema 1".
	datumV1 := datum{
		Time: 0,
		Data: map[string]any{
			"s1": &Basic{0},
		},
		generationID: 1,
	}

	ftdc.writeDatum(datumV1)
	datumV1.Time = 1
	datumV1.Data["s1"].(*Basic).Foo = 1
	ftdc.writeDatum(datumV1)

	// Write two more datapoints with "schema 2".
	datumV2 := datum{
		Time: 2,
		Data: map[string]any{
			"s2": &Basic{2},
		},
		generationID: 2,
	}
	ftdc.writeDatum(datumV2)
	datumV2.Time = 3
	datumV2.Data["s2"].(*Basic).Foo = 3
	ftdc.writeDatum(datumV2)

	parsed, err := Parse(serializedData)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("Parsed data:", parsed)

	// There are four datapoints in total.
	test.That(t, len(parsed), test.ShouldEqual, 4)

	// The first two datapoints use "schema 1", the `s1` name.
	for idx, datum := range flatDatumsToDatums(parsed[:2]) {
		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s1"].Foo also == idx.
		test.That(t, datum.Data["s1"].(map[string]float32)["Foo"], test.ShouldEqual, idx)
	}

	for idx := 2; idx < len(parsed); idx++ {
		datum := parsed[idx].asDatum()

		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s2"].Foo also == idx.
		test.That(t, datum.Data["s2"].(map[string]float32)["Foo"], test.ShouldEqual, idx)
	}
}

func TestCustomFormatRoundtripRich(t *testing.T) {
	// This FTDC test will write to this `serializedData`.
	serializedData := bytes.NewBuffer(nil)

	logger := logging.NewTestLogger(t)
	ftdc := NewWithWriter(serializedData, logger.Sublogger("ftdc"))

	numDatumsPerSchema := 10
	for idx := 0; idx < numDatumsPerSchema; idx++ {
		datumV1 := datum{
			Time: int64(idx),
			Data: map[string]any{
				"s1": Statser1{0, idx, 1.0},
			},
			generationID: 1,
		}

		ftdc.writeDatum(datumV1)
	}

	for idx := numDatumsPerSchema; idx < 2*numDatumsPerSchema; idx++ {
		datumV2 := datum{
			Time: int64(idx),
			Data: map[string]any{
				"s1": Statser1{idx, idx, 1.0},
				// The second metric here is to test a value that flips between a diff and no diff.
				"s2": Statser2{0, 1 + (idx / 3), 100.0},
			},
			generationID: 2,
		}

		ftdc.writeDatum(datumV2)
	}

	flatDatums, err := Parse(serializedData)
	test.That(t, err, test.ShouldBeNil)
	datums := flatDatumsToDatums(flatDatums)
	logger.Info("Parsed data:", datums)

	// There are twenty datapoints in total.
	test.That(t, len(datums), test.ShouldEqual, 2*numDatumsPerSchema)

	// The first two datapoints use "schema 1", the `s1` name.
	for idx, datum := range datums[:numDatumsPerSchema] {
		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s1"].Foo also == idx.
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric1"], test.ShouldEqual, 0)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric2"], test.ShouldEqual, idx)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric3"], test.ShouldEqual, 1)
	}

	for idx := numDatumsPerSchema; idx < len(datums); idx++ {
		datum := datums[idx]

		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s2"].Foo also == idx.
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric1"], test.ShouldEqual, idx)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric2"], test.ShouldEqual, idx)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric3"], test.ShouldEqual, 1)

		test.That(t, datum.Data["s2"].(map[string]float32)["Metric1"], test.ShouldEqual, 0)
		test.That(t, datum.Data["s2"].(map[string]float32)["Metric2"], test.ShouldEqual, 1+(idx/3))
		test.That(t, datum.Data["s2"].(map[string]float32)["Metric3"], test.ShouldEqual, 100)
	}
}

func TestReflection(t *testing.T) {
	fields, err := getFieldsForStruct(reflect.ValueOf(&Basic{100}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields, test.ShouldResemble,
		[]string{"Foo"})

	fields, err = getFieldsForStruct(reflect.ValueOf(Statser1{100, 0, 44.4}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields, test.ShouldResemble,
		[]string{"Metric1", "Metric2", "Metric3"})
}

type TopLevel struct {
	X      int
	Nested Nested
}

type Nested struct {
	Y      int
	Deeper struct {
		Z uint8
	}
}

func TestNestedReflection(t *testing.T) {
	val := &TopLevel{100, Nested{200, struct{ Z uint8 }{255}}}
	fields, err := getFieldsForStruct(reflect.ValueOf(val))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields, test.ShouldResemble,
		[]string{"X", "Nested.Y", "Nested.Deeper.Z"})
}

type Complex struct {
	F1 float32
	F2 struct {
		F3 float32
		F4 float32
	}
	F5 struct {
		F6 float32
	}
	F7  float32
	F8  struct{}
	F9  float32
	F10 struct {
		F11 float32
		F12 float32
		F13 float32
	}
	F14 struct {
		F15 struct {
			F16 struct {
				F17 float32
			}
		}
	}
}

func TestNestedReflectionParity(t *testing.T) {
	// We use reflection in two paths:
	// - Walking a "stats" object to create a schema.
	// - Walking a "stats" object to get/flatten all of the values.
	//
	// It's critical that these two walks happen in the same order. Such that keys from the schema
	// walk match their corresponding values.
	complexObj := Complex{
		F1: 1,
		F2: struct {
			F3 float32
			F4 float32
		}{3, 4},
		F5: struct {
			F6 float32
		}{6},
		F7: 7,
		// F8 is tricky -- there are no leaf nodes. Therefore it must not be part of neither the
		// schema nor value output.
		F8: struct{}{},
		F9: 9,
		F10: struct {
			F11 float32
			F12 float32
			F13 float32
		}{11, 12, 13},
		F14: struct {
			F15 struct {
				F16 struct {
					F17 float32
				}
			}
		}{
			F15: struct {
				F16 struct {
					F17 float32
				}
			}{
				F16: struct {
					F17 float32
				}{17},
			},
		},
	}

	fields, err := getFieldsForStruct(reflect.ValueOf(complexObj))
	test.That(t, err, test.ShouldBeNil)
	// There will be one "field" for each number in the above `Complex` structure.
	test.That(t, fields, test.ShouldResemble,
		[]string{"F1", "F2.F3", "F2.F4", "F5.F6", "F7", "F9", "F10.F11", "F10.F12", "F10.F13", "F14.F15.F16.F17"})
	values, err := flattenStruct(reflect.ValueOf(complexObj))
	test.That(t, err, test.ShouldBeNil)
	// For convenience, the number values match the field name.
	test.That(t, values, test.ShouldResemble,
		[]float32{1, 3, 4, 6, 7, 9, 11, 12, 13, 17})
}

type nestsAny struct {
	Number float32
	Struct any
}

func TestNestedAny(t *testing.T) {
	logger := logging.NewTestLogger(t)

	stat := nestsAny{10, struct{ X int }{5}}
	fields, err := getFieldsForStruct(reflect.ValueOf(stat))
	logger.Info("Fields:", fields, "Err:", err)
	test.That(t, fields, test.ShouldResemble, []string{"Number", "Struct.X"})

	values, err := flattenStruct(reflect.ValueOf(stat))
	logger.Info("Values:", values, "Err:", err)
	test.That(t, values, test.ShouldResemble, []float32{10, 5})

	stat = nestsAny{10, nil}
	fields, err = getFieldsForStruct(reflect.ValueOf(stat))
	logger.Info("Fields:", fields, "Err:", err)
	test.That(t, fields, test.ShouldResemble, []string{"Number"})

	values, err = flattenStruct(reflect.ValueOf(stat))
	logger.Info("Values:", values, "Err:", err)
	test.That(t, values, test.ShouldResemble, []float32{10})
}

func TestWeirdStats(t *testing.T) {
	logger := logging.NewTestLogger(t)

	aChannel := make(chan struct{})
	stat := nestsAny{10, struct {
		aChannel      *chan struct{}
		aString       string
		hiddenNumeric bool
		anArray       [5]int
	}{
		aChannel:      &aChannel,
		aString:       "definitely a string and not a numeric",
		hiddenNumeric: true,
		anArray:       [5]int{5, 4, 3, 2, 1},
	}}

	fields, err := getFieldsForStruct(reflect.ValueOf(stat))
	logger.Info("Fields:", fields, " Err:", err)
	test.That(t, fields, test.ShouldResemble, []string{"Number", "Struct.hiddenNumeric"})

	values, err := flattenStruct(reflect.ValueOf(stat))
	logger.Info("Values:", values, " Err:", err)
	test.That(t, values, test.ShouldResemble, []float32{10, 1})
}

func TestNilNestedStats(t *testing.T) {
	logger := logging.NewTestLogger(t)

	stat := nestsAny{10, nil}

	fields, err := getFieldsForStruct(reflect.ValueOf(stat))
	logger.Info("Fields:", fields, " Err:", err)
	test.That(t, fields, test.ShouldResemble, []string{"Number"})

	values, err := flattenStruct(reflect.ValueOf(stat))
	logger.Info("Values:", values, " Err:", err)
	test.That(t, values, test.ShouldResemble, []float32{10})
}
