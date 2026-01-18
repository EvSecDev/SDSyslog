package metrics

import (
	"testing"
	"time"
)

func setupRegistryWithData(t *testing.T) (mockRegistry *Registry, mockedTimeSlices map[string]time.Time) {
	t.Helper()

	mockRegistry = New()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	interval := time.Minute

	ts1 := mockRegistry.NewTimeSlice(base.Add(0*time.Minute), interval)
	ts2 := mockRegistry.NewTimeSlice(base.Add(1*time.Minute), interval)
	ts3 := mockRegistry.NewTimeSlice(base.Add(2*time.Minute), interval)

	mockRegistry.Add(ts1, []Metric{
		// Base gauge
		{
			Name:        "queue_depth",
			Description: "queue depth",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Gauge,
			Timestamp:   ts1,
			Value: MetricValue{
				Raw:      uint64(10),
				Unit:     "count",
				Interval: interval,
			},
		},
		// Same name, different namespace
		{
			Name:        "queue_depth",
			Description: "queue depth http",
			Namespace:   []string{"Receiver", "HTTP"},
			Type:        Gauge,
			Timestamp:   ts1,
			Value: MetricValue{
				Raw:      5, // int
				Unit:     "count",
				Interval: interval,
			},
		},
		// Summary metric
		{
			Name:        "elapsed_time",
			Description: "processing time",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Summary,
			Timestamp:   ts1,
			Value: MetricValue{
				Raw:      100.0,
				Unit:     "ms",
				Interval: interval,
			},
		},
	})

	mockRegistry.Add(ts2, []Metric{
		// Gauge increases
		{
			Name:        "queue_depth",
			Description: "queue depth",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Gauge,
			Timestamp:   ts2,
			Value: MetricValue{
				Raw:      int64(20),
				Unit:     "count",
				Interval: interval,
			},
		},
		// Counter
		{
			Name:        "requests_total",
			Description: "requests processed",
			Namespace:   []string{"Receiver", "HTTP"},
			Type:        Counter,
			Timestamp:   ts2,
			Value: MetricValue{
				Raw:      uint64(5),
				Unit:     "count",
				Interval: interval,
			},
		},
		// Same metric, different unit (Discover test)
		{
			Name:        "elapsed_time",
			Description: "processing time",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Summary,
			Timestamp:   ts2,
			Value: MetricValue{
				Raw:      "150", // string numeric
				Unit:     "us",
				Interval: interval,
			},
		},
	})

	mockRegistry.Add(ts3, []Metric{
		// Negative gauge
		{
			Name:        "queue_depth",
			Description: "queue depth",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Gauge,
			Timestamp:   ts3,
			Value: MetricValue{
				Raw:      -5,
				Unit:     "count",
				Interval: interval,
			},
		},
		// Non-numeric value for aggregation error
		{
			Name:        "bad_metric",
			Description: "broken",
			Namespace:   []string{"Receiver", "Ingest"},
			Type:        Gauge,
			Timestamp:   ts3,
			Value: MetricValue{
				Raw:      struct{}{},
				Unit:     "count",
				Interval: interval,
			},
		},
	})

	mockedTimeSlices = map[string]time.Time{
		"ts1": ts1,
		"ts2": ts2,
		"ts3": ts3,
	}
	return
}
