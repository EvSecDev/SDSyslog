package protocol

import "reflect"

// Size returns an estimate of the total in-memory footprint of Payload,
// including stable Go runtime overhead (string/slice headers + backing data).
func (payload Payload) Size() (bytes int) {
	// Fixed-size fields
	bytes += 8  // HostID (int)
	bytes += 8  // MsgID (int)
	bytes += 8  // MessageSeq (int)
	bytes += 8  // MessageSeqMax (int)
	bytes += 24 // Timestamp (time.Time)
	bytes += 8  // PaddingLen (int)

	// String headers (16B each on 64-bit)
	bytes += 16 // RemoteIP
	bytes += 16 // Hostname

	// String backing storage ----
	bytes += len(payload.RemoteIP)
	bytes += len(payload.Hostname)

	// Data
	bytes += 24                // []byte header
	bytes += len(payload.Data) // backing array

	// CustomFields map
	bytes += 8 // approximate map header pointer
	for key, value := range payload.CustomFields {
		bytes += len(key) + 16
		bytes += valueSizeApprox(reflect.ValueOf(value))
	}

	return
}
func valueSizeApprox(v reflect.Value) (byteSize int) {
	if !v.IsValid() {
		return 0
	}
	const ptrSize = 8 // 64-bit pointer size

	switch v.Kind() {
	case reflect.String:
		byteSize = 16 + v.Len() // header + bytes
	case reflect.Bool:
		byteSize = 1 + 1 // 1 byte for value + 1 byte padding overhead
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		byteSize = int(v.Type().Size())
	case reflect.Float32, reflect.Float64:
		byteSize = int(v.Type().Size())
	case reflect.Slice:
		size := 24 // slice header: ptr + len + cap
		for i := 0; i < v.Len(); i++ {
			size += valueSizeApprox(v.Index(i))
		}
		byteSize = size
	case reflect.Map:
		size := 8 // map header pointer
		for _, key := range v.MapKeys() {
			size += valueSizeApprox(key)
			size += valueSizeApprox(v.MapIndex(key))
		}
		byteSize = size
	case reflect.Interface:
		if v.IsNil() {
			byteSize = 16 // empty interface header
		}
		byteSize = 16 + valueSizeApprox(v.Elem()) // header + value
	case reflect.Pointer:
		if v.IsNil() {
			byteSize = ptrSize
		}
		byteSize = ptrSize + valueSizeApprox(v.Elem())
	case reflect.Struct:
		size := 0
		for i := 0; i < v.NumField(); i++ {
			size += valueSizeApprox(v.Field(i))
		}
		byteSize = size
	default:
		byteSize = 0
	}
	return
}
