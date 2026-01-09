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
			expectedErr:    false, // Should trigger EOF, no error reported
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
