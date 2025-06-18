// Package perf exposes application performance utilities.
package perf

import (
	"context"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"

	"go.viam.com/utils"
)

const (
	envVarStackDriverProjectID = "STACKDRIVER_PROJECT_ID"
)

// Exporter wrapper around Trace and Metric exporter for OpenCensus.
type Exporter interface {
	// Start will start the exporting of metrics and return any errors if failed to start.
	Start() error
	// Stop will stop all exporting and flush remaining metrics.
	Stop()
}

// CloudOptions are options for the production cloud exporter to Stackdriver (Cloud Monitoring).
type CloudOptions struct {
	Context      context.Context
	Logger       utils.ZapCompatibleLogger
	MetricPrefix string // Optional metric prefix.
}

// NewCloudExporter creates a new Stackdriver (Cloud Monitoring) OpenCensus exporter with all options setup views registered..
func NewCloudExporter(opts CloudOptions) (Exporter, error) {
	sdOpts := stackdriver.Options{
		Context: opts.Context,
		OnError: func(err error) {
			opts.Logger.Errorw("opencensus stackdriver error", "error", err)
		},
		// ReportingInterval sets the frequency of reporting metrics to stackdriver backend.
		ReportingInterval: 60 * time.Second,
		MetricPrefix:      opts.MetricPrefix,
		// TraceSpansBufferMaxBytes sets the maximum buffer size to 50MB before spans are dropped.
		TraceSpansBufferMaxBytes: 50 << 20,
	}

	// Allow a custom stackdriver project.
	if os.Getenv(envVarStackDriverProjectID) != "" {
		sdOpts.ProjectID = os.Getenv(envVarStackDriverProjectID)
	}

	// For Cloud Run applications use
	// See: https://cloud.google.com/run/docs/container-contract#env-vars
	if os.Getenv("K_SERVICE") != "" {
		// Allow for local testing with GCP_COMPUTE_ZONE
		var err error
		zone := os.Getenv("GCP_COMPUTE_ZONE")
		if zone == "" {
			// Get from GCP Metadata
			if zone, err = metadata.ZoneWithContext(sdOpts.Context); err != nil {
				return nil, err
			}
		}

		// Allow for local testing with GCP_INSTANCE_ID
		instanceID := os.Getenv("GCP_INSTANCE_ID")
		if instanceID == "" {
			// Get from GCP Metadata
			if instanceID, err = metadata.InstanceIDWithContext(sdOpts.Context); err != nil {
				return nil, err
			}
		}

		// We're using GAE resource even though we're running on Cloud Run. GCP only allows
		// for a limited subset of resource types when creating custom metrics. The default "Global"
		// is vague, `generic_node` is better but doesn't have built in label for version/module.
		// GAE is essentially Cloud Run application under the hood and the resource labels with the
		// type match to Cloud Run. With a vague resource type we need to add labels on each metric
		// which makes the UI in Cloud Monitoring a little hard to reason about the labels on the
		// metric vs resource.
		//
		// See: https://cloud.google.com/monitoring/custom-metrics/creating-metrics#create-metric-desc
		sdOpts.MonitoredResource = &gaeResource{
			projectID:  os.Getenv(envVarStackDriverProjectID),
			module:     os.Getenv("K_SERVICE"),
			version:    os.Getenv("K_REVISION"),
			instanceID: instanceID,
			location:   zone,
		}
		sdOpts.DefaultMonitoringLabels = &stackdriver.Labels{}
	}

	sd, err := stackdriver.NewExporter(sdOpts)
	if err != nil {
		return nil, err
	}

	e := sdExporter{
		sdExporter: sd,
	}

	return &e, nil
}

type sdExporter struct {
	sdExporter *stackdriver.Exporter
}

// Starts the applications stats/span monitoring. Registers views and starts trace/metric exporters to opencensus.
func (e *sdExporter) Start() error {
	if err := registerApplicationViews(); err != nil {
		return err
	}

	if err := e.sdExporter.StartMetricsExporter(); err != nil {
		return err
	}
	trace.RegisterExporter(e.sdExporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	return nil
}

// Stop all exporting.
func (e *sdExporter) Stop() {
	e.sdExporter.StopMetricsExporter()
	trace.UnregisterExporter(e.sdExporter)
	e.sdExporter.Flush()
}

type gaeResource struct {
	projectID  string // GCP project ID
	module     string // GAE/Cloud Run app name
	version    string // GAE/Cloud Run app version
	instanceID string // unique id for task
	location   string // GCP zone
}

func (r *gaeResource) MonitoredResource() (resType string, labels map[string]string) {
	return "gae_instance", map[string]string{
		"project_id":  r.projectID,
		"module_id":   r.module,
		"version_id":  r.version,
		"instance_id": r.instanceID,
		"location":    r.location,
	}
}
