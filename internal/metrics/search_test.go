package metrics

import (
	"sdsyslog/internal/tests/utils"
	"slices"
	"testing"
	"time"
)

func TestRegistry_Search(t *testing.T) {
	reg, ts := setupRegistryWithData(t)

	tests := []struct {
		name            string
		metricName      string
		namespacePrefix []string
		start           time.Time
		end             time.Time
		want            int
	}{
		{"all metrics", "", nil, time.Time{}, time.Time{}, 12},
		{"exact name only", "queue", nil, time.Time{}, time.Time{}, 0},
		{"queue_depth all namespaces", "queue_depth", nil, time.Time{}, time.Time{}, 4},
		{"queue_depth ingest only", "queue_depth", []string{"Receiver", "Ingest"}, time.Time{}, time.Time{}, 3},
		{"namespace prefix Receiver", "", []string{"Receiver"}, time.Time{}, time.Time{}, 12},
		{"negative value included", "queue_depth", []string{"Receiver", "Ingest"}, ts["ts3"], ts["ts3"], 1},
		{"time window exact bounds", "", nil, ts["ts2"], ts["ts3"], 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := reg.Search(tt.metricName, tt.namespacePrefix, tt.start, tt.end)
			if len(results) != tt.want {
				t.Fatalf("expected %d results, got %d", tt.want, len(results))
			}
		})
	}
}

func TestRegistry_Aggregate(t *testing.T) {
	reg, ts := setupRegistryWithData(t)

	tests := []struct {
		name          string
		aggType       string
		metric        string
		namespace     []string
		wantNamespace []string
		want          float64
		wantError     string
	}{
		{
			name:          "sum mixed types",
			aggType:       MetricSum,
			metric:        "queue_depth",
			namespace:     []string{"Receiver", "Ingest"},
			wantNamespace: []string{"Receiver", "Ingest"},
			want:          25, // 10 + 20 + (-5)
		},
		{
			name:          "min negative",
			aggType:       MetricMin,
			metric:        "queue_depth",
			namespace:     []string{"Receiver", "Ingest"},
			wantNamespace: []string{"Receiver", "Ingest"},
			want:          -5,
		},
		{
			name:          "max mixed types",
			aggType:       MetricMax,
			metric:        "queue_depth",
			namespace:     []string{"Receiver", "Ingest"},
			wantNamespace: []string{"Receiver", "Ingest"},
			want:          20,
		},
		{
			name:          "avg mixed types",
			aggType:       MetricAvg,
			metric:        "queue_depth",
			namespace:     []string{"Receiver", "Ingest"},
			wantNamespace: []string{"Receiver", "Ingest"},
			want:          25.0 / 3.0,
		},
		{
			name:          "string numeric aggregation",
			aggType:       MetricSum,
			metric:        "elapsed_time",
			namespace:     []string{"Receiver", "Ingest"},
			wantNamespace: []string{"Receiver", "Ingest"},
			want:          250,
		},
		{
			name:          "namespace collapse",
			aggType:       MetricSum,
			metric:        "pop_count",
			namespace:     []string{"Receiver"},
			wantNamespace: []string{"Receiver"},
			want:          10,
		},
		{
			name:      "non-numeric error",
			aggType:   MetricSum,
			metric:    "bad_metric",
			namespace: []string{"Receiver", "Ingest"},
			want:      0,
			wantError: "non-numeric value for aggregation sum",
		},
		{
			name:      "unsupported cross-metric-type agg",
			aggType:   MetricSum,
			metric:    "latency",
			namespace: []string{"Receiver"},
			want:      0,
			wantError: "cannot aggregate metrics of different units",
		},
		{
			name:      "unsupported agg type",
			aggType:   "p99",
			metric:    "queue_depth",
			namespace: []string{"Receiver", "Ingest"},
			want:      0,
			wantError: "unsupported aggregation type: p99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reg.Aggregate(
				tt.aggType,
				tt.metric,
				tt.namespace,
				ts["ts1"],
				ts["ts3"],
			)
			matches, err := utils.MatchErrorString(err, tt.wantError)
			if err != nil {
				t.Fatalf("%v", err)
			} else if matches {
				return
			}

			if !slices.Equal(result.Namespace, tt.wantNamespace) {
				t.Fatalf("expected namespace %v but got namespace %v", result.Namespace, tt.wantNamespace)
			}

			if result.Value.Raw != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, result.Value.Raw)
			}
		})
	}
}

func TestRegistry_Aggregate_NoResults(t *testing.T) {
	reg, _ := setupRegistryWithData(t)
	_, err := reg.Aggregate(
		MetricSum,
		"missing",
		[]string{"Receiver"},
		time.Time{},
		time.Time{},
	)

	if err == nil {
		t.Fatalf("expected error for empty aggregation result")
	}
}

func TestRegistry_Discover(t *testing.T) {
	reg, _ := setupRegistryWithData(t)

	tests := []struct {
		name      string
		unit      string
		mType     MetricType
		ns        []string
		wantCount int
	}{
		{"all", "", "", nil, 9},
		{"all2", "", "", []string{}, 9},
		{"elapsed_time both units", "ms", "", nil, 2},
		{"counter only", "", Counter, nil, 1},
		{"ingest namespace only", "", "", []string{"Receiver", "Ingest"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := reg.Discover("", "", tt.ns, tt.unit, tt.mType)
			if len(results) != tt.wantCount {
				t.Fatalf("expected %d results, got %d", tt.wantCount, len(results))
			}
		})
	}
}
