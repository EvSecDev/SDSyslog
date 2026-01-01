package journald

import (
	"bufio"
	"strings"
	"testing"
)

func TestExtractEntry(t *testing.T) {
	tests := []struct {
		name           string
		input          *bufio.Reader
		expectedFields map[string]string
		expectedErr    bool
	}{
		{
			name: "basic",
			input: bufio.NewReader(strings.NewReader(
				"key1=value1\n" +
					"key2=value2\n\n",
			)),
			expectedFields: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expectedErr: false,
		},
		{
			name:           "empty entry",
			input:          bufio.NewReader(strings.NewReader("")),
			expectedFields: map[string]string{},
			expectedErr:    true, // Expecting error because no fields are found
		},
		{
			name: "binary field",
			input: bufio.NewReader(strings.NewReader(
				"binaryKey\n\x08\x00\x00\x00\x00\x00\x00\x00" +
					"abcdefgh\n",
			)),
			expectedFields: map[string]string{
				"binaryKey": "abcdefgh",
			},
			expectedErr: false,
		},
		{
			name: "binary field with too large data",
			input: bufio.NewReader(strings.NewReader(
				"largeBinaryKey\n\x01\x00\x00\x00\x00\x00\x00\x00" +
					// Data size exceeds 10MB (you'd adjust it for your test environment)
					strings.Repeat("a", 1024*1024*11) + "\n",
			)),
			expectedFields: map[string]string{},
			expectedErr:    true, // Expecting error because binary size exceeds limit
		},
		{
			name: "missing newline after binary data",
			input: bufio.NewReader(strings.NewReader(
				"binaryKeyWithoutNewline\n\x08\x00\x00\x00\x00\x00\x00\x00" +
					"abcdefgh", // Missing newline at the end
			)),
			expectedFields: map[string]string{},
			expectedErr:    true, // Expecting error because the newline after binary data is missing
		},
		{
			name: "malformed length for binary field",
			input: bufio.NewReader(strings.NewReader(
				"binaryKey\n\x10\x00\x00\x00\x00\x00\x00\x00" +
					"abcdefgh" +
					"\n",
			)),
			expectedFields: map[string]string{},
			expectedErr:    true, // Expecting error due to mismatch between length and actual data size
		},
		{
			name: "fields with empty values",
			input: bufio.NewReader(strings.NewReader(
				"emptyField=\n" +
					"anotherField=\n\n",
			)),
			expectedFields: map[string]string{
				"emptyField":   "",
				"anotherField": "",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ExtractEntry(tt.input)
			if err != nil && !tt.expectedErr {
				t.Fatalf("expected no error, but got '%s'", err)
			}
			if err == nil && tt.expectedErr {
				t.Fatalf("expected error, but got no error")
			}
			if err != nil && tt.expectedErr {
				return
			}

			expectedFieldsCpy := tt.expectedFields

			for key, value := range fields {
				expectedVal, validKey := expectedFieldsCpy[key]
				if !validKey {
					t.Fatalf("found unexpected key in output '%s'", key)
				}
				delete(expectedFieldsCpy, key) // Remove from copy to check for remaining after

				if expectedVal != value {
					t.Fatalf("key='%s'; expected value to be '%s', but got '%s'", key, expectedVal, value)
				}
			}

			if len(expectedFieldsCpy) > 0 {
				t.Errorf("expected additional fields in test output, but output did not contain them. Missing %d fields: \n%v\n", len(expectedFieldsCpy), expectedFieldsCpy)
			}
		})
	}
}

func TestExtractCursor(t *testing.T) {
	tests := []struct {
		name           string
		fields         map[string]string
		expectedCursor string
		expectedErr    bool
	}{
		{
			name:           "no fields",
			fields:         map[string]string{},
			expectedCursor: "",
			expectedErr:    true,
		},
		{
			name: "valid cursor",
			fields: map[string]string{
				"__CURSOR": "s=373c352f9c524db6b8fee1bed92a65d3;i=6aafc3;b=0813e623df9b403886bdca2f0ff32aa9;m=2202c79af;t=646f4d80c4c69;x=1e6b6a93e9890234",
			},
			expectedCursor: "373c352f9c524db6b8fee1bed92a65d3",
			expectedErr:    false,
		},
		{
			name: "missing cursor main field",
			fields: map[string]string{
				"__CURSOR": "d=373c352f9c524db6b8fee1bed92a65d3;i=6aafc3;b=0813e623df9b403886bdca2f0ff32aa9;m=2202c79af;t=646f4d80c4c69;x=1e6b6a93e9890234",
			},
			expectedCursor: "",
			expectedErr:    true,
		},
		{
			name: "missing cursor field",
			fields: map[string]string{
				"MESSAGE": "hello",
			},
			expectedCursor: "",
			expectedErr:    true,
		},
		{
			name: "empty cursor main field",
			fields: map[string]string{
				"__CURSOR": "s=;i=6aafc3;b=0813e623df9b403886bdca2f0ff32aa9;m=2202c79af;t=646f4d80c4c69;x=1e6b6a93e9890234",
			},
			expectedCursor: "",
			expectedErr:    true,
		},
		{
			name: "cursor different order",
			fields: map[string]string{
				"__CURSOR": "i=6aafc3;b=0813e623df9b403886bdca2f0ff32aa9;m=2202c79af;s=373c352f9c524db6b8fee1bed92a65d3;t=646f4d80c4c69;x=1e6b6a93e9890234",
			},
			expectedCursor: "",
			expectedErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor, err := ExtractCursor(tt.fields)
			if err != nil && !tt.expectedErr {
				t.Fatalf("expected no error, but got '%s'", err)
			}
			if err == nil && tt.expectedErr {
				t.Fatalf("expected error, but got no error")
			}
			if err != nil && tt.expectedErr {
				return
			}
			if cursor != tt.expectedCursor {
				t.Errorf("expected cursor '%s', but got cursor '%s'", tt.expectedCursor, cursor)
			}
		})
	}
}
