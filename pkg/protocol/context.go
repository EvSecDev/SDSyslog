package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// Converts any value to byte format with associated type.
func serializeAnyValue(value any) (valType uint8, data []byte, err error) {
	rv := reflect.ValueOf(value)
	rt := rv.Type()

	var buf bytes.Buffer
	switch rt.Kind() {
	case reflect.Int8:
		valType = ContextInt8
		buf.WriteByte(byte(value.(int8)))
	case reflect.Int16:
		valType = ContextInt16
		err = binary.Write(&buf, binary.BigEndian, int16(value.(int16)))
		if err != nil {
			return
		}
	case reflect.Int32:
		valType = ContextInt32
		err = binary.Write(&buf, binary.BigEndian, int32(value.(int32)))
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
		err = binary.Write(&buf, binary.BigEndian, math.Float32bits(value.(float32)))
		if err != nil {
			return
		}
	case reflect.Float64:
		valType = ContextFloat64
		err = binary.Write(&buf, binary.BigEndian, math.Float64bits(value.(float64)))
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

	switch valType {
	case ContextInt8:
		var b byte
		b, err = buf.ReadByte()
		if err != nil {
			return
		}
		num := int8(b)
		value = num
	case ContextInt16:
		var num int16
		if err = binary.Read(buf, binary.BigEndian, &num); err != nil {
			return
		}
		value = num
	case ContextInt32:
		var num int32
		if err = binary.Read(buf, binary.BigEndian, &num); err != nil {
			return
		}
		value = num
	case ContextInt64:
		var num int64
		if err = binary.Read(buf, binary.BigEndian, &num); err != nil {
			return
		}
		value = num
	case ContextFloat32:
		var bits uint32
		if err = binary.Read(buf, binary.BigEndian, &bits); err != nil {
			return
		}
		value = math.Float32frombits(bits)
	case ContextFloat64:
		var bits uint64
		if err = binary.Read(buf, binary.BigEndian, &bits); err != nil {
			return
		}
		value = math.Float64frombits(bits)
	case ContextBool:
		var b byte
		b, err = buf.ReadByte()
		if err != nil {
			return
		}
		if b != 0 {
			value = true
		} else {
			value = false
		}
	case ContextString:
		if buf.Len() == 0 {
			value = ""
			return
		}

		strBytes := make([]byte, buf.Len())
		if _, err = buf.Read(strBytes); err != nil {
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
