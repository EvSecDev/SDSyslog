package metrics

import (
	"reflect"
	"testing"
	"time"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    Metric
		expected JMetric
	}{
		{
			name: "basic metric all fields",
			input: Metric{
				Name:        "count",
				Description: "count of the items in the place",
				Namespace:   []string{"Receiver", "Ingest"},
				Value: MetricValue{
					Raw:      int(45),
					Unit:     "cnt",
					Interval: time.Second,
				},
				Type:      Counter,
				Timestamp: time.Date(2001, time.January, 1, 1, 1, 1, 1, time.UTC),
			},
			expected: JMetric{
				Name:        "count",
				Description: "count of the items in the place",
				Namespace:   "Receiver/Ingest",
				Value: JMetricValue{
					Raw:      "45",
					Unit:     "cnt",
					Interval: "1s",
				},
				Type:      "counter",
				Timestamp: "2001-01-01T01:01:01.000000001Z",
			},
		},
		{
			name: "empty namespace",
			input: Metric{
				Name:      "empty_ns",
				Namespace: nil,
				Value: MetricValue{
					Raw:      1,
					Interval: time.Second,
				},
				Type:      Gauge,
				Timestamp: time.Unix(0, 0).UTC(),
			},
			expected: JMetric{
				Name:      "empty_ns",
				Namespace: "",
				Value: JMetricValue{
					Raw:      "1",
					Interval: "1s",
				},
				Type:      "gauge",
				Timestamp: "1970-01-01T00:00:00Z",
			},
		},
		{
			name: "single namespace element",
			input: Metric{
				Name:      "single_ns",
				Namespace: []string{"Only"},
				Value: MetricValue{
					Raw:      float64(3.14),
					Interval: time.Millisecond * 500,
				},
				Type:      Gauge,
				Timestamp: time.Unix(10, 0).UTC(),
			},
			expected: JMetric{
				Name:      "single_ns",
				Namespace: "Only",
				Value: JMetricValue{
					Raw:      "3.14",
					Interval: "500ms",
				},
				Type:      "gauge",
				Timestamp: "1970-01-01T00:00:10Z",
			},
		},
		{
			name: "raw string value",
			input: Metric{
				Name: "string_raw",
				Value: MetricValue{
					Raw:      "already-a-string",
					Interval: time.Second,
				},
				Type:      Counter,
				Timestamp: time.Unix(1, 0).UTC(),
			},
			expected: JMetric{
				Name: "string_raw",
				Value: JMetricValue{
					Raw:      "already-a-string",
					Interval: "1s",
				},
				Type:      "counter",
				Timestamp: "1970-01-01T00:00:01Z",
			},
		},
		{
			name: "raw boolean value",
			input: Metric{
				Name: "bool_raw",
				Value: MetricValue{
					Raw:      true,
					Interval: 0,
				},
				Type:      Gauge,
				Timestamp: time.Time{},
			},
			expected: JMetric{
				Name: "bool_raw",
				Value: JMetricValue{
					Raw:      "true",
					Interval: "0s",
				},
				Type:      "gauge",
				Timestamp: "0001-01-01T00:00:00Z",
			},
		},
		{
			name:  "mostly empty metric",
			input: Metric{},
			expected: JMetric{
				Namespace: "",
				Value: JMetricValue{
					Raw:      "<nil>",
					Interval: "0s",
				},
				Type:      "",
				Timestamp: "0001-01-01T00:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.input.Convert()

			if !reflect.DeepEqual(tt.expected, output) {
				t.Fatalf("expected does not match test output:\nExpected: %v\nGot:      %v\n", tt.expected, output)
			}
		})
	}
}
