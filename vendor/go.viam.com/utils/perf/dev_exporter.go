package perf

// based on "go.opencensus.io/examples/exporter"

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
	"go.opencensus.io/trace"

	"go.viam.com/utils"
)

// developmentExporter exports metrics and span to log file.
type developmentExporter struct {
	mu             sync.Mutex
	children       map[string][]mySpanInfo
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              DevelopmentExporterOptions
}

// DevelopmentExporterOptions provides options for DevelopmentExporter.
type DevelopmentExporterOptions struct {
	// ReportingInterval is a time interval between two successive metrics
	// export.
	ReportingInterval time.Duration

	// MetricsDisabled determines if metrics reporting is disabled or not.
	MetricsDisabled bool

	// TracesDisabled determines if trace reporting is disabled or not.
	TracesDisabled bool
}

type mySpanInfo struct {
	toPrint string
	id      string
}

var reZero = regexp.MustCompile(`^0+$`)

// NewDevelopmentExporter creates a new log exporter.
func NewDevelopmentExporter() Exporter {
	return NewDevelopmentExporterWithOptions(DevelopmentExporterOptions{
		ReportingInterval: 10 * time.Second,
	})
}

// NewDevelopmentExporterWithOptions creates a new log exporter with the given options.
func NewDevelopmentExporterWithOptions(options DevelopmentExporterOptions) Exporter {
	return &developmentExporter{
		children: map[string][]mySpanInfo{},
		reader:   metricexport.NewReader(),
		o:        options,
	}
}

// Start starts the metric and span data exporter.
func (e *developmentExporter) Start() error {
	if err := registerApplicationViews(); err != nil {
		return err
	}

	if !e.o.TracesDisabled {
		trace.RegisterExporter(e)
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	}
	if !e.o.MetricsDisabled {
		e.initReaderOnce.Do(func() {
			var err error
			e.ir, err = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
			utils.UncheckedError(err)
		})
		e.ir.ReportingInterval = e.o.ReportingInterval
		return e.ir.Start()
	}
	return nil
}

// Stop stops the metric and span data exporter.
func (e *developmentExporter) Stop() {
	if !e.o.TracesDisabled {
		trace.UnregisterExporter(e)
	}
	if !e.o.MetricsDisabled {
		e.ir.Stop()
	}
}

// Close closes any files that were opened for logging.
func (e *developmentExporter) Close() {
}

// ExportMetrics exports to log.
func (e *developmentExporter) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	metricsTransform := make(map[string]interface{}, len(metrics))

	transformPoint := func(point metricdata.Point) interface{} {
		switch v := point.Value.(type) {
		case *metricdata.Distribution:
			dv := v
			return map[string]interface{}{
				"count":      dv.Count,
				"sum":        dv.Sum,
				"sum_sq_dev": dv.SumOfSquaredDeviation,
			}
		default:
			return point.Value
		}
	}

	for _, metric := range metrics {
		if len(metric.TimeSeries) == 0 {
			continue
		}
		if len(metric.Descriptor.LabelKeys) == 0 {
			if len(metric.TimeSeries) == 0 || len(metric.TimeSeries[0].Points) == 0 {
				continue
			}
			metricsTransform[metric.Descriptor.Name] = transformPoint(metric.TimeSeries[0].Points[0])
			continue
		}

		var pointVals []interface{}
		for _, ts := range metric.TimeSeries {
			if len(ts.Points) == 0 {
				continue
			}
			labels := make([][]string, 0, len(metric.Descriptor.LabelKeys))
			for idx, key := range metric.Descriptor.LabelKeys {
				labels = append(labels, []string{key.Key, ts.LabelValues[idx].Value})
			}
			if len(labels) == 1 {
				pointVals = append(pointVals, map[string]interface{}{
					strings.Join(labels[0], ":"): transformPoint(ts.Points[0]),
				})
				continue
			}
			pointVals = append(pointVals, map[string]interface{}{
				"labels": labels,
				"value":  transformPoint(ts.Points[0]),
			})
		}
		metricsTransform[metric.Descriptor.Name] = pointVals
	}
	md, err := json.MarshalIndent(metricsTransform, "", "  ")
	if err != nil {
		return err
	}
	log.Println(string(md))
	return nil
}

func (e *developmentExporter) printTree(root, padding string) {
	for _, s := range e.children[root] {
		log.Printf("%s %s\n", padding, s.toPrint)
		e.printTree(s.id, padding+"  ")
	}
	delete(e.children, root)
}

// ExportSpan exports a SpanData to log.
func (e *developmentExporter) ExportSpan(sd *trace.SpanData) {
	e.mu.Lock()
	defer e.mu.Unlock()

	length := (sd.EndTime.UnixNano() - sd.StartTime.UnixNano()) / (1000 * 1000)
	myinfo := fmt.Sprintf("%s %d ms", sd.Name, length)

	if sd.Annotations != nil {
		for _, a := range sd.Annotations {
			myinfo = myinfo + " " + a.Message
		}
	}

	spanID := hex.EncodeToString(sd.SpanID[:])
	parentSpanID := hex.EncodeToString(sd.ParentSpanID[:])

	if !reZero.MatchString(parentSpanID) {
		e.children[parentSpanID] = append(e.children[parentSpanID], mySpanInfo{myinfo, spanID})
		return
	}

	// i'm the top of the tree, go me
	log.Println(myinfo)
	e.printTree(hex.EncodeToString(sd.SpanContext.SpanID[:]), "  ")
}
