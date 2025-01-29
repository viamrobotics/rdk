package ftdc

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"

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
func writeSchema(schema *schema, output io.Writer) error {
	// New schema byte
	if _, err := output.Write([]byte{0x1}); err != nil {
		return fmt.Errorf("Error writing schema bit: %w", err)
	}

	encoder := json.NewEncoder(output)
	// `json.Encoder.Encode` assumes it convenient to append a newline character at the very
	// end. This newline has been included in the format specification. Parsers must read over that.
	if err := encoder.Encode(schema.fieldOrder); err != nil {
		return fmt.Errorf("Error writing schema: %w", err)
	}

	return nil
}

// writeDatum writes out the three data format parts associated with every reading: the time, the
// diff bits and the values. See `writeSchema` for a full decsription of the file format.
//
// This may only call this when `len(curr) > 0`. `prev` may be nil or empty. If `prev` is non-empty,
// `len(prev)` must equal `len(curr)`.
func writeDatum(time int64, prev, curr []float32, output io.Writer) error {
	numPts := len(curr)
	if len(prev) != 0 && numPts != len(prev) {
		//nolint:stylecheck
		return fmt.Errorf("Bad input sizes. Prev: %v Curr: %v", len(prev), len(curr))
	}

	// We first have to calculate the "diff bits".
	diffs := make([]float32, numPts)
	if len(prev) == 0 {
		// If there was no previous reading to compare against, assume it was all zeroes.
		copy(diffs, curr)
	} else {
		for idx := range curr {
			// We record the difference in the current reading compared to the previous reading for
			// each metric.
			diffs[idx] = curr[idx] - prev[idx]
		}
	}

	// One bit per datapoint. And one leading bit for the "metric document identifier" bit.
	numBits := numPts + 1

	// Some math magic to calculate the number of bytes required to write out `numBits`. Use the
	// following examples to build an understanding of how this works:
	//
	// If numBits < 8 then numBytes = 1,
	// ElseIf numBits < 16 then numBytes = 2,
	// ElseIf numBits < 24 then numBytes = 3, etc...
	numBytes := 1 + ((numBits - 1) / 8)

	// Now that we've calculated the diffs, and know how many bytes we need to represent the diff
	// (and metric document identifier), we create a byte array to bitwise-or into.
	diffBits := make([]byte, numBytes)
	for diffIdx := range diffs {
		// Leading bit is the "schema change" bit. For a "data header", the "schema bit" value is 0.
		// Start "diff bits" at index 1.
		bitIdx := diffIdx + 1
		byteIdx := bitIdx / 8
		bitOffset := bitIdx % 8

		// When using floating point numbers, it's customary to avoid `== 0` and `!= 0`. And instead
		// compare to some small (epsilon) value.
		if math.Abs(float64(diffs[diffIdx])) > epsilon {
			diffBits[byteIdx] |= (1 << bitOffset)
		}
	}

	if _, err := output.Write(diffBits); err != nil {
		return fmt.Errorf("Error writing diff bits: %w", err)
	}

	// Write time between diff bits and values.
	if err := binary.Write(output, binary.BigEndian, time); err != nil {
		return fmt.Errorf("Error writing time: %w", err)
	}

	// Write out values for metrics that changed across reading.
	for idx, diff := range diffs {
		if math.Abs(float64(diff)) > epsilon {
			if err := binary.Write(output, binary.BigEndian, curr[idx]); err != nil {
				return fmt.Errorf("Error writing values: %w", err)
			}
		}
	}

	return nil
}

var errNotStruct = errors.New("stats object is not a struct")

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
		for inp.Kind() == reflect.Pointer || inp.Kind() == reflect.Interface {
			if inp.IsNil() {
				return inp
			}

			inp = inp.Elem()
		}
		return inp
	}

	rVal := flattenPtr(item)
	if rVal.Kind() != reflect.Struct {
		return []float32{}, nil
	}

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
		case rField.Kind() == reflect.Bool:
			if rField.Bool() {
				numbers = append(numbers, 1)
			} else {
				numbers = append(numbers, 0)
			}
		case rField.Kind() == reflect.Struct ||
			rField.Kind() == reflect.Pointer ||
			rField.Kind() == reflect.Interface:
			subNumbers, err := flattenStruct(rField)
			if err != nil {
				return nil, err
			}
			numbers = append(numbers, subNumbers...)
		case isNumeric(rField.Kind()):
			//nolint:stylecheck
			return nil, fmt.Errorf("A numeric type was forgotten to be included. Kind: %v", rField.Kind())
		default:
			// Getting the keys for a structure will ignore these types. Such as the antagonistic
			// `channel`, or `string`. We follow suit in ignoring these types.
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
// Nested structures will walk and return a "dot delimited" name. E.g:
//
//	type ParentFoo {
//	  Healthy Bool
//	  FooField Foo
//	}
//
// Will return `["Healthy", "FooField.PowerPct", "FooField.Pos"]`.
func getFieldsForStruct(item reflect.Value) ([]string, error) {
	flattenPtr := func(inp reflect.Value) reflect.Value {
		for inp.Kind() == reflect.Pointer || inp.Kind() == reflect.Interface {
			if inp.IsNil() {
				return inp
			}
			inp = inp.Elem()
		}
		return inp
	}

	rVal := flattenPtr(item)
	if rVal.Kind() != reflect.Struct {
		return nil, errNotStruct
	}

	rType := rVal.Type()
	var fields []string
	for memberIdx := 0; memberIdx < rVal.NumField(); memberIdx++ {
		structField := rType.Field(memberIdx)
		fieldVal := rVal.Field(memberIdx)
		derefedVal := flattenPtr(fieldVal)
		if isNumeric(derefedVal.Kind()) {
			fields = append(fields, structField.Name)
			continue
		}

		if derefedVal.Kind() == reflect.Struct {
			subFields, err := getFieldsForStruct(derefedVal)
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

	for name, stats := range data {
		mapOrder = append(mapOrder, name)
		fieldsForItem, err := getFieldsForStruct(reflect.ValueOf(stats))
		if err != nil {
			return nil, &schemaError{name, err}
		}

		for _, field := range fieldsForItem {
			// We insert a `.` into every metric/field name we get a recording for. This property is
			// assumed elsewhere.
			fields = append(fields, fmt.Sprintf("%v.%v", name, field))
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
func flatten(datum datum, schema *schema) ([]float32, error) {
	ret := make([]float32, 0, len(schema.fieldOrder))

	for _, key := range schema.mapOrder {
		// Walk over the datum in `mapOrder` to ensure we gather values in the order consistent with
		// the current schema.
		stats, exists := datum.Data[key]
		if !exists {
			//nolint
			return nil, fmt.Errorf("Missing statser name. Name: %v", key)
		}

		numbers, err := flattenStruct(reflect.ValueOf(stats))
		if err != nil {
			return nil, err
		}
		ret = append(ret, numbers...)
	}

	return ret, nil
}

// FlatDatum has the same information as a `datum`, but without the arbitrarily nested `Data`
// map. Using dots to join keys as per the disk format. So where a `Data` map might be:
//
// { "webrtc": { "connections": 5, "bytesSent": 8096 } }
//
// A `FlatDatum` would be:
//
// [ Reading{"webrtc.connections", 5}, Reading{"webrtc.bytesSent", 8096} ].
type FlatDatum struct {
	// Time is a 64 bit integer representing nanoseconds since the epoch.
	Time     int64
	Readings []Reading
}

// Reading is a "fully qualified" metric name paired with a value.
type Reading struct {
	MetricName string
	Value      float32
}

// ConvertedTime turns the `Time` int64 value in nanoseconds since the epoch into a `time.Time`
// object in the UTC timezone.
func (flatDatum *FlatDatum) ConvertedTime() time.Time {
	nanosPerSecond := int64(1_000_000_000)
	seconds := flatDatum.Time / nanosPerSecond
	nanos := flatDatum.Time % nanosPerSecond
	return time.Unix(seconds, nanos).UTC()
}

// asDatum converts the flat array of `Reading`s into a `datum` object with a two layer `Data` map.
func (flatDatum *FlatDatum) asDatum() datum {
	var metricNames []string
	var values []float32
	for _, reading := range flatDatum.Readings {
		metricNames = append(metricNames, reading.MetricName)
		values = append(values, reading.Value)
	}

	ret := datum{
		Time: flatDatum.Time,
		Data: hydrate(metricNames, values),
	}

	return ret
}

// Parse reads the entire contents from `rawReader` and returns a list of `Datum`. If an error
// occurs, the []Datum parsed up until the place of the error will be returned, in addition to a
// non-nil error.
func Parse(rawReader io.Reader) ([]FlatDatum, error) {
	logger := logging.NewLogger("")
	logger.SetLevel(logging.ERROR)

	return ParseWithLogger(rawReader, logger)
}

// ParseWithLogger parses with a logger for output.
func ParseWithLogger(rawReader io.Reader, logger logging.Logger) ([]FlatDatum, error) {
	ret := make([]FlatDatum, 0)

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
		logger.Debugw("Diff bits",
			"changedFieldIndexes", diffedFieldsIndexes,
			"changedFieldNames", schema.FieldNamesForIndexes(diffedFieldsIndexes))

		// The next eight bytes after the diff bits is the time in nanoseconds since the 1970 epoch.
		var dataTime int64
		if err = binary.Read(reader, binary.BigEndian, &dataTime); err != nil {
			logger.Debugw("Error reading time", "error", err)
			return ret, err
		}
		logger.Debugw("Read time", "time", dataTime, "seconds", dataTime/1e9)

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
		ret = append(ret, FlatDatum{
			Time:     dataTime,
			Readings: schema.Zip(data),
		})
		logger.Debugw("Hydrated data", "data", ret[len(ret)-1].Readings)
	}

	return ret, nil
}

func flatDatumsToDatums(inp []FlatDatum) []datum {
	ret := make([]datum, len(inp))
	for idx, flatDatum := range inp {
		ret[idx] = flatDatum.asDatum()
	}

	return ret
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
// names. Returning a two layer map. The top-level map is keyed on a "system" (corresponding to an
// `FTDC.Add` call) and the lower level map corresponds to the keys and values struct a `Stats` call
// returns. There's no business requirement that nested structures only be represented as two
// layers. It's just this way for simplicity of the type system and implementation. And right now,
// only tests are concerned with the advantage of `Hydrate`ing data.
func (schema *schema) Hydrate(data []float32) map[string]any {
	return hydrate(schema.fieldOrder, data)
}

func hydrate(fullyQualifiedMetricNames []string, values []float32) map[string]any {
	ret := make(map[string]any)
	for fieldIdx, metricName := range fullyQualifiedMetricNames {
		//nolint:gocritic
		statsName := metricName[:strings.Index(metricName, ".")]
		metricName = metricName[strings.Index(metricName, ".")+1:]

		if mp, exists := ret[statsName]; exists {
			mp.(map[string]float32)[metricName] = values[fieldIdx]
		} else {
			mp := map[string]float32{
				metricName: values[fieldIdx],
			}
			ret[statsName] = mp
		}
	}

	return ret
}

// Zip walks the schema and input `data` as parallel arrays and pairs up the metric names with their
// corresponding reading. The metric names are "fully qualified" with their statser "system"
// name. Using dots as delimiters representing the original structure.
func (schema *schema) Zip(data []float32) []Reading {
	ret := make([]Reading, len(schema.fieldOrder))
	for fieldIdx, metricName := range schema.fieldOrder {
		ret[fieldIdx] = Reading{metricName, data[fieldIdx]}
	}

	return ret
}

// FieldNamesForIndexes maps the integers to their string form as defined in the schema. This is
// useful for creating human consumable output.
func (schema *schema) FieldNamesForIndexes(fieldIdxs []int) []string {
	ret := make([]string, len(fieldIdxs))
	for idx, fieldIdx := range fieldIdxs {
		ret[idx] = schema.fieldOrder[fieldIdx]
	}

	return ret
}
