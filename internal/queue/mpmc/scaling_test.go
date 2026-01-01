package mpmc

import "testing"

func TestTrend(t *testing.T) {
	tests := []struct {
		name      string
		values    []uint64
		queueSize int
		wantUp    bool
		wantDown  bool
	}{
		// BASIC SCALE UP
		{
			name:      "Strong upward trend triggers scale-up",
			values:    []uint64{40, 50, 60, 75}, // occupancy=75% > 70
			queueSize: 100,
			wantUp:    true,
			wantDown:  false,
		},
		{
			name:      "Upward trend but last 3 not consistent = no scale",
			values:    []uint64{40, 50, 49, 75}, // break in trend at 49
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Upward trend but below threshold = no scale",
			values:    []uint64{10, 20, 30, 40}, // 40% < 70%
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// BASIC SCALE DOWN
		{
			name:      "Strong downward trend triggers scale-down",
			values:    []uint64{20, 18, 10, 5}, // occupancy=5% < 15
			queueSize: 100,
			wantUp:    false,
			wantDown:  true,
		},
		{
			name:      "Downward trend but last 3 not consistent = no scale",
			values:    []uint64{20, 18, 19, 10}, // break at 19
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Downward trend but above threshold = no scale",
			values:    []uint64{80, 60, 50, 40}, // 40% > 15%
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// NOISY BUT CONSISTENT
		{
			name:      "Noisy upward but consistently rising last 3 = not enough upward",
			values:    []uint64{30, 32, 31, 40, 80},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Noisy downward but consistently falling last 3 = scale-down",
			values:    []uint64{45, 48, 44, 30, 10},
			queueSize: 100,
			wantUp:    false,
			wantDown:  true,
		},

		// FLAT / NO TREND
		{
			name:      "Flat values = no scale",
			values:    []uint64{50, 50, 50, 50},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Flat near high threshold = no scale",
			values:    []uint64{69, 70, 70, 71}, // rising but not 3 consistent rises?
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// THRESHOLD EDGE CASES
		{
			name:      "Upward trend but exactly 70% = no scale (threshold is > 70)",
			values:    []uint64{60, 65, 68, 70},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Downward trend but exactly 15% = no scale (threshold is < 15)",
			values:    []uint64{20, 18, 17, 15},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// TREND LENGTH < 3
		{
			name:      "Upward but only last 2 consistent = no scale",
			values:    []uint64{10, 20, 15, 80},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Downward but only last 2 consistent = no scale",
			values:    []uint64{50, 49, 48, 47},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// TOO FEW SAMPLES
		{
			name:      "Only 1 value = no scale",
			values:    []uint64{80},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},
		{
			name:      "Only 2 values = no scale",
			values:    []uint64{80, 90},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// MIXED TRENDS
		{
			name:      "Mixed up/down = no scale",
			values:    []uint64{30, 50, 40, 60, 70},
			queueSize: 100,
			wantUp:    false,
			wantDown:  false,
		},

		// REALISTIC BURST
		{
			name:      "One-time jump upward does = scale up",
			values:    []uint64{20, 21, 22, 90},
			queueSize: 100,
			wantUp:    true,
			wantDown:  false,
		},
		{
			name:      "One-time drop downward = scale down",
			values:    []uint64{60, 59, 58, 5},
			queueSize: 100,
			wantUp:    false,
			wantDown:  true,
		},

		// QUEUE FULL / EMPTY
		{
			name:      "Queue completely full and rising = scale-up",
			values:    []uint64{80, 90, 95, 100}, // consistent rising
			queueSize: 100,
			wantUp:    true,
			wantDown:  false,
		},
		{
			name:      "Queue empty and shrinking = scale-down",
			values:    []uint64{10, 5, 2, 0}, // consistent falling
			queueSize: 100,
			wantUp:    false,
			wantDown:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up, down := Trend(tt.values, tt.queueSize)

			if up != tt.wantUp || down != tt.wantDown {
				t.Errorf("Trend(%v) = (up=%v, down=%v), want (up=%v, down=%v)",
					tt.values, up, down, tt.wantUp, tt.wantDown)
			}

			if up && down {
				t.Errorf("Invalid state: cannot scale up AND down simultaneously")
			}
		})
	}
}
