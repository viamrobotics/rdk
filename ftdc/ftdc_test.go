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

type mockStatser struct {
	stats any
}

func (ms *mockStatser) Stats() any {
	return ms.stats
}

// TestCopeWithChangingSchema asserts that FTDC copes with schema's that change. Originally FTDC was
// designed such that an explicit call to `ftdc.Add` was required to allow for a schema change. But
// it became clear over time that we had to allow for deviations. Such as how network stats
// reporting the list of network interfaces can change.
func TestCopeWithChangingSchema(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `mockStatser` returns whatever we ask it to. Such that we can change the schema without a
	// call to `ftdc.Add`.
	statser := mockStatser{
		stats: struct {
			X int
		}{5},
	}

	ftdc.Add("mock", &statser)
	datum := ftdc.constructDatum()
	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	statser.stats = struct {
		X int
		Y float64
	}{3, 4.0}
	datum = ftdc.constructDatum()
	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	datums, _ /*variable lastTimestampRead*/, err := Parse(ftdcData)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(datums), test.ShouldEqual, 2)

	test.That(t, datums[0].asDatum().Data["mock"], test.ShouldResemble, map[string]float32{"X": 5})
	test.That(t, datums[1].asDatum().Data["mock"], test.ShouldResemble, map[string]float32{"X": 3, "Y": 4})
}

// TestCopeWithSubtleSchemaChange is similar to TestCopeWithChangingSchema except that it keeps the
// number of flattened fields the same. Only the field names changed.
func TestCopeWithSubtleSchemaChange(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `mockStatser` returns whatever we ask it to. Such that we can change the schema without a
	// call to `ftdc.Add`.
	statser := mockStatser{
		stats: struct {
			X int
		}{5},
	}

	ftdc.Add("mock", &statser)
	datum := ftdc.constructDatum()
	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	statser.stats = struct {
		Y int
	}{3}
	datum = ftdc.constructDatum()
	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	datums, _ /*variable lastTimestampRead*/, err := Parse(ftdcData)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(datums), test.ShouldEqual, 2)

	test.That(t, datums[0].asDatum().Data["mock"], test.ShouldResemble, map[string]float32{"X": 5})
	test.That(t, datums[1].asDatum().Data["mock"], test.ShouldResemble, map[string]float32{"Y": 3})
}

type mapStatser struct {
	KeyName string
}

func (mapStatser *mapStatser) Stats() any {
	return map[string]float32{mapStatser.KeyName: 42}
}

func TestMapStatser(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// ftdcData will be appended to on each call to `writeDatum`. At the end of the test we can pass
	// this to `parse` to assert we have the expected results.
	ftdcData := bytes.NewBuffer(nil)
	ftdc := NewWithWriter(ftdcData, logger.Sublogger("ftdc"))

	// `foo` implements `Statser`.
	foo1 := &foo{x: 1, y: 2}
	ftdc.Add("foo1", foo1)

	mapStatser1 := mapStatser{"A"}
	// `mapStatser` implements `Statser`, but returns a map instead of a struct.
	ftdc.Add("mapStatser", &mapStatser1)

	datum := ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 2)
	test.That(t, datum.Data["foo1"], test.ShouldNotBeNil)
	test.That(t, datum.Data["mapStatser"], test.ShouldResemble, map[string]float32{"A": 42})

	err := ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	mapStatser1.KeyName = "B"
	datum = ftdc.constructDatum()
	test.That(t, len(datum.Data), test.ShouldEqual, 2)
	test.That(t, datum.Data["foo1"], test.ShouldNotBeNil)
	test.That(t, datum.Data["mapStatser"], test.ShouldResemble, map[string]float32{"B": 42})

	// This time writing the datum works.
	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	// Verify the contents of the ftdc data.
	datums, _ /*variable lastTimestampRead*/, err := ParseWithLogger(ftdcData, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(datums), test.ShouldEqual, 2)
	test.That(t, datums[0].asDatum().Data["foo1"], test.ShouldResemble, map[string]float32{"X": 1, "Y": 2})
	test.That(t, datums[0].asDatum().Data["mapStatser"], test.ShouldResemble, map[string]float32{"A": 42})
	test.That(t, datums[1].asDatum().Data["foo1"], test.ShouldResemble, map[string]float32{"X": 1, "Y": 2})
	test.That(t, datums[1].asDatum().Data["mapStatser"], test.ShouldResemble, map[string]float32{"B": 42})
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

	_, values, err := walk(datum.Data, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, values, test.ShouldResemble, []float32{1, 2})

	err = ftdc.writeDatum(datum)
	test.That(t, err, test.ShouldBeNil)

	flatDatums, _ /*variable lastTimestampRead*/, err := ParseWithLogger(ftdcData, logger)
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
		datums, lastTimestampRead, err := ParseWithLogger(ftdcFile, logger)
		logger.SetLevel(logging.DEBUG)
		test.That(t, err, test.ShouldBeNil)

		// lastReadTimestamp must be between [0, 1000).
		test.That(t, lastTimestampRead, test.ShouldBeGreaterThanOrEqualTo, 0)
		test.That(t, lastTimestampRead, test.ShouldBeLessThan, 1000)

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
		leftTime, err := parseTimeFromFilename(left.Name())
		test.That(t, err, test.ShouldBeNil)
		rightTime, err := parseTimeFromFilename(right.Name())
		test.That(t, err, test.ShouldBeNil)
		return rightTime.Compare(leftTime)
	})
	logger.Info("Orig files:")
	for _, f := range origFiles {
		logger.Info("  ", f.Name(), "ModTime:", f.ModTime())
	}

	// Delete excess FTDC files. Check that we now have exactly the max number of allowed files.
	ftdc.checkAndDeleteOldFiles()
	leftoverFiles := getFTDCFiles(t, ftdc.ftdcDir, logger)
	test.That(t, len(leftoverFiles), test.ShouldEqual, ftdc.maxNumFiles)
	slices.SortFunc(leftoverFiles, func(left, right fs.FileInfo) int {
		// Sort in descending order.
		leftTime, err := parseTimeFromFilename(left.Name())
		test.That(t, err, test.ShouldBeNil)
		rightTime, err := parseTimeFromFilename(right.Name())
		test.That(t, err, test.ShouldBeNil)
		return rightTime.Compare(leftTime)
	})

	logger.Info("Leftover files:")
	for _, f := range leftoverFiles {
		logger.Info("  ", f.Name(), "ModTime:", f.ModTime())
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
