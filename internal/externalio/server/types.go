package server

import (
	"context"
	metricGlb "sdsyslog/internal/metrics"
	"time"
)

type httpLogWriter struct {
	ctx context.Context
}

type Jerror struct {
	Msg string `json:"error"`
}

type DataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
type Discoverer func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
