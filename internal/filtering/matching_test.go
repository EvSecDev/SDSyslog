package filtering

import (
	"strings"
	"testing"
)

func TestTrimDurationPrecision(t *testing.T) {
	tests := []struct {
		name                    string
		input                   string
		filter                  Filter
		expectedValidationError string
		expectedMatch           bool
	}{
		{
			name:  "basic exact match",
			input: "this is some text",
			filter: Filter{
				Exact: "this is some text",
			},
			expectedMatch: true,
		},
		{
			name:  "basic exact non-match",
			input: "this is some text",
			filter: Filter{
				Exact: "different text",
			},
			expectedMatch: false,
		},
		{
			name:  "basic contains match",
			input: "this is some text",
			filter: Filter{
				Contains: "is some",
			},
			expectedMatch: true,
		},
		{
			name:  "basic prefix match",
			input: "this is some text",
			filter: Filter{
				Prefix: "this is",
			},
			expectedMatch: true,
		},
		{
			name:  "basic suffix match",
			input: "this is some text",
			filter: Filter{
				Suffix: "text",
			},
			expectedMatch: true,
		},
		// And filter
		{
			name:  "and match success",
			input: "error disk full",
			filter: Filter{
				And: []Filter{
					{Prefix: "error"},
					{Contains: "disk"},
				},
			},
			expectedMatch: true,
		},
		{
			name:  "and match fail",
			input: "error network",
			filter: Filter{
				And: []Filter{
					{Prefix: "error"},
					{Contains: "disk"},
				},
			},
			expectedMatch: false,
		},

		// Or filter
		{
			name:  "or match first",
			input: "panic occurred",
			filter: Filter{
				Or: []Filter{
					{Exact: "panic occurred"},
					{Contains: "disk"},
				},
			},
			expectedMatch: true,
		},
		{
			name:  "or match contains",
			input: "a panic occurred on disk 0",
			filter: Filter{
				Or: []Filter{
					{Contains: "panic occurred"},
					{Contains: "disk"},
				},
			},
			expectedMatch: true,
		},
		{
			name:  "or match second",
			input: "disk full",
			filter: Filter{
				Or: []Filter{
					{Exact: "panic occurred"},
					{Contains: "disk"},
				},
			},
			expectedMatch: true,
		},
		{
			name:  "or match fail",
			input: "network error",
			filter: Filter{
				Or: []Filter{
					{Exact: "panic occurred"},
					{Contains: "disk"},
				},
			},
			expectedMatch: false,
		},

		// Not filter
		{
			name:  "not match success",
			input: "network error",
			filter: Filter{
				Not: &Filter{Contains: "disk"},
			},
			expectedMatch: true,
		},
		{
			name:  "not match fail",
			input: "disk full",
			filter: Filter{
				Not: &Filter{Contains: "disk"},
			},
			expectedMatch: false,
		},

		// Nested combination
		{
			name:  "nested combination",
			input: "error disk full",
			filter: Filter{
				Or: []Filter{
					{
						And: []Filter{
							{Prefix: "error"},
							{Contains: "disk"},
						},
					},
					{Exact: "panic"},
				},
			},
			expectedMatch: true,
		},

		// Validation errors
		{
			name:                    "empty node",
			filter:                  Filter{},
			expectedValidationError: "filter node is empty",
		},
		{
			name: "multiple operators/leaves",
			filter: Filter{
				Prefix: "error",
				Suffix: "full",
			},
			expectedValidationError: "filter node must have exactly one operator/leaf",
		},
		{
			name: "And invalid children",
			filter: Filter{
				And: []Filter{
					{
						And: []Filter{},
					},
				},
			},
			expectedValidationError: "and[0]: filter node is empty",
		},
		{
			name: "Or invalid children",
			filter: Filter{
				Or: []Filter{
					{
						And: []Filter{},
					},
				},
			},
			expectedValidationError: "or[0]: filter node is empty",
		},
		{
			name: "Not empty children",
			filter: Filter{
				Not: &Filter{},
			},
			expectedValidationError: "not: filter node is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if err != nil && tt.expectedValidationError == "" {
				t.Fatalf("expected no error from filter validation, but got '%v'", err)
			}
			if err == nil && tt.expectedValidationError != "" {
				t.Fatalf("expected validation error %q, but got nil", tt.expectedValidationError)
			}
			if err != nil && strings.Contains(err.Error(), tt.expectedValidationError) {
				return
			}
			if err != nil && !strings.Contains(err.Error(), tt.expectedValidationError) {
				t.Fatalf("expected validation error %q, but got '%v'", tt.expectedValidationError, err)
			}

			matches := tt.filter.Match([]byte(tt.input))
			if matches != tt.expectedMatch {
				t.Fatalf("input %q: expect match=%v - got match=%v", tt.input, tt.expectedMatch, matches)
			}
		})
	}
}
