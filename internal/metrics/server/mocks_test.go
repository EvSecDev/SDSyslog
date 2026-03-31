package server

import (
	"sdsyslog/internal/metrics"
	"time"
)

func mockDiscoverer(results []metrics.Metric) Discoverer {
	return func(name, desc string, ns []string, unit string, mt metrics.MetricType) []metrics.Metric {
		return results
	}
}

func mockDataSearcher(results []metrics.Metric) DataSearcher {
	return func(name string, ns []string, start, end time.Time) []metrics.Metric {
		return results
	}
}

func mockAggSearcher(result metrics.Metric, err error) AggSearcher {
	return func(agg, name string, ns []string, start, end time.Time) (metrics.Metric, error) {
		return result, err
	}
}
