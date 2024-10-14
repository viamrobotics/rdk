package ftdc

import (
	"bufio"
	"bytes"
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

type Schema struct {
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
// full description of the file format follows:
//
// To build intuition for the file format, let's start with a simple example input being serialized
// with a simple format. Let's say we have two data points (two "datum"):
//
// Datum1 = {time: 123, motor: {powerPct: 0.2, pos: 5000}, gps: {lat: 40.7128, long: -74.0060}}
// Datum2 = {time: 124, motor: {powerPct: 0.2, pos: 5001}, gps: {lat: 40.7128, long: -74.0061}}
//
// We can serialize those two data points as pure json into a file. One after the other:
//
// {"time", 123, "motor": {"powerPct": 0.2, "pos": 5000}, gps: {"lat": 40.7128, "long": -74.0060}}\n
// {"time", 124, "motor": {"powerPct": 0.2, "pos": 5001}, gps: {"lat": 40.7128, "long": -74.0061}}
//
// While json can be a bit wasteful in its own right, there are two properties of the data we want
// to collect that allow us greatly compress data:
//   - The metrics we're recording (e.g: `motor.powerPct`, `gps.lat`, ...) are often exactly the same
//     between the readings.
//   - Many of the values we're recording (e.g: the `0.2`) do not change between readings.
//
// Thus, the two tricks employed are to:
// - Only write metric names when things change.
// - Minimize how much space is used to write out a value that hasn't changed.
//
// Using a pseudo EBNF notation, an FTDC file is:
// FTDC = ftdc_doc*
//
// ftdc_doc = schema | metric
//
// schema =
//
//	schema_identifier : 0x01 (a full byte of value 1)
//	schema : <array of strings serialized as JSON, including a trailing \n(0xa)>
//
// metric_reading =
//
//	metric_identifier : 0b0 (a single bit of value 0)
//	diff_bit : bit* + byte alignment padding
//	time: int64 <Golang: `time.Now().Unix()`. Nanoseconds since the 1970 epoch.>
//	values : float32*
//
// Because a metric reading is not meaningful without a schema, a file will always start with a
// schema document. The first byte of a schema document is 0x01 followed by a JSON list of strings
// and a UNIX newline (0x0a). The JSON strings are "flattened" using a dot to concatenate the map
// key with the metric name. E.g:
//
// 0000 0001 ["motor.powerPct", "motor.pos", "gps.lat", "gps.long"]\n
// 7       0
//
// Following a schema document will be 0 or more metric documents. A metric reading has one diff bit
// per reading (i.e: the "size" of the schema). In our example, that's four bits.  A diff bit is set
// to `0` if the new reading for a given metric is same as the immediately prior reading. A diff bit
// is set to `1` if the readings differ. Each reading that differs will have one 32-bit float value
// written as part of this metric reading document. A metric can contain numeric values that are not
// 32-bit floats. This format is lossy. We don't expect to need the fully ~7 (`math.log10(2**23)`)
// significant figures of precision a float32 provides.
//
// The diff bits immediately follow the metric bit value of 0. In other words, the first byte
// containing the metric bit is packed/merged with the first (up to seven) diff bits. The remaining
// diff bytes each contain up to eight diff bits. The last diff byte may not have eight metrics to
// fill out a full byte. A full byte will be written none the less for alignment. The higher bits
// will be wasted.
//
// Note that the number of diff bytes to write/read is a function of the number of fields in the
// most recent schema.
//
// The initial metric reading immediately following a schema document does not have a diff to
// compare against. In this case the format assumes a prior value of `0` for each metric. To
// continue our example, let's consider the first metric reading document.
//
// For clarity, the diff bits are described in a binary representation detailing the exact
// bits. Where bit-0 is the metric bit (defined as 0) and the diff bits 1 through 4 (inclusive) are
// set to 1. And the numbers written for time/readings are annotated. All numbers are big-endian
// encoded for no good/benchmarked reason.
//
// 0001 1110 <64bit time> <32bit "motor.powerPct"> <32bit "motor.pos"> <32bit "gps.lat"> <32bit "gps.long">
// 7       0
//
// Now let's see what the second datum will look like. First we calculate which metric readings have
// changed:
// - motor.powerPct: 0.2 -> 0.2 (no diff)
// - motor.pos: 5000 -> 5001 (diff)
// - gps.lat: 40.7128 -> 40.7128 (no diff)
// - gps.long: -74.0060 -> -74.0061 (diff)
//
// Giving us the following encoding:
//
// 0001 0100 <64bit time> <32bit "motor.pos"> <32bit "gps.long">
// 7       0
//
// If we now get a new datum that changes the schema (e.g: remove the motor), we can write out a new
// schema document:
//
// 0000 0001 ["gps.lat", "gps.long"]\n
// 7       0
//
// To maybe illuminate the necessity of the schema/metric identifier, when a parser is about to read
// a new document, it needs to know whether:
// - to interpret the bytes as json for a new schema, or
// - as values for a new metric reading
//
// A parser can read a single byte and look at the least significant bit to determine which path to
// take.
func writeSchema(schema *Schema, output io.Writer) {
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
		for idx := range curr {
			diffs[idx] = curr[idx]
		}
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

// getFieldsForItem returns the (flattened) list of strings for a metric structure. For example the
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
func getFieldsForItem(item any) []string {
	var fields []string
	rType := reflect.TypeOf(item)
	if val := reflect.ValueOf(item); val.Kind() == reflect.Pointer {
		rType = val.Elem().Type()
	}

	for memberIdx := 0; memberIdx < rType.NumField(); memberIdx++ {
		fields = append(fields, rType.Field(memberIdx).Name)
	}

	return fields
}

// getSchema returns a schema for a full FTDC datum. It immortalizes two properties:
// - mapOrder: The order to iterate future input `map[string]any` data.
// - fieldOrder: The order diff bits and values are to be written in.
//
// For correctness, it must be the case that the `mapOrder` and `fieldOrder` are consistent. I.e: if
// the `mapOrder` is `A` then `B`, the `fieldOrder` must list all of the fields of `A` first,
// followed by all the fields of `B`.
func getSchema(data map[string]any) *Schema {
	var mapOrder []string
	var fields []string

	for key, stats := range data {
		mapOrder = append(mapOrder, key)
		for _, field := range getFieldsForItem(stats) {
			// We insert a `.` into every metric/field name we get a recording for. This property is
			// assumed elsewhere.
			fields = append(fields, fmt.Sprintf("%v.%v", key, field))
		}
	}

	return &Schema{
		mapOrder:   mapOrder,
		fieldOrder: fields,
	}
}

// flatten takes an input `Datum` and a `mapOrder` from the current `Schema` and returns a list of
// `float32`s representing the readings. Similar to `getFieldsForItem`, there are constraints on
// input data shape that this code currently does not validate.
func flatten(datum Datum, mapOrder []string) []float32 {
	ret := make([]float32, 0, 10*len(mapOrder))

	for _, key := range mapOrder {
		// Walk over the datum in `mapOrder` to ensure we gather values in the order consistent with
		// the current schema.
		stats, exists := datum.Data[key]
		if !exists {
			fmt.Println("Missing data. Need to know how much to skip in the output float32")
			return nil
		}

		rVal := reflect.ValueOf(stats)
		if rVal.Kind() == reflect.Pointer {
			rVal = rVal.Elem()
		}

		// Use reflection to walk the member fields of an individual set of metric readings. We rely
		// on reflection always walking fields in the same order.
		//
		// Note, while reflection in this way isn't intrinsically expensive, it is making more small
		// function calls and allocations than some more raw alternatives. For example, we can have
		// the "schema" keep a (field, offset, type) index and we instead access get a single unsafe
		// pointer to each structure and walk out index to pull out the relevant numbers.
		for memberIdx := 0; memberIdx < rVal.NumField(); memberIdx++ {
			rField := rVal.Field(memberIdx)
			switch {
			case rField.CanInt():
				ret = append(ret, float32(rField.Int()))
			case rField.CanFloat():
				ret = append(ret, float32(rField.Float()))
			default:
				// Embedded structs? Just grab a global logger for now. A second pass will better
				// validate inputs/remove limitations. And thread through a proper logger if still
				// necessary.
				logging.Global().Warn("Bad number type. Type:", rField.Type())
				// Ignore via writing a 0 and continue.
				ret = append(ret, 0)
			}
		}
	}

	return ret
}

// parse reads the entire contents from `rawReader` and returns a list of `Datum`. If an error
// occurs, the Datum parsed up to that point will be returned.
func parse(rawReader io.Reader) ([]Datum, error) {
	ret := make([]Datum, 0)

	// prevValues are the previous values used for producing the diff bits. This is overwritten when
	// a new metrics reading is made. and nilled out when the schema changes.
	var prevValues []float32

	// bufio's Reader allows for peeking and potentially better control over how much data to read
	// from disk at a time.
	reader := bufio.NewReader(rawReader)
	var schema *Schema = nil
	for {
		peek, err := reader.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}

			return ret, err
		}

		// If the first bit of the first byte is `1`, the next block of data is a schema
		// document. The rest of the bits (for diffing) are irrelevant and will be zero. Thus the
		// check against `0x1`.
		if peek[0] == 0x1 {
			// Consume the 0x1 byte.
			_, _ = reader.ReadByte()

			// Read json and position the cursor at the next FTDC document. The JSON reader may
			// "over-read", so `readSchema` assembles a new reader positioned at the right spot. The
			// schema bytes themselves are expected to be a list of strings, e.g: `["metricName1",
			// "metricName2"]`.
			schema, reader = readSchema(reader)

			// We cannot diff against values from the old schema.
			prevValues = nil
			continue
		} else if schema == nil {
			return nil, errors.New("First byte of FTDC data must be the magic one")
		}

		// This FTDC document is a metric document. Read the "diff bits" that describe which metrics
		// have changed since the prior metric document. Note, the reader is positioned on the
		// "packed byte" where the first bit is not a diff bit. `readDiffBits` must account for
		// that.
		diffedFieldsIndexes := readDiffBits(reader, schema)

		// The next eight bytes after the diff bits is the time in nanoseconds since the 1970 epoch.
		var dataTime int64
		if err = binary.Read(reader, binary.BigEndian, &dataTime); err != nil {
			return ret, err
		}

		// Read the payload. There will be one float32 value for each diff bit set to `1`, i.e:
		// `len(diffedFields)`.
		data, err := readData(reader, schema, diffedFieldsIndexes, prevValues)
		if err != nil {
			return ret, err
		}

		// The old `prevValues` is no longer needed. Set the `prevValues` to the new hydrated
		// `data`.
		prevValues = data

		// Construct a `Datum` that hydrates/merged the full set of float32 metrics with the metric
		// names as written in the most recent schema document.
		ret = append(ret, Datum{
			Time: dataTime,
			Data: schema.Hydrate(data),
		})
	}

	return ret, nil
}

// readSchema expects to be positioned on the beginning of a json list data type (a left square
// bracket `[`) and consumes bytes until that list (of strings) is complete.
//
// readSchema returns the described schema and a new reader that's positioned on the first byte of
// the next ftdc document.
func readSchema(reader *bufio.Reader) (*Schema, *bufio.Reader) {
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
	ch, _ := retReader.ReadByte()
	if ch != '\n' {
		panic("not a newline")
	}

	// We now have fields, e.g: ["metric1.Foo", "metric1.Bar", "metric2.Foo"]. The `mapOrder` should
	// be ["metric1", "metric2"]. It's undefined behavior for a `mapOrder` metric name key to be
	// split around a different metric name. E.g: ["metric1.Alpha", "metric2.Beta",
	// "metric1.Gamma"].
	var mapOrder []string
	metricNameSet := make(map[string]struct{})
	for _, field := range fields {
		metricName := field[:strings.Index(field, ".")]
		if _, exists := metricNameSet[metricName]; !exists {
			mapOrder = append(mapOrder, metricName)
			metricNameSet[metricName] = struct{}{}
		}
	}

	return &Schema{
		fieldOrder: fields,
		mapOrder:   mapOrder,
	}, retReader
}

// readDiffBits returns a list of integers that index into the `Schema` representing the set of
// metrics that have changed. Note that the first byte of the input reader is "packed" with the
// schema bit. Thus the first byte can represent 7 metrics and the remaining bytes can each
// represent 8 metrics.
func readDiffBits(reader *bufio.Reader, schema *Schema) []int {
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
func readData(reader *bufio.Reader, schema *Schema, diffedFields []int, prevValues []float32) ([]float32, error) {
	if prevValues != nil && len(prevValues) != len(schema.fieldOrder) {
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
			binary.Read(reader, binary.BigEndian, &ret[dataIdx])
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
// names. Returning a
func (schema *Schema) Hydrate(data []float32) map[string]any {
	ret := make(map[string]any)
	for fieldIdx, metricName := range schema.fieldOrder {
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

// dumpInmemBuffer is a helper for debugging. It can be used for printing the contents of the output
// ftdc file at specific moments in time. To perhaps narrow down where in code an unexpected byte
// was written.
func dumpInmemBuffer(buf *bytes.Buffer) {
	for idx, val := range buf.Bytes() {
		fmt.Printf("%x ", val)
		if idx%8 == 0 {
			fmt.Println()
		}
	}
	fmt.Println()
}
