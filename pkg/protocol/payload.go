package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto"
	"sort"
	"time"
	"unicode/utf8"
)

// Creates individual packet inner payload
func ValidatePayload(request Payload) (proto innerWireFormat, err error) {
	// MessageHostID
	if request.HostID == 0 {
		err = fmt.Errorf("host ID cannot be zero")
		return
	}
	proto.HostID = uint32(request.HostID)

	// MessageID
	proto.MsgID = uint32(request.MsgID)
	if request.MsgID == 0 {
		err = fmt.Errorf("message ID cannot be zero")
		return
	}

	// Sequence ID and Sequence Max
	if request.MessageSeq > request.MessageSeqMax {
		err = fmt.Errorf("message sequence cannot be larger than maximum sequence")
		return
	}
	proto.MessageSeq = uint16(request.MessageSeq)
	proto.MessageSeqMax = uint16(request.MessageSeqMax)

	// Timestamp: Convert time.Time to epoch milliseconds
	proto.Timestamp = uint64(request.Timestamp.UnixMilli())

	// Hostname: Clean and validate
	proto.Hostname = cleanStringToBytes(request.Hostname, maxHostnameLen)

	// Context: Serialize
	if len(request.CustomFields) > 0 {
		proto.ContextFields = make([]contextWireFormat, 0, len(request.CustomFields))

		// Ensure predictable order of wire format context fields
		keys := make([]string, 0, len(request.CustomFields))
		for k := range request.CustomFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var totalCtxLen int
		for _, key := range keys {
			value := request.CustomFields[key]

			cleanKey := cleanStringToBytes(key, maxCtxKeyLen)
			if len(cleanKey) < minCtxKeyLen {
				err = fmt.Errorf("custom field key cannot be less than %d byte(s)", minCtxKeyLen)
				return
			}

			var cleanType uint8
			var cleanValue []byte
			if value == nil {
				// no value, use placeholder
				cleanType = ContextString
				cleanValue = []byte(emptyFieldChar)
			} else {
				cleanType, cleanValue, err = serializeAnyValue(value)
				if err != nil {
					err = fmt.Errorf("invalid custom field '%s': %w", cleanKey, err)
					return
				}
				if len(cleanValue) < minCtxValLen {
					cleanValue = []byte(emptyFieldChar)
				}
				if len(cleanValue) > maxCtxValLen {
					err = fmt.Errorf("value too long for key '%s' (must be less than %d bytes)", key, maxCtxValLen)
					return
				}
				// Only strings enforce utf8
				if cleanType == ContextString {
					if !utf8.Valid(cleanValue) {
						err = fmt.Errorf("non-UTF8 context field value for key '%s' is unsupported", string(cleanKey))
						return
					}
				}
			}

			newCtxEntry := contextWireFormat{
				Key:     cleanKey,
				valType: cleanType,
				Value:   cleanValue,
			}
			proto.ContextFields = append(proto.ContextFields, newCtxEntry)
			// Peeking true length of eventual serialized bytes
			totalCtxLen += lenCtxKeyNxtLen + len(newCtxEntry.Key) + lenCtxKeyTerminator +
				lenCtxTypeVal +
				lenCtxValNxtLen + len(newCtxEntry.Value) + lenCtxValTerminator
		}

		if totalCtxLen < minCtxSecLenWithData {
			err = fmt.Errorf("custom field serialized length is below expected minimum")
			return
		}

		if totalCtxLen > maxCtxSectionLen {
			err = fmt.Errorf("context section exceeds maximum custom field section limit (%d bytes)", maxCtxSectionLen)
			return
		}
	}

	// LogText: Clean and validate
	proto.Data = cleanBytes(request.Data)
	if !utf8.Valid(proto.Data) {
		err = fmt.Errorf("non-UTF8 data is unsupported")
		return
	}

	// Padding Length (random padding is generated as part of the trailer)
	if request.PaddingLen < minPaddingLen || request.PaddingLen > maxPaddingLen {
		err = fmt.Errorf("invalid padding length %d: must be between %d and %d", request.PaddingLen, minPaddingLen, maxPaddingLen)
		return
	}
	proto.PaddingLen = request.PaddingLen

	return
}

// Validates and extracts packet inner payload
func ParsePayload(proto innerWireFormat) (validated Payload, err error) {
	// Validate HostID
	if proto.HostID == 0 {
		err = fmt.Errorf("empty host ID")
		return
	}
	validated.HostID = int(proto.HostID)

	// Validate MsgID
	if proto.MsgID == 0 {
		err = fmt.Errorf("empty msg ID")
		return
	}
	validated.MsgID = int(proto.MsgID)

	// Validate Sequence ID and Sequence Max
	if proto.MessageSeq > proto.MessageSeqMax {
		err = fmt.Errorf("message sequence greater than maximum: %d > %d", proto.MessageSeq, proto.MessageSeqMax)
		return
	}
	validated.MessageSeq = int(proto.MessageSeq)
	validated.MessageSeqMax = int(proto.MessageSeqMax)

	// Validate Timestamp: Convert from milliseconds back to time.Time
	validated.Timestamp = time.UnixMilli(int64(proto.Timestamp))

	// Validate Hostname length and convert back to string
	if len(proto.Hostname) == 0 {
		err = fmt.Errorf("empty hostname")
		return
	}
	if len(proto.Hostname) > maxHostnameLen {
		err = fmt.Errorf("exceeded maximum hostname length %d", maxHostnameLen)
		return
	}
	if !isPrintableASCII(proto.Hostname) {
		err = fmt.Errorf("non-ASCII hostname '%s' is not supported", proto.Hostname)
		return
	}
	validated.Hostname = string(proto.Hostname)

	// Context: Deserialize
	if len(proto.ContextFields) > 0 {
		validated.CustomFields = make(map[string]any, len(proto.ContextFields))

		for _, field := range proto.ContextFields {
			// Key
			keyLength := len(field.Key)
			if keyLength < minCtxKeyLen || keyLength > maxCtxKeyLen {
				err = fmt.Errorf("invalid context field key length: %d", keyLength)
				return
			}
			if !isPrintableASCII(field.Key) {
				err = fmt.Errorf("non-ASCII context field key '%s' is not supported", field.Key)
				return
			}

			// Value
			valueLength := len(field.Value)
			if valueLength < minCtxValLen || valueLength > maxCtxValLen {
				err = fmt.Errorf("invalid context field value length: %d", valueLength)
				return
			}
			// Only strings enforce utf8
			if field.valType == ContextString {
				if !utf8.Valid(field.Value) {
					err = fmt.Errorf("non-UTF8 context field value for key '%s' is unsupported", string(field.Key))
					return
				}
			}

			var value any
			value, err = deserializeAnyValue(field.valType, field.Value)
			if err != nil {
				err = fmt.Errorf("invalid custom field '%s': %w", string(field.Key), err)
				return
			}

			validated.CustomFields[string(field.Key)] = value
		}
	}

	// Validate data length and convert back to string
	if len(proto.Data) == 0 {
		err = fmt.Errorf("empty data text")
		return
	}
	if !utf8.Valid(proto.Data) {
		err = fmt.Errorf("non-UTF8 data is unsupported")
		return
	}
	validated.Data = proto.Data

	// Validate PaddingLen
	if proto.PaddingLen < minPaddingLen || proto.PaddingLen > maxPaddingLen {
		err = fmt.Errorf("invalid PaddingLen %d: must be between %d and %d", proto.PaddingLen, minPaddingLen, maxPaddingLen)
		return
	}
	validated.PaddingLen = proto.PaddingLen

	return
}

// Calculates the total byte size of the protocol, both inner and outer
func CalculateProtocolOverhead(suiteID uint8, primaryPayload Payload) (fixedOverhead int, err error) {
	cryptoInfo, validSuiteID := crypto.GetSuiteInfo(suiteID)
	if !validSuiteID {
		err = fmt.Errorf("unknown crypto suite ID %d", suiteID)
		return
	}

	// Outer length is fixed based on chosen crypto suite
	outerTotal := crypto.SuiteIDLen +
		cryptoInfo.KeySize +
		cryptoInfo.NonceSize +
		cryptoInfo.CipherOverhead

	// Calculate variable lengths from primary payload
	// padding length changes, omit from this calculation
	innerVariableLength := len(primaryPayload.Hostname)
	if len(primaryPayload.CustomFields) > 0 {
		for key, value := range primaryPayload.CustomFields {
			innerVariableLength += ctxFieldOverhead

			innerVariableLength += len(key)

			var data []byte
			_, data, err = serializeAnyValue(value)
			if err != nil {
				err = fmt.Errorf("failed to get serialized length for field '%s'", key)
				return
			}
			innerVariableLength += len(data)
		}
	}
	innerVariableLength += lenContextSectionNxtLen + lenContextSectionTerminator

	// Return sum of overheads
	fixedOverhead = outerTotal + minInnerPayloadLenFixedOnly + innerVariableLength
	return
}
