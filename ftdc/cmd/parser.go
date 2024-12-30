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
	"slices"
	"strings"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/logging"
)

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
	// metricFiles contain the actual data points to be graphed. A "top level" gnuplot will
	// reference them.
	metricFiles map[string]*os.File

	tempdir string

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
}

func defaultGraphOptions() graphOptions {
	return graphOptions{
		minTimeSeconds: 0,
		maxTimeSeconds: math.MaxInt64,
	}
}

func nolintPrintln(str ...any) {
	// This is a CLI. It's acceptable to output to stdout.
	//nolint:forbidigo
	fmt.Println(str...)
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

func newGnuPlotWriter(graphOptions graphOptions) *gnuplotWriter {
	tempdir, err := os.MkdirTemp("", "ftdc_parser")
	if err != nil {
		panic(err)
	}

	return &gnuplotWriter{
		metricFiles: make(map[string]*os.File),
		tempdir:     tempdir,
		options:     graphOptions,
	}
}

func (gpw *gnuplotWriter) getDatafile(metricName string) io.Writer {
	if datafile, created := gpw.metricFiles[metricName]; created {
		return datafile
	}

	datafile, err := os.CreateTemp(gpw.tempdir, "")
	if err != nil {
		panic(err)
	}
	gpw.metricFiles[metricName] = datafile

	return datafile
}

func (gpw *gnuplotWriter) addPoint(timeSeconds int64, metricName string, metricValue float32) {
	if timeSeconds < gpw.options.minTimeSeconds || timeSeconds > gpw.options.maxTimeSeconds {
		fmt.Println("Returning:", metricName)
		return
	}

	writelnf(gpw.getDatafile(metricName), "%v %.5f", timeSeconds, metricValue)
}

type RatioMetric struct {
	Numerator string
	// An empty string Denominator will use the datum read timestamp value for its denominator.
	Denominator string
}

var ratioMetricToFields map[string]RatioMetric = map[string]RatioMetric{
	"UserCPU":   RatioMetric{"UserCPUSecs", "ElapsedTimeSecs"},
	"SystemCPU": RatioMetric{"SystemCPUSecs", "ElapsedTimeSecs"},
	// PerSec ratios use an empty string denominator.
	"HeadersProcessedPerSec": RatioMetric{"HeadersProcessed", ""},
}

type RatioReading struct {
	GraphName string
	// Seconds since epoch.
	Time        int64
	Numerator   float32
	Denominator float64
	IsRate      bool
}

func (rr RatioReading) AsPercentage() float32 {
	fmt.Println("As percentage. Metric:", rr.GraphName, "Val:", float32(float64(rr.Numerator)/rr.Denominator*100))
	return float32(float64(rr.Numerator) / rr.Denominator * 100)
}

func (rr RatioReading) AsRate() float32 {
	fmt.Println("As rate. Metric:", rr.GraphName, "Val:", float32(float64(rr.Numerator)/rr.Denominator*100))
	return float32(float64(rr.Numerator) / rr.Denominator)
}

func (rr *RatioReading) Diff(other *RatioReading) RatioReading {
	return RatioReading{
		rr.GraphName,
		rr.Time,
		rr.Numerator - other.Numerator,
		rr.Denominator - other.Denominator,
		rr.IsRate,
	}
}

var globalRatios map[string][]RatioMetric

func pullRatios(reading ftdc.Reading, readingTs int64, ratioGraphs map[string]*RatioReading, ratioMetrics map[string]RatioMetric) bool {
	ret := false
	for ratioMetricName, ratioMetric := range ratioMetrics {
		if strings.HasSuffix(reading.MetricName, ratioMetric.Numerator) {
			ret = true

			// `metricIdentifier` is expected to be of the form `rdk.foo_module.`. Leave the
			// trailing dot as we would be about to re-add it.
			metricIdentifier := strings.TrimSuffix(reading.MetricName, ratioMetric.Numerator)
			// E.g: `rdk.foo_module.User CPU%'.
			graphName := fmt.Sprint(metricIdentifier, ratioMetricName)
			if _, exists := ratioGraphs[graphName]; !exists {
				ratioGraphs[graphName] = &RatioReading{GraphName: graphName, Time: readingTs, IsRate: ratioMetric.Denominator == ""}
			}

			ratioGraphs[graphName].Numerator = reading.Value
			if ratioMetric.Denominator == "" {
				ratioGraphs[graphName].Denominator = float64(readingTs)
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
			if _, exists := ratioGraphs[graphName]; !exists {
				ratioGraphs[graphName] = &RatioReading{GraphName: graphName, Time: readingTs, IsRate: false}
			}

			ratioGraphs[graphName].Denominator = float64(reading.Value)
			continue
		}
	}

	return ret
}

func (gpw *gnuplotWriter) addFlatDatum(datum ftdc.FlatDatum) map[string]*RatioReading {
	ratioGraphs := make(map[string]*RatioReading)

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
		isRatioMetric := pullRatios(reading, datum.ConvertedTime().Unix(), ratioGraphs, ratioMetricToFields)
		if isRatioMetric {
			continue
		}

		gpw.addPoint(datum.ConvertedTime().Unix(), reading.MetricName, reading.Value)
	}

	// for metricName, ratioReading := range ratioGraphs {
	//  	gpw.addPoint(ratioReading.Time, metricName, ratioReading.AsPercentage())
	// }
	return ratioGraphs
}

// Ratios are averaged over a "recent history". This window size refers to a time in seconds, but we
// actually measure with respect to consecutive FTDC readings. The output value will use the system
// clock difference to compute a correct rate. We just accept there may be fuzziness with respect to
// how recent of a history we're actually using.
//
// Consider adding logging when two FTDC readings `windowSizeSecs` apart is not reflecting by their
// system time difference.
const windowSizeSecs = 5

func (gpw *gnuplotWriter) writeDeferredValues(deferredValues []map[string]*RatioReading) {
	for idx, currReadings := range deferredValues {
		if idx < windowSizeSecs {
			continue
		}

		prevReadings := deferredValues[idx-windowSizeSecs]
		for metricName, currRatioReading := range currReadings {
			var diff RatioReading
			if prevRatioReading, exists := prevReadings[metricName]; exists {
				diff = currRatioReading.Diff(prevRatioReading)
			} else {
				continue
			}

			if diff.IsRate {
				gpw.addPoint(currRatioReading.Time, metricName, diff.AsRate())
			} else {
				gpw.addPoint(currRatioReading.Time, metricName, diff.AsPercentage())
			}
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

	// Write a png with width of 1000 pixels and 200 pixels of height per metric/graph.
	writelnf(gnuFile, "set term png size %d, %d", 1000, 200*len(gpw.metricFiles))

	nolintPrintln("Output file: `plot.png`")
	// The output filename
	writeln(gnuFile, "set output 'plot.png'")

	// We're making separate graphs instead of a single big graph. The graphs will be arranged in a
	// rectangle with 1 column and X rows. Where X is the number of metrics.  Add some margins for
	// aesthetics.
	writelnf(gnuFile, "set multiplot layout %v,1 margins 0.05,0.9, 0.05,0.9 spacing screen 0, char 5", len(gpw.metricFiles))

	//  Axis labeling/formatting/type information.
	writeln(gnuFile, "set timefmt '%s'")
	writeln(gnuFile, "set format x '%H:%M:%S'")
	writeln(gnuFile, "set xlabel 'Time'")
	writeln(gnuFile, "set xdata time")

	// FTDC does not have negative numbers, so start the Y-axis at 0. Except that some metrics may
	// want to be negative like position or voltages? Revisit if this can be more granular as a
	// per-graph setting rather than a global.
	writeln(gnuFile, "set yrange [0:*]")

	for _, nameFilePair := range sorted(gpw.metricFiles) {
		metricName, file := nameFilePair.Key, nameFilePair.Val
		writelnf(gnuFile, "plot '%v' using 1:2 with lines linestyle 7 lw 4 title '%v'", file.Name(), strings.ReplaceAll(metricName, "_", "\\_"))
		utils.UncheckedErrorFunc(file.Close)
	}

	return gnuFile.Name()
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
			deferredValues := make([]map[string]*RatioReading, 0)
			gpw := newGnuPlotWriter(graphOptions)
			for _, flatDatum := range data {
				deferredValues = append(deferredValues, gpw.addFlatDatum(flatDatum))
			}

			gpw.writeDeferredValues(deferredValues)

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
			nolintPrintln("`quit` or Ctrl-d to exit")
		case strings.HasPrefix(cmd, "range "):
			pieces := strings.SplitN(cmd, " ", 3)
			// TrimSpace to remove the newline.
			start, end := pieces[1], pieces[2]

			if start == "start" {
				graphOptions.minTimeSeconds = 0
			} else {
				goTime, err := time.Parse("2006-01-02T15:04:05", start)
				if err != nil {
					// This is a CLI. It's acceptable to output to stdout.
					//nolint:forbidigo
					fmt.Printf("Error parsing start time. Working example: `2024-09-24T18:00:00` Inp: %q Err: %v\n", start, err)
					continue
				}
				graphOptions.minTimeSeconds = goTime.Unix()
			}

			if end == "end" {
				graphOptions.maxTimeSeconds = math.MaxInt64
			} else {
				goTime, err := time.Parse("2006-01-02T15:04:05", end)
				if err != nil {
					// This is a CLI. It's acceptable to output to stdout.
					//nolint:forbidigo
					fmt.Printf("Error parsing end time. Working example: `2024-09-24T18:00:00` Inp: %q Err: %v\n", end, err)
					continue
				}
				graphOptions.maxTimeSeconds = goTime.Unix()
			}
		case strings.HasPrefix(cmd, "reset range"):
			graphOptions.minTimeSeconds = 0
			graphOptions.maxTimeSeconds = math.MaxInt64
		case cmd == "refresh" || cmd == "r":
			fmt.Println("Refreshing graphs with new data")
		case len(cmd) == 0:
			render = false
		default:
			nolintPrintln("Unknown command. Type `h` for help.")
			render = false
		}
	}
}
