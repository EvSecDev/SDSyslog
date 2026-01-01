package metrics

import (
	"fmt"
	"strings"
	"time"
)

// Converts internal metric type to export (JSON) metric
func (inMetric Metric) Convert() (outMetric JMetric) {
	// One to one conversions
	outMetric.Name = inMetric.Name
	outMetric.Description = inMetric.Description
	outMetric.Value.Unit = inMetric.Value.Unit

	// Simple conversions
	outMetric.Namespace = strings.Join(inMetric.Namespace, "/")
	outMetric.Type = string(inMetric.Type)
	outMetric.Value.Interval = inMetric.Value.Interval.String()

	// Format changes
	outMetric.Timestamp = inMetric.Timestamp.Format(time.RFC3339Nano)
	outMetric.Value.Raw = fmt.Sprintf("%v", inMetric.Value.Raw)
	return
}
