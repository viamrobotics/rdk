// main provides a CLI tool for viewing `.ftdc` files emitted by the `viam-server`.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.viam.com/utils"

	"go.viam.com/rdk/ftdc"
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

func newGnuPlotWriter() *gnuplotWriter {
	tempdir, err := os.MkdirTemp("", "ftdc_parser")
	if err != nil {
		panic(err)
	}

	return &gnuplotWriter{
		metricFiles: make(map[string]*os.File),
		tempdir:     tempdir,
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

func (gpw *gnuplotWriter) addPoint(timeInt int64, metricName string, metricValue float32) {
	writelnf(gpw.getDatafile(metricName), "%v %.2f", timeInt, metricValue)
}

func (gpw *gnuplotWriter) addFlatDatum(datum ftdc.FlatDatum) {
	for _, reading := range datum.Readings {
		gpw.addPoint(datum.Time, reading.MetricName, reading.Value)
	}
}

// RenderAndClose writes out the "top-level" file and closes all file handles.
func (gpw *gnuplotWriter) RenderAndClose() {
	gnuFile, err := os.CreateTemp(gpw.tempdir, "main")
	if err != nil {
		panic(err)
	}
	defer utils.UncheckedErrorFunc(gnuFile.Close)

	// We are a CLI, it's appropriate to write to stdout.
	//
	//nolint:forbidigo
	fmt.Println("GNUPlot File:", gnuFile.Name())
	writelnf(gnuFile, "set term png size %d, %d", 1000, 200*len(gpw.metricFiles))
	writeln(gnuFile, "set output 'plot.png'")
	writelnf(gnuFile, "set multiplot layout %v,1 margins 0.05,0.9, 0.05,0.9 spacing screen 0, char 5", len(gpw.metricFiles))
	writeln(gnuFile, "set timefmt '%s'")
	writeln(gnuFile, "set format x '%H:%M:%S'")
	writeln(gnuFile, "set xlabel 'Time'")
	writeln(gnuFile, "set xdata time")
	writeln(gnuFile, "set yrange [0:*]")

	for metricName, file := range gpw.metricFiles {
		writelnf(gnuFile, "plot '%v' using 1:2 with lines linestyle 7 lw 4 title '%v'", file.Name(), strings.ReplaceAll(metricName, "_", "\\_"))
		utils.UncheckedErrorFunc(file.Close)
	}
}

func main() {
	if len(os.Args) < 2 {
		// We are a CLI, it's appropriate to write to stdout.
		//
		//nolint:forbidigo
		fmt.Println("Expected an FTDC filename. E.g: go run parser.go <path-to>/viam-server.ftdc")
		return
	}

	ftdcFile, err := os.Open(os.Args[1])
	if err != nil {
		// We are a CLI, it's appropriate to write to stdout.
		//
		//nolint:forbidigo
		fmt.Println("Error opening file. File:", os.Args[1], "Err:", err)
		//nolint:forbidigo
		fmt.Println("Expected an FTDC filename. E.g: go run parser.go <path-to>/viam-server.ftdc")
		return
	}

	data, err := ftdc.Parse(ftdcFile)
	if err != nil {
		panic(err)
	}

	gpw := newGnuPlotWriter()
	for _, flatDatum := range data {
		gpw.addFlatDatum(flatDatum)
	}

	gpw.RenderAndClose()
}
