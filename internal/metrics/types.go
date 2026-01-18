package metrics

import (
	"sync"
	"time"
)

type Registry struct {
	mu      sync.RWMutex
	metrics map[time.Time]map[string]map[string]Metric // key0=timestamp, key1=namespace, key2=name
}

type MetricType string

const (
	Counter MetricType = "counter" // always increasing
	Gauge   MetricType = "gauge"   // can go up/down
	Summary MetricType = "summary" // avg/min/max
)

// Container for a metric and associated data
type Metric struct {
	Name        string // e.g. elapsed_time, queue_depth
	Description string
	Namespace   []string // e.g. "Receiver/Ingest/Listener/0"
	Value       MetricValue
	Type        MetricType
	Timestamp   time.Time // time when the metric was recorded
}

// Specific value of a metric
type MetricValue struct {
	Raw      interface{}   // uint64, float64
	Unit     string        // e.g., "ns", "bytes", "count"
	Interval time.Duration // measurement window, 1m, 5m, 15, ect
}

// JSON version
type JMetric struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Namespace   string       `json:"namespace"`
	Value       JMetricValue `json:"value"`
	Type        string       `json:"type"`
	Timestamp   string       `json:"timestamp"`
}

// Specific value of a metric
type JMetricValue struct {
	Raw      string `json:"raw"`
	Unit     string `json:"unit"`
	Interval string `json:"interval"`
}
