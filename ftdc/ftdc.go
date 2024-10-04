package ftdc

import (
	"bytes"
	"io"
	"os"

	"go.viam.com/rdk/logging"
)

type Datum struct {
	Time int64
	Data map[string]any

	generationId int
}

type FTDC struct {
	// Fields used to generate and serialize FTDC output to bytes.
	//
	// inputGenerationId changes when new pieces are added to FTDC at runtmie that change the
	// schema.
	inputGenerationId int
	// outputGenerationId represents the last schema written to the FTDC output stream.
	outputGenerationId int
	// The schema used describe how new Datums are serialized.
	currSchema *Schema
	// The serialization format compares new metrics to the prior metric reading to determine what
	// to write. `prevFlatData` is the field used to create a diff that's serialized.
	prevFlatData []float32

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

func New(logger logging.Logger) *FTDC {
	return &FTDC{
		logger: logger,
	}
}

func NewWithWriter(writer io.Writer, logger logging.Logger) *FTDC {
	return &FTDC{
		logger:       logger,
		outputWriter: writer,
	}
}

// newDatum takes an ftdc reading ("Datum") as input and serializes + writes it to the backing
// medium (e.g: a file). See `writeSchema`s documentation for a full description of the file format.
func (ftdc *FTDC) newDatum(datum Datum) error {
	toWrite, err := ftdc.getWriter()
	if err != nil {
		return err
	}

	// The input `datum` being processed is for a different schema than we were previously using.
	if datum.generationId != ftdc.outputGenerationId {
		// Compute the new schema and write that to disk.
		ftdc.currSchema = getSchema(datum.Data)
		writeSchema(ftdc.currSchema, toWrite)

		// Update the `outputGenerationId` to reflect the new schema.
		ftdc.outputGenerationId = datum.generationId

		data := flatten(datum, ftdc.currSchema.mapOrder)
		// Write the new data point to disk. When schema changes, we do not do any diffing. We write
		// a raw value for each metric.
		writeDatum(datum.Time, nil, data, toWrite, ftdc)
		ftdc.prevFlatData = data

		return nil
	}

	// The input `datum` is for the same schema as the prior datum. Flatten the values and write a
	// datum entry diffed against the `prevFlatData`.
	data := flatten(datum, ftdc.currSchema.mapOrder)
	writeDatum(datum.Time, ftdc.prevFlatData, data, toWrite, ftdc)
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
	ftdc.currOutputFile, err = os.Create("./viam-server-custom.ftdc")
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
