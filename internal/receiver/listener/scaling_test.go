package listener

import "testing"

func TestTrend(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		wantUp   bool
		wantDown bool
	}{
		{
			name:     "Strong upward trend triggers scale-up",
			values:   []float64{52, 55, 60, 65, 70},
			wantUp:   true,
			wantDown: false,
		},
		{
			name:     "Strong downward trend triggers scale-down",
			values:   []float64{30, 25, 22, 15, 5},
			wantUp:   false,
			wantDown: true,
		},
		{
			name:     "Average high but slope flat = no scale",
			values:   []float64{55, 55, 56, 54, 55},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Average low but slope flat = no scale",
			values:   []float64{18, 20, 19, 21, 20},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Noisy upward trend still triggers scale-up",
			values:   []float64{50, 54, 53, 57, 60},
			wantUp:   true,
			wantDown: false,
		},
		{
			name:     "Noisy downward trend still triggers scale-down",
			values:   []float64{25, 22, 26, 20, 1},
			wantUp:   false,
			wantDown: true,
		},
		{
			name:     "Upward trend but avg < threshold = no scale-up",
			values:   []float64{10, 15, 18, 20, 25},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Downward trend but avg > threshold = no scale-down",
			values:   []float64{60, 55, 52, 50, 48},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Flat line = no scaling",
			values:   []float64{40, 40, 40, 40, 40},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Constant high but no trend = no scale-up",
			values:   []float64{75, 75, 75, 75, 75},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Constant low but no trend = no scale-down",
			values:   []float64{10, 10, 10, 10, 10},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Single value = cannot compute trend",
			values:   []float64{50},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Two identical values = zero slope denom",
			values:   []float64{30, 30},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Nearly flat but positive slope too small = no scale-up",
			values:   []float64{52.3, 52.8, 53, 53.1, 53.2},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Nearly flat but negative slope too small = no scale-down",
			values:   []float64{22, 21.8, 21.9, 21.7, 21.6},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Chaotic noise around threshold = no scaling",
			values:   []float64{48, 60, 52, 49, 55},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Huge upward spike but short trend = no scale",
			values:   []float64{20, 21, 22, 23, 80},
			wantUp:   false,
			wantDown: false,
		},
		{
			name:     "Huge downward spike but short trend = no scale",
			values:   []float64{80, 78, 77, 75, 10},
			wantUp:   false,
			wantDown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up, down := Trend(tt.values)

			if up != tt.wantUp || down != tt.wantDown {
				t.Errorf("Trend(%v) = (up=%v, down=%v), want (up=%v, down=%v)",
					tt.values, up, down, tt.wantUp, tt.wantDown)
			}

			// Sanity check: cannot scale up and down at same time
			if up && down {
				t.Errorf("Invalid result: scaleUp and scaleDown both true")
			}
		})
	}
}
