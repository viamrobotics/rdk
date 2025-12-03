// Package parser provides a CLI tool for viewing `.ftdc` files emitted by the `viam-server`.
package parser

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/iancoleman/orderedmap"
	"go.viam.com/utils"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
)

// graphInfo points to an OS file containing the data points for a single graph. We also record the
// min/max values for scaling purposes when generating plots.
type graphInfo struct {
	file   *os.File
	minVal float32
	maxVal float32

	prevVal float32
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

	// timesToInclude is a slice of unix timestamps. The "nearest" datapoint to each value will be
	// added. If timesToInclude is nil, all data points are to be added.
	timesToInclude []int64
	// timeSliceNanos is the amount of time in nanos (on average) between adjacent members of
	// `timesToInclude`. When all input datapoints are used, we expect this to be ~1 second worth of
	// nanos. If we throw away datapoints, it will be larger.
	timeSliceNanos time.Duration
	// shouldIncludePointStorage are "local" variables used for successive calls to
	// `shouldIncludePoint`.
	shouldIncludePointStorage struct {
		nextTimeIdx int
	}

	// The earliest datapoint being plotted. This is used to help ensure every chart starts plotting
	// from the same time.
	firstTimeSecs int64
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

	// vertLinesAtSeconds will draw a blue vertical line for each element. The items are expected to be
	// in units of seconds since the epoch. These are to represent "events" that are of interest to a
	// user (where the user would like to find correlations in other metrics.)
	vertLinesAtSeconds []int64

	// fileBoundariesAtSeconds will draw a yellow vertical line for each element. The items
	// are expected to be in units of seconds since the epoch. These are to represent the
	// ending timestamps of files in the case that the parser was run on a directory with
	// multiple files.
	fileBoundariesAtSeconds []int64

	// maxPoints is how many data points will actually be graphed for each plot. Too many data
	// points can be distracting. The algorithm is to divide the min/max time in `maxPoints`
	// "equally distanced" timestamps. A user needing more fine-grained information is expected to
	// zoom in.
	maxPoints int

	// selectList is a list of metric names to always show at the top of the generated image.
	// This option will override hideAllZeroes and show a graph at the top even if all values of the graph is 0.
	selectList *orderedmap.OrderedMap
}

// defaultGraphOptions returns a default set of graph options. It adds all but the last
// passed-in file boundary timestamps to `vertLinesAtSeconds`, so that vertical lines will
// be automatically rendered at file boundaries.
func defaultGraphOptions(fileBoundaryTimestamps []int64) graphOptions {
	for i := range fileBoundaryTimestamps {
		fileBoundaryTimestamps[i] /= 1e9
	}

	return graphOptions{
		minTimeSeconds:          0,
		maxTimeSeconds:          math.MaxInt64,
		hideAllZeroes:           true,
		vertLinesAtSeconds:      make([]int64, 0),
		fileBoundariesAtSeconds: fileBoundaryTimestamps[:len(fileBoundaryTimestamps)-1],
		maxPoints:               1000,
		selectList:              orderedmap.New(),
	}
}

// NolintPrintln outputs to stdout.
func NolintPrintln(str ...any) {
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

func newGnuPlotWriter(graphOptions graphOptions, numDatapoints int, minTime, maxTime int64) *gnuplotWriter {
	tempdir, err := os.MkdirTemp("", "ftdc_parser")
	if err != nil {
		panic(err)
	}

	// The `firstTime` represents the first datapoint all created graphs will contain. Such that
	// metrics that come into existence later in a robot's lifetime will still have a graph starting
	// at the same time as all other graphs.
	firstTimeSecs := minTime / time.Second.Nanoseconds()
	var timesToInclude []int64
	// If we include every datapoint, the timeslice ought to be ~1 second.
	timeSliceNanos := time.Second.Nanoseconds()
	if numDatapoints > graphOptions.maxPoints {
		// If there are more datapoints than we wish to graph, calculate the approximate evenly
		// spaced timestamps to use.
		timesToInclude = make([]int64, graphOptions.maxPoints)
		timeSliceNanos = (maxTime - minTime) / int64(graphOptions.maxPoints)

		timeToInclude := minTime
		for idx := 0; idx < graphOptions.maxPoints; idx++ {
			timesToInclude[idx] = timeToInclude
			timeToInclude += timeSliceNanos
		}

		// If we've decided to downsample, the `firstTime` ought to reflect the earliest
		// `timeToInclude`.
		firstTimeSecs = timesToInclude[0] / time.Second.Nanoseconds()
	}

	return &gnuplotWriter{
		metricFiles:    make(map[string]*graphInfo),
		tempdir:        tempdir,
		options:        graphOptions,
		timesToInclude: timesToInclude,
		timeSliceNanos: time.Duration(timeSliceNanos),
		firstTimeSecs:  firstTimeSecs,
	}
}

// shouldIncludePoint returns one of the input `FlatDatum`s or nil. If a `FlatDatum` is returned,
// the caller is expected to add it to the output graph. If nil is returned, neither are to be
// added.
//
// This method expects that every datapoint in a dataset (other than the first and last) will be
// passed in twice. First as a "next" and immediately followed as a "this".
func (gpw *gnuplotWriter) shouldIncludePoint(this, next *ftdc.FlatDatum) *ftdc.FlatDatum {
	if gpw.timesToInclude == nil {
		return this
	}

	if gpw.shouldIncludePointStorage.nextTimeIdx >= len(gpw.timesToInclude) {
		return nil
	}

	nextTimeToInclude := gpw.timesToInclude[gpw.shouldIncludePointStorage.nextTimeIdx]
	if this.Time > nextTimeToInclude {
		// There may be a bubble in FTDC data such that we didn't capture any data for a few
		// seconds. Return the current value and also bump the `nextTimeIdx` to restore the
		// invariant that the `nextTimeToInclude` > `this.Time` as long as we do not exceed
		// `len(gpw.timesToInclude)-1`.
		for gpw.shouldIncludePointStorage.nextTimeIdx < len(gpw.timesToInclude) &&
			gpw.timesToInclude[gpw.shouldIncludePointStorage.nextTimeIdx] < this.Time {
			gpw.shouldIncludePointStorage.nextTimeIdx++
		}

		return this
	}

	// If `this` and `next` straddle the `nextTimeToInclude`, we'll return a point to graph.
	returnAPoint := this.Time <= nextTimeToInclude && next.Time >= nextTimeToInclude
	if !returnAPoint {
		return nil
	}

	gpw.shouldIncludePointStorage.nextTimeIdx++
	// Dan: For simplicity we always return the "earlier" time. In a healthy system where we are
	// choosing datapoints, the "time interval" to graph will be much larger than the one second
	// ftdc data capture rate (e.g: graph every 15 seconds). Choosing either value there is "safe".
	//
	// But in an "unhealthy" system, we may have a scenario for example where we want* to graph
	// timestamps 10 and 20. But the three data points to choose from are 0, 15 and 30. It's not
	// ideal to choose "15" twice (because it's the closest value). To avoid complicating the
	// algorithm (by remembering the last value we selected, or looking ahead at more values), we
	// can just choose to return the earlier value. Giving us 0 and 15 in this example, which is a
	// reasonable decision to make.
	return this
}

// getGraphInfo returns the cached `graphInfo` object for a given metric, or creates a new
// one. Returns whether a new one was created.
func (gpw *gnuplotWriter) getGraphInfo(metricName string) (*graphInfo, bool) {
	if gi, created := gpw.metricFiles[metricName]; created {
		return gi, false
	}

	datafile, err := os.CreateTemp(gpw.tempdir, "")
	if err != nil {
		panic(err)
	}

	ret := &graphInfo{
		file: datafile,
	}
	gpw.metricFiles[metricName] = ret

	return ret, true
}

func (gpw *gnuplotWriter) copyPreviousPoint(timeSeconds int64, metricName string) {
	if timeSeconds < gpw.options.minTimeSeconds || timeSeconds > gpw.options.maxTimeSeconds {
		return
	}

	// Do not use `getGraphInfo`. As we are returning in the case where this is newly created. And
	// the actual code that writes a first data point would not know its the first write.
	//
	// Perhaps we should either:
	// - Only call `copyPreviousPoint` when there is a previous point or
	// - Determine if a graph is new by looking at how many points we've added. Not the existence of
	//   the graph in our internal map.
	gi, exists := gpw.metricFiles[metricName]
	if !exists {
		return
	}

	gpw.addPoint(timeSeconds, metricName, gi.prevVal)
}

func (gi *graphInfo) writeStartingDatapoints(firstTimeSecs, datapointTimeSecs int64) {
	// For newly created files:
	// - Ensure the first datapoint is at `firstTime`.
	// - If the first datapoint is more than a second after `firstTime`, write a 0-value
	//   datapoint just prior.
	//
	// Motivation for the latter: consider a metric that comes into existence (much) later in a
	// robot life. We want the graph to have nice spike up. Rather than a long slow rise from the
	// beginning of time.
	if datapointTimeSecs > firstTimeSecs {
		writelnf(gi.file, "%v 0.0", firstTimeSecs)
	}

	if datapointTimeSecs-1 > firstTimeSecs {
		writelnf(gi.file, "%v 0.0", datapointTimeSecs-1)
	}
}

func (gpw *gnuplotWriter) addPoint(timeSeconds int64, metricName string, metricValue float32) {
	if timeSeconds < gpw.options.minTimeSeconds || timeSeconds > gpw.options.maxTimeSeconds {
		return
	}

	// While we're adding points, track the min/max values we saw. This can be used to better scale
	// graphs. As we've found gnuplots auto scaling to be a bit clunky.
	gi, newlyCreated := gpw.getGraphInfo(metricName)
	if newlyCreated {
		startingTime := gpw.firstTimeSecs
		if gpw.options.minTimeSeconds > startingTime {
			startingTime = gpw.options.minTimeSeconds
		}

		gi.writeStartingDatapoints(startingTime, timeSeconds)
	}

	gi.prevVal = metricValue
	gi.minVal = min(gi.minVal, metricValue)
	gi.maxVal = max(gi.maxVal, metricValue)
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
	"DataSentBytesPerSec":    {"dataSentBytes", ""},

	// Dan: Just tacking these on -- omitted metrics from this list does not mean they shouldn't* be
	// here. Also, personally, sometimes I think not* doing PerSec for these can also be
	// useful. Maybe we should consider including both the raw and rate graphs. Instead of replacing
	// the raw values with a rate graph.
	"GetImagePerSec":            {"GetImage", ""},
	"GetReadingsPerSec":         {"GetReadings", ""},
	"GetImagesPerSec":           {"GetImages", ""},
	"DoCommandPerSec":           {"DoCommand", ""},
	"MoveStraightLatencyMillis": {"MoveStraight.timeSpent", "MoveStraight"},
	"GetClassificationsPerSec":  {"GetClassifications", ""},
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

func (rr *ratioReading) diffAgainstZero(denominator float64) ratioReading {
	return ratioReading{
		rr.GraphName,
		rr.Time,
		rr.Numerator,
		rr.Denominator - denominator,
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

		// `ratioMetric` with a denominator will become the division of the two named
		// metrics. Omitting a denominator implies a "per second", hence we don't need further ftdc
		// metrics to compute those rates.
		if ratioMetric.Denominator != "" && strings.HasSuffix(reading.MetricName, ratioMetric.Denominator) {
			ret = true

			// `metricIdentifier` is expected to be of the form `rdk.foo_module.`. Leave the
			// trailing dot as we would be about to re-add it.
			metricIdentifier := strings.TrimSuffix(reading.MetricName, ratioMetric.Denominator)
			// E.g: `rdk.foo_module.User CPU%'.
			graphName := fmt.Sprint(metricIdentifier, ratioMetricName)
			if _, exists := outDeferredReadings[graphName]; !exists {
				// All of these values computed as "numerator/denominator". `isRate` only controls
				// whether we interpret the result as a percentage or not. `isRate` false is a
				// percentage. `isRate` true stays as the raw division.
				isRate := false

				// This creates graphs for metrics of the form: "total time spent in move straight"
				// divided by "number of move straight calls".
				if strings.HasSuffix(ratioMetric.Numerator, ".timeSpent") {
					isRate = true
				}
				outDeferredReadings[graphName] = &ratioReading{GraphName: graphName, Time: readingTS, isRate: isRate}
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
// Consider adding logging when two FTDC readings `windowSize` apart is not reflecting by their
// system time difference.
const windowSize = 5 * time.Second

// The deferredValues input is in FTDC reading order. On a responsive system, adjacent items in the
// slice should be one second apart.
func (gpw *gnuplotWriter) writeDeferredValues(deferredValues []map[string]*ratioReading, logger logging.Logger) {
	// "deferred values" are computed by subtracting adjacent values. We iterate through
	// `deferredValues` in slice order to compare "adjacent" elements. But because graphing readings
	// from truly adjacent elements (every second) can be noisy, we dampen that by looking back
	// `windowSizeSecs`.
	for idx, currReadings := range deferredValues {
		if idx == 0 {
			// The first element cannot be compared to anything. It would create a divide by zero
			// problem.
			continue
		}

		// If the window size is 5 seconds and the time slice was 2.5 seconds per "index", we'll
		// want to "look back" 2 "index units".
		lookback := int(windowSize.Nanoseconds() / gpw.timeSliceNanos.Nanoseconds())
		if lookback < 0 {
			// If the `timeSliceNanos` value is larger than five seconds, just look back one index
			// element.
			lookback = 1
		}

		// `forCompare` is the index element to compare the "current" element pointed to by `idx`.
		forCompare := idx - lookback
		if forCompare < 0 {
			// At the beginning of data, we will compute rates, but just shrink the window size to
			// what's available.
			forCompare = 0
		}

		prevReadings := deferredValues[forCompare]
		for metricName, currRatioReading := range currReadings {
			var diff ratioReading
			if prevRatioReading, exists := prevReadings[metricName]; exists {
				diff = currRatioReading.diff(prevRatioReading)
			} else {
				// We expect this to happen when there's only one reading in some window size. This
				// can be very spammy, so it's at the debug level. A bug could easily introduce
				// unexpected cases to enter this code path.
				logger.Debugw("Deferred value missing a previous value to diff.",
					"metricName", metricName, "time", currRatioReading.Time)

				// For cases where a metric comes into existence as non-zero, we can assume it's
				// older readings would have been zero. We additionally make an assumption that this
				// is a time metric.
				diff = currRatioReading.diffAgainstZero(float64(currRatioReading.Time) - windowSize.Seconds())
			}

			value, err := diff.toValue()
			if err != nil {
				// The denominator did not change -- divide by zero error. E.g: there were no calls
				// to a given RPC in the last window slice.
				logger.Debugw("Error computing deferred value",
					"metricName", metricName, "time", currRatioReading.Time, "err", err)
				// Copy the last point. Such that all graphs ought to have the same "last"
				// datapoint.
				gpw.copyPreviousPoint(currRatioReading.Time, metricName)
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
		NolintPrintln("error running gnuplot:", err)
		NolintPrintln("gnuplot output:", string(outputBytes))
	}
}

func (gpw *gnuplotWriter) writeSinglePlot(metricName string, graphInfo *graphInfo, gnuFile *os.File) {
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

	// File boundaries are the same as vertical lines above, but should be represented
	// with yellow lines (5) instead of blue, and should not have titles, as many file
	// boundaries can crowd out the actual metric's title.
	for idx := range gpw.options.fileBoundariesAtSeconds {
		writeln(gnuFile, ",\\")
		writef(gnuFile,
			"\t'%v' using 1:2 with lines linestyle 5 lw 4 notitle",
			filepath.Join(gpw.tempdir, fmt.Sprintf("fb-%d.txt", idx)))
	}

	// The trailing newline for the above calls to write out a single plot.
	writeln(gnuFile, "")
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
	//
	// Since we will only ever render one plot per metric, it is acceptable to just use the length of
	// gpw.metricFiles.
	writelnf(gnuFile, "set term png size %d, %d crop", 1000, 200*len(gpw.metricFiles))

	// Log the tempdir in case one wants to go back and see/edit how the graph was generated. A user
	// can rerun `gnuplot /<tmpdir>/main<unique value>` to recreate `plot.png` with the new
	// settings/data.
	NolintPrintln("Gnuplot dir:", gpw.tempdir)
	NolintPrintln("Output file: `plot.png`")
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

	var minY int64
	var maxY int64

	// For each metric name in selectList, we will output a single gnuplot `plot` directive
	// that will generate a single graph.
	for _, metricName := range gpw.options.selectList.Keys() {
		graphInfo := gpw.metricFiles[metricName]

		// The minY/maxY values start at 0. So this the range from minY -> maxY will fall into one
		// of the three buckets (in order of likelihood):
		//
		// - [0, positive number]
		// - [negative number, positive number]
		// - [negative number, 0]
		minY = min(minY, int64(graphInfo.minVal))
		maxY = max(maxY, int64(graphInfo.maxVal))
		if minY == 0 && maxY == 0 {
			maxY = 1
		}

		gpw.writeSinglePlot(metricName, graphInfo, gnuFile)

		utils.UncheckedErrorFunc(graphInfo.file.Close)
	}

	allZeroesHidden := 0

	// For each metric file, we will output a single gnuplot `plot` directive that will generate a
	// single graph.
	for _, nameFilePair := range sorted(gpw.metricFiles) {
		metricName, graphInfo := nameFilePair.Key, nameFilePair.Val

		// if already rendered, skip
		if _, ok := gpw.options.selectList.Get(metricName); ok {
			continue
		}

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
		minY = min(minY, int64(graphInfo.minVal))
		maxY = max(maxY, int64(graphInfo.maxVal))
		if minY == 0 && maxY == 0 {
			maxY = 1
		}
		gpw.writeSinglePlot(metricName, graphInfo, gnuFile)

		utils.UncheckedErrorFunc(graphInfo.file.Close)
	}
	if allZeroesHidden > 0 {
		NolintPrintln("Hid metrics that only had 0s for data. Cnt:", allZeroesHidden)
		NolintPrintln("Use `show zeroes` to show them.")
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

	// Actually write out the `fb-<number>.txt` plots.
	for idx, fileBoundaryX := range gpw.options.fileBoundariesAtSeconds {
		fileBoundaryFile, err := os.Create(filepath.Join(gpw.tempdir, fmt.Sprintf("fb-%v.txt", idx)))
		defer utils.UncheckedErrorFunc(fileBoundaryFile.Close)
		if err != nil {
			panic(err)
		}

		writelnf(fileBoundaryFile, "%v %d", fileBoundaryX, minY)
		writelnf(fileBoundaryFile, "%v 0", fileBoundaryX)
		writelnf(fileBoundaryFile, "%v %d", fileBoundaryX, maxY)
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

// getFTDCData returns a slice of FlatDatums from the path it was passed and a slice of
// "last timestamps" representing file boundaries. If path leads to an .ftdc file, only
// that file is parsed. If it leads to a directory, all .ftdc files in that directory will
// get parsed and the combined slice of FlatDatums will get returned. The subdirectories
// of that directory will NOT get explored.
func getFTDCData(ftdcPath string, logger logging.Logger) ([]ftdc.FlatDatum, []int64, error) {
	info, err := os.Stat(ftdcPath)
	if err != nil {
		return nil, nil, err
	}

	// If path is not a directory, we can just open the file and get its datums.
	if !info.IsDir() {
		//nolint:gosec
		ftdcFile, err := os.Open(ftdcPath)
		if err != nil {
			return nil, nil, err
		}
		//nolint:errcheck
		defer ftdcFile.Close()

		flatDatums, lastTimestamp, err := ftdc.ParseWithLogger(ftdcFile, logger)
		logger.Debugw("File boundary found", "timestamp_ns", lastTimestamp)
		return flatDatums, []int64{lastTimestamp}, err
	}

	// If path is a directory, we will walk it and get all of the FTDC datums.
	flatDatums := make([]ftdc.FlatDatum, 0)
	fileBoundaryTimestamps := make([]int64, 0)
	err = filepath.WalkDir(ftdcPath, fs.WalkDirFunc(func(path string, d fs.DirEntry, walkErr error) error {
		// For now, no recursive parsing.
		if d.IsDir() && path != ftdcPath {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".ftdc") {
			return nil
		}

		if walkErr != nil {
			return walkErr
		}

		//nolint:gosec
		ftdcReader, err := os.Open(path)
		if err != nil {
			return err
		}
		//nolint:errcheck
		defer ftdcReader.Close()

		ftdcData, lastTimestamp, err := ftdc.ParseWithLogger(ftdcReader, logger)
		if err != nil {
			logger.Warnw("Error getting ftdc data from file", "path", path, "err", err)
			return nil
		}

		flatDatums = append(flatDatums, ftdcData...)

		// The last timestamp parsed is equivalent to a file boundary timestamp.
		logger.Debugw("File boundary found", "timestamp_ns", lastTimestamp)
		fileBoundaryTimestamps = append(fileBoundaryTimestamps, lastTimestamp)

		return nil
	}))
	if err != nil {
		return nil, nil, err
	}

	if len(flatDatums) < 1 {
		return nil, nil, errors.New("provided a directory with no FTDC files")
	}

	return flatDatums, fileBoundaryTimestamps, nil
}

func renderPlot(data []ftdc.FlatDatum, graphOptions graphOptions, logger logging.Logger) *gnuplotWriter {
	deferredValues := make([]map[string]*ratioReading, 0)
	gpw := newGnuPlotWriter(graphOptions, len(data), data[0].Time, data[len(data)-1].Time)
	for idx := 0; idx < len(data)-1; idx++ {
		thisDatum, nextDatum := data[idx], data[idx+1]
		if pt := gpw.shouldIncludePoint(&thisDatum, &nextDatum); pt != nil {
			deferredValues = append(deferredValues, gpw.addFlatDatum(*pt))
		}
	}
	if gpw.timesToInclude == nil {
		// If we're including all of the data points, don't forget the last one.
		deferredValues = append(deferredValues, gpw.addFlatDatum(data[len(data)-1]))
	}

	gpw.writeDeferredValues(deferredValues, logger)

	gpw.Render()
	return gpw
}

// LaunchREPL opens an ftdc file or directory, plots it, and runs a cli for it.
func LaunchREPL(ftdcFilepath string) {
	logger := logging.NewLogger("parser")
	data, fileBoundaryTimestamps, err := getFTDCData(filepath.Clean(ftdcFilepath), logger)
	if err != nil {
		NolintPrintln("Error getting ftdc data from path:", ftdcFilepath, "Err:", err)
		NolintPrintln(`Expected an FTDC filename or a directory. E.g: go run main.go
		<path-to>/viam-server.ftdc or a directory with .ftdc files`)
		return
	}

	stdinReader := bufio.NewReader(os.Stdin)

	graphOptions := defaultGraphOptions(fileBoundaryTimestamps)

	gpw := renderPlot(data, graphOptions, logger)
	for {
		render := true

		// This is a CLI. It's acceptable to output to stdout.
		//nolint:forbidigo
		fmt.Print("$ ")
		cmd, err := stdinReader.ReadString('\n')
		cmd = strings.TrimSpace(cmd)
		switch {
		case err != nil && errors.Is(err, io.EOF):
			NolintPrintln("\nExiting...")
			return
		case cmd == "quit":
			NolintPrintln("Exiting...")
			return
		case cmd == "h" || cmd == "help":
			render = false
			NolintPrintln("range <start> <end>")
			NolintPrintln("-  Only plot datapoints within the given range. \"zoom in\"")
			NolintPrintln("-  E.g: range 2024-09-24T18:00:00 2024-09-24T18:30:00")
			NolintPrintln("-       range start 2024-09-24T18:30:00")
			NolintPrintln("-       range 2024-09-24T18:00:00 end")
			NolintPrintln("-  All times in UTC")
			NolintPrintln()
			NolintPrintln("reset range")
			NolintPrintln("-  Unset any prior range. \"zoom out to full\"")
			NolintPrintln()
			NolintPrintln("ev, event <timestamp>")
			NolintPrintln("-  Add a vertical marker at a timestamp representing an event of interest.")
			NolintPrintln("-  E.g: ev 2024-09-24T18:15:00")
			NolintPrintln()
			NolintPrintln("select <metric1,metric2>")
			NolintPrintln("-  Comma-separated list of metric names (can be regex) to show top of the generated plot.png image.")
			NolintPrintln("-  If metric name matches multiple regexes it will only get selected once.")
			NolintPrintln("-  E.g: select metric1,metric2.*")
			NolintPrintln()
			NolintPrintln("deselect <metric1,metric2>")
			NolintPrintln("-  Comma-separated list of metric names (can be regex) to stop showing at the top of the generated plot.png image.")
			NolintPrintln("-  E.g: deselect metric1,metric2.*")
			NolintPrintln("-       deselect all")
			NolintPrintln()
			NolintPrintln("show zeroes")
			NolintPrintln("-  Generate graphs without omitting plots with all zeroes.")
			NolintPrintln()
			NolintPrintln("hide zeroes")
			NolintPrintln("-  Generate graphs omitting plots with all zeroes.")
			NolintPrintln()
			NolintPrintln("`quit` or Ctrl-d to exit")
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
		case strings.HasPrefix(cmd, "select "):
			withoutCmd := strings.TrimPrefix(cmd, "select ")
			pieces := strings.Split(withoutCmd, ",")
			if len(pieces) == 0 {
				break
			}
			additions := make([]string, 0)

			// sorting it so that additions are alphabetical and predictable
			for _, nameFilePair := range sorted(gpw.metricFiles) {
				metricName := nameFilePair.Key
				if _, ok := graphOptions.selectList.Get(metricName); ok {
					continue
				}
				for _, piece := range pieces {
					trimmedPattern := strings.TrimSpace(piece)
					matched, err := regexp.MatchString(fmt.Sprintf("(?i)%v", trimmedPattern), metricName)
					if err != nil {
						NolintPrintln("Error matching input:", trimmedPattern, "Err:", err)
						continue
					}
					if matched {
						graphOptions.selectList.Set(metricName, true)
						additions = append(additions, metricName)
						break
					}
				}
			}
			NolintPrintln("Added metrics to select list:", additions)
			NolintPrintln()
			NolintPrintln("New list of metrics to print at top of generated image:", graphOptions.selectList.Keys())
		case strings.HasPrefix(cmd, "deselect "):
			withoutCmd := strings.TrimPrefix(cmd, "deselect ")
			pieces := strings.Split(withoutCmd, ",")
			if len(pieces) == 0 {
				break
			}
			selectList := graphOptions.selectList
			graphOptions.selectList = orderedmap.New()
			if strings.TrimSpace(withoutCmd) == "all" {
				NolintPrintln("Removed all metrics to be printed at top of generated image")
				break
			}
			removed := make([]string, 0)
			for _, metricName := range selectList.Keys() {
				matchedOnce := false
				for _, piece := range pieces {
					trimmedPattern := strings.TrimSpace(piece)
					matched, err := regexp.MatchString(fmt.Sprintf("(?i)%v", trimmedPattern), metricName)
					if err != nil {
						NolintPrintln("Error matching input:", trimmedPattern, "Err:", err)
						continue
					}
					if matched {
						matchedOnce = true
						break
					}
				}
				if matchedOnce {
					removed = append(removed, metricName)
				} else {
					graphOptions.selectList.Set(metricName, true)
				}
			}
			NolintPrintln("Removed metrics from select list:", removed)
			NolintPrintln()
			NolintPrintln("New list of metrics to print at top of generated image:", graphOptions.selectList.Keys())
		case cmd == "show zeroes":
			graphOptions.hideAllZeroes = false
			NolintPrintln("Generating graphs without omitting plots with all zeroes")
		case cmd == "hide zeroes":
			graphOptions.hideAllZeroes = true
			NolintPrintln("Generating graphs omitting plots with all zeroes")
		case len(cmd) == 0:
			render = false
		default:
			NolintPrintln("Unknown command. Type `h` for help.")
			render = false
		}

		if render {
			gpw = renderPlot(data, graphOptions, logger)
		}
	}
}
