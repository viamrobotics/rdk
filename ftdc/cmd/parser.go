// main provides a CLI tool for viewing `.ftdc` files emitted by the `viam-server`.
package main

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
)

// graphInfo points to an OS file containing the data points for a single graph. We also record the
// min/max values for scaling purposes when generating plots.
type graphInfo struct {
	file   *os.File
	minVal int64
	maxVal int64
}

// gnuplotWriter organizes all of the output for `gnuplot` to create a graph from FTDC
// data. Notably:
//   - Each graph consists of all the readings for an individual metric. There is one file per metric
//     and each file contains all of the (time, value) points to graph.
//   - There is additionally one "top-level" file. This is the file to call `gnuplot` against. This
//     file contains all layout/styling information. This file will additionally have one line per
//     graph. Each of these lines will contain the OS file path for the above filenames.
//   - Each graph will have the same bounds on the X (Time) axis. Scanning vertically through the
//     graphs at the same horizontal position will show readings as of a common point in time.
type gnuplotWriter struct {
	// metricFiles contains the actual data points to be graphed. The map key is a metric name and
	// the value encapsulates the file with that metrics' datapoints. In addition to other metadata
	// for plotting that data (e.g: min/max values for scaling). A "top level" gnuplot will
	// reference them.
	metricFiles map[string]*graphInfo

	// tempdir is a temporary directory for writing out all of the files that gnuplot will use to
	// create a graph. This is expected to be of the form `/tmp/ftdc<random digits>`.
	tempdir string

	// options is a set of user-input graphOptions. Such as when they ask for a specific time slice
	// of data.
	options graphOptions
}

type kvPair[K, V any] struct {
	Key K
	Val V
}

func sorted[K cmp.Ordered, V any](mp map[K]V) []kvPair[K, V] {
	ret := make([]kvPair[K, V], 0, len(mp))
	for key, val := range mp {
		ret = append(ret, kvPair[K, V]{key, val})
	}

	slices.SortFunc(ret, func(left, right kvPair[K, V]) int {
		if left.Key < right.Key {
			return -1
		}
		if right.Key < left.Key {
			return 1
		}

		return 0
	})

	return ret
}

type graphOptions struct {
	// minTimeSeconds and maxTimeSeconds control which datapoints should render based on their
	// timestamp. The default is all datapoints (minTimeSeconds: 0, maxTimeSeconds: MaxInt64).
	minTimeSeconds int64
	maxTimeSeconds int64

	// hideAllZeroes will omit graphs where all values are 0. Generally speaking, graphs that only
	// contain zeroes won't correlate to anything interesting. A lot of graphs (e.g: component
	// `State`s) are all zeroes. Avoid graphing them to help users navigate the data that's more
	// interesting.
	hideAllZeroes bool

	// vertLinesAtSeconds will draw a vertical line for each element. The items are expected to be
	// in units of seconds since the epoch. These are to represent "events" that are of interest to a
	// user (where the user would like to find correlations in other metrics.)
	vertLinesAtSeconds []int64
}

func defaultGraphOptions() graphOptions {
	return graphOptions{
		minTimeSeconds:     0,
		maxTimeSeconds:     math.MaxInt64,
		hideAllZeroes:      true,
		vertLinesAtSeconds: make([]int64, 0),
	}
}

func nolintPrintln(str ...any) {
	// This is a CLI. It's acceptable to output to stdout.
	//nolint:forbidigo
	fmt.Println(str...)
}

// write is a wrapper for Fprint that panics on any error.
func write(toWrite io.Writer, str string) {
	_, err := fmt.Fprint(toWrite, str)
	if err != nil {
		panic(err)
	}
}

// writeln is a wrapper for Fprintln that panics on any error.
func writeln(toWrite io.Writer, str string) {
	_, err := fmt.Fprintln(toWrite, str)
	if err != nil {
		panic(err)
	}
}

// writelnf will string format the latter arguments and call writeln.
func writelnf(toWrite io.Writer, formatStr string, args ...any) {
	writeln(toWrite, fmt.Sprintf(formatStr, args...))
}

// writef will string format the latter arguments and call write.
func writef(toWrite io.Writer, formatStr string, args ...any) {
	write(toWrite, fmt.Sprintf(formatStr, args...))
}

func newGnuPlotWriter(graphOptions graphOptions) *gnuplotWriter {
	tempdir, err := os.MkdirTemp("", "ftdc_parser")
	if err != nil {
		panic(err)
	}

	return &gnuplotWriter{
		metricFiles: make(map[string]*graphInfo),
		tempdir:     tempdir,
		options:     graphOptions,
	}
}

func (gpw *gnuplotWriter) getGraphInfo(metricName string) *graphInfo {
	if gi, created := gpw.metricFiles[metricName]; created {
		return gi
	}

	datafile, err := os.CreateTemp(gpw.tempdir, "")
	if err != nil {
		panic(err)
	}

	ret := &graphInfo{
		file: datafile,
	}
	gpw.metricFiles[metricName] = ret

	return ret
}

func (gpw *gnuplotWriter) addPoint(timeSeconds int64, metricName string, metricValue float32) {
	if timeSeconds < gpw.options.minTimeSeconds || timeSeconds > gpw.options.maxTimeSeconds {
		return
	}

	// While we're adding points, track the min/max values we saw. This can be used to better scale
	// graphs. As we've found gnuplots auto scaling to be a bit clunky.
	gi := gpw.getGraphInfo(metricName)
	gi.minVal = min(gi.minVal, int64(metricValue))
	gi.maxVal = max(gi.maxVal, int64(metricValue))
	writelnf(gi.file, "%v %.5f", timeSeconds, metricValue)
}

// ratioMetric describes which two FTDC metrics that should be combined to create a computed
// value. Such as "CPU %". Can also be used to express "requests per second".
type ratioMetric struct {
	Numerator string
	// An empty string Denominator will use the datum read timestamp value for its denominator. For
	// graphing a per-second rate.
	Denominator string
}

// ratioMetricToFields is a global variable identifying the metric names that are to be graphed as
// some ratio. The two members (`Numerator` and `Denominator`) refer to the suffix* of a metric
// name. For example, `UserCPUSecs` will appear under `proc.viam-server.UserCPUSecs` as well as
// `proc.modules.<foo>.UserCPUSecs`. If the `Denominator` is the empty string, the
// `ratioReading.Time` value will be used.
//
// When computing rates for metrics across two "readings", we simply subtract the numerators and
// denominator and divide the differences. We use the `windowSizeSecs` to pick which "readings"
// should be compared. This creates a sliding window. We (currently) bias this window to better
// portray "recent" resource utilization.
var ratioMetricToFields = map[string]ratioMetric{
	"UserCPU":   {"UserCPUSecs", "ElapsedTimeSecs"},
	"SystemCPU": {"SystemCPUSecs", "ElapsedTimeSecs"},
	// PerSec ratios use an empty string denominator.
	"HeadersProcessedPerSec": {"HeadersProcessed", ""},
	"TxPacketsPerSec":        {"TxPackets", ""},
	"RxPacketsPerSec":        {"RxPackets", ""},
	"TxBytesPerSec":          {"TxBytes", ""},
	"RxBytesPerSec":          {"RxBytes", ""},

	// Dan: Just tacking these on -- omitted metrics from this list does not mean they shouldn't* be
	// here. Also, personally, sometimes I think not* doing PerSec for these can also be
	// useful. Maybe we should consider including both the raw and rate graphs. Instead of replacing
	// the raw values with a rate graph.
	"GetImagePerSec":    {"GetImage", ""},
	"GetReadingsPerSec": {"GetReadings", ""},
}

// ratioReading is a reading of two metrics described by `ratioMetric`. This is what will be graphed.
type ratioReading struct {
	GraphName string
	// Seconds since epoch.
	Time        int64
	Numerator   float32
	Denominator float64

	// `isRate` == false will multiply by 100 for displaying as a percentage. Otherwise just display
	// the quotient.
	isRate bool
}

const epsilon = 1e-9

func (rr ratioReading) toValue() (float32, error) {
	if math.Abs(rr.Denominator) < epsilon {
		return 0.0, fmt.Errorf("divide by zero error, metric: %v", rr.GraphName)
	}

	if rr.isRate {
		return float32(float64(rr.Numerator) / rr.Denominator), nil
	}

	// A percentage
	return float32(float64(rr.Numerator) / rr.Denominator * 100), nil
}

func (rr *ratioReading) diff(other *ratioReading) ratioReading {
	return ratioReading{
		rr.GraphName,
		rr.Time,
		rr.Numerator - other.Numerator,
		rr.Denominator - other.Denominator,
		rr.isRate,
	}
}

// pullRatios returns true if any of the `ratioMetrics` match the input `reading`. If so, a new
// `ratioReading` is added to the `outDeferredReadings`.
func pullRatios(
	reading ftdc.Reading,
	readingTS int64,
	ratioMetrics map[string]ratioMetric,
	outDeferredReadings map[string]*ratioReading,
) bool {
	ret := false
	for ratioMetricName, ratioMetric := range ratioMetrics {
		if strings.HasSuffix(reading.MetricName, ratioMetric.Numerator) {
			ret = true

			// `metricIdentifier` is expected to be of the form `rdk.foo_module.`. Leave the
			// trailing dot as we would be about to re-add it.
			metricIdentifier := strings.TrimSuffix(reading.MetricName, ratioMetric.Numerator)
			// E.g: `rdk.foo_module.User CPU%'.
			graphName := fmt.Sprint(metricIdentifier, ratioMetricName)
			if _, exists := outDeferredReadings[graphName]; !exists {
				outDeferredReadings[graphName] = &ratioReading{GraphName: graphName, Time: readingTS, isRate: ratioMetric.Denominator == ""}
			}

			outDeferredReadings[graphName].Numerator = reading.Value
			if ratioMetric.Denominator == "" {
				outDeferredReadings[graphName].Denominator = float64(readingTS)
			}

			continue
		}

		if ratioMetric.Denominator != "" && strings.HasSuffix(reading.MetricName, ratioMetric.Denominator) {
			ret = true

			// `metricIdentifier` is expected to be of the form `rdk.foo_module.`. Leave the
			// trailing dot as we would be about to re-add it.
			metricIdentifier := strings.TrimSuffix(reading.MetricName, ratioMetric.Denominator)
			// E.g: `rdk.foo_module.User CPU%'.
			graphName := fmt.Sprint(metricIdentifier, ratioMetricName)
			if _, exists := outDeferredReadings[graphName]; !exists {
				outDeferredReadings[graphName] = &ratioReading{GraphName: graphName, Time: readingTS, isRate: false}
			}

			outDeferredReadings[graphName].Denominator = float64(reading.Value)
			continue
		}
	}

	return ret
}

func (gpw *gnuplotWriter) addFlatDatum(datum ftdc.FlatDatum) map[string]*ratioReading {
	// deferredReadings is an accumulator for readings of metrics that are used together to create a
	// graph. Such as `UserCPUSecs` / `ElapsedTimeSecs`.
	deferredReadings := make(map[string]*ratioReading)

	// There are two kinds of metrics. "Simple" metrics that can simply be passed through to the
	// gnuplotWriter. And "ratio" metrics that combine two different readings.
	//
	// For the ratio metrics, we use a two pass algorithm. The first pass will pair together all of
	// the necessary numerators and denominators. The second pass will write the computed datapoint
	// to the underlying gnuplotWriter.
	//
	// Ratio metrics are identified by the metric suffix. E.g: `rdk.custom_module.UserCPUSecs` will
	// be classified as a (numerator in a) ratio metric. We must also take care to record the prefix
	// of the ratio metric, the "metric identifier". There may be `rdk.foo_module.UserCPUSecs` in
	// addition to `rdk.bar_modular.UserCPUSecs`. Which should create two CPU% graphs.
	for _, reading := range datum.Readings {
		// pullRatios will identify if the metric is a "ratio" metric. If so, we do not currently
		// know what to graph and `pullRatios` will accumulate the relevant information into
		// `deferredReadings`.
		isRatioMetric := pullRatios(reading, datum.ConvertedTime().Unix(), ratioMetricToFields, deferredReadings)
		if isRatioMetric {
			// Ratio metrics need to be compared to some prior ratio metric to create a data
			// point. We do not output any information now. We instead accumulate all of these
			// results to be later used. These are named "deferred values".
			continue
		}

		gpw.addPoint(datum.ConvertedTime().Unix(), reading.MetricName, reading.Value)
	}

	return deferredReadings
}

// Ratios are averaged over a "recent history". This window size refers to a time in seconds, but we
// actually measure with respect to consecutive FTDC readings. The output value will use the system
// clock difference to compute a correct rate. We just accept there may be fuzziness with respect to
// how recent of a history we're actually using.
//
// Consider adding logging when two FTDC readings `windowSizeSecs` apart is not reflecting by their
// system time difference.
const windowSizeSecs = 5

// The deferredValues input is in FTDC reading order. On a responsive system, adjacent items in the
// slice should be one second apart.
func (gpw *gnuplotWriter) writeDeferredValues(deferredValues []map[string]*ratioReading, logger logging.Logger) {
	for idx, currReadings := range deferredValues {
		if idx == 0 {
			// The first element cannot be compared to anything. It would create a divide by zero
			// problem.
			continue
		}

		// `forCompare` is the index element to compare the "current" element pointed to by `idx`.
		forCompare := idx - windowSizeSecs
		if forCompare < 0 {
			forCompare = 0
		}

		prevReadings := deferredValues[forCompare]
		for metricName, currRatioReading := range currReadings {
			var diff ratioReading
			if prevratioReading, exists := prevReadings[metricName]; exists {
				diff = currRatioReading.diff(prevratioReading)
			} else {
				logger.Infow("Deferred value missing a previous value to diff",
					"metricName", metricName, "time", currRatioReading.Time)
				continue
			}

			value, err := diff.toValue()
			if err != nil {
				// The denominator did not change -- divide by zero error.
				logger.Warnw("Error computing defered value", "metricName", metricName, "time", currRatioReading.Time, "err", err)
				continue
			}
			gpw.addPoint(currRatioReading.Time, metricName, value)
		}
	}
}

// Render runs the compiler and invokes gnuplot, creating an image file.
func (gpw *gnuplotWriter) Render() {
	filename := gpw.CompileAndClose()
	// The filename was generated by this program -- not via user input.
	//nolint:gosec
	gnuplotCmd := exec.Command("gnuplot", filename)
	outputBytes, err := gnuplotCmd.CombinedOutput()
	if err != nil {
		nolintPrintln("error running gnuplot:", err)
		nolintPrintln("gnuplot output:", string(outputBytes))
	}
}

// Compile writes out all of the underlying files for gnuplot. And returns the "top-level" filename
// that can be input to gnuplot. The returned filename is an absolute path.
func (gpw *gnuplotWriter) CompileAndClose() string {
	gnuFile, err := os.CreateTemp(gpw.tempdir, "main")
	if err != nil {
		panic(err)
	}
	defer utils.UncheckedErrorFunc(gnuFile.Close)

	// Write a png with width of 1000 pixels and 200 pixels of height per metric/graph. This works
	// well for small numbers of graphs, but for large numbers, we get a bunch of extra whitespace
	// at the top/bottom. Adding `crop` will trim that whitespace.
	writelnf(gnuFile, "set term png size %d, %d crop", 1000, 200*len(gpw.metricFiles))

	// Log the tempdir in case one wants to go back and see/edit how the graph was generated. A user
	// can rerun `gnuplot /<tmpdir>/main<unique value>` to recreate `plot.png` with the new
	// settings/data.
	nolintPrintln("Gnuplot dir:", gpw.tempdir)
	nolintPrintln("Output file: `plot.png`")
	// The output filename
	writeln(gnuFile, "set output 'plot.png'")

	// We're making separate graphs instead of a single big graph. The graphs will be arranged in a
	// rectangle with 1 column and X rows. Where X is the number of metrics.  Add some margins for
	// aesthetics.
	//
	// The first margin is the left-hand margin. We set it a bit bigger to allow for larger numbers
	// for labeling the Y-axis values.
	writelnf(gnuFile, "set multiplot layout %v,1 margins 0.10,0.9, 0.05,0.9 spacing screen 0, char 5", len(gpw.metricFiles))

	//  Axis labeling/formatting/type information.
	writeln(gnuFile, "set timefmt '%s'")

	// This omits the month-day-year. Probably worth adding in for whoever researches how to do it.
	writeln(gnuFile, "set format x '%H:%M:%S'")
	writeln(gnuFile, "set xlabel 'Time'")
	writeln(gnuFile, "set xdata time")

	allZeroesHidden := 0
	var minY int64
	var maxY int64

	// For each metric file, we will output a single gnuplot `plot` directive that will generate a
	// single graph.
	for _, nameFilePair := range sorted(gpw.metricFiles) {
		metricName, graphInfo := nameFilePair.Key, nameFilePair.Val
		if gpw.options.hideAllZeroes && graphInfo.minVal == 0 && graphInfo.maxVal == 0 {
			allZeroesHidden++
			continue
		}

		// The minY/maxY values start at 0. So this the range from minY -> maxY will fall into one
		// of the three buckets (in order of likelihood):
		//
		// - [0, positive number]
		// - [negative number, positive number]
		// - [negative number, 0]
		minY = min(minY, graphInfo.minVal)
		maxY = max(maxY, graphInfo.maxVal)

		// If we're graphing something that has the same value for all readings, bump the yrange to
		// avoid gnuplot complaints. E.g: `set yrange [0:0]` turns into `set yrange [0:1]`.
		if graphInfo.minVal == graphInfo.maxVal {
			graphInfo.maxVal = graphInfo.minVal + 1
		}

		// Set the lower/upper limits on the Y-axis for the graph. We use 2* maxVal to allow extra
		// headroom for writing out the legend.
		writelnf(gnuFile, "set yrange [%v:%v]", graphInfo.minVal, 2*float64(graphInfo.maxVal))

		//nolint
		// We write the plot line without a trailing newline. In case we want to add the vertical
		// lines. An example output for the following writes might be:
		// plot '/tmp/ftdc_parser1500397090/3060093295' using 1:2 with lines linestyle 7 lw 4 title 'rdk-internal:service:web/builtin.ResStats.RPCServer.WebRTCGrpcStats.CallTicketsAvailable',\
		// 		'/tmp/ftdc_parser1500397090/vert-0.txt' using 1:2 with lines linestyle 6 lw 4 title '2025-02-22 21:13:00 +0000 UTC',\
		// 		'/tmp/ftdc_parser1500397090/vert-1.txt' using 1:2 with lines linestyle 6 lw 4 title '2025-02-22 21:13:40 +0000 UTC'
		//
		//
		// linestyle 7 is red, 6 is blue, lw is line-width (or weight) -- makes it thicker. The
		// title is what's used in the legend.
		writef(gnuFile, "plot '%v' using 1:2 with lines linestyle 7 lw 4 title '%v'", graphInfo.file.Name(), strings.ReplaceAll(metricName, "_", "\\_"))

		// "vertical lines" for events are rendered as another set of data points for a
		// `plot`. Because the vertical lines are at the same x-value/time for each graph, we can
		// re-use the same file at a pre-determined name. These files will be written out next after
		// we've accumulated all of the min/max Y values.
		for idx, vertLineX := range gpw.options.vertLinesAtSeconds {
			writeln(gnuFile, ",\\")
			writef(gnuFile,
				"\t'%v' using 1:2 with lines linestyle 6 lw 4 title '%v'",
				filepath.Join(gpw.tempdir, fmt.Sprintf("vert-%d.txt", idx)),
				time.Unix(vertLineX, 0).UTC())
		}

		// The trailing newline for the above calls to write out a single plot.
		writeln(gnuFile, "")

		utils.UncheckedErrorFunc(graphInfo.file.Close)
	}
	if allZeroesHidden > 0 {
		nolintPrintln("Hid metrics that only had 0s for data. Cnt:", allZeroesHidden)
		// Dan: perhaps add a command to disable this. E.g:
		//   nolintPrintln("Use `set show-all-zeroes 1` to show them.")
	}

	// Actually write out the `vert-<number>.txt` plots.
	for idx, vertLineX := range gpw.options.vertLinesAtSeconds {
		vertFile, err := os.Create(filepath.Join(gpw.tempdir, fmt.Sprintf("vert-%v.txt", idx)))
		defer utils.UncheckedErrorFunc(vertFile.Close)
		if err != nil {
			panic(err)
		}

		// For same reason, trying to graph two points (time, -inf) -> (time, +inf) does not
		// render. So we instead write (time, -inf) -> (time, 0) -> (time, +inf). Which does work...
		writelnf(vertFile, "%v %d", vertLineX, minY)
		writelnf(vertFile, "%v 0", vertLineX)
		writelnf(vertFile, "%v %d", vertLineX, maxY)
	}

	return gnuFile.Name()
}

func parseStringAsTime(inp string) (time.Time, error) {
	goTime, err := time.Parse("2006-01-02T15:04:05", inp)
	if err != nil {
		// This is a CLI. It's acceptable to output to stdout.
		//nolint:forbidigo
		fmt.Printf("Error parsing start time. Working example: `2024-09-24T18:00:00` Inp: %q Err: %v\n", inp, err)
		return time.Time{}, err
	}

	return goTime, nil
}

func main() {
	if len(os.Args) < 2 {
		nolintPrintln("Expected an FTDC filename. E.g: go run parser.go <path-to>/viam-server.ftdc")
		return
	}

	ftdcFile, err := os.Open(os.Args[1])
	if err != nil {
		nolintPrintln("Error opening file. File:", os.Args[1], "Err:", err)
		nolintPrintln("Expected an FTDC filename. E.g: go run parser.go <path-to>/viam-server.ftdc")
		return
	}

	logger := logging.NewLogger("parser")
	data, err := ftdc.ParseWithLogger(ftdcFile, logger)
	if err != nil {
		panic(err)
	}

	stdinReader := bufio.NewReader(os.Stdin)
	render := true
	graphOptions := defaultGraphOptions()
	for {
		if render {
			deferredValues := make([]map[string]*ratioReading, 0)
			gpw := newGnuPlotWriter(graphOptions)
			for _, flatDatum := range data {
				deferredValues = append(deferredValues, gpw.addFlatDatum(flatDatum))
			}

			gpw.writeDeferredValues(deferredValues, logger)

			gpw.Render()
		}
		render = true

		// This is a CLI. It's acceptable to output to stdout.
		//nolint:forbidigo
		fmt.Print("$ ")
		cmd, err := stdinReader.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		switch {
		case err != nil && errors.Is(err, io.EOF):
			nolintPrintln("\nExiting...")
			return
		case cmd == "quit":
			nolintPrintln("Exiting...")
			return
		case cmd == "h" || cmd == "help":
			render = false
			nolintPrintln("range <start> <end>")
			nolintPrintln("-  Only plot datapoints within the given range. \"zoom in\"")
			nolintPrintln("-  E.g: range 2024-09-24T18:00:00 2024-09-24T18:30:00")
			nolintPrintln("-       range start 2024-09-24T18:30:00")
			nolintPrintln("-       range 2024-09-24T18:00:00 end")
			nolintPrintln("-  All times in UTC")
			nolintPrintln()
			nolintPrintln("reset range")
			nolintPrintln("-  Unset any prior range. \"zoom out to full\"")
			nolintPrintln()
			nolintPrintln("r, refresh")
			nolintPrintln("-  Regenerate the plot.png image. Useful when a current viam-server is running.")
			nolintPrintln()
			nolintPrintln("`quit` or Ctrl-d to exit")
		case strings.HasPrefix(cmd, "range "):
			pieces := strings.SplitN(cmd, " ", 3)
			// TrimSpace to remove the newline.
			start, end := pieces[1], pieces[2]

			if start == "start" {
				graphOptions.minTimeSeconds = 0
			} else {
				if goTime, err := parseStringAsTime(start); err == nil {
					// parseStringAsTime outputs an error message for us.
					graphOptions.minTimeSeconds = goTime.Unix()
				}
			}

			if end == "end" {
				graphOptions.maxTimeSeconds = math.MaxInt64
			} else {
				if goTime, err := parseStringAsTime(end); err == nil {
					// parseStringAsTime outputs an error message for us.
					graphOptions.maxTimeSeconds = goTime.Unix()
				}
			}
		case strings.HasPrefix(cmd, "reset range"):
			graphOptions.minTimeSeconds = 0
			graphOptions.maxTimeSeconds = math.MaxInt64
		case strings.HasPrefix(cmd, "ev ") || strings.HasPrefix(cmd, "event "):
			pieces := strings.SplitN(cmd, " ", 2)
			if goTime, err := parseStringAsTime(pieces[1]); err == nil {
				// parseStringAsTime outputs an error message for us.
				graphOptions.vertLinesAtSeconds = append(graphOptions.vertLinesAtSeconds, goTime.Unix())
			}
		case cmd == "refresh" || cmd == "r":
			nolintPrintln("Refreshing graphs with new data")
		case len(cmd) == 0:
			render = false
		default:
			nolintPrintln("Unknown command. Type `h` for help.")
			render = false
		}
	}
}
