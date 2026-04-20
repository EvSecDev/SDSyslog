package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
)

// Converts any value to byte format with associated type.
func serializeAnyValue(value any) (valType uint8, data []byte, err error) {
	rv := reflect.ValueOf(value)
	rt := rv.Type()

	var buf bytes.Buffer
	switch rt.Kind() {
	case reflect.Slice:
		if rt.Elem().Kind() == reflect.Uint8 {
			valType = ContextSliceBytes

			b := rv.Bytes()
			err = binary.Write(&buf, binary.BigEndian, uint32(len(b)))
			if err != nil {
				return
			}

			_, err = buf.Write(b)
			if err != nil {
				return
			}
		} else {
			err = fmt.Errorf("unsupported slice type %T", value)
			return
		}
	case reflect.Int8:
		valType = ContextInt8
		val, ok := value.(int8)
		if !ok {
			err = fmt.Errorf("failed to assert any value as int8: value=%+v type=%T", value, value)
			return
		}
		buf.WriteByte(byte(val))
	case reflect.Int16:
		valType = ContextInt16
		val, ok := value.(int16)
		if !ok {
			err = fmt.Errorf("failed to assert any value as int16: value=%+v type=%T", value, value)
			return
		}
		err = binary.Write(&buf, binary.BigEndian, val)
		if err != nil {
			return
		}
	case reflect.Int32:
		valType = ContextInt32
		val, ok := value.(int32)
		if !ok {
			err = fmt.Errorf("failed to assert any value as int32: value=%+v type=%T", value, value)
			return
		}
		err = binary.Write(&buf, binary.BigEndian, val)
		if err != nil {
			return
		}
	case reflect.Int, reflect.Int64:
		valType = ContextInt64
		switch v := value.(type) {
		case int64:
			err = binary.Write(&buf, binary.BigEndian, v)
			if err != nil {
				return
			}
		case int:
			err = binary.Write(&buf, binary.BigEndian, int64(v))
			if err != nil {
				return
			}
		default:
			err = fmt.Errorf("ContextInt64 field Value type mismatch: %T", value)
			return
		}
	case reflect.Float32:
		valType = ContextFloat32
		val, ok := value.(float32)
		if !ok {
			err = fmt.Errorf("failed to assert any value as float32: value=%+v type=%T", value, value)
			return
		}
		err = binary.Write(&buf, binary.BigEndian, math.Float32bits(val))
		if err != nil {
			return
		}
	case reflect.Float64:
		valType = ContextFloat64
		val, ok := value.(float64)
		if !ok {
			err = fmt.Errorf("failed to assert any value as float64: value=%+v type=%T", value, value)
			return
		}
		err = binary.Write(&buf, binary.BigEndian, math.Float64bits(val))
		if err != nil {
			return
		}
	case reflect.Bool:
		valType = ContextBool
		if rv.Bool() {
			buf.WriteByte(0x01)
		} else {
			buf.WriteByte(0x00)
		}
	case reflect.String:
		valType = ContextString
		_, err = buf.Write(cleanBytes([]byte(rv.String())))
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("unsupported value type %T", value)
	}
	data = buf.Bytes()
	return
}

// Converts byte format and associated type back to any value
func deserializeAnyValue(valType uint8, data []byte) (value any, err error) {
	buf := bytes.NewReader(data)

	// helper to ensure no trailing bytes remain
	ensureFullyConsumed := func() (err error) {
		if buf.Len() != 0 {
			err = fmt.Errorf("extra trailing data: %d bytes", buf.Len())
			return
		}
		return
	}

	switch valType {
	case ContextSliceBytes:
		// Require at least 4 bytes for length
		if len(data) < 4 {
			err = io.ErrUnexpectedEOF
			return
		}

		var length uint32
		err = binary.Read(buf, binary.BigEndian, &length)
		if err != nil {
			return
		}

		// Enforce consistency: remaining bytes must match length exactly
		if int(length) != buf.Len() {
			err = fmt.Errorf("invalid slice length: declared=%d actual=%d", length, buf.Len())
			return
		}

		b := make([]byte, length)
		_, err = io.ReadFull(buf, b)
		if err != nil {
			return
		}

		err = ensureFullyConsumed()
		if err != nil {
			err = fmt.Errorf("byte slice: %w", err)
			return
		}

		value = b
	case ContextInt8:
		if len(data) != 1 {
			err = fmt.Errorf("invalid int8 length: %d", len(data))
			return
		}
		var b byte
		b, err = buf.ReadByte()
		if err != nil {
			return
		}
		num := int8(b)
		value = num
	case ContextInt16:
		if len(data) != 2 {
			err = fmt.Errorf("invalid int16 length: %d", len(data))
			return
		}
		var num int16
		err = binary.Read(buf, binary.BigEndian, &num)
		if err != nil {
			return
		}
		value = num
	case ContextInt32:
		if len(data) != 4 {
			err = fmt.Errorf("invalid int32 length: %d", len(data))
			return
		}
		var num int32
		err = binary.Read(buf, binary.BigEndian, &num)
		if err != nil {
			return
		}
		value = num
	case ContextInt64:
		if len(data) != 8 {
			err = fmt.Errorf("invalid int64 length: %d", len(data))
			return
		}
		var num int64
		err = binary.Read(buf, binary.BigEndian, &num)
		if err != nil {
			return
		}
		value = num
	case ContextFloat32:
		if len(data) != 4 {
			err = fmt.Errorf("invalid float32 length: %d", len(data))
			return
		}
		var bits uint32
		err = binary.Read(buf, binary.BigEndian, &bits)
		if err != nil {
			return
		}
		rawVal := math.Float32frombits(bits)

		// Reject non-canonical values
		if math.IsNaN(float64(rawVal)) || math.IsInf(float64(rawVal), 0) {
			err = fmt.Errorf("invalid float32 value: not a number or infinity")
			return
		}

		// Normalize -0 to +0
		if rawVal == 0 {
			rawVal = 0
		}
		value = rawVal
	case ContextFloat64:
		if len(data) != 8 {
			err = fmt.Errorf("invalid float64 length: %d", len(data))
			return
		}
		var bits uint64
		err = binary.Read(buf, binary.BigEndian, &bits)
		if err != nil {
			return
		}
		rawVal := math.Float64frombits(bits)

		if math.IsNaN(rawVal) || math.IsInf(rawVal, 0) {
			err = fmt.Errorf("invalid float64 value")
			return
		}
		if rawVal == 0 {
			rawVal = 0
		}

		value = rawVal
	case ContextBool:
		if len(data) != 1 {
			err = fmt.Errorf("invalid bool length: %d", len(data))
			return
		}
		var b byte
		b, err = buf.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 0x00:
			value = false
			return
		case 0x01:
			value = true
			return
		default:
			err = fmt.Errorf("invalid bool value: %d", b)
			return
		}
	case ContextString:
		if buf.Len() == 0 {
			value = ""
			return
		}

		strBytes := make([]byte, buf.Len())
		_, err = io.ReadFull(buf, strBytes)
		if err != nil {
			return
		}

		err = ensureFullyConsumed()
		if err != nil {
			return
		}

		if !utf8.Valid(strBytes) {
			err = fmt.Errorf("invalid UTF-8 string")
			return
		}
		value = string(strBytes)
	default:
		err = fmt.Errorf("unsupported value type: %d", valType)
	}
	return
}

// Creates user-readable string from various types.
// If type is unsupported, returned string will be empty
func FormatValue(value any) (text string) {
	switch x := value.(type) {
	case string:
		text = x
	case []byte:
		text = fmt.Sprintf("%x", x)
	case int:
		text = strconv.Itoa(x)
	case int8:
		text = strconv.FormatInt(int64(x), 10)
	case int16:
		text = strconv.FormatInt(int64(x), 10)
	case int32:
		text = strconv.FormatInt(int64(x), 10)
	case int64:
		text = strconv.FormatInt(x, 10)
	case uint:
		text = strconv.FormatUint(uint64(x), 10)
	case uint8:
		text = strconv.FormatUint(uint64(x), 10)
	case uint16:
		text = strconv.FormatUint(uint64(x), 10)
	case uint32:
		text = strconv.FormatUint(uint64(x), 10)
	case uint64:
		text = strconv.FormatUint(x, 10)
	case float32:
		text = strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		text = strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			text = "true"
		} else {
			text = "false"
		}
	}
	return
}
