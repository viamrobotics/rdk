package ftdc

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

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
	datums, err := Parse(ftdcData)
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

type badStatser struct{}

func (badStatser badStatser) Stats() any {
	// Returning maps are disallowed.
	return map[string]float32{"X": 42}
}

func TestRemoveBadStatser(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `foo` implements `Statser`.
	foo1 := &foo{x: 1, y: 2}
	ftdc.Add("foo1", foo1)

	// `badStatser` implements `Statser`, but returns a map instead of a struct. This will fail at
	// `writeDatum`.
	ftdc.Add("badStatser", badStatser{})

	// constructDatum should succeed as it does not perform validation.
	datum := ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 2)
	test.That(t, datum.Data["foo1"], test.ShouldNotBeNil)
	test.That(t, datum.Data["badStatser"], test.ShouldNotBeNil)

	// writeDatum will discover the map and error out.
	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldNotBeNil)

	// We can additionally verify the error is a schemaError that identifies the statser that
	// misbehaved.
	var schemaError *schemaError
	test.That(t, errors.As(err, &schemaError), test.ShouldBeTrue)
	test.That(t, schemaError.statserName, test.ShouldEqual, "badStatser")

	// The `writeDatum` error should auto-remove `badStatser`. Verify only `foo1` is returned on a
	// following call to `constructDatum`.
	datum = ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 1)
	test.That(t, datum.Data["foo1"], test.ShouldNotBeNil)
	test.That(t, datum.Data["badStatser"], test.ShouldBeNil)

	// This time writing the datum works.
	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	// Verify the contents of the ftdc data.
	datums, err := ParseWithLogger(ftdcData, logger)
	test.That(t, err, test.ShouldBeNil)

	// We called `writeDatum` twice, but only the second succeeded.
	test.That(t, len(datums), test.ShouldEqual, 1)
	test.That(t, datums[0].Data["foo1"], test.ShouldResemble, map[string]float32{"X": 1, "Y": 2})
}

type nestedStatser struct {
	x int
	z int
}

type nestedStatserTopLevel struct {
	X int
	Y struct {
		Z int
	}
}

func (statser *nestedStatser) Stats() any {
	return nestedStatserTopLevel{
		X: statser.x,
		Y: struct {
			Z int
		}{statser.z},
	}
}

func TestNestedStructs(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	statser := &nestedStatser{}
	ftdc.Add("nested", statser)

	datum := ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 1)
	test.That(t, datum.Data["nested"], test.ShouldNotBeNil)

	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	statser.x = 1
	statser.z = 2

	datum = ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 1)
	test.That(t, datum.Data["nested"], test.ShouldNotBeNil)

	schema, schemaErr := getSchema(datum.Data)
	test.That(t, schemaErr, test.ShouldBeNil)
	flattened, err := flatten(datum, schema)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, flattened, test.ShouldResemble, []float32{1, 2})

	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	datums, err := ParseWithLogger(ftdcData, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(datums), test.ShouldEqual, 2)
	test.That(t, len(datums[0].Data), test.ShouldEqual, 1)
	test.That(t, datums[0].Data["nested"], test.ShouldResemble, map[string]float32{
		"X":   0,
		"Y.Z": 0,
	})
	test.That(t, datums[1].Data["nested"], test.ShouldResemble, map[string]float32{
		"X":   1,
		"Y.Z": 2,
	})
}

// TestStatsWriterContinuesOnSchemaError asserts that "schema errors" are handled by removing the
// violating statser, but otherwise FTDC keeps going.
func TestStatsWriterContinuesOnSchemaError(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `badStatser` implements `Statser` but returns a `map` which is disallowed.
	badStatser := &badStatser{}
	ftdc.Add("badStatser", badStatser)

	// Construct a datum with a `badStatser` reading that contains a map.
	datum := ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 1)
	test.That(t, datum.Data["badStatser"], test.ShouldNotBeNil)

	// Start the `statsWriter` and manually push it the bad `datum`.
	go ftdc.statsWriter()
	ftdc.datumCh <- datum
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		ftdc.mu.Lock()
		defer ftdc.mu.Unlock()

		// Assert that the `statsWriter`, via calls to `writeDatum` identify the bad `map` value and
		// remove the `badStatser`.
		test.That(tb, len(ftdc.statsers), test.ShouldEqual, 0)
	})

	// Assert that `statsWriter` is still operating by waiting for 1 second.
	select {
	case <-ftdc.outputWorkerDone:
		t.Fatalf("A bad statser caused FTDC to abort")
	case <-time.After(time.Second):
		break
	}

	// Closing the `datumCh` will cause the `statsWriter` to exit as it no longer can get input.
	close(ftdc.datumCh)

	// Wait for the `statsWriter` goroutine to exit.
	<-ftdc.outputWorkerDone
}
