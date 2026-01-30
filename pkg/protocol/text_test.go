package protocol

import (
	"bytes"
	"testing"
)

func TestCleanStringToBytes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		want      []byte
	}{
		{
			name:      "empty input uses emptyFieldChar",
			input:     "",
			maxLength: 10,
			want:      []byte(emptyFieldChar),
		},
		{
			name:      "removes non-printable ASCII",
			input:     "ab\x00cd\nEF",
			maxLength: 10,
			want:      []byte("abcdEF"),
		},
		{
			name:      "removes non-ASCII unicode",
			input:     "hello✓世界",
			maxLength: 10,
			want:      []byte("hello"),
		},
		{
			name:      "truncates to maxLength",
			input:     "abcdefghijklmnopqrstuvwxyz",
			maxLength: 5,
			want:      []byte("abcde"),
		},
		{
			name:      "printable ASCII preserved",
			input:     "!@#ABCxyz",
			maxLength: 20,
			want:      []byte("!@#ABCxyz"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanStringToBytes(tt.input, tt.maxLength)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("cleanStringToBytes(%q, %d) = %q, want %q",
					tt.input, tt.maxLength, got, tt.want)
			}
		})
	}
}

func TestCleanBytes(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{
			name: "no null bytes",
			in:   []byte{1, 2, 3},
			want: []byte{1, 2, 3},
		},
		{
			name: "removes single null byte",
			in:   []byte{1, 0, 2},
			want: []byte{1, 2},
		},
		{
			name: "removes multiple null bytes",
			in:   []byte{0, 1, 0, 2, 0},
			want: []byte{1, 2},
		},
		{
			name: "all null bytes",
			in:   []byte{0, 0, 0},
			want: []byte{},
		},
		{
			name: "empty slice",
			in:   []byte{},
			want: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanBytes(tt.in)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("cleanBytes(%v) = %v, want %v",
					tt.in, got, tt.want)
			}
		})
	}
}

func TestIsPrintableASCII(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{
			name: "empty slice is printable",
			in:   []byte{},
			want: true,
		},
		{
			name: "printable ASCII",
			in:   []byte("Hello!~"),
			want: true,
		},
		{
			name: "contains control character",
			in:   []byte{0x19},
			want: false,
		},
		{
			name: "contains DEL",
			in:   []byte{0x7F},
			want: false,
		},
		{
			name: "contains extended ASCII",
			in:   []byte{0x80},
			want: false,
		},
		{
			name: "mixed printable and non-printable",
			in:   []byte("ABC\x00DEF"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrintableASCII(tt.in)
			if got != tt.want {
				t.Fatalf("isPrintableASCII(%v) = %v, want %v",
					tt.in, got, tt.want)
			}
		})
	}
}
