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
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

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

	uploader *uploader
	logger   logging.Logger
}

// New creates a new *FTDC. This FTDC object will write FTDC formatted files into the input
// `ftdcDirectory`.
func New(ftdcDirectory string, logger logging.Logger) *FTDC {
	ret := newFTDC(logger)
	ret.maxFileSizeBytes = 10_000_000
	ret.maxNumFiles = 20
	ret.ftdcDir = ftdcDirectory
	return ret
}

// NewWithWriter creates a new *FTDC that outputs bytes to the specified writer.
func NewWithWriter(writer io.Writer, logger logging.Logger) *FTDC {
	ret := newFTDC(logger)
	ret.outputWriter = writer
	return ret
}

// NewWithUploader creates a new *FTDC that will also upload FTDC files to cloud.
func NewWithUploader(ftdcDirectory string, cloudConn rpc.ClientConn, partID string, logger logging.Logger) *FTDC {
	ret := New(ftdcDirectory, logger)
	if cloudConn != nil {
		ret.uploader = newUploader(cloudConn, ftdcDirectory, partID, logger.Sublogger("uploader"))

		// It's imperative this operation is performed `Start` is called. Once `Start` is called, a
		// new file can be written out with data from the current running `viam-server`.
		files, err := getFTDCFilesDescendingTimeOrder(ftdcDirectory, logger)
		if len(files) > 0 && err == nil {
			// Be conservative and only upload a file if there was no directory walking error. To
			// avoid double inserting an FTDC file into cloud due to a robot disk issue.
			ret.uploader.addFileToUpload(files[0].name)
		}
	}
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

// Add registers a new statser that will be recorded in future FTDC loop iterations.
func (ftdc *FTDC) Add(name string, statser Statser) {
	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()

	for _, statser := range ftdc.statsers {
		if statser.name == name {
			ftdc.logger.Warnw("Trying to add conflicting ftdc section", "name", name)
			// FTDC output is broken down into separate "sections". The `name` is used to label each
			// section. We return here to predictably include one of the `Add`ed statsers.
			return
		}
	}

	ftdc.logger.Debugw("Added statser", "name", name, "type", fmt.Sprintf("%T", statser))
	ftdc.statsers = append(ftdc.statsers, namedStatser{
		name:    name,
		statser: statser,
	})
}

// Remove removes a statser that was previously `Add`ed with the given `name`.
func (ftdc *FTDC) Remove(name string) {
	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()

	for idx, statser := range ftdc.statsers {
		if statser.name == name {
			ftdc.logger.Debugw("Removed statser", "name", name, "type", fmt.Sprintf("%T", statser.statser))
			ftdc.statsers = slices.Delete(ftdc.statsers, idx, idx+1)
			return
		}
	}

	ftdc.logger.Warnw("Did not find statser to remove", "name", name)
}

// Start spins off the background goroutine for collecting + writing FTDC data. It's normal for tests
// to _not_ call `Start`. Tests can simulate the same functionality by calling `constructDatum` and `writeDatum`.
func (ftdc *FTDC) Start() {
	ftdc.readStatsWorker = utils.NewStoppableWorkerWithTicker(time.Second, ftdc.statsReader)
	utils.PanicCapturingGo(ftdc.statsWriter)

	// The `fileDeleter` goroutine mostly aligns with the "stoppable worker with ticker"
	// pattern. But it has the additional desire that if file deletion exits with a panic, all of
	// FTDC should stop.
	utils.PanicCapturingGoWithCallback(ftdc.fileDeleter, func(err any) {
		ftdc.logger.Warnw("File deleter errored, stopping FTDC", "err", err)
		ftdc.StopAndJoin(context.Background())
	})
	if ftdc.uploader != nil {
		ftdc.uploader.start()
	}
}

func (ftdc *FTDC) statsReader(ctx context.Context) {
	datum := ftdc.constructDatum()

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

			ftdc.logger.Errorw("Error writing ftdc data. Shutting down FTDC.", "err", err)
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
	ftdc.stopOnce.Do(func() {
		// Only one caller should close the datum channel. And it should be the caller that called
		// stop on the worker writing to the channel.
		if ftdc.readStatsWorker != nil {
			ftdc.readStatsWorker.Stop()
		}
		close(ftdc.datumCh)
	})

	if ftdc.uploader != nil {
		ftdc.uploader.stopAndJoin()
	}

	// Closing the `statsCh` signals to the `outputWorker` to complete and exit. We use a timeout to
	// limit how long we're willing to wait for the `outputWorker` to drain.
	select {
	case <-ftdc.outputWorkerDone:
	case <-time.After(10 * time.Second):
	}
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
	copy(statsers, ftdc.statsers)
	ftdc.mu.Unlock()

	for idx := range statsers {
		namedStatser := &statsers[idx]
		datum.Data[namedStatser.name] = namedStatser.statser.Stats()
	}

	return datum
}

// walk accepts a datum and the previous schema and will return:
// - the new schema. If the schema is unchanged, this will be the same pointer value as `previousSchema`.
// - the flattened float32 data points.
// - an error. All errors (for now) are terminal -- the input datum cannot be output.
func walk(datum map[string]any, previousSchema *schema) (*schema, []float32, error) {
	schemaChanged := false

	var (
		fields         []string
		values         []float32
		iterationOrder []string
	)

	// In the steady state, we will have an existing schema. Use that for a `datum` iteration order.
	if previousSchema != nil {
		fields = make([]string, 0, len(previousSchema.fieldOrder))
		values = make([]float32, 0, len(previousSchema.fieldOrder))
		iterationOrder = previousSchema.mapOrder
	} else {
		// If this is the first data point, we'll walk the map in... map order.
		schemaChanged = true
		iterationOrder = make([]string, 0, len(datum))
		for key := range datum {
			iterationOrder = append(iterationOrder, key)
		}
	}

	// Record the order we iterate through the keys in the input `datum`. We return this in the case
	// we learn the schema changed.
	datumMapOrder := make([]string, 0, len(datum))

	// Create a set out of the `inputSchema.mapOrder` as we iterate over it. This will be used to
	// see if new keys have been added to the `datum` map that were not in the `previousSchema`.
	mapOrderSet := make(map[string]struct{})
	for _, key := range iterationOrder {
		mapOrderSet[key] = struct{}{}

		// Walk over the datum in `mapOrder` to ensure we gather values in the order consistent with
		// the current schema.
		stats, exists := datum[key]
		if !exists {
			// There was a `Statser` in the previous `datum` that no longer exists. Note the schema
			// changed and move on.
			schemaChanged = true
			continue
		}

		// Get all of the field names and values from the `stats` object.
		itemFields, itemNumbers, err := flatten(reflect.ValueOf(stats))
		if err != nil {
			return nil, nil, err
		}

		datumMapOrder = append(datumMapOrder, key)
		// For each field we found, prefix it with the `datum` key (the `Statser` name).
		for idx := range itemFields {
			fields = append(fields, fmt.Sprintf("%v.%v", key, itemFields[idx]))
		}
		values = append(values, itemNumbers...)
	}

	// Check for a schema change by walking all of the keys (`Statser`s) in the input `datum`. Look
	// for anything new.
	for dataKey, stats := range datum {
		if _, exists := mapOrderSet[dataKey]; exists {
			// The steady-state is that every key in the input `datum` matches the prior
			// `datum`/schema.
			continue
		}

		// We found a statser that did not exist before. Let's add it to our results.
		schemaChanged = true
		itemFields, itemNumbers, err := flatten(reflect.ValueOf(stats))
		if err != nil {
			return nil, nil, err
		}

		datumMapOrder = append(datumMapOrder, dataKey)
		// Similarly, prefix fields with the `Statser` name.
		for idx := range itemFields {
			fields = append(fields, fmt.Sprintf("%v.%v", dataKey, itemFields[idx]))
		}
		values = append(values, itemNumbers...)
	}

	// Even if the keys in the `datum` stayed the same, the values returned by an individual `Stats`
	// call may have changed. This ought to be rare, as this results in writing out a new schema and
	// is consequently inefficient. But we prefer to have less FTDC data than inaccurate data, or
	// more simply, failing.
	if previousSchema != nil && !slices.Equal(previousSchema.fieldOrder, fields) {
		schemaChanged = true
	}

	// If the schema changed, return a new schema object with the updated schema.
	if schemaChanged {
		return &schema{datumMapOrder, fields}, values, nil
	}

	return previousSchema, values, nil
}

func (ftdc *FTDC) writeDatum(datum datum) error {
	toWrite, err := ftdc.getWriter()
	if err != nil {
		return err
	}

	// walk will return the schema it found alongside the flattened data. Errors are terminal. If
	// the schema is the same, the `newSchema` pointer will match `ftdc.currSchema` and `err` will
	// be nil.
	newSchema, flatData, err := walk(datum.Data, ftdc.currSchema)
	if err != nil {
		return err
	}

	// In the happy path where the schema hasn't changed, the `walk` function is guaranteed to
	// return the same schema object.
	if ftdc.currSchema != newSchema {
		ftdc.currSchema = newSchema
		if err = writeSchema(ftdc.currSchema, toWrite); err != nil {
			return err
		}

		// Write the new data point to disk. When schema changes, we do not do any diffing. We write
		// a raw value for each metric.
		ftdc.prevFlatData = nil
	}

	if err = writeDatum(datum.Time, ftdc.prevFlatData, flatData, toWrite); err != nil {
		return err
	}
	ftdc.prevFlatData = flatData

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
		utils.UncheckedError(ftdc.currOutputFile.Close())
		if ftdc.uploader != nil {
			// Dan: For now we only upload "completed" during the runtime of a viam-server. There's
			// no harm in uploading leftover files from a prior run, but the current bang for the
			// buck was deemed not worth it.
			ftdc.uploader.addFileToUpload(ftdc.currOutputFile.Name())
		}
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

	// Assign the `outputWriter`. The `outputWriter` is an abstraction for where FTDC formatted
	// bytes go. Testing often prefers to just write bytes into memory (and consequently construct
	// an FTDC with `NewWithWriter`). While in production we obviously want to persist bytes on
	// disk.
	ftdc.outputWriter = io.MultiWriter(&ftdc.bytesWrittenCounter, ftdc.currOutputFile)

	// The schema was last persisted in the prior FTDC file. To ensure this file can be understood
	// without it, we start it with a copy of the schema. We achieve this by erasing the
	// `currSchema` value. Such that the caller/`writeDatum` will behave as if this is a "schema
	// change".
	ftdc.currSchema = nil

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

func getFTDCFilesDescendingTimeOrder(ftdcDir string, logger logging.Logger) ([]fileTime, error) {
	var files []fileTime

	// Walk the `ftdcDir` and gather all of the found files into the captured `files` variable.
	err := filepath.Walk(ftdcDir, filepath.WalkFunc(func(path string, info fs.FileInfo, walkErr error) error {
		if !strings.HasSuffix(path, ".ftdc") {
			return nil
		}

		if walkErr != nil {
			logger.Warnw("Unexpected walk error. Continuing under the assumption any actual* problem will",
				"be caught by the assertions.", "err", walkErr)
			return nil
		}

		parsedTime, err := parseTimeFromFilename(path)
		if err == nil {
			files = append(files, fileTime{path, parsedTime})
		} else {
			logger.Warnw("Error parsing time from FTDC file", "filename", path)
		}
		return nil
	}))
	if err != nil {
		return nil, err
	}

	slices.SortFunc(files, func(left, right fileTime) int {
		// Sort in descending order.
		return right.time.Compare(left.time)
	})

	return files, nil
}

func (ftdc *FTDC) checkAndDeleteOldFiles() error {
	files, err := getFTDCFilesDescendingTimeOrder(ftdc.ftdcDir, ftdc.logger)
	if err != nil {
		return err
	}

	if len(files) <= ftdc.maxNumFiles {
		// We have yet to hit our file limit. Keep all of the files.
		ftdc.logger.Debugw("Inside the budget for ftdc files", "numFiles", len(files), "maxNumFiles", ftdc.maxNumFiles)
		return nil
	}

	slices.SortFunc(files, func(left, right fileTime) int {
		return right.time.Compare(left.time)
	})

	// The files are conveniently in descending time order. If we, for example, have 30 files and we
	// want to keep the newest 10, we delete the trailing 20 files.
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
// Example filename: `countingBytesTest1228324349/viam-server-2024-11-18T20-37-01Z.ftdc`.
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
