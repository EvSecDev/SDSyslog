package metrics

import (
	"sdsyslog/internal/global"
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
		{"all metrics", "", nil, time.Time{}, time.Time{}, 8},
		{"exact name only", "queue", nil, time.Time{}, time.Time{}, 0},
		{"queue_depth all namespaces", "queue_depth", nil, time.Time{}, time.Time{}, 4},
		{"queue_depth ingest only", "queue_depth", []string{"Receiver", "Ingest"}, time.Time{}, time.Time{}, 3},
		{"namespace prefix Receiver", "", []string{"Receiver"}, time.Time{}, time.Time{}, 8},
		{"negative value included", "queue_depth", []string{"Receiver", "Ingest"}, ts["ts3"], ts["ts3"], 1},
		{"time window exact bounds", "", nil, ts["ts2"], ts["ts3"], 5},
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
		name      string
		aggType   string
		metric    string
		want      float64
		wantError bool
	}{
		{"sum mixed types", global.MetricSum, "queue_depth", 25, false}, // 10 + 20 + (-5)
		{"min negative", global.MetricMin, "queue_depth", -5, false},
		{"max mixed types", global.MetricMax, "queue_depth", 20, false},
		{"avg mixed types", global.MetricAvg, "queue_depth", 25.0 / 3.0, false},
		{"string numeric aggregation", global.MetricSum, "elapsed_time", 250, false},
		{"non-numeric error", global.MetricSum, "bad_metric", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reg.Aggregate(
				tt.aggType,
				tt.metric,
				[]string{"Receiver", "Ingest"},
				ts["ts1"],
				ts["ts3"],
			)

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
		global.MetricSum,
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
		{"all", "", "", nil, 6},
		{"elapsed_time both units", "ms", "", nil, 1},
		{"counter only", "", Counter, nil, 1},
		{"ingest namespace only", "", "", []string{"Receiver", "Ingest"}, 4},
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
