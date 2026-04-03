package protocol

import (
	"sdsyslog/internal/filtering"
	"testing"
	"time"
)

func TestMessageFilter(t *testing.T) {
	tests := []struct {
		name          string
		filter        MessageFilter
		input         Message
		expectedMatch bool
	}{
		{
			name: "exact field match",
			filter: MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: "field1",
				},
				FieldsValue: &filtering.Filter{
					Exact: "value1",
				},
				UseAnd: true,
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"field1": "value1",
				},
				Data: []byte("log message"),
			},
			expectedMatch: true,
		},
		{
			name:   "No filters message bypasses",
			filter: MessageFilter{},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"field1": "value1",
				},
				Data: []byte("log message"),
			},
			expectedMatch: false,
		},
		{
			name: "field value or message contains",
			filter: MessageFilter{
				FieldsValue: &filtering.Filter{
					Contains: "check",
				},
				Data: &filtering.Filter{
					Contains: "status",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"field1": "some data",
					"field2": "checking status",
					"field3": "other",
				},
				Data: []byte("status is ok"),
			},
			expectedMatch: true,
		},
		{
			name: "prefix and suffix",
			filter: MessageFilter{
				FieldsValue: &filtering.Filter{
					And: []filtering.Filter{
						{
							Prefix: "in",
						},
						{
							Suffix: "out",
						},
					},
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"field1": "some data",
					"field3": "in through out",
				},
				Data: []byte("a"),
			},
			expectedMatch: true,
		},
		{
			name: "prefix field no match",
			filter: MessageFilter{
				FieldsKey: &filtering.Filter{
					Suffix: "test",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"field1": "some test data",
					"field3": "test through out",
				},
				Data: []byte("a test"),
			},
			expectedMatch: false,
		},
		{
			name: "UseAnd fails when one condition false",
			filter: MessageFilter{
				Data: &filtering.Filter{
					Contains: "error",
				},
				FieldsValue: &filtering.Filter{
					Contains: "ok",
				},
				UseAnd: true,
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"status": "ok",
				},
				Data: []byte("all good"),
			},
			expectedMatch: false,
		},
		{
			name: "UseAnd false returns true if one matches",
			filter: MessageFilter{
				Data: &filtering.Filter{
					Contains: "error",
				},
				FieldsValue: &filtering.Filter{
					Contains: "ok",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"status": "ok",
				},
				Data: []byte("all good"),
			},
			expectedMatch: true,
		},
		{
			name: "field key suffix match",
			filter: MessageFilter{
				FieldsKey: &filtering.Filter{
					Suffix: "id",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"user_id": 123,
				},
				Data: []byte("a"),
			},
			expectedMatch: true,
		},
		{
			name: "fields value no match",
			filter: MessageFilter{
				FieldsValue: &filtering.Filter{
					Contains: "error",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"a": "ok",
					"b": "fine",
				},
				Data: []byte("message"),
			},
			expectedMatch: false,
		},
		{
			name: "not filter inverts result",
			filter: MessageFilter{
				Data: &filtering.Filter{
					Not: &filtering.Filter{
						Contains: "error",
					},
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields:    map[string]any{},
				Data:      []byte("all good"),
			},
			expectedMatch: true,
		},
		{
			name: "filter or match",
			filter: MessageFilter{
				Data: &filtering.Filter{
					Or: []filtering.Filter{
						{Contains: "warn"},
						{Contains: "error"},
					},
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields:    map[string]any{},
				Data:      []byte("warn: disk space"),
			},
			expectedMatch: true,
		},
		{
			name: "filter and fails",
			filter: MessageFilter{
				Data: &filtering.Filter{
					And: []filtering.Filter{
						{Contains: "warn"},
						{Contains: "disk"},
					},
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields:    map[string]any{},
				Data:      []byte("warn: cpu high"),
			},
			expectedMatch: false,
		},
		{
			name: "non string field value",
			filter: MessageFilter{
				FieldsValue: &filtering.Filter{
					Contains: "42",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"id": 42,
				},
				Data: []byte("msg"),
			},
			expectedMatch: true,
		},
		{
			name: "empty fields map",
			filter: MessageFilter{
				FieldsValue: &filtering.Filter{
					Contains: "value",
				},
			},
			input: Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields:    map[string]any{},
				Data:      []byte("msg"),
			},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if err != nil {
				t.Fatalf("unexpected error validating filter: %v", err)
			}

			match := tt.filter.Match(&tt.input)
			if match != tt.expectedMatch {
				t.Fatalf("match mismatch result: expected match=%v - got match=%v", tt.expectedMatch, match)
			}
		})
	}
}
