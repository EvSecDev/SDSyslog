package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sdsyslog/pkg/crypto/registry"
)

// Serializes inner packet payload into transport payload
// Does NOT validate fields against protocol spec.
func ConstructInnerPayload(fields *innerWireFormat) (payload []byte, err error) {
	var buf bytes.Buffer

	// HEADER
	err = binary.Write(&buf, binary.BigEndian, fields.HostID)
	if err != nil {
		err = fmt.Errorf("%w: HostID: %w", ErrSerialization, err)
		return
	}
	err = binary.Write(&buf, binary.BigEndian, fields.MsgID)
	if err != nil {
		err = fmt.Errorf("%w: MsgID: %w", ErrSerialization, err)
		return
	}
	err = binary.Write(&buf, binary.BigEndian, fields.MessageSeq)
	if err != nil {
		err = fmt.Errorf("%w: MessageSeq: %w", ErrSerialization, err)
		return
	}
	err = binary.Write(&buf, binary.BigEndian, fields.MessageSeqMax)
	if err != nil {
		err = fmt.Errorf("%w: MessageSeqMax: %w", ErrSerialization, err)
		return
	}

	// METADATA
	// Timestamp
	err = binary.Write(&buf, binary.BigEndian, fields.Timestamp)
	if err != nil {
		err = fmt.Errorf("%w: Timestamp: %w", ErrSerialization, err)
		return
	}
	// Hostname
	if len(fields.Hostname) < minHostnameLen {
		err = fmt.Errorf("%w: Hostname: field cannot be empty", ErrProtocolViolation)
		return
	}
	buf.WriteByte(uint8(len(fields.Hostname)))
	err = writeFixedLength(&buf, fields.Hostname, len(fields.Hostname))
	if err != nil {
		err = fmt.Errorf("%w: Hostname: %w", ErrSerialization, err)
		return
	}
	buf.WriteByte(terminatorByte)
	// Signature
	err = buf.WriteByte(fields.SignatureID)
	if err != nil {
		err = fmt.Errorf("%w: signature ID", ErrSerialization)
		return
	}
	err = buf.WriteByte(uint8(len(fields.Signature)))
	if err != nil {
		err = fmt.Errorf("%w: signature length", ErrSerialization)
		return
	}
	if len(fields.Signature) > minSignatureLen {
		err = writeFixedLength(&buf, fields.Signature, len(fields.Signature))
		if err != nil {
			err = fmt.Errorf("%w: Signature: %w", ErrSerialization, err)
			return
		}
	}

	// CONTEXT - fields
	var contextBuffer bytes.Buffer // temporary buffer to gather all the fields
	for _, ctxField := range fields.ContextFields {
		// Key
		contextBuffer.WriteByte(uint8(len(ctxField.Key)))
		err = writeFixedLength(&contextBuffer, ctxField.Key, len(ctxField.Key))
		if err != nil {
			err = fmt.Errorf("%w: Context field key: %w", ErrSerialization, err)
			return
		}
		contextBuffer.WriteByte(terminatorByte)

		// Type
		contextBuffer.WriteByte(ctxField.valType)

		// Value
		contextBuffer.WriteByte(uint8(len(ctxField.Value)))
		err = writeFixedLength(&contextBuffer, ctxField.Value, len(ctxField.Value))
		if err != nil {
			err = fmt.Errorf("%w: Context field value: %w", ErrSerialization, err)
			return
		}
		contextBuffer.WriteByte(terminatorByte)
	}
	// CONTEXT - section
	if contextBuffer.Len() > maxCtxSectionLen {
		err = fmt.Errorf("%w: context section length (%d) exceeds maximum section length: %d",
			ErrProtocolViolation, contextBuffer.Len(), maxCtxSectionLen)
		return
	}
	if contextBuffer.Len() > 0 {
		err = writeUint16(&buf, uint16(contextBuffer.Len()))
		if err != nil {
			err = fmt.Errorf("%w: Context section length: %w", ErrSerialization, err)
			return
		}

		err = writeFixedLength(&buf, contextBuffer.Bytes(), contextBuffer.Len())
		if err != nil {
			err = fmt.Errorf("%w: Context section: %w", ErrSerialization, err)
			return
		}
	} else {
		err = writeUint16(&buf, customFieldsEmptyMarker)
		if err != nil {
			err = fmt.Errorf("%w: Context section marker length: %w", ErrSerialization, err)
			return
		}
	}
	buf.WriteByte(terminatorByte)

	// DATA
	if len(fields.Data) == 0 {
		err = fmt.Errorf("%w: Data: field cannot be empty", ErrProtocolViolation)
		return
	}
	if len(fields.Data) > maxDataLen {
		err = fmt.Errorf("%w: data field length (%d) exceeds maximum field length: %d",
			ErrProtocolViolation, len(fields.Data), maxDataLen)
		return
	}
	err = writeUint16(&buf, uint16(len(fields.Data)))
	if err != nil {
		err = fmt.Errorf("%w: Data length: %w", ErrSerialization, err)
		return
	}
	err = writeFixedLength(&buf, fields.Data, len(fields.Data))
	if err != nil {
		err = fmt.Errorf("%w: Data: %w", ErrSerialization, err)
		return
	}

	// TRAILER
	padding := make([]byte, fields.PaddingLen)
	_, err = io.ReadFull(rand.Reader, padding)
	if err != nil {
		err = fmt.Errorf("failed to generate random padding: %w", err)
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
		err = fmt.Errorf("failed to write uint16: %w", err)
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
		err = fmt.Errorf("failed to write: %w", err)
		return
	}

	return
}

// Deserializes transport payload into inner payload.
// Does NOT validate fields against protocol spec (only validates BASIC length)
func DeconstructInnerPayload(payload []byte) (fields *innerWireFormat, err error) {
	fields = &innerWireFormat{}

	// Immediately reject invalid length
	if len(payload) < minInnerPayloadLen {
		err = fmt.Errorf("%w: invalid payload length %d: must be minimum length of %d",
			ErrProtocolViolation, len(payload), minInnerPayloadLen)
		return
	}

	buf := bytes.NewReader(payload)

	// HEADER
	err = binary.Read(buf, binary.BigEndian, &fields.HostID)
	if err != nil {
		err = fmt.Errorf("%w: HostID: %w", ErrSerialization, err)
		return
	}
	err = binary.Read(buf, binary.BigEndian, &fields.MsgID)
	if err != nil {
		err = fmt.Errorf("%w: MsgID: %w", ErrSerialization, err)
		return
	}
	err = binary.Read(buf, binary.BigEndian, &fields.MessageSeq)
	if err != nil {
		err = fmt.Errorf("%w: MessageSeq: %w", ErrSerialization, err)
		return
	}
	err = binary.Read(buf, binary.BigEndian, &fields.MessageSeqMax)
	if err != nil {
		err = fmt.Errorf("%w: MessageSeqMax: %w", ErrSerialization, err)
		return
	}

	// METADATA
	err = binary.Read(buf, binary.BigEndian, &fields.Timestamp)
	if err != nil {
		err = fmt.Errorf("%w: Timestamp: %w", ErrSerialization, err)
		return
	}
	// Hostname
	var hostnameLen uint8
	err = binary.Read(buf, binary.BigEndian, &hostnameLen)
	if err != nil {
		err = fmt.Errorf("%w: Hostname length: %w", ErrSerialization, err)
		return
	}
	if hostnameLen == 0 {
		err = fmt.Errorf("%w: hostname cannot be empty", ErrProtocolViolation)
		return
	}
	if hostnameLen > uint8(maxHostnameLen) {
		err = fmt.Errorf("%w: hostname exceeds maximum length %d",
			ErrProtocolViolation, maxHostnameLen)
		return
	}
	fields.Hostname = make([]byte, hostnameLen)
	_, err = io.ReadFull(buf, fields.Hostname)
	if err != nil {
		err = fmt.Errorf("%w: Hostname: %w", ErrSerialization, err)
		return
	}
	err = readTerminator(buf, "Hostname")
	if err != nil {
		return
	}
	// Signature
	var sigID uint8
	err = binary.Read(buf, binary.BigEndian, &sigID)
	if err != nil {
		err = fmt.Errorf("%w: signature ID: %w", ErrSerialization, err)
		return
	}
	suite, validID := registry.GetSignatureInfo(sigID)
	if !validID {
		err = fmt.Errorf("%w: ID %d", ErrUnknownSignatureSuite, sigID)
		return
	}
	var sigLen uint8
	err = binary.Read(buf, binary.BigEndian, &sigLen)
	if err != nil {
		err = fmt.Errorf("%w: signature length: %w", ErrSerialization, err)
		return
	}
	if sigID == 0 && sigLen > 0 {
		err = fmt.Errorf("%w: signature ID of 0 and signature length greater than zero is invalid",
			ErrProtocolViolation)
		return
	}
	if sigID > 0 && sigLen == 0 {
		err = fmt.Errorf("%w: signature length field cannot be zero when non-zero signature ID is present",
			ErrProtocolViolation)
		return
	}
	// Empty sig lengths skip signature field
	if sigLen > 0 {
		if sigLen > uint8(suite.MaxSignatureLength) || sigLen < uint8(suite.MinSignatureLength) {
			err = fmt.Errorf("%w: signature length %d for id %d must be between %d and %d bytes",
				ErrProtocolViolation, sigLen, sigID, suite.MinSignatureLength, suite.MaxSignatureLength)
			return
		}
		fields.Signature = make([]byte, sigLen)
		_, err = io.ReadFull(buf, fields.Signature)
		if err != nil {
			err = fmt.Errorf("%w: Signature field: %w", ErrSerialization, err)
			return
		}
	}
	fields.SignatureID = sigID

	// CONTEXT
	err = deconstructContextSection(buf, fields)
	if err != nil {
		return
	}

	// Data
	var dataLen uint16
	err = binary.Read(buf, binary.BigEndian, &dataLen)
	if err != nil {
		err = fmt.Errorf("%w: data field length: %w", ErrSerialization, err)
		return
	}
	if dataLen == 0 {
		err = fmt.Errorf("%w: data field cannot be empty", ErrProtocolViolation)
		return
	}
	if len(fields.Data) > maxDataLen {
		err = fmt.Errorf("%w: data field length (%d) exceeds maximum field length: %d",
			ErrProtocolViolation, len(fields.Data), maxDataLen)
		return
	}
	if int(dataLen) > buf.Len() {
		err = fmt.Errorf("%w: declared data field length exceeds remaining available payload buffer bytes",
			ErrProtocolViolation)
		return
	}
	fields.Data = make([]byte, dataLen)
	_, err = io.ReadFull(buf, fields.Data)
	if err != nil {
		err = fmt.Errorf("%w: data field: %w", ErrSerialization, err)
		return
	}

	// TRAILER
	// Length should be all left over bytes in the reader
	fields.PaddingLen = buf.Len()

	return
}

func deconstructContextSection(buf *bytes.Reader, fields *innerWireFormat) (err error) {
	var ctxSecLen uint16
	err = binary.Read(buf, binary.BigEndian, &ctxSecLen)
	if err != nil {
		err = fmt.Errorf("%w: Context section length: %w", ErrSerialization, err)
		return
	}
	if ctxSecLen == 0 {
		err = fmt.Errorf("%w: context section length cannot be empty", ErrProtocolViolation)
		return
	}
	if ctxSecLen != customFieldsEmptyMarker {
		// Custom fields present, extract
		rawContextSection := make([]byte, ctxSecLen)
		_, err = io.ReadFull(buf, rawContextSection)
		if err != nil {
			err = fmt.Errorf("%w: Context section: %w", ErrSerialization, err)
			return
		}
		contextReader := bytes.NewReader(rawContextSection)
		for {
			var keyLen uint8
			keyLen, err = contextReader.ReadByte()
			if errors.Is(err, io.EOF) {
				break // end of context section
			}
			if err != nil {
				err = fmt.Errorf("%w: failed to read context field key length: %w",
					ErrSerialization, err)
				return
			}

			fieldKey := make([]byte, keyLen)
			_, err = io.ReadFull(contextReader, fieldKey)
			if err != nil {
				err = fmt.Errorf("%w: context field key: %w", ErrSerialization, err)
				return
			}
			err = readTerminator(contextReader, "Context field key")
			if err != nil {
				return
			}

			var valType uint8
			valType, err = contextReader.ReadByte()
			if err != nil {
				err = fmt.Errorf("%w: failed to read context field value type: %w", ErrSerialization, err)
				return
			}

			var valLen uint8
			valLen, err = contextReader.ReadByte()
			if err != nil {
				err = fmt.Errorf("%w: failed to read context field value length: %w", ErrSerialization, err)
				return
			}
			fieldValue := make([]byte, valLen)
			_, err = io.ReadFull(contextReader, fieldValue)
			if err != nil {
				err = fmt.Errorf("%w: context field value: %w", ErrSerialization, err)
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
			err = fmt.Errorf("%w: encountered EOF with %d bytes left in context field reader",
				ErrSerialization, contextReader.Len())
			return
		}
	}
	err = readTerminator(buf, "Context section")
	if err != nil {
		return
	}
	return
}

// Reads terminator character and checks its validity.
// Supplied field name is for error enrichment.
func readTerminator(buf *bytes.Reader, fieldBeingTerminated string) (err error) {
	term, err := buf.ReadByte()
	if err != nil {
		err = fmt.Errorf("failed to read %s terminator: %w", fieldBeingTerminated, err)
		return
	}
	if term != terminatorByte {
		err = fmt.Errorf("expected null %s terminator, got 0x%02X", fieldBeingTerminated, term)
		return
	}
	return
}
