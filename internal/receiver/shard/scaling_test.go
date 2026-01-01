package shard

import "testing"

func TestTrend(t *testing.T) {
	tests := []struct {
		name      string
		rawValues []uint64
		wantUp    bool
		wantDown  bool
	}{
		// Basic cases
		{
			name:      "insufficient data",
			rawValues: []uint64{100},
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "steady upward trend",
			rawValues: []uint64{100, 106, 113, 120},
			wantUp:    true,
			wantDown:  false,
		},
		{
			name:      "steady downward trend",
			rawValues: []uint64{120, 118, 115, 110},
			wantUp:    false,
			wantDown:  true,
		},
		{
			name:      "flat trend",
			rawValues: []uint64{100, 101, 100, 101},
			wantUp:    false,
			wantDown:  false,
		},

		// Spike-resistance tests
		{
			name:      "single positive spike",
			rawValues: []uint64{100, 101, 150, 102, 103},
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "single negative spike",
			rawValues: []uint64{100, 101, 90, 102, 103},
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "multiple spikes up and down",
			rawValues: []uint64{100, 130, 102, 160, 104, 105},
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "small upward trend with spike",
			rawValues: []uint64{100, 102, 150, 104, 106},
			wantUp:    false, // spike ignored, underlying trend < threshold
			wantDown:  false,
		},
		{
			name:      "strong upward trend above threshold",
			rawValues: []uint64{100, 110, 120, 135},
			wantUp:    true,
			wantDown:  false,
		},
		{
			name:      "strong downward trend above threshold",
			rawValues: []uint64{150, 140, 130, 115},
			wantUp:    false,
			wantDown:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUp, gotDown := Trend(tt.rawValues)
			if gotUp != tt.wantUp || gotDown != tt.wantDown {
				t.Errorf("Trend(%v) = (up=%v, down=%v), want (up=%v, down=%v)",
					tt.rawValues, gotUp, gotDown, tt.wantUp, tt.wantDown)
			}
		})
	}
}

func TestTrendLatency(t *testing.T) {
	tests := []struct {
		name              string
		sumSpacing        []uint64
		totalFragments    []uint64
		timedOutFragments []uint64
		wantUp            bool
		wantDown          bool
	}{
		// Basic cases
		{
			name:              "insufficient data",
			sumSpacing:        []uint64{140, 80},
			totalFragments:    []uint64{5},
			timedOutFragments: []uint64{0},
			wantUp:            false,
			wantDown:          false,
		},
		{
			name:              "steady upward trend",
			sumSpacing:        []uint64{20, 25, 29, 34, 45},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{0, 4, 7, 11, 24},
			wantUp:            true,
			wantDown:          false,
		},
		{
			name:              "steady downward trend",
			sumSpacing:        []uint64{50, 45, 40, 35, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{2, 0, 0, 0, 0},
			wantUp:            false,
			wantDown:          true,
		},
		{
			name:              "flat trend",
			sumSpacing:        []uint64{30, 30, 30, 30, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{1, 1, 1, 1, 1},
			wantUp:            false,
			wantDown:          false,
		},
		{
			name:              "single positive spike",
			sumSpacing:        []uint64{30, 30, 60, 30, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{1, 1, 5, 1, 1},
			wantUp:            false,
			wantDown:          false,
		},
		{
			name:              "single negative spike",
			sumSpacing:        []uint64{30, 30, 10, 30, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{1, 1, 0, 1, 1},
			wantUp:            false,
			wantDown:          false,
		},
		{
			name:              "multiple spikes up and down",
			sumSpacing:        []uint64{30, 60, 30, 10, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{1, 5, 1, 0, 1},
			wantUp:            false,
			wantDown:          false,
		},
		{
			name:              "small upward trend with spike",
			sumSpacing:        []uint64{30, 31, 33, 32, 34},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{0, 1, 1, 1, 2},
			wantUp:            true,
			wantDown:          false,
		},
		{
			name:              "strong upward trend above threshold",
			sumSpacing:        []uint64{20, 25, 30, 40, 50},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{1, 3, 5, 10, 20},
			wantUp:            true,
			wantDown:          false,
		},
		{
			name:              "strong downward trend above threshold",
			sumSpacing:        []uint64{50, 45, 40, 35, 30},
			totalFragments:    []uint64{200, 200, 200, 200, 200},
			timedOutFragments: []uint64{0, 0, 0, 0, 0},
			wantUp:            false,
			wantDown:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUp, gotDown := TrendLatency(tt.sumSpacing, tt.totalFragments, tt.timedOutFragments)
			if gotUp != tt.wantUp || gotDown != tt.wantDown {
				t.Errorf("got (up=%v, down=%v), (want up=%v, down=%v)",
					gotUp, gotDown, tt.wantUp, tt.wantDown)
			}
		})
	}
}
