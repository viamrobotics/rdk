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

	parsed, err := parse(serializedData)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("Parsed data:", parsed)

	// There are four datapoints in total.
	test.That(t, len(parsed), test.ShouldEqual, 4)

	// The first two datapoints use "schema 1", the `s1` name.
	for idx, datum := range parsed[:2] {
		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s1"].Foo also == idx.
		test.That(t, datum.Data["s1"].(map[string]float32)["Foo"], test.ShouldEqual, idx)
	}

	for idx := 2; idx < len(parsed); idx++ {
		datum := parsed[idx]

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

	datums := 10
	for idx := 0; idx < datums; idx++ {
		datumV1 := datum{
			Time: int64(idx),
			Data: map[string]any{
				"s1": Statser1{0, idx, 1.0},
			},
			generationID: 1,
		}

		ftdc.writeDatum(datumV1)
	}

	for idx := datums; idx < 2*datums; idx++ {
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

	parsed, err := parse(serializedData)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("Parsed data:", parsed)

	// There are twenty datapoints in total.
	test.That(t, len(parsed), test.ShouldEqual, 2*datums)

	// The first two datapoints use "schema 1", the `s1` name.
	for idx, datum := range parsed[:datums] {
		// Time == idx is a property of the constructed input.
		test.That(t, datum.Time, test.ShouldEqual, idx)
		// Similarly, Data["s1"].Foo also == idx.
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric1"], test.ShouldEqual, 0)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric2"], test.ShouldEqual, idx)
		test.That(t, datum.Data["s1"].(map[string]float32)["Metric3"], test.ShouldEqual, 1)
	}

	for idx := datums; idx < len(parsed); idx++ {
		datum := parsed[idx]

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
	fields, err := getFieldsForStruct(reflect.TypeOf(&Basic{100}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields, test.ShouldResemble,
		[]string{"Foo"})

	fields, err = getFieldsForStruct(reflect.TypeOf(Statser1{100, 0, 44.4}))
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
	fields, err := getFieldsForStruct(reflect.TypeOf(&TopLevel{100, Nested{200, struct{ Z uint8 }{255}}}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields, test.ShouldResemble,
		[]string{"X", "Nested.Y", "Nested.Deeper.Z"})
}
