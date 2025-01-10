package ftdc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

// datum combines the `Stats` call to all registered `Statser`s at some "time". The hierarchy of
// terminology:
// - A `datum` is the aggregation of a single call to each `Statser.Stats()` at some "time".
// - A Statser.`Stats` return value is a collection of "reading"s from the "subsystem" `name`.
// - "Metric name": Each field name in the structure returned by the `Stats` call is a "metric name".
// - A "value" is the numeric value of a metric at one specific point in time.
// - A "reading" is a "metric name" and a "value" at the given `datum.Time`.
//
// A example fully described `datum` object:
//
//	datum: { <-- datum
//	    Time: 1000,
//	    Data: {
//	        "resource_manager": struct resourceManagerStats { <-- Stats
//	            NumComponents: 10, <-- Reading
//	            NumErrorState: 1, <-- Reading...
//	            NumReconfigures: 19991,
//	        },
//	        "webrtc": struct webRTCStats { <-- Stats
//	            NumPeerConnectionsTotal: 5004,
//	            CurrentPeerConnections: 6,
//	            VideoDataSentGB: 1.997,
//	        }
//	        "data_manager": struct dataManagerStats { <-- Stats
//	            DataFilesUploaded: 8842,
//	            DataFilesToUpload: 6,
//	            NumErrorsUploadingDataFiles: 0,
//	        }
//	    }
//	}
//
// Where `resource_manager` is a "subsystem name".
//
// And where `NumComponents` without a corresponding value is simply a "metric name". And "5004"
// without context of a metric name is a simply a "value". Those terms are more relevant to the FTDC
// file format.
type datum struct {
	// Time in nanoseconds since the epoch.
	Time int64
	Data map[string]any

	generationID int
}

// Statser implements Stats.
type Statser interface {
	// The Stats method must return a struct with public field members that are either:
	// - Numbers (e.g: int, float64, byte, etc...)
	// - A "recursive" structure that has the same properties as this return value (public field
	//   members with numbers, or more structures).
	//
	// The return value must always have the same schema. That is, the returned value has the same
	// set of keys/structure members. A simple structure where all the member fields are numbers
	// satisfies this requirement.
	//
	// The return value may not (yet) be a `map`. Even if the returned map always has the same keys.
	Stats() any
}

type namedStatser struct {
	name    string
	statser Statser
}

// FTDC is a tool for storing observability data on disk in a compact binary format for production
// debugging.
type FTDC struct {
	// mu protects the `statser` member. The `statser` member is modified during user calls to `Add`
	// and `Remove`. Additionally, there's a concurrent background reader of the `statser` member.
	mu       sync.Mutex
	statsers []namedStatser

	// Fields used to generate and serialize FTDC output to bytes.
	//
	// inputGenerationID changes when new pieces are added to FTDC at runtime that change the
	// schema.
	inputGenerationID int
	// outputGenerationID represents the last schema written to the FTDC `outputWriter`.
	outputGenerationID int
	// The schema used describe how new Datums are serialized.
	currSchema *schema
	// The serialization format compares new metrics to the prior metric reading to determine what
	// to write. `prevFlatData` is the field used to create a diff that's serialized. For
	// simplicity, all metrics are massaged into a 32-bit float. See `custom_format.go` for a more
	// detailed description.
	prevFlatData []float32

	readStatsWorker  *utils.StoppableWorkers
	datumCh          chan datum
	outputWorkerDone chan struct{}
	stopOnce         sync.Once

	// Fields used to manage where serialized FTDC bytes are written.
	outputWriter io.Writer
	// bytesWrittenCounter will count bytes that are written to the `outputWriter`. We use an
	// `io.Writer` implementer for this, as opposed to just counting by hand, primarily as a
	// convenience for working with `json.NewEncoder(writer).Encode(...)`. This counter is folded
	// into the above `outputWriter`.
	bytesWrittenCounter countingWriter
	currOutputFile      *os.File
	maxFileSizeBytes    int64
	maxNumFiles         int
	// ftdcDir controls where FTDC data files will be written.
	ftdcDir string

	logger logging.Logger
}

// New creates a new *FTDC. This FTDC object will write FTDC formatted files into the input
// `ftdcDirectory`.
func New(ftdcDirectory string, logger logging.Logger) *FTDC {
	ret := newFTDC(logger)
	ret.maxFileSizeBytes = 1_000_000
	ret.maxNumFiles = 10
	ret.ftdcDir = ftdcDirectory
	return ret
}

// NewWithWriter creates a new *FTDC that outputs bytes to the specified writer.
func NewWithWriter(writer io.Writer, logger logging.Logger) *FTDC {
	ret := newFTDC(logger)
	ret.outputWriter = writer
	return ret
}

// DefaultDirectory returns a directory to write FTDC data files in. Each unique "part" running on a
// single computer will get its own directory.
func DefaultDirectory(viamHome, partID string) string {
	return filepath.Join(viamHome, "diagnostics.data", partID)
}

func newFTDC(logger logging.Logger) *FTDC {
	return &FTDC{
		// Allow for some wiggle before blocking producers.
		datumCh:          make(chan datum, 20),
		outputWorkerDone: make(chan struct{}),
		logger:           logger,
	}
}

// Add regsiters a new staters that will be recorded in future FTDC loop iterations.
func (ftdc *FTDC) Add(name string, statser Statser) {
	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()

	for _, statser := range ftdc.statsers {
		if statser.name == name {
			ftdc.logger.Warnw("Trying to add conflicting ftdc section", "name", name,
				"generationId", ftdc.inputGenerationID)
			// FTDC output is broken down into separate "sections". The `name` is used to label each
			// section. We return here to predictably include one of the `Add`ed statsers.
			return
		}
	}

	ftdc.logger.Debugw("Added statser", "name", name,
		"type", fmt.Sprintf("%T", statser), "generationId", ftdc.inputGenerationID)
	ftdc.statsers = append(ftdc.statsers, namedStatser{
		name:    name,
		statser: statser,
	})
	ftdc.inputGenerationID++
}

// Remove removes a statser that was previously `Add`ed with the given `name`.
func (ftdc *FTDC) Remove(name string) {
	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()

	for idx, statser := range ftdc.statsers {
		if statser.name == name {
			ftdc.logger.Debugw("Removed statser", "name", name,
				"type", fmt.Sprintf("%T", statser.statser), "generationId", ftdc.inputGenerationID)
			ftdc.statsers = slices.Delete(ftdc.statsers, idx, idx+1)
			ftdc.inputGenerationID++
			return
		}
	}

	ftdc.logger.Warnw("Did not find statser to remove",
		"name", name, "generationId", ftdc.inputGenerationID)
}

// Start spins off the background goroutine for collecting + writing FTDC data. It's normal for tests
// to _not_ call `Start`. Tests can simulate the same functionality by calling `constructDatum` and `writeDatum`.
func (ftdc *FTDC) Start() {
	if runtime.GOOS == "windows" {
		// note: this logs a panic on RDK start on windows.
		ftdc.logger.Warn("FTDC not implemented on windows, not starting")
		return
	}
	ftdc.readStatsWorker = utils.NewStoppableWorkerWithTicker(time.Second, ftdc.statsReader)
	utils.PanicCapturingGo(ftdc.statsWriter)

	// The `fileDeleter` goroutine mostly aligns with the "stoppable worker with ticker"
	// pattern. But it has the additional desire that if file deletion exits with a panic, all of
	// FTDC should stop.
	utils.PanicCapturingGoWithCallback(ftdc.fileDeleter, func(err any) {
		ftdc.logger.Warnw("File deleter errored, stopping FTDC", "err", err)
		ftdc.StopAndJoin(context.Background())
	})
}

func (ftdc *FTDC) statsReader(ctx context.Context) {
	datum := ftdc.constructDatum()
	if datum.generationID == 0 {
		// No "statsers" were `Add`ed. No data to write out.
		return
	}

	// `Debugw` does not seem to serialize any of the `datum` value.
	ftdc.logger.Debugf("Metrics collected. Datum: %+v", datum)

	select {
	case ftdc.datumCh <- datum:
		break
	case <-ftdc.outputWorkerDone:
		break
	case <-ctx.Done():
		break
	}
}

func (ftdc *FTDC) statsWriter() {
	defer func() {
		if ftdc.currOutputFile != nil {
			utils.UncheckedError(ftdc.currOutputFile.Close())
		}
		close(ftdc.outputWorkerDone)
	}()

	datumsWritten := 0
	for datum := range ftdc.datumCh {
		var schemaErr *schemaError
		if err := ftdc.writeDatum(datum); err != nil && !errors.As(err, &schemaErr) {
			// This code path ignores `errNotStruct` errors and shuts down on everything else.  An
			// `errNotStruct` happens when some registered `Statser` returned a `map` instead of a
			// `struct`. The lower level `writeDatum` call has handled the error by removing the
			// `Statser` from "registry". But bubbles it up to signal that no `Datum` was written.
			// The errors that do get handled here are expected to simply be FS/disk failure errors.

			ftdc.logger.Error("Error writing ftdc data. Shutting down FTDC.", "err", err)
			// To shut down, we just exit. Closing the `ftdc.outputWorkerDone`. The `statsReader`
			// goroutine will eventually observe that channel was closed and also exit.
			return
		}

		// FSync the ftdc data once every 30 iterations (roughly every 30 seconds).
		datumsWritten++
		if datumsWritten%30 == 0 && ftdc.currOutputFile != nil {
			utils.UncheckedError(ftdc.currOutputFile.Sync())
		}
	}
}

// StopAndJoin stops the background worker started by `Start`. It is only legal to call this after
// `Start` returns. It's normal for tests to _not_ call `StopAndJoin`. Tests that have spun up the
// `statsWriter` by hand, without the `statsReader` can `close(ftdc.datumCh)` followed by
// `<-ftdc.outputWorkerDone` to stop+wait for the `statsWriter`.
func (ftdc *FTDC) StopAndJoin(ctx context.Context) {
	if runtime.GOOS == "windows" {
		return
	}
	ftdc.stopOnce.Do(func() {
		// Only one caller should close the datum channel. And it should be the caller that called
		// stop on the worker writing to the channel.
		ftdc.readStatsWorker.Stop()
		close(ftdc.datumCh)
	})

	// Closing the `statsCh` signals to the `outputWorker` to complete and exit. We use a timeout to
	// limit how long we're willing to wait for the `outputWorker` to drain.
	select {
	case <-ftdc.outputWorkerDone:
	case <-time.After(10 * time.Second):
	}
}

// conditionalRemoveStatser first checks the generation matches before removing the `name` Statser.
func (ftdc *FTDC) conditionalRemoveStatser(name string, generationID int) {
	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()

	// This function gets called by the "write ftdc" actor. Which is concurrent to a user
	// adding/removing `Statser`s. If the datum/name that created a problem came from a different
	// "generation", optimistically guess that the user fixed the problem, and avoid removing a
	// perhaps working `Statser`.
	//
	// In the (honestly, more likely) event, the `Statser` is still bad, we will eventually succeed
	// in removing it. As later `Datum` objects to write will have an updated `generationId`.
	if generationID != ftdc.inputGenerationID {
		ftdc.logger.Debugw("Not removing statser due to concurrent operation",
			"datumGenerationId", generationID, "ftdcGenerationId", ftdc.inputGenerationID)
		return
	}

	for idx, statser := range ftdc.statsers {
		if statser.name == name {
			ftdc.logger.Debugw("Removed statser", "name", name,
				"type", fmt.Sprintf("%T", statser.statser), "generationId", ftdc.inputGenerationID)
			ftdc.statsers = slices.Delete(ftdc.statsers, idx, idx+1)
			ftdc.inputGenerationID++
			return
		}
	}

	ftdc.logger.Warnw("Did not find statser to remove", "name", name, "generationId", ftdc.inputGenerationID)
}

// constructDatum walks all of the registered `statser`s to construct a `datum`.
func (ftdc *FTDC) constructDatum() datum {
	datum := datum{
		Time: time.Now().UnixNano(),
		Data: map[string]any{},
	}

	// RSDK-9650: Take the mutex to make a copy of the list of `ftdc.statsers` objects. Such that we
	// can release the mutex before calling any `Stats` methods. It may be the case where the
	// `Stats` method acquires some other mutex/resource.  E.g: acquiring resources from the
	// resource graph. Which is the starting point for creating a deadlock scenario.
	ftdc.mu.Lock()
	statsers := make([]namedStatser, len(ftdc.statsers))
	datum.generationID = ftdc.inputGenerationID
	copy(statsers, ftdc.statsers)
	ftdc.mu.Unlock()

	for idx := range statsers {
		namedStatser := &statsers[idx]
		datum.Data[namedStatser.name] = namedStatser.statser.Stats()
	}

	return datum
}

// writeDatum takes an ftdc reading ("Datum") as input and serializes + writes it to the backing
// medium (e.g: a file). See `writeSchema`s documentation for a full description of the file format.
func (ftdc *FTDC) writeDatum(datum datum) error {
	toWrite, err := ftdc.getWriter()
	if err != nil {
		return err
	}

	// The input `datum` being processed is for a different schema than we were previously using.
	if datum.generationID != ftdc.outputGenerationID {
		// Compute the new schema and write that to disk.
		newSchema, schemaErr := getSchema(datum.Data)
		if schemaErr != nil {
			ftdc.logger.Warnw("Could not generate schema for statser",
				"statser", schemaErr.statserName, "err", schemaErr.err)
			// We choose to remove the misbehaving statser such that subsequent datums will be
			// well-formed.
			ftdc.conditionalRemoveStatser(schemaErr.statserName, datum.generationID)
			return schemaErr
		}

		ftdc.currSchema = newSchema
		if err = writeSchema(ftdc.currSchema, toWrite); err != nil {
			return err
		}

		// Update the `outputGenerationId` to reflect the new schema.
		ftdc.outputGenerationID = datum.generationID

		data, err := flatten(datum, ftdc.currSchema)
		if err != nil {
			return err
		}

		// Write the new data point to disk. When schema changes, we do not do any diffing. We write
		// a raw value for each metric.
		if err = writeDatum(datum.Time, nil, data, toWrite); err != nil {
			return err
		}
		ftdc.prevFlatData = data

		return nil
	}

	// The input `datum` is for the same schema as the prior datum. Flatten the values and write a
	// datum entry diffed against the `prevFlatData`.
	data, err := flatten(datum, ftdc.currSchema)
	if err != nil {
		return err
	}

	if err = writeDatum(datum.Time, ftdc.prevFlatData, data, toWrite); err != nil {
		return err
	}
	ftdc.prevFlatData = data
	return nil
}

// getWriter returns an io.Writer xor error for writing schema/data information. `getWriter` is only
// expected to be called by `writeDatum`.
func (ftdc *FTDC) getWriter() (io.Writer, error) {
	// If we have an `outputWriter` without a `currOutputFile`, it means ftdc was constructed with
	// an explicit writer. We will use the passed in writer for all operations. No file will ever be
	// created.
	if ftdc.outputWriter != nil && ftdc.currOutputFile == nil {
		return ftdc.outputWriter, nil
	}

	// Note to readers, until this function starts mutating `outputWriter` and `currOutputFile`, you
	// can safely assume:
	//
	//   `outputWriter == nil if and only if currOutputFile == nil`.
	//
	// In case that helps reading the following logic.

	// If we have an active outputWriter and we have not exceeded our FTDC file rotation quota, we
	// can just return.
	if ftdc.outputWriter != nil && ftdc.bytesWrittenCounter.count < ftdc.maxFileSizeBytes {
		return ftdc.outputWriter, nil
	}

	// If we're in the logic branch where we have exceeded our FTDC file rotation quota, we first
	// close the `currOutputFile`.
	if ftdc.currOutputFile != nil {
		// Dan: An error closing a file (any resource for that matter) is not an error. I will die
		// on that hill.
		utils.UncheckedError(ftdc.currOutputFile.Close())
	}

	var err error
	// It's unclear in what circumstance we'd expect creating a new file to fail. Try 5 times for no
	// good reason before giving up entirely and shutting down FTDC.
	for numTries := 0; numTries < 5; numTries++ {
		// The viam process is expected to be run as root. The FTDC directory must be readable by
		// "other" users.
		//
		//nolint:gosec
		if err = os.MkdirAll(ftdc.ftdcDir, 0o755); err != nil {
			ftdc.logger.Warnw("Failed to create FTDC directory", "dir", ftdc.ftdcDir, "err", err)
			return nil, err
		}

		now := time.Now().UTC()
		// lint wants 0o600 file permissions. We don't expect the unix user someone is ssh'ed in as
		// to be on the same unix user as is running the viam-server process. Thus the file needs to
		// be accessible by anyone.
		//
		//nolint:gosec
		ftdc.currOutputFile, err = os.OpenFile(path.Join(ftdc.ftdcDir,
			// Filename example: viam-server-2024-10-04T18-42-02.ftdc
			fmt.Sprintf("viam-server-%d-%02d-%02dT%02d-%02d-%02dZ.ftdc",
				now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())),
			// Create a new file in read+write mode. `O_EXCL` is used to guarantee a new file is
			// created. If the filename already exists, that flag changes the `os.OpenFile` behavior
			// to return an error.
			os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			break
		}
		ftdc.logger.Warnw("FTDC failed to open file", "err", err)

		// If the error is some unexpected filename collision, wait a second to change the filename.
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, err
	}

	// New file, reset the bytes written counter.
	ftdc.bytesWrittenCounter.count = 0

	// When we create a new file, we must rewrite the schema. If we do not, a file may be useless
	// without its "ancestors".
	//
	// As a hack, we decrement the `outputGenerationID` to force a new schema to be written.
	ftdc.outputGenerationID--

	// Assign the `outputWriter`. The `outputWriter` is an abstraction for where FTDC formatted
	// bytes go. Testing often prefers to just write bytes into memory (and consequently construct
	// an FTDC with `NewWithWriter`). While in production we obviously want to persist bytes on
	// disk.
	ftdc.outputWriter = io.MultiWriter(&ftdc.bytesWrittenCounter, ftdc.currOutputFile)

	return ftdc.outputWriter, nil
}

func (ftdc *FTDC) fileDeleter() {
	for {
		select {
		// The fileDeleter's goroutine lifetime should match the robot/FTDC lifetime. Borrow the
		// `readStatsWorker`s context to track that.
		case <-ftdc.readStatsWorker.Context().Done():
			return
		case <-time.After(time.Second):
		}

		if err := ftdc.checkAndDeleteOldFiles(); err != nil {
			ftdc.logger.Warnw("Error checking FTDC files", "err", err)
		}
	}
}

// fileTime pairs a file with a time value.
type fileTime struct {
	name string
	time time.Time
}

func (ftdc *FTDC) checkAndDeleteOldFiles() error {
	var files []fileTime

	// Walk the `ftdcDir` and gather all of the found files into the captured `files` variable.
	err := filepath.Walk(ftdc.ftdcDir, filepath.WalkFunc(func(path string, info fs.FileInfo, walkErr error) error {
		if !strings.HasSuffix(path, ".ftdc") {
			return nil
		}

		if walkErr != nil {
			ftdc.logger.Warnw("Unexpected walk error. Continuing under the assumption any actual* problem will",
				"be caught by the assertions.", "err", walkErr)
			return nil
		}

		parsedTime, err := parseTimeFromFilename(path)
		if err == nil {
			files = append(files, fileTime{path, parsedTime})
		} else {
			ftdc.logger.Warnw("Error parsing time from FTDC file", "filename", path)
		}
		return nil
	}))
	if err != nil {
		return err
	}

	if len(files) <= ftdc.maxNumFiles {
		// We have yet to hit our file limit. Keep all of the files.
		ftdc.logger.Debugw("Inside the budget for ftdc files", "numFiles", len(files), "maxNumFiles", ftdc.maxNumFiles)
		return nil
	}

	slices.SortFunc(files, func(left, right fileTime) int {
		// Sort in descending order. Such that files indexed first are safe. This eases walking the
		// slice of files.
		return right.time.Compare(left.time)
	})

	// If we, for example, have 30 files and we want to keep the newest 10, we delete the trailing
	// 20 files.
	for _, file := range files[ftdc.maxNumFiles:] {
		ftdc.logger.Debugw("Deleting aged out FTDC file", "filename", file.name)
		if err := os.Remove(file.name); err != nil {
			ftdc.logger.Warnw("Error removing FTDC file", "filename", file.name)
		}
	}

	return nil
}

// filenameTimeRe matches the files produced by ftdc. Filename <-> regex parity is exercised by file
// deletion testing. Filename generation uses padding such that we can rely on there before 2/4
// digits for every numeric value.
//
//nolint
// Example filename: countingBytesTest1228324349/viam-server-2024-11-18T20-37-01Z.ftdc
var filenameTimeRe = regexp.MustCompile(`viam-server-(\d{4})-(\d{2})-(\d{2})T(\d{2})-(\d{2})-(\d{2})Z.ftdc`)

func parseTimeFromFilename(path string) (time.Time, error) {
	allMatches := filenameTimeRe.FindAllStringSubmatch(path, -1)
	if len(allMatches) != 1 || len(allMatches[0]) != 7 {
		return time.Time{}, errors.New("filename did not match pattern")
	}

	// There's exactly one match and 7 groups. The first "group" is the whole string. We only care
	// about the numbers.
	matches := allMatches[0][1:]

	var numVals [6]int
	for idx := 0; idx < 6; idx++ {
		val, err := strconv.Atoi(matches[idx])
		if err != nil {
			return time.Time{}, err
		}

		numVals[idx] = val
	}

	return time.Date(numVals[0], time.Month(numVals[1]), numVals[2], numVals[3], numVals[4], numVals[5], 0, time.UTC), nil
}

type countingWriter struct {
	count int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	cw.count += int64(len(p))
	return len(p), nil
}
