package ftdc

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	datum1, datum2 := datums[0].asDatum(), datums[1].asDatum()
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
	test.That(t, datums[0].asDatum().Data["foo1"], test.ShouldResemble, map[string]float32{"X": 1, "Y": 2})
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

	flatDatums, err := ParseWithLogger(ftdcData, logger)
	test.That(t, err, test.ShouldBeNil)
	datums := flatDatumsToDatums(flatDatums)
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

func TestCountingBytes(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// Isolate all of the files we're going to create to a single, fresh directory.
	ftdcFileDir, err := os.MkdirTemp("./", "countingBytesTest")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(ftdcFileDir)

	// We must not use `NewWithWriter`. Forcing a writer for FTDC data is not compatible with FTDC
	// file rotation.
	ftdc := New(ftdcFileDir, logger.Sublogger("ftdc"))
	// Expect a log rotation after 1,000 bytes. For a changing `foo` object, this is ~60 datums.
	ftdc.maxFileSizeBytes = 1000

	timesRolledOver := 0
	foo := &foo{}
	ftdc.Add("foo", foo)
	for cnt := 0; cnt < 1000; cnt++ {
		foo.x = cnt
		foo.y = 2 * cnt

		datum := ftdc.constructDatum()
		datum.Time = int64(cnt)
		err := ftdc.writeDatum(datum)
		test.That(t, err, test.ShouldBeNil)

		// If writing a datum takes the bytes written to larger than configured max file size, an
		// explicit call to `getWriter` should create a new file and reset the count.
		if ftdc.bytesWrittenCounter.count >= ftdc.maxFileSizeBytes {
			// We're about to write a new ftdc file. The ftdc file names are a function of
			// "now". Given the test runs fast, the generated name will collide (names only use
			// seconds resolution).  Make a subdirectory to avoid a naming conflict.
			ftdc.ftdcDir, err = os.MkdirTemp(ftdcFileDir, "subdirectory")
			test.That(t, err, test.ShouldBeNil)

			_, err = ftdc.getWriter()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ftdc.bytesWrittenCounter.count, test.ShouldBeLessThan, 1000)
			timesRolledOver++
		}
	}
	logger.Info("Rolled over:", timesRolledOver)

	// Assert that the test rolled over at least once. Otherwise the test "passed" without
	// exercising the intended code.
	test.That(t, timesRolledOver, test.ShouldBeGreaterThan, 0)

	// We're about to walk all of the output FTDC files to make some assertions. Many assertions are
	// isolated to within a single FTDC file, but the following two assertions are in aggregate
	// across all of the files/data:
	// - That the number of FTDC files found corresponds to the number of times we've "rolled over".
	// - That every "time" in the [0, 1000) range we constructed a datum from is found.
	numFTDCFiles := 0
	timeSeen := make(map[int64]struct{})
	filepath.Walk(ftdcFileDir, filepath.WalkFunc(func(path string, info fs.FileInfo, walkErr error) error {
		logger.Info("Path:", path)
		if !strings.HasSuffix(path, ".ftdc") {
			return nil
		}

		if walkErr != nil {
			logger.Info("Unexpected walk error. Continuing under the assumption any actual* problem will",
				"be caught by the assertions. WalkErr:", walkErr)
			return nil
		}

		// We have an FTDC file. Count it for a later test assertion.
		numFTDCFiles++

		// Additionally, parse the file (in isolation) and assert its contents are correct.
		ftdcFile, err := os.Open(path)
		test.That(t, err, test.ShouldBeNil)
		defer ftdcFile.Close()

		// Temporarily set the log level to INFO to avoid spammy logs. Debug logs during parsing are
		// only interesting when parsing fails.
		logger.SetLevel(logging.INFO)
		datums, err := ParseWithLogger(ftdcFile, logger)
		logger.SetLevel(logging.DEBUG)
		test.That(t, err, test.ShouldBeNil)

		for _, flatDatum := range datums {
			// Each datum contains two metrics: `foo.X` and `foo.Y`. The "time" must be between [0,
			// 1000).
			test.That(t, flatDatum.Time, test.ShouldBeGreaterThanOrEqualTo, 0)
			test.That(t, flatDatum.Time, test.ShouldBeLessThan, 1000)

			// Assert the "time" is new.
			test.That(t, timeSeen, test.ShouldNotContainKey, flatDatum.Time)
			// Mark the "time" as seen.
			timeSeen[flatDatum.Time] = struct{}{}

			// As per construction:
			// - `foo.X` must be equal to the "time" and
			// - `foo.Y` must be `2*time`.
			datum := flatDatum.asDatum()
			test.That(t, datum.Data["foo"].(map[string]float32)["X"], test.ShouldEqual, flatDatum.Time)
			test.That(t, datum.Data["foo"].(map[string]float32)["Y"], test.ShouldEqual, 2*flatDatum.Time)
		}

		return nil
	}))
	test.That(t, len(timeSeen), test.ShouldEqual, 1000)

	// There will be 1 FTDC file per `timesRolledOver`. And an additional file for first call to
	// `writeDatum`. Thus the subtraction of `1` to get the right equation.
	test.That(t, timesRolledOver, test.ShouldEqual, numFTDCFiles-1)
}

func TestParseTimeFromFile(t *testing.T) {
	timeVal, err := parseTimeFromFilename("countingBytesTest1228324349/viam-server-2024-11-18T20-37-01Z.ftdc")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, timeVal.Year(), test.ShouldEqual, 2024)
	test.That(t, timeVal.Month(), test.ShouldEqual, time.Month(11))
	test.That(t, timeVal.Day(), test.ShouldEqual, 18)
	test.That(t, timeVal.Hour(), test.ShouldEqual, 20)
	test.That(t, timeVal.Minute(), test.ShouldEqual, 37)
	test.That(t, timeVal.Second(), test.ShouldEqual, 1)
}

func TestFileDeletion(t *testing.T) {
	// This test takes ~10 seconds due to file naming limitations. This test creates FTDC files on
	// disk whose names include the timestamp with seconds resolution. In this case FTDC has to wait
	// a second before being able to create the next file.
	logger := logging.NewTestLogger(t)

	// Isolate all of the files we're going to create to a single, fresh directory.
	ftdcFileDir, err := os.MkdirTemp("./", "fileDeletionTest")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(ftdcFileDir)

	// We must not use `NewWithWriter`. Forcing a writer for FTDC data is not compatible with FTDC
	// file rotation.
	ftdc := New(ftdcFileDir, logger.Sublogger("ftdc"))

	// Expect a log rotation after 1,000 bytes. For a changing `foo` object, this is ~60 datums.
	ftdc.maxFileSizeBytes = 1000
	ftdc.maxNumFiles = 3

	timesRolledOver := 0
	foo := &foo{}
	ftdc.Add("foo", foo)

	// These settings should result in ~8 rollovers -> ~9 total files.
	for cnt := 0; cnt < 500; cnt++ {
		foo.x = cnt
		foo.y = 2 * cnt

		datum := ftdc.constructDatum()
		datum.Time = int64(cnt)
		err := ftdc.writeDatum(datum)
		test.That(t, err, test.ShouldBeNil)

		// If writing a datum takes the bytes written to larger than configured max file size, an
		// explicit call to `getWriter` should create a new file and reset the count.
		if ftdc.bytesWrittenCounter.count >= ftdc.maxFileSizeBytes {
			// We're about to write a new ftdc file. The ftdc file names are a function of
			// "now". Given the test runs fast, the generated name will collide (names only use
			// seconds resolution). We accept this slowdown for this test.
			_, err = ftdc.getWriter()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ftdc.bytesWrittenCounter.count, test.ShouldBeLessThan, 1000)
			timesRolledOver++
		}
	}

	// We created FTDC files by hand without the background deleter goroutine running. Assert that
	// we have more than the max allowed. Otherwise the test will trivially "pass".
	origFiles := getFTDCFiles(t, ftdc.ftdcDir, logger)
	test.That(t, len(origFiles), test.ShouldBeGreaterThan, ftdc.maxNumFiles)
	slices.SortFunc(origFiles, func(left, right fs.FileInfo) int {
		// Sort in descending order. After deletion, the "leftmost" files should remain. The
		// "rightmost" should be removed.
		return right.ModTime().Compare(left.ModTime())
	})
	logger.Info("Orig files:")
	for _, f := range origFiles {
		logger.Info("  ", f.Name(), " ModTime: ", f.ModTime())
	}

	// Delete excess FTDC files. Check that we now have exactly the max number of allowed files.
	ftdc.checkAndDeleteOldFiles()
	leftoverFiles := getFTDCFiles(t, ftdc.ftdcDir, logger)
	test.That(t, len(leftoverFiles), test.ShouldEqual, ftdc.maxNumFiles)
	slices.SortFunc(leftoverFiles, func(left, right fs.FileInfo) int {
		// Sort in descending order.
		return right.ModTime().Compare(left.ModTime())
	})

	logger.Info("Leftover files:")
	for _, f := range leftoverFiles {
		logger.Info("  ", f.Name(), " ModTime: ", f.ModTime())
	}

	// We've sorted both files in descending timestamp order as per their filename. Assert that the
	// "newest original" files are still remaining.
	for idx := 0; idx < len(leftoverFiles); idx++ {
		test.That(t, leftoverFiles[idx].Name(), test.ShouldEqual, origFiles[idx].Name())
	}

	// And assert the "oldest original" files are no longer found.
	for idx := len(leftoverFiles); idx < len(origFiles); idx++ {
		// The `fs.FileInfo` returned by `os.Lstat` does not include the directory as part of its
		// file name. Reconstitute the relative path before testing.
		_, err := os.Lstat(filepath.Join(ftdc.ftdcDir, origFiles[idx].Name()))
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("File should be deleted. Lstat error: %v", err)
		}
	}
}

func getFTDCFiles(t *testing.T, dir string, logger logging.Logger) []fs.FileInfo {
	var ret []fs.FileInfo
	err := filepath.Walk(dir, filepath.WalkFunc(func(path string, info fs.FileInfo, walkErr error) error {
		if !strings.HasSuffix(path, ".ftdc") {
			return nil
		}

		if walkErr != nil {
			logger.Info("Unexpected walk error. Continuing under the assumption any actual* problem will",
				"be caught by the assertions. WalkErr:", walkErr)
			return nil
		}

		ret = append(ret, info)
		return nil
	}))
	test.That(t, err, test.ShouldBeNil)

	return ret
}
