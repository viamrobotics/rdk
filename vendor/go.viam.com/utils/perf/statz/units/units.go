// Package units contains unit definitions for statz
package units

// Unit is a determinate standard quantity of measurement.
type Unit string

// Units to give hint to Cloud Monitoring.
// See units from: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricDescriptor
const (
	Dimensionless Unit = "1"
	Bytes         Unit = "By"
	Bit           Unit = "bit"
	Milliseconds  Unit = "ms"
	Microseconds  Unit = "us"
	Second        Unit = "s"
	Minute        Unit = "min"
	Hour          Unit = "h"
	Day           Unit = "d"
)
