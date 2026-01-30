package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestSerializeAnyValue(t *testing.T) {
	testCases := []struct {
		name          string
		input         any
		expectedType  uint8
		expectedValue []byte
		expectedErr   bool
	}{
		{
			name:          "int8 positive",
			input:         int8(42),
			expectedType:  ContextInt8,
			expectedValue: []byte{0x2a},
		},
		{
			name:          "int8 negative",
			input:         int8(-1),
			expectedType:  ContextInt8,
			expectedValue: []byte{0xff},
		},
		{
			name:          "int16",
			input:         int16(0x1234),
			expectedType:  ContextInt16,
			expectedValue: []byte{0x12, 0x34},
		},
		{
			name:          "int32",
			input:         int32(0x12345678),
			expectedType:  ContextInt32,
			expectedValue: []byte{0x12, 0x34, 0x56, 0x78},
		},
		{
			name:          "int64",
			input:         int64(0x0102030405060708),
			expectedType:  ContextInt64,
			expectedValue: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name:          "int coerced to int64",
			input:         int(12345),
			expectedType:  ContextInt64,
			expectedValue: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30, 0x39},
		},
		{
			name:         "float32",
			input:        float32(1.5),
			expectedType: ContextFloat32,
			expectedValue: func() []byte {
				var b [4]byte
				binary.BigEndian.PutUint32(b[:], math.Float32bits(1.5))
				return b[:]
			}(),
		},
		{
			name:         "float64",
			input:        float64(1.5),
			expectedType: ContextFloat64,
			expectedValue: func() []byte {
				var b [8]byte
				binary.BigEndian.PutUint64(b[:], math.Float64bits(1.5))
				return b[:]
			}(),
		},
		{
			name:          "bool true",
			input:         true,
			expectedType:  ContextBool,
			expectedValue: []byte{0x01},
		},
		{
			name:          "bool false",
			input:         false,
			expectedType:  ContextBool,
			expectedValue: []byte{0x00},
		},
		{
			name:          "short string",
			input:         "hello",
			expectedType:  ContextString,
			expectedValue: []byte("hello"),
		},
		{
			name:          "empty string",
			input:         "",
			expectedType:  ContextString,
			expectedValue: []byte{},
		},
		{
			name:          "string with utf8",
			input:         "héllø",
			expectedType:  ContextString,
			expectedValue: cleanBytes([]byte("héllø")),
		},
		{
			name:        "unsupported type slice",
			input:       []int{1, 2, 3},
			expectedErr: true,
		},
		{
			name:        "unsupported type map",
			input:       map[string]int{"a": 1},
			expectedErr: true,
		},
		{
			name:        "unsupported type struct",
			input:       struct{}{},
			expectedErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			valType, value, err := serializeAnyValue(tt.input)
			if err == nil && tt.expectedErr {
				t.Errorf("expected error but got nil")
			} else if !tt.expectedErr && err != nil {
				t.Errorf("expected no error, but got '%v'", err)
			} else if tt.expectedErr && err != nil {
				return
			}

			if valType != tt.expectedType {
				t.Errorf("expected value type '%x', but got '%x'", tt.expectedType, valType)
			}
			if !bytes.Equal(value, tt.expectedValue) {
				t.Errorf("Mismatched values:\n Expected: '%x'\n Got      '%x'", tt.expectedValue, value)
			}
		})
	}
}

func TestDeserializeAnyValue(t *testing.T) {
	testCases := []struct {
		name        string
		inputType   uint8
		inputValue  []byte
		expectedOut any
		expectedErr bool
	}{
		{
			name:        "int8",
			inputType:   ContextInt8,
			inputValue:  []byte{0xff},
			expectedOut: int8(-1),
		},
		{
			name:        "int16",
			inputType:   ContextInt16,
			inputValue:  []byte{0x12, 0x34},
			expectedOut: int16(0x1234),
		},
		{
			name:        "int32",
			inputType:   ContextInt32,
			inputValue:  []byte{0x12, 0x34, 0x56, 0x78},
			expectedOut: int32(0x12345678),
		},
		{
			name:        "int64",
			inputType:   ContextInt64,
			inputValue:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
			expectedOut: int64(1),
		},
		{
			name:      "float32",
			inputType: ContextFloat32,
			inputValue: func() []byte {
				var b [4]byte
				binary.BigEndian.PutUint32(b[:], math.Float32bits(1.5))
				return b[:]
			}(),
			expectedOut: float32(1.5),
		},
		{
			name:      "float64",
			inputType: ContextFloat64,
			inputValue: func() []byte {
				var b [8]byte
				binary.BigEndian.PutUint64(b[:], math.Float64bits(1.5))
				return b[:]
			}(),
			expectedOut: float64(1.5),
		},
		{
			name:        "bool true",
			inputType:   ContextBool,
			inputValue:  []byte{0x01},
			expectedOut: true,
		},
		{
			name:        "bool false",
			inputType:   ContextBool,
			inputValue:  []byte{0x00},
			expectedOut: false,
		},
		{
			name:        "string",
			inputType:   ContextString,
			inputValue:  []byte("hello"),
			expectedOut: "hello",
		},
		{
			name:        "empty string",
			inputType:   ContextString,
			inputValue:  []byte{},
			expectedOut: "",
		},
		{
			name:        "unsupported type",
			inputType:   0xff,
			inputValue:  []byte{0x00},
			expectedErr: true,
		},
		{
			name:        "short int16",
			inputType:   ContextInt16,
			inputValue:  []byte{0x01},
			expectedErr: true,
		},
		{
			name:        "short int64",
			inputType:   ContextInt64,
			inputValue:  []byte{0x01, 0x02},
			expectedErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			value, err := deserializeAnyValue(tt.inputType, tt.inputValue)
			if err == nil && tt.expectedErr {
				t.Errorf("expected error but got nil")
			} else if !tt.expectedErr && err != nil {
				t.Errorf("expected no error, but got '%v'", err)
			} else if tt.expectedErr && err != nil {
				return
			}

			assertEqualAnyPanic(t, tt.expectedOut, value)
		})
	}
}

func TestAnyValueRoundTrip(t *testing.T) {
	testCases := []struct {
		name        string
		input       any
		expectError bool
	}{
		{name: "int8 zero", input: int8(0)},
		{name: "int8 negative", input: int8(-128)},
		{name: "int16", input: int16(-12345)},
		{name: "int32", input: int32(12345678)},
		{name: "int64", input: int64(-123456789)},
		{name: "float32", input: float32(3.14159)},
		{name: "float64", input: float64(-1.23456789)},
		{name: "bool true", input: true},
		{name: "bool false", input: false},
		{name: "empty string", input: ""},
		{name: "ascii string", input: "hello"},
		{name: "utf8 string", input: "héllø 🌍"},
		{name: "slice unsupported", input: []int{1, 2}, expectError: true},
		{name: "map unsupported", input: map[string]int{"a": 1}, expectError: true},
		{name: "struct unsupported", input: struct{}{}, expectError: true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			valType, data, err := serializeAnyValue(tt.input)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected serialize error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			out, err := deserializeAnyValue(valType, data)
			if err != nil {
				t.Fatalf("deserialize failed: %v", err)
			}

			// Strict equality: same type, same value
			assertEqualAnyPanic(t, tt.input, out)
		})
	}
}

func assertEqualAnyPanic(t *testing.T, a, b any) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("assert equal failed: %v", r)
		}
	}()

	if a == nil || b == nil {
		if a != b {
			panic("one value is nil, the other is not")
		}
		return
	}

	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb {
		panic(fmt.Sprintf(
			"type mismatch: %v vs %v",
			ta, tb,
		))
	}

	if ta.Comparable() {
		if a != b {
			panic(fmt.Sprintf(
				"values not equal (type %v): %v vs %v",
				ta, a, b,
			))
		}
		return
	}

	switch va := a.(type) {
	case []byte:
		if !bytes.Equal(va, b.([]byte)) {
			panic("[]byte values not equal")
		}
		return

	case time.Time:
		if !va.Equal(b.(time.Time)) {
			panic(fmt.Sprintf(
				"time.Time values not equal: %v vs %v",
				va, b,
			))
		}
		return
	}

	if !reflect.DeepEqual(a, b) {
		panic(fmt.Sprintf(
			"values not deeply equal (type %v): %#v vs %#v",
			ta, a, b,
		))
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "string value",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "byte slice",
			input: []byte{0x01, 0xAB, 0xFF},
			want:  "01abff",
		},
		{
			name:  "empty byte slice",
			input: []byte{},
			want:  "",
		},
		{
			name:  "int",
			input: int(42),
			want:  "42",
		},
		{
			name:  "int negative",
			input: int(-42),
			want:  "-42",
		},
		{
			name:  "int8",
			input: int8(-128),
			want:  "-128",
		},
		{
			name:  "int16",
			input: int16(32767),
			want:  "32767",
		},
		{
			name:  "int32",
			input: int32(-2147483648),
			want:  "-2147483648",
		},
		{
			name:  "int64",
			input: int64(9223372036854775807),
			want:  "9223372036854775807",
		},
		{
			name:  "uint",
			input: uint(42),
			want:  "42",
		},
		{
			name:  "uint8",
			input: uint8(255),
			want:  "255",
		},
		{
			name:  "uint16",
			input: uint16(65535),
			want:  "65535",
		},
		{
			name:  "uint32",
			input: uint32(4294967295),
			want:  "4294967295",
		},
		{
			name:  "uint64",
			input: uint64(18446744073709551615),
			want:  "18446744073709551615",
		},
		{
			name:  "float32 simple",
			input: float32(1.5),
			want:  "1.5",
		},
		{
			name:  "float32 scientific",
			input: float32(1e20),
			want:  "1e+20",
		},
		{
			name:  "float64 simple",
			input: float64(3.141592653589793),
			want:  "3.141592653589793",
		},
		{
			name:  "float64 integer value",
			input: float64(10),
			want:  "10",
		},
		{
			name:  "bool true",
			input: true,
			want:  "true",
		},
		{
			name:  "bool false",
			input: false,
			want:  "false",
		},
		{
			name:  "nil value",
			input: nil,
			want:  "",
		},
		{
			name:  "struct unsupported",
			input: struct{ A int }{A: 1},
			want:  "",
		},
		{
			name:  "slice unsupported",
			input: []int{1, 2, 3},
			want:  "",
		},
		{
			name:  "map unsupported",
			input: map[string]int{"a": 1},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatValue(tt.input)
			if got != tt.want {
				t.Fatalf("FormatValue(%#v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
