package ftdc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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
	Time int64
	Data map[string]any

	generationID int
}

// Statser implements Stats.
type Statser interface {
	// The Stats method must return a struct with public field members that are either:
	// - Numbers (e.g: int, float64, byte, etc...)
	// - A "recursive" structure that has the same properties as this return value (public field
	//   members with numbers, or more structures). (NOT YET SUPPORTED)
	//
	// The return value must not be a map. This is to enforce a "schema" constraints.
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

	// Fields used to manage where serialized FTDC bytes are written.
	//
	// When debug is true, the `outputWriter` will "tee" data to both the `currOutputFile` and
	// `inmemBuffer`. Otherwise `outputWriter` will just refer to the `currOutputFile`.
	debug          bool
	outputWriter   io.Writer
	currOutputFile *os.File
	// inmemBuffer will remain nil when `debug` is false.
	inmemBuffer *bytes.Buffer

	logger logging.Logger
}

// New creates a new *FTDC.
func New(logger logging.Logger) *FTDC {
	return NewWithWriter(nil, logger)
}

// NewWithWriter creates a new *FTDC that outputs bytes to the specified writer.
func NewWithWriter(writer io.Writer, logger logging.Logger) *FTDC {
	return &FTDC{
		// Allow for some wiggle before blocking producers.
		datumCh:          make(chan datum, 20),
		outputWorkerDone: make(chan struct{}),
		logger:           logger,
		outputWriter:     writer,
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

// Start spins off the background goroutine for collecting/writing FTDC data.
func (ftdc *FTDC) Start() {
	ftdc.readStatsWorker = utils.NewStoppableWorkerWithTicker(time.Second, ftdc.statsReader)
	utils.PanicCapturingGo(ftdc.statsWriter)
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

		datumsWritten++
		if datumsWritten%30 == 0 && ftdc.currOutputFile != nil {
			utils.UncheckedError(ftdc.currOutputFile.Sync())
		}
	}
}

// StopAndJoin stops the background worker started by `Start`. It is only legal to call this after
// `Start` returns.
func (ftdc *FTDC) StopAndJoin(ctx context.Context) {
	ftdc.readStatsWorker.Stop()
	close(ftdc.datumCh)

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
		Time: time.Now().Unix(),
		Data: map[string]any{},
	}

	ftdc.mu.Lock()
	defer ftdc.mu.Unlock()
	datum.generationID = ftdc.inputGenerationID
	for idx := range ftdc.statsers {
		namedStatser := &ftdc.statsers[idx]
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
// expected to be called by `newDatum`.
func (ftdc *FTDC) getWriter() (io.Writer, error) {
	if ftdc.outputWriter != nil {
		return ftdc.outputWriter, nil
	}

	var err error
	ftdc.currOutputFile, err = os.Create("./viam-server.ftdc")
	if err != nil {
		ftdc.logger.Warnw("FTDC failed to open file", "err", err)
		return nil, err
	}

	if ftdc.debug {
		ftdc.inmemBuffer = bytes.NewBuffer(nil)
		ftdc.outputWriter = io.MultiWriter(ftdc.currOutputFile, ftdc.inmemBuffer)
	} else {
		ftdc.outputWriter = ftdc.currOutputFile
	}

	return ftdc.outputWriter, nil
}
