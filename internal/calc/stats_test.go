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
			name:        "no trimming exact division",
			values:      []uint64{1, 2, 3},
			trimPercent: 0,
			want:        2, // (1+2+3)/3 = 2
		},
		{
			name:        "truncates fractional result",
			values:      []uint64{1, 2},
			trimPercent: 0,
			want:        1, // (1+2)/2 = 1 (truncated from 1.5)
		},
		{
			name:        "trimming with truncation",
			values:      []uint64{1, 2, 3, 100},
			trimPercent: 0.25,
			want:        2, // trim 1 from each side -> {2,3} -> 5/2 = 2 (truncated from 2.5)
		},
		{
			name:        "trim percent too large",
			values:      []uint64{10, 20, 30},
			trimPercent: 0.5,
			want:        20, // only middle remains
		},
		{
			name:        "negative trim percent treated as zero",
			values:      []uint64{7, 8, 9},
			trimPercent: -1,
			want:        8, // (7+8+9)/3 = 8
		},
		{
			name:        "large values overflow-safe",
			values:      []uint64{1000000, 2000000, 3000000, 4000000},
			trimPercent: 0.25,
			want:        2500000, // {2000000, 3000000} -> 5000000/2
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
			want:        2.5, // (1+2+3+4)/4 = 2.5
		},
		{
			name:        "fractional inputs",
			values:      []float64{1.5, 2.5, 3.5, 4.5},
			trimPercent: 0,
			want:        3, // (1.5+2.5+3.5+4.5)/4 = 3.0
		},
		{
			name:        "fractional trim result",
			values:      []float64{1, 2, 3, 100},
			trimPercent: 0.25,
			want:        2.5, // trim 1 from each side -> {2,3} -> 5/2 = 2.5
		},
		{
			name:        "trim percent too large",
			values:      []float64{10.5, 20.5, 30.5},
			trimPercent: 0.5,
			want:        20.5, // only middle remains
		},
		{
			name:        "negative trim percent treated as zero",
			values:      []float64{1.1, 2.2, 3.3},
			trimPercent: -1,
			want:        2.2, // (1.1+2.2+3.3)/3 = 2.2
		},
		{
			name:        "outlier removed fractional mean",
			values:      []float64{10, 11, 12, 1000},
			trimPercent: 0.25,
			want:        11.5, // {11,12} -> 23/2 = 11.5
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
