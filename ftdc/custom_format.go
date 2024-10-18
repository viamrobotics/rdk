package ftdc

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"go.viam.com/rdk/logging"
)

// epsilon is a small value for determining whether a float is 0.0.
const epsilon = 1e-9

type schema struct {
	// A `Datum`s data is a map[string]any. Even if two datum's have maps with the same keys, we do
	// not assume ranging over the map will yield the same order. Thus we explicitly write down an
	// order to walk the map of new data values.
	//
	// mapOrder is only used for writing out new data points.
	mapOrder []string

	// fieldOrder is flattened list of strings representing individual metrics. Fields use a
	// dot-notation to represent structure/nesting. E.g: "leftMotor.PowerPct".
	fieldOrder []string
}

// writeSchema writes down names for metrics in the form of a json array. All subsequent calls to
// `writeDatum` will assume this "header" representation until the next call to `writeSchema`. A
// full description of the file format is recorded in `doc.go`.
func writeSchema(schema *schema, output io.Writer) {
	// New schema byte
	if _, err := output.Write([]byte{0x1}); err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(output)
	// `json.Encoder.Encode` assumes it convenient to append a newline character at the very
	// end. This newline has been included in the format specification. Parsers must read over that.
	if err := encoder.Encode(schema.fieldOrder); err != nil {
		panic(err)
	}
}

// writeDatum writes out the three data format parts associated with every reading: the time, the
// diff bits and the values. See `writeSchema` for a full decsription of the file format.
//
// This may only call this when `len(curr) > 0`. `prev` may be nil or empty. If `prev` is non-empty,
// `len(prev)` must equal `len(curr)`.
func writeDatum(time int64, prev, curr []float32, output io.Writer) {
	numPts := len(curr)
	if numPts == 0 {
		// Handled by the caller.
		panic("No points?")
	}

	if len(prev) != 0 && numPts != len(prev) {
		// Handled by the caller.
		panic(fmt.Sprintf("Bad input sizes. Prev: %v Curr: %v", len(prev), len(curr)))
	}

	// We first have to calculate the "diff bits".
	diffs := make([]float32, numPts)
	if len(prev) == 0 {
		// If there was no previous reading to compare against, assume it was all zeroes.
		copy(diffs, curr)
	} else {
		for idx := range curr {
			diffs[idx] = curr[idx] - prev[idx]
		}
	}

	// One bit per datapoint. And one leading bit for the "metric document identifier" bit.
	numBits := numPts + 1

	// If numBits < 8 then numBytes = 1,
	// ElseIf numBits < 16 then numBytes = 2,
	// ElseIf numBits < 24 then numBytes = 3, etc...
	numBytes := 1 + ((numBits - 1) / 8)

	// Now that we've calculated the diffs, and know how many bytes we need to represent the diff
	// (and metric document identifier), we create a byte array to bitwise-or into.
	diffBits := make([]byte, numBytes)
	for diffIdx := range diffs {
		// Leading bit is the "schema change" bit with a value of `0`. For a "data header", the
		// "schema bit" value is 0.  Start "diff bits" at index 1.
		bitIdx := diffIdx + 1
		byteIdx := bitIdx / 8
		bitOffset := bitIdx % 8

		// When using floating point numbers, it's customary to avoid `== 0` and `!= 0`. And instead
		// compare to some small (epsilon) value.
		if diffs[diffIdx] > epsilon {
			diffBits[byteIdx] |= (1 << bitOffset)
		}
	}

	if _, err := output.Write(diffBits); err != nil {
		panic(err)
	}

	// Write time between diff bits and values.
	if err := binary.Write(output, binary.BigEndian, time); err != nil {
		panic(err)
	}

	// Write out values for metrics that changed across reading.
	for idx, diff := range diffs {
		if diff > epsilon {
			if err := binary.Write(output, binary.BigEndian, curr[idx]); err != nil {
				panic(err)
			}
		}
	}
}

var notStructError = errors.New("Stats object is not a struct")

func isNumeric(kind reflect.Kind) bool {
	return kind == reflect.Bool ||
		kind == reflect.Int ||
		kind == reflect.Int8 || kind == reflect.Int16 || kind == reflect.Int32 || kind == reflect.Int64 ||
		kind == reflect.Uint ||
		kind == reflect.Uint8 || kind == reflect.Uint16 || kind == reflect.Uint32 || kind == reflect.Uint64 ||
		kind == reflect.Float32 || kind == reflect.Float64
}

func flattenStruct(item reflect.Value) ([]float32, error) {
	flattenPtr := func(inp reflect.Value) reflect.Value {
		for inp.Kind() == reflect.Pointer {
			inp = inp.Elem()
		}
		return inp
	}

	rVal := flattenPtr(item)

	var numbers []float32
	// Use reflection to walk the member fields of an individual set of metric readings. We rely
	// on reflection always walking fields in the same order.
	//
	// Note, while reflection in this way isn't intrinsically expensive, it is making more small
	// function calls and allocations than some more raw alternatives. For example, we can have
	// the "schema" keep a (field, offset, type) index and we instead access get a single unsafe
	// pointer to each structure and walk out index to pull out the relevant numbers.
	for memberIdx := 0; memberIdx < rVal.NumField(); memberIdx++ {
		rField := flattenPtr(rVal.Field(memberIdx))
		switch {
		case rField.CanUint():
			numbers = append(numbers, float32(rField.Uint()))
		case rField.CanInt():
			numbers = append(numbers, float32(rField.Int()))
		case rField.CanFloat():
			numbers = append(numbers, float32(rField.Float()))
		case rField.Kind() == reflect.Struct:
			subNumbers, err := flattenStruct(rField)
			if err != nil {
				return nil, err
			}
			numbers = append(numbers, subNumbers...)
		default:
			// Embedded structs? Just grab a global logger for now. A second pass will better
			// validate inputs/remove limitations. And thread through a proper logger if still
			// necessary.
			logging.Global().Warn("Bad number type. Type:", rField.Type())
			// Ignore via writing a 0 and continue.
			numbers = append(numbers, 0)
		}
	}

	return numbers, nil
}

// getFieldsForStruct returns the (flattened) list of strings for a metric structure. For example the
// following type:
//
//	type Foo {
//	  PowerPct float64
//	  Pos      int
//	}
//
// Will return `["PowerPct", "Pos"]`.
//
// The function right now does not recursively walk data structures. We assume for now that the
// caller will only feed "already flat" structures into FTDC. Later commits will better validate
// input and remove limitations.
func getFieldsForStruct(item reflect.Type) ([]string, error) {
	flattenPtr := func(inp reflect.Type) reflect.Type {
		for inp.Kind() == reflect.Pointer {
			inp = inp.Elem()
		}
		return inp
	}

	rType := flattenPtr(item)
	if rType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w Type: %T", notStructError, item)
	}

	var fields []string
	for memberIdx := 0; memberIdx < rType.NumField(); memberIdx++ {
		structField := rType.Field(memberIdx)
		fieldType := flattenPtr(structField.Type)
		if isNumeric(fieldType.Kind()) {
			fields = append(fields, structField.Name)
			continue
		}

		if fieldType.Kind() == reflect.Struct {
			subFields, err := getFieldsForStruct(fieldType)
			if err != nil {
				return nil, err
			}

			for _, subField := range subFields {
				fields = append(fields, fmt.Sprintf("%v.%v", structField.Name, subField))
			}
		}
	}

	return fields, nil
}

type schemaError struct {
	statserName string
	err         error
}

func (err *schemaError) Error() string {
	return fmt.Sprintf("SchemaError: %s StatserName: %s", err.err.Error(), err.statserName)
}

// getSchema returns a schema for a full FTDC datum. It immortalizes two properties:
// - mapOrder: The order to iterate future input `map[string]any` data.
// - fieldOrder: The order diff bits and values are to be written in.
//
// For correctness, it must be the case that the `mapOrder` and `fieldOrder` are consistent. I.e: if
// the `mapOrder` is `A` then `B`, the `fieldOrder` must list all of the fields of `A` first,
// followed by all the fields of `B`.
func getSchema(data map[string]any) (*schema, *schemaError) {
	var mapOrder []string
	var fields []string

	for key, stats := range data {
		mapOrder = append(mapOrder, key)
		fieldsForItem, err := getFieldsForStruct(reflect.TypeOf(stats))
		if err != nil {
			return nil, &schemaError{key, err}
		}

		for _, field := range fieldsForItem {
			// We insert a `.` into every metric/field name we get a recording for. This property is
			// assumed elsewhere.
			fields = append(fields, fmt.Sprintf("%v.%v", key, field))
		}
	}

	return &schema{
		mapOrder:   mapOrder,
		fieldOrder: fields,
	}, nil
}

// flatten takes an input `Datum` and a `mapOrder` from the current `Schema` and returns a list of
// `float32`s representing the readings. Similar to `getFieldsForItem`, there are constraints on
// input data shape that this code currently does not validate.
func flatten(datum datum, schema *schema) []float32 {
	ret := make([]float32, 0, len(schema.fieldOrder))

	for _, key := range schema.mapOrder {
		// Walk over the datum in `mapOrder` to ensure we gather values in the order consistent with
		// the current schema.
		stats, exists := datum.Data[key]
		if !exists {
			logging.Global().Warn("Missing data. Need to know how much to skip in the output float32")
			return nil
		}

		numbers, err := flattenStruct(reflect.ValueOf(stats))
		if err != nil {
			panic(err)
		}
		for _, number := range numbers {
			ret = append(ret, number)
		}
	}

	return ret
}

func parse(rawReader io.Reader) ([]datum, error) {
	logger := logging.NewLogger("")
	logger.SetLevel(logging.ERROR)

	return parseWithLogger(rawReader, logger)
}

// parse reads the entire contents from `rawReader` and returns a list of `Datum`. If an error
// occurs, the []Datum parsed up until the place of the error will be returned, in addition to a
// non-nil error.
func parseWithLogger(rawReader io.Reader, logger logging.Logger) ([]datum, error) {
	ret := make([]datum, 0)

	// prevValues are the previous values used for producing the diff bits. This is overwritten when
	// a new metrics reading is made. and nilled out when the schema changes.
	var prevValues []float32

	// bufio's Reader allows for peeking and potentially better control over how much data to read
	// from disk at a time.
	reader := bufio.NewReader(rawReader)
	var schema *schema
	for {
		peek, err := reader.Peek(1)
		if err != nil {
			logger.Debugw("Beginning peek error", "error", err)
			if errors.Is(err, io.EOF) {
				break
			}

			return ret, err
		}

		// If the first bit of the first byte is `1`, the next block of data is a schema
		// document. The rest of the bits (for diffing) are irrelevant and will be zero. Thus the
		// check against `0x1`.
		if peek[0] == 0x1 {
			//nolint
			//
			// Justifying the nolint: if `Peek(1)` does not return an error, `ReadByte` must not be
			// able to return an error.
			//
			// Consume the 0x1 byte.
			_, _ = reader.ReadByte()

			// Read json and position the cursor at the next FTDC document. The JSON reader may
			// "over-read", so `readSchema` assembles a new reader positioned at the right spot. The
			// schema bytes themselves are expected to be a list of strings, e.g: `["metricName1",
			// "metricName2"]`.
			schema, reader = readSchema(reader)
			logger.Debugw("Schema bit", "parsedSchema", schema)

			// We cannot diff against values from the old schema.
			prevValues = nil
			continue
		} else if schema == nil {
			return nil, errors.New("first byte of FTDC data must be the magic 0x1 representing a new schema")
		}

		// This FTDC document is a metric document. Read the "diff bits" that describe which metrics
		// have changed since the prior metric document. Note, the reader is positioned on the
		// "packed byte" where the first bit is not a diff bit. `readDiffBits` must account for
		// that.
		diffedFieldsIndexes := readDiffBits(reader, schema)
		logger.Debugw("Diff bits", "changedFields", diffedFieldsIndexes)

		// The next eight bytes after the diff bits is the time in nanoseconds since the 1970 epoch.
		var dataTime int64
		if err = binary.Read(reader, binary.BigEndian, &dataTime); err != nil {
			logger.Debugw("Error reading time", "error", err)
			return ret, err
		}
		logger.Debugw("Read time", "time", dataTime)

		// Read the payload. There will be one float32 value for each diff bit set to `1`, i.e:
		// `len(diffedFields)`.
		data, err := readData(reader, schema, diffedFieldsIndexes, prevValues)
		if err != nil {
			logger.Debugw("Error reading data", "error", err)
			return ret, err
		}
		logger.Debugw("Read data", "data", data)

		// The old `prevValues` is no longer needed. Set the `prevValues` to the new hydrated
		// `data`.
		prevValues = data

		// Construct a `Datum` that hydrates/merged the full set of float32 metrics with the metric
		// names as written in the most recent schema document.
		ret = append(ret, datum{
			Time: dataTime,
			Data: schema.Hydrate(data),
		})
		logger.Debugw("Hydrated data", "data", ret[len(ret)-1].Data)
	}

	return ret, nil
}

// readSchema expects to be positioned on the beginning of a json list data type (a left square
// bracket `[`) and consumes bytes until that list (of strings) is complete.
//
// readSchema returns the described schema and a new reader that's positioned on the first byte of
// the next ftdc document.
func readSchema(reader *bufio.Reader) (*schema, *bufio.Reader) {
	decoder := json.NewDecoder(reader)
	if !decoder.More() {
		panic("no json")
	}

	// While the FTDC metrics persisted has structure, we flatten the metric names into a single
	// list of strings. We use dots (`.`) to signify nesting. Metric names with dots will result in
	// an ambiguous parsing.
	var fields []string
	if err := decoder.Decode(&fields); err != nil {
		panic(err)
	}

	// The JSON decoder can consume bytes from the input `reader` that are beyond the end of the
	// json data. Those unused bytes are accessible via the `decoder.Buffered` call. Assemble a new
	// reader with the remaining bytes from the decoder, followed by the remaining bytes from the
	// input `reader`.
	retReader := bufio.NewReader(io.MultiReader(decoder.Buffered(), reader))

	// Consume a newline character. The JSON Encoder will unconditionally append a newline that the
	// JSON decoder will not* consume. This is a sharp edge of the Golang JSON API.
	ch, err := retReader.ReadByte()
	if ch != '\n' || err != nil {
		panic("not a newline")
	}

	// We now have fields, e.g: ["metric1.Foo", "metric1.Bar", "metric2.Foo"]. The `mapOrder` should
	// be ["metric1", "metric2"]. It's undefined behavior for a `mapOrder` metric name key to be
	// split around a different metric name. E.g: ["metric1.Alpha", "metric2.Beta",
	// "metric1.Gamma"].
	var mapOrder []string
	metricNameSet := make(map[string]struct{})
	for _, field := range fields {
		//nolint:gocritic
		metricName := field[:strings.Index(field, ".")]
		if _, exists := metricNameSet[metricName]; !exists {
			mapOrder = append(mapOrder, metricName)
			metricNameSet[metricName] = struct{}{}
		}
	}

	return &schema{
		fieldOrder: fields,
		mapOrder:   mapOrder,
	}, retReader
}

// readDiffBits returns a list of integers that index into the `Schema` representing the set of
// metrics that have changed. Note that the first byte of the input reader is "packed" with the
// schema bit. Thus the first byte can represent 7 metrics and the remaining bytes can each
// represent 8 metrics.
func readDiffBits(reader *bufio.Reader, schema *schema) []int {
	// 1 diff bit per metric + 1 bit for the packed "schema bit".
	numBits := len(schema.fieldOrder) + 1

	// If numBits < 8 then numBytes = 1,
	// ElseIf numBits < 16 then numBytes = 2,
	// ElseIf numBits < 24 then numBytes = 3, etc...
	numBytes := 1 + ((numBits - 1) / 8)

	diffBytes := make([]byte, numBytes)
	_, err := io.ReadFull(reader, diffBytes)
	if err != nil {
		panic(err)
	}

	var ret []int
	for fieldIdx := 0; fieldIdx < len(schema.fieldOrder); fieldIdx++ {
		// The 0th metric is addressed via the 1st bit. This is due to the shifting caused by the
		// packed schema bit.
		bitIdx := fieldIdx + 1

		// Divide by eight bits per byte to find the offset into the `diffBytes` array.
		diffByteOffset := bitIdx / 8

		// And take the remainder to address the indidivual bit.
		bitOffset := bitIdx % 8

		if bitValue := diffBytes[diffByteOffset] & (1 << bitOffset); bitValue > 0 {
			// The bit is `1`, add the correlated `fieldIdx` to the return value of metrics that
			// changed.
			ret = append(ret, fieldIdx)
		}
	}

	return ret
}

// readData returns the "hydrated" metrics for a data reading. For example, if there are ten metrics
// and none of them changed, the returned []float32 will be identical to `prevValues`. `prevValues`
// is the post-hydration list and consequently matches the `schema.fieldOrder` size.
func readData(reader *bufio.Reader, schema *schema, diffedFields []int, prevValues []float32) ([]float32, error) {
	if prevValues != nil && len(prevValues) != len(schema.fieldOrder) {
		//nolint
		return nil, fmt.Errorf("Parser error. Mismatched `prevValues` and schema size. PrevValues: %d Schema: %d",
			len(prevValues), len(schema.fieldOrder))
	}

	var ret []float32

	// For each metric in the schema:
	for dataIdx := 0; dataIdx < len(schema.fieldOrder); dataIdx++ {
		// See if the metric index exists in the `diffedFields` array.
		diffFromPrev := false
		for _, fieldIdx := range diffedFields {
			if dataIdx == fieldIdx {
				diffFromPrev = true
				break
			}
		}

		if diffFromPrev {
			// If the metric existed, it's because there was a fresh reading in the input
			// `reader`. Parse the value from the `reader`.
			ret = append(ret, 0.0)
			if err := binary.Read(reader, binary.BigEndian, &ret[dataIdx]); err != nil {
				return nil, err
			}
		} else {
			// Otherwise, the metric did not change. Use the previous value.
			if prevValues == nil {
				// The parser and writer agree that the `prevValues` is `0.0` for all metrics
				// following a schema change.
				ret = append(ret, 0.0)
			} else {
				ret = append(ret, prevValues[dataIdx])
			}
		}
	}

	return ret, nil
}

// Hydrate takes the input []float slice of `data` and matches those to their corresponding metric
// names. Returning a.
func (schema *schema) Hydrate(data []float32) map[string]any {
	ret := make(map[string]any)
	for fieldIdx, metricName := range schema.fieldOrder {
		//nolint:gocritic
		statsName := metricName[:strings.Index(metricName, ".")]
		metricName = metricName[strings.Index(metricName, ".")+1:]

		if mp, exists := ret[statsName]; exists {
			mp.(map[string]float32)[metricName] = data[fieldIdx]
		} else {
			mp := map[string]float32{
				metricName: data[fieldIdx],
			}
			ret[statsName] = mp
		}
	}
	return ret
}
