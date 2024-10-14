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
