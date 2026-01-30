package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

// Serializes inner packet payload into transport payload
// Does NOT validate fields against protocol spec.
func ConstructInnerPayload(fields innerWireFormat) (payload []byte, err error) {
	var buf bytes.Buffer

	// HEADER
	if err = binary.Write(&buf, binary.BigEndian, fields.HostID); err != nil {
		err = fmt.Errorf("failed to serialize HostID: %v", err)
		return
	}
	if err = binary.Write(&buf, binary.BigEndian, fields.MsgID); err != nil {
		err = fmt.Errorf("failed to serialize MsgID: %v", err)
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
	if err = binary.Write(&buf, binary.BigEndian, fields.Timestamp); err != nil {
		err = fmt.Errorf("failed to serialize Timestamp: %v", err)
		return
	}
	// Hostname
	if len(fields.Hostname) < minHostnameLen {
		err = fmt.Errorf("failed to serialize Hostname: field cannot be empty")
		return
	}
	buf.WriteByte(uint8(len(fields.Hostname)))
	if err = writeFixedLength(&buf, fields.Hostname, len(fields.Hostname)); err != nil {
		err = fmt.Errorf("failed to serialize Hostname: %v", err)
		return
	}
	buf.WriteByte(terminatorByte)

	// CONTEXT - fields
	var contextBuffer bytes.Buffer // temporary buffer to gather all the fields
	for _, ctxField := range fields.ContextFields {
		// Key
		contextBuffer.WriteByte(uint8(len(ctxField.Key)))
		if err = writeFixedLength(&contextBuffer, ctxField.Key, len(ctxField.Key)); err != nil {
			err = fmt.Errorf("failed to serialize Context field key: %v", err)
			return
		}
		contextBuffer.WriteByte(terminatorByte)

		// Type
		contextBuffer.WriteByte(ctxField.valType)

		// Value
		contextBuffer.WriteByte(uint8(len(ctxField.Value)))
		if err = writeFixedLength(&contextBuffer, ctxField.Value, len(ctxField.Value)); err != nil {
			err = fmt.Errorf("failed to serialize Context field value: %v", err)
			return
		}
		contextBuffer.WriteByte(terminatorByte)
	}
	// CONTEXT - section
	if contextBuffer.Len() > maxCtxSectionLen {
		err = fmt.Errorf("context section length (%d) exceeds maximum section length: %d", contextBuffer.Len(), maxCtxSectionLen)
		return
	}
	if contextBuffer.Len() > 0 {
		if err = writeUint16(&buf, uint16(contextBuffer.Len())); err != nil {
			err = fmt.Errorf("failed to serialize Context section length: %v", err)
			return
		}

		if err = writeFixedLength(&buf, contextBuffer.Bytes(), contextBuffer.Len()); err != nil {
			err = fmt.Errorf("failed to serialize Context section: %v", err)
			return
		}
	} else {
		if err = writeUint16(&buf, uint16(customFieldsEmptyMarker)); err != nil {
			err = fmt.Errorf("failed to serialize Context section marker length: %v", err)
			return
		}
	}
	buf.WriteByte(terminatorByte)

	// DATA
	if len(fields.Data) == 0 {
		err = fmt.Errorf("failed to serialize Data: field cannot be empty")
		return
	}
	if len(fields.Data) > maxDataLen {
		err = fmt.Errorf("log text length (%d) exceeds maximum field length: %d", len(fields.Data), maxDataLen)
		return
	}
	if err = writeUint16(&buf, uint16(len(fields.Data))); err != nil {
		err = fmt.Errorf("failed to serialize Data length: %v", err)
		return
	}
	if err = writeFixedLength(&buf, fields.Data, len(fields.Data)); err != nil {
		err = fmt.Errorf("failed to serialize Data: %v", err)
		return
	}
	buf.WriteByte(terminatorByte)

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
	if len(data) != length {
		err = fmt.Errorf("expected %d bytes, got %d", length, len(data))
		return
	}

	_, err = buf.Write(data)
	if err != nil {
		err = fmt.Errorf("failed to write: %v", err)
		return
	}

	return
}

// Deserializes transport payload into inner payload.
// Does NOT validate fields against protocol spec (only validates BASIC length)
func DeconstructInnerPayload(payload []byte) (fields innerWireFormat, err error) {
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
	if err = binary.Read(buf, binary.BigEndian, &fields.MsgID); err != nil {
		err = fmt.Errorf("failed to deserialize MsgID: %v", err)
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
	if err = binary.Read(buf, binary.BigEndian, &fields.Timestamp); err != nil {
		err = fmt.Errorf("failed to deserialize Timestamp: %v", err)
		return
	}
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
	err = readTerminator(buf, "Hostname")
	if err != nil {
		return
	}

	// CONTEXT
	var ctxSecLen uint16
	if err = binary.Read(buf, binary.BigEndian, &ctxSecLen); err != nil {
		err = fmt.Errorf("failed to deserialize Context section length: %v", err)
		return
	}
	if ctxSecLen == 0 {
		err = fmt.Errorf("context section length cannot be empty")
		return
	}
	if ctxSecLen != uint16(customFieldsEmptyMarker) {
		// Custom fields present, extract
		rawContextSection := make([]byte, ctxSecLen)
		if _, err = io.ReadFull(buf, rawContextSection); err != nil {
			err = fmt.Errorf("failed to deserialize Context section: %v", err)
			return
		}
		contextReader := bytes.NewReader(rawContextSection)
		for {
			var keyLen uint8
			keyLen, err = contextReader.ReadByte()
			if err == io.EOF {
				break // end of context section
			}
			if err != nil {
				err = fmt.Errorf("failed to read context field key length: %v", err)
				return
			}

			fieldKey := make([]byte, keyLen)
			if _, err = io.ReadFull(contextReader, fieldKey); err != nil {
				err = fmt.Errorf("failed to deserialize context field key: %v", err)
				return
			}
			err = readTerminator(contextReader, "Context field key")
			if err != nil {
				return
			}

			var valType uint8
			valType, err = contextReader.ReadByte()
			if err != nil {
				err = fmt.Errorf("failed to read context field value type: %v", err)
				return
			}

			var valLen uint8
			valLen, err = contextReader.ReadByte()
			if err != nil {
				err = fmt.Errorf("failed to read context field value length: %v", err)
				return
			}
			fieldValue := make([]byte, valLen)
			if _, err = io.ReadFull(contextReader, fieldValue); err != nil {
				err = fmt.Errorf("failed to deserialize context field value: %v", err)
				return
			}
			err = readTerminator(contextReader, "Context field value")
			if err != nil {
				return
			}

			extractedField := contextWireFormat{
				Key:     fieldKey,
				valType: valType,
				Value:   fieldValue,
			}
			fields.ContextFields = append(fields.ContextFields, extractedField)
		}

		if contextReader.Len() != 0 {
			err = fmt.Errorf("encountered EOF with %d bytes left in context field reader", contextReader.Len())
			return
		}
	}
	err = readTerminator(buf, "Context section")
	if err != nil {
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
	fields.Data = make([]byte, logTextLen)
	if _, err = io.ReadFull(buf, fields.Data); err != nil {
		err = fmt.Errorf("failed to deserialize LogText: %v", err)
		return
	}
	err = readTerminator(buf, "LogText")
	if err != nil {
		return
	}

	// TRAILER
	// Length should be all left over bytes in the reader
	fields.PaddingLen = buf.Len()

	return
}

// Reads terminator character and checks its validity.
// Supplied field name is for error enrichment.
func readTerminator(buf *bytes.Reader, fieldBeingTerminated string) (err error) {
	term, err := buf.ReadByte()
	if err != nil {
		err = fmt.Errorf("failed to read %s terminator: %v", fieldBeingTerminated, err)
		return
	}
	if term != terminatorByte {
		err = fmt.Errorf("expected null %s terminator, got 0x%02X", fieldBeingTerminated, term)
		return
	}
	return
}
