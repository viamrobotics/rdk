package ftdc

import (
	"bytes"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

type fooStats struct {
	X int
	Y int
}

type foo struct {
	x int
	y int
}

func (foo *foo) Stats() any {
	return fooStats{X: foo.x, Y: foo.y}
}

func TestFTDCSchemaGenerations(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `foo` implements `Statser`.
	foo1 := &foo{}

	// Generations are a way of keeping track of when schema's change.
	preAddGenerationID := ftdc.inputGenerationID
	// In the initial and steady states, the input and output generations are equal.
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, preAddGenerationID)

	// Calling `Add` changes the schema. The `inputGenerationID` is incremented (to "1") to denote
	// this.
	ftdc.Add("foo1", foo1)
	test.That(t, ftdc.inputGenerationID, test.ShouldEqual, preAddGenerationID+1)
	// The `outputGenerationID` is still at "0".
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, preAddGenerationID)

	// Constructing a datum will:
	// - Have data for the `foo1` Statser.
	// - Be stamped with the `inputGenerationID` of 1.
	datum := ftdc.constructDatum()
	test.That(t, datum.generationID, test.ShouldEqual, ftdc.inputGenerationID)
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, preAddGenerationID)

	// writeDatum will serialize the datum in the "custom format". Part of the bytes written will be
	// the new schema at generation "1". The `outputGenerationID` will be updated to reflect that
	// "1" is the "current schema".
	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, ftdc.inputGenerationID)

	// We are going to add another `Statser`. Assert that the `inputGenerationID` gets incremented after calling `Add`.
	foo2 := &foo{}
	preAddGenerationID = ftdc.inputGenerationID
	ftdc.Add("foo2", foo2)
	test.That(t, ftdc.inputGenerationID, test.ShouldEqual, preAddGenerationID+1)

	// Updating the values on the `foo` objects changes the output of the "stats" object they return
	// as part of `constructDatum`.
	foo1.x = 1
	foo2.x = 2
	// Constructing a datum will (again) copy the `inputGenerationID` as its own `generationID`. The
	// `outputGenerationID` has not been changed yet and still matches the value prior* to adding
	// `foo2`.
	datum = ftdc.constructDatum()
	test.That(t, datum.generationID, test.ShouldEqual, ftdc.inputGenerationID)
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, preAddGenerationID)

	// Writing the second datum updates the `outputGenerationID` and we are again in the steady
	// state.
	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ftdc.outputGenerationID, test.ShouldEqual, ftdc.inputGenerationID)

	// Go back and parse the written data. There two be two datum objects due to two calls to
	// `writeDatum`.
	datums, err := parse(ftdcData)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(datums), test.ShouldEqual, 2)

	// Assert the datum1.Time <= datum2.Time. Don't trust consecutive clock reads to get an updated
	// value.
	datum1, datum2 := datums[0], datums[1]
	test.That(t, datum1.Time, test.ShouldBeLessThanOrEqualTo, datum2.Time)

	// Assert the contents of the datum. While we seemingly only assert on the "values" of X and Y
	// for `foo1` and `foo2`, we're indirectly depending on the schema information being
	// persisted/parsed correctly.
	test.That(t, len(datum1.Data), test.ShouldEqual, 1)
	// When we support nesting of structures, the `float32` type on these map assertions will be
	// wrong. It will become `any`s.
	test.That(t, datum1.Data["foo1"], test.ShouldResemble, map[string]float32{"X": 0, "Y": 0})

	// Before writing the second datum, we changed the values of `X` on `foo1` and `foo2`.
	test.That(t, len(datum2.Data), test.ShouldEqual, 2)
	test.That(t, datum2.Data["foo1"], test.ShouldResemble, map[string]float32{"X": 1, "Y": 0})
	test.That(t, datum2.Data["foo2"], test.ShouldResemble, map[string]float32{"X": 2, "Y": 0})
}
