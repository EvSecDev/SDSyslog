package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

// Serializes inner packet payload into transport payload
// Does not validate fields against protocol spec
func ConstructInnerPayload(fields InnerWireFormat) (payload []byte, err error) {
	var buf bytes.Buffer

	// HEADER
	if err = binary.Write(&buf, binary.BigEndian, fields.HostID); err != nil {
		err = fmt.Errorf("failed to serialize HostID: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.LogID); err != nil {
		err = fmt.Errorf("failed to serialize LogID: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.MessageSeq); err != nil {
		err = fmt.Errorf("failed to serialize MessageSeq: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.MessageSeqMax); err != nil {
		err = fmt.Errorf("failed to serialize MessageSeqMax: %v", err)
		return
	}

	// METADATA
	if err = binary.Write(&buf, binary.BigEndian, fields.Facility); err != nil {
		err = fmt.Errorf("failed to serialize Facility: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.Severity); err != nil {
		err = fmt.Errorf("failed to serialize Severity: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.Timestamp); err != nil {
		err = fmt.Errorf("failed to serialize Timestamp: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.ProcessID); err != nil {
		err = fmt.Errorf("failed to serialize ProcessID: %v", err)
		return
	}

	// CONTEXT
	// Hostname
	if err = writeByte(&buf, uint8(len(fields.Hostname))); err != nil {
		err = fmt.Errorf("failed to serialize Hostname length: %v", err)
		return
	}
	if err = writeFixedLength(&buf, fields.Hostname, len(fields.Hostname)); err != nil {
		err = fmt.Errorf("failed to serialize Hostname: %v", err)
		return
	}
	if err = writeByte(&buf, 0); err != nil { // Null byte terminator
		err = fmt.Errorf("failed to serialize Hostname terminator: %v", err)
		return
	}

	// ApplicationName
	if err = writeByte(&buf, uint8(len(fields.ApplicationName))); err != nil {
		err = fmt.Errorf("failed to serialize ApplicationName length: %v", err)
		return
	}
	if err = writeFixedLength(&buf, fields.ApplicationName, len(fields.ApplicationName)); err != nil {
		err = fmt.Errorf("failed to serialize ApplicationName: %v", err)
		return
	}
	if err = writeByte(&buf, 0); err != nil { // Null byte terminator
		err = fmt.Errorf("failed to serialize ApplicationName terminator: %v", err)
		return
	}

	// DATA
	if len(fields.LogText) > maxLogTextLen {
		err = fmt.Errorf("log text length (%d) exceeds maximum field length: %d", len(fields.LogText), maxLogTextLen)
		return
	}
	if err = writeUint16(&buf, uint16(len(fields.LogText))); err != nil {
		err = fmt.Errorf("failed to serialize LogText length: %v", err)
		return
	}
	if err = writeFixedLength(&buf, fields.LogText, len(fields.LogText)); err != nil {
		err = fmt.Errorf("failed to serialize LogText: %v", err)
		return
	}
	if err = writeByte(&buf, 0); err != nil {
		err = fmt.Errorf("failed to serialize LogText terminator: %v", err)
		return
	}

	// TRAILER
	padding := make([]byte, fields.PaddingLen)
	_, err = io.ReadFull(rand.Reader, padding)
	if err != nil {
		err = fmt.Errorf("failed to generate random padding: %v", err)
		return
	}
	buf.Write(padding)

	// Return the serialized payload
	payload = buf.Bytes()
	return
}

// Write single byte to provided buffer (big endian)
func writeByte(buf *bytes.Buffer, b uint8) (err error) {
	err = binary.Write(buf, binary.BigEndian, b)
	return
}

// Write two bytes to provided buffer (big endian)
func writeUint16(buf *bytes.Buffer, b uint16) (err error) {
	err = binary.Write(buf, binary.BigEndian, b)
	if err != nil {
		err = fmt.Errorf("failed to write uint16: %v", err)
		return
	}
	return
}

// Writes provided data of exact length to provided buffer
func writeFixedLength(buf *bytes.Buffer, data []byte, length int) (err error) {
	// Require length to be correct
	if len(data) > length {
		err = fmt.Errorf("data exceeds expected length: %d", length)
		return
	}

	_, err = buf.Write(data)
	if err != nil {
		err = fmt.Errorf("failed to write: %v", err)
		return
	}

	return
}

// Deserializes transport payload into inner payload
// Does not validate fields against protocol spec (only validates length)
func DeconstructInnerPayload(payload []byte) (fields InnerWireFormat, err error) {
	// Immediately reject invalid length
	if len(payload) < minInnerPayloadLen {
		err = fmt.Errorf("invalid payload length %d: must be minimum length of %d", len(payload), minInnerPayloadLen)
		return
	}

	buf := bytes.NewReader(payload)

	// HEADER
	if err = binary.Read(buf, binary.BigEndian, &fields.HostID); err != nil {
		err = fmt.Errorf("failed to deserialize HostID: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.LogID); err != nil {
		err = fmt.Errorf("failed to deserialize LogID: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.MessageSeq); err != nil {
		err = fmt.Errorf("failed to deserialize MessageSeq: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.MessageSeqMax); err != nil {
		err = fmt.Errorf("failed to deserialize MessageSeqMax: %v", err)
		return
	}

	// METADATA
	if err = binary.Read(buf, binary.BigEndian, &fields.Facility); err != nil {
		err = fmt.Errorf("failed to deserialize Facility: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.Severity); err != nil {
		err = fmt.Errorf("failed to deserialize Severity: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.Timestamp); err != nil {
		err = fmt.Errorf("failed to deserialize Timestamp: %v", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &fields.ProcessID); err != nil {
		err = fmt.Errorf("failed to deserialize ProcessID: %v", err)
		return
	}

	// CONTEXT
	// Hostname
	var hostnameLen uint8
	if err = binary.Read(buf, binary.BigEndian, &hostnameLen); err != nil {
		err = fmt.Errorf("failed to deserialize Hostname length: %v", err)
		return
	}
	if hostnameLen == 0 {
		err = fmt.Errorf("hostname cannot be empty")
		return
	}
	if hostnameLen > uint8(maxHostnameLen) {
		err = fmt.Errorf("hostname exceeds maximum length %d", maxHostnameLen)
		return
	}
	fields.Hostname = make([]byte, hostnameLen)
	if _, err = io.ReadFull(buf, fields.Hostname); err != nil {
		err = fmt.Errorf("failed to deserialize Hostname: %v", err)
		return
	}
	// Read terminator and check for presence
	term, err := buf.ReadByte()
	if err != nil {
		err = fmt.Errorf("failed to read Hostname terminator: %v", err)
		return
	}
	if term != terminatorByte {
		err = fmt.Errorf("expected null Hostname terminator, got 0x%02X", term)
		return
	}

	// ApplicationName
	var appNameLen uint8
	if err = binary.Read(buf, binary.BigEndian, &appNameLen); err != nil {
		err = fmt.Errorf("failed to deserialize ApplicationName length: %v", err)
		return
	}
	if appNameLen == 0 {
		err = fmt.Errorf("application name cannot be empty")
		return
	}
	if appNameLen > uint8(maxAppNameLen) {
		err = fmt.Errorf("application name exceeds maximum length %d", maxAppNameLen)
		return
	}
	fields.ApplicationName = make([]byte, appNameLen)
	if _, err = io.ReadFull(buf, fields.ApplicationName); err != nil {
		err = fmt.Errorf("failed to deserialize ApplicationName: %v", err)
		return
	}
	// Read terminator and check for presence
	term, err = buf.ReadByte()
	if err != nil {
		err = fmt.Errorf("failed to read ApplicationName terminator: %v", err)
		return
	}
	if term != terminatorByte {
		err = fmt.Errorf("expected null ApplicationName terminator, got 0x%02X", term)
		return
	}

	// LogText
	var logTextLen uint16
	if err = binary.Read(buf, binary.BigEndian, &logTextLen); err != nil {
		err = fmt.Errorf("failed to deserialize LogText length: %v", err)
		return
	}
	if logTextLen == 0 {
		err = fmt.Errorf("log text cannot be empty")
		return
	}
	fields.LogText = make([]byte, logTextLen)
	if _, err = io.ReadFull(buf, fields.LogText); err != nil {
		err = fmt.Errorf("failed to deserialize LogText: %v", err)
		return
	}
	// Read terminator and check for presence
	term, err = buf.ReadByte()
	if err != nil {
		err = fmt.Errorf("failed to read LogText terminator: %v", err)
		return
	}
	if term != terminatorByte {
		err = fmt.Errorf("expected null LogText terminator, got 0x%02X", term)
		return
	}

	// TRAILER
	// Length should be all left over bytes in the reader
	fields.PaddingLen = buf.Len()

	return
}
