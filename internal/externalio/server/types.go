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

// Metric functions passed in by the Send/Receive daemon from the metrics package
type AggSearcher func(aggType string, name string, namespace []string, start, end time.Time) (result metricGlb.Metric, err error)
type DataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
type Discoverer func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
