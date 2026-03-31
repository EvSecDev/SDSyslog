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

type BulkRequest struct {
	SearchFilters []MetricFilter `json:"filters"`
}

type MetricFilter struct {
	SearchType      string `json:"type"`
	AggregationType string `json:"aggregationType,omitempty"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	StartTime       string `json:"startTime"`
	EndTime         string `json:"endTime"`
}

// Metric functions passed in by the Send/Receive daemon from the metrics package
type AggSearcher func(aggType string, name string, namespace []string, start, end time.Time) (result metricGlb.Metric, err error)
type DataSearcher func(name string, namespacePrefix []string, start, end time.Time) []metricGlb.Metric
type Discoverer func(name, description string, namespacePrefix []string, unit string, metricType metricGlb.MetricType) []metricGlb.Metric
