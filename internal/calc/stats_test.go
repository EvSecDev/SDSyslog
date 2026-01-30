package calc

import "testing"

func TestTrimmedMeanUint64(t *testing.T) {
	tests := []struct {
		name        string
		values      []uint64
		trimPercent float64
		want        uint64
	}{
		{
			name:        "empty slice",
			values:      nil,
			trimPercent: 0.1,
			want:        0,
		},
		{
			name:        "no trimming",
			values:      []uint64{1, 2, 3, 4},
			trimPercent: 0,
			want:        2, // (1+2+3+4)/4 = 2
		},
		{
			name:        "simple trimming",
			values:      []uint64{1, 2, 3, 100},
			trimPercent: 0.25,
			want:        2, // trim 1 from each side -> {2,3}
		},
		{
			name:        "trim percent too large",
			values:      []uint64{10, 20, 30},
			trimPercent: 0.5,
			want:        20, // only middle remains
		},
		{
			name:        "negative trim percent treated as zero",
			values:      []uint64{5, 5, 5},
			trimPercent: -1,
			want:        5,
		},
		{
			name:        "outlier removed",
			values:      []uint64{10, 11, 12, 1000},
			trimPercent: 0.25,
			want:        11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimmedMeanUint64(tt.values, tt.trimPercent)
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestTrimmedMeanFloat64(t *testing.T) {
	tests := []struct {
		name        string
		values      []float64
		trimPercent float64
		want        float64
	}{
		{
			name:        "empty slice",
			values:      nil,
			trimPercent: 0.1,
			want:        0,
		},
		{
			name:        "no trimming",
			values:      []float64{1, 2, 3, 4},
			trimPercent: 0,
			want:        2.5,
		},
		{
			name:        "simple trimming",
			values:      []float64{1, 2, 3, 100},
			trimPercent: 0.25,
			want:        2.5,
		},
		{
			name:        "trim percent too large",
			values:      []float64{10, 20, 30},
			trimPercent: 0.5,
			want:        20,
		},
		{
			name:        "negative trim percent treated as zero",
			values:      []float64{5, 5, 5},
			trimPercent: -1,
			want:        5,
		},
		{
			name:        "outlier removed",
			values:      []float64{10, 11, 12, 1000},
			trimPercent: 0.25,
			want:        11.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimmedMeanFloat64(tt.values, tt.trimPercent)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
