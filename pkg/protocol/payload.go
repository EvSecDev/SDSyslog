package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/pkg/crypto/registry"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Creates individual packet inner payload
func ConstructPayload(request Payload, sigID uint8) (proto innerWireFormat, err error) {
	// MessageHostID
	if request.HostID == 0 {
		err = fmt.Errorf("%w: host ID cannot be zero", ErrInvalidPayload)
		return
	}
	proto.HostID = uint32(request.HostID)

	// MessageID
	proto.MsgID = uint32(request.MsgID)
	if request.MsgID == 0 {
		err = fmt.Errorf("%w: message ID cannot be zero", ErrInvalidPayload)
		return
	}

	// Sequence ID and Sequence Max
	if request.MessageSeq > request.MessageSeqMax {
		err = fmt.Errorf("%w: message sequence cannot be larger than maximum sequence", ErrInvalidPayload)
		return
	}
	proto.MessageSeq = uint16(request.MessageSeq)
	proto.MessageSeqMax = uint16(request.MessageSeqMax)

	// Timestamp: Convert time.Time to epoch milliseconds
	proto.Timestamp = uint64(request.Timestamp.UnixMilli())

	// Hostname: Clean and validate
	// Strip trust markers if in the hostname
	request.Hostname = strings.ReplaceAll(request.Hostname, HostPrefixUnkSig, "")
	request.Hostname = strings.ReplaceAll(request.Hostname, HostPrefixUnverified, "")
	proto.Hostname = cleanStringToBytes(request.Hostname, maxHostnameLen)

	if sigID > 0 {
		if len(request.Signature) > 0 {
			// Validate pre-computed signature
			suite, validID := registry.GetSignatureInfo(sigID)
			if !validID {
				err = fmt.Errorf("%w: %w: ID %d",
					ErrInvalidPayload, ErrUnknownSignatureSuite, sigID)
				return
			}
			if sigID != request.SignatureID {
				err = fmt.Errorf("%w: pre-computed payload signature ID %d does not match requested signature ID %d",
					ErrInvalidPayload, request.SignatureID, sigID)
				return
			}
			if len(request.Signature) > suite.MaxSignatureLength || len(request.Signature) < suite.MinSignatureLength {
				err = fmt.Errorf("%w: signature length %d for id %d must be between %d and %d bytes",
					ErrInvalidPayload, len(request.Signature), sigID, suite.MinSignatureLength, suite.MaxSignatureLength)
				return
			}

			// Permit pre-computed signature pass-through
			proto.Signature = request.Signature
		} else {
			// Concatenate values for signing
			bytesToSign := proto.SerializeForSignature()

			// Signature: sign timestamp and hostname
			proto.Signature, err = wrappers.CreateSignature(bytesToSign, sigID)
			if err != nil {
				err = fmt.Errorf("%w: %w", ErrCryptoFailure, err)
				return
			}
		}
		if len(proto.Signature) < minSignatureLen || len(proto.Signature) > maxSignatureLen {
			err = fmt.Errorf("%w: invalid signature length: must be between %d and %d bytes",
				ErrInvalidPayload, minSignatureLen, maxSignatureLen)
			return
		}
	}
	proto.SignatureID = sigID

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
				err = fmt.Errorf("%w: %w: key cannot be less than %d byte(s)",
					ErrInvalidPayload, ErrInvalidContextField, minCtxKeyLen)
				return
			}
			if len(cleanKey) > maxCtxKeyLen {
				err = fmt.Errorf("%w: %w: key %q: cannot be more than %d byte(s)",
					ErrInvalidPayload, ErrInvalidContextField, cleanKey, minCtxKeyLen)
				return
			}

			var cleanType uint8
			var cleanValue []byte
			if value == nil {
				// no value, use placeholder
				cleanType = ContextString
				cleanValue = []byte(EmptyFieldChar)
			} else {
				cleanType, cleanValue, err = serializeAnyValue(value)
				if err != nil {
					err = fmt.Errorf("%w: key %q: %w",
						ErrSerialization, cleanKey, err)
					return
				}
				if len(cleanValue) < minCtxValLen {
					cleanValue = []byte(EmptyFieldChar)
				}
				if len(cleanValue) > MaxCtxValLen {
					err = fmt.Errorf("%w: %w: field %q: value size of %d too long, must be less than %d bytes",
						ErrInvalidPayload, ErrInvalidContextField, key, len(cleanValue), MaxCtxValLen)
					return
				}
				// Only strings enforce utf8
				if cleanType == ContextString {
					if !utf8.Valid(cleanValue) {
						err = fmt.Errorf("%w: %w: key %q: non-UTF8 values are unsupported",
							ErrInvalidPayload, ErrInvalidContextField, string(cleanKey))
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
			err = fmt.Errorf("%w: custom fields serialized length is below expected minimum",
				ErrInvalidPayload)
			return
		}

		if totalCtxLen > maxCtxSectionLen {
			err = fmt.Errorf("%w: context section exceeds maximum custom field section limit (%d bytes)",
				ErrInvalidPayload, maxCtxSectionLen)
			return
		}
	}

	// Data Field - direct passthrough
	proto.Data = request.Data

	// Padding Length (random padding is generated as part of the trailer)
	if request.PaddingLen < minPaddingLen || request.PaddingLen > maxPaddingLen {
		err = fmt.Errorf("%w: invalid padding length %d: must be between %d and %d",
			ErrInvalidPayload, request.PaddingLen, minPaddingLen, maxPaddingLen)
		return
	}
	proto.PaddingLen = request.PaddingLen

	return
}

// Validates and extracts packet inner payload
func DeconstructPayload(proto innerWireFormat) (validated Payload, err error) {
	// Validate HostID
	if proto.HostID == 0 {
		err = fmt.Errorf("%w: empty host ID", ErrInvalidPayload)
		return
	}
	validated.HostID = int(proto.HostID)

	// Validate MsgID
	if proto.MsgID == 0 {
		err = fmt.Errorf("%w: empty msg ID", ErrInvalidPayload)
		return
	}
	validated.MsgID = int(proto.MsgID)

	// Validate Sequence ID and Sequence Max
	if proto.MessageSeq > proto.MessageSeqMax {
		err = fmt.Errorf("%w: message sequence greater than maximum: %d > %d",
			ErrInvalidPayload, proto.MessageSeq, proto.MessageSeqMax)
		return
	}
	validated.MessageSeq = int(proto.MessageSeq)
	validated.MessageSeqMax = int(proto.MessageSeqMax)

	// Validate Timestamp: Convert from milliseconds back to time.Time
	validated.Timestamp = time.UnixMilli(int64(proto.Timestamp))

	// Validate Hostname length and convert back to string
	if len(proto.Hostname) == 0 {
		err = fmt.Errorf("%w: empty hostname", ErrInvalidPayload)
		return
	}
	if len(proto.Hostname) > maxHostnameLen {
		err = fmt.Errorf("%w: exceeded maximum hostname length %d",
			ErrInvalidPayload, maxHostnameLen)
		return
	}
	if !isPrintableASCII(proto.Hostname) {
		err = fmt.Errorf("%w: non-ASCII hostname '%s' is not supported",
			ErrInvalidPayload, proto.Hostname)
		return
	}
	// Strip trust markers if in the hostname
	proto.Hostname = bytes.ReplaceAll(proto.Hostname, []byte(HostPrefixUnkSig), nil)
	proto.Hostname = bytes.ReplaceAll(proto.Hostname, []byte(HostPrefixUnverified), nil)
	validated.Hostname = string(proto.Hostname)

	pubKey, knownHost := wrappers.LookupPinnedSender(validated.Hostname)
	if knownHost && proto.SignatureID == 0 {
		// Pinned key without signature - Drop
		err = fmt.Errorf("%w: sender has a pinned key but received packet has no signature",
			ErrInvalidPayload)
		return
	} else if !knownHost && proto.SignatureID == 0 {
		// No pinned key and no signature - allow and mark untrusted
		validated.Hostname = HostPrefixUnverified + validated.Hostname
	} else if !knownHost && proto.SignatureID != 0 {
		// No pinned key and gratuitous signature - no verification attempt, mark as unknown
		validated.Hostname = HostPrefixUnkSig + validated.Hostname
	} else if knownHost && proto.SignatureID != 0 {
		// Pinned key with signature: verify timestamp and hostname
		bytesToVerify := proto.SerializeForSignature()
		var valid bool
		valid, err = wrappers.VerifySignature(pubKey, bytesToVerify, proto.Signature, proto.SignatureID)
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrCryptoFailure, err)
			return
		}
		if !valid {
			err = fmt.Errorf("%w: payload with alleged hostname %q has invalid signature",
				ErrInvalidPayload, validated.Hostname)
			return
		}
	}
	validated.SignatureID = proto.SignatureID
	validated.Signature = proto.Signature

	// Context: Deserialize
	if len(proto.ContextFields) > 0 {
		validated.CustomFields = make(map[string]any, len(proto.ContextFields))

		for _, field := range proto.ContextFields {
			// Key
			keyLength := len(field.Key)
			if keyLength < minCtxKeyLen {
				err = fmt.Errorf("%w: %w: key cannot be less than %d byte(s)",
					ErrInvalidPayload, ErrInvalidContextField, minCtxKeyLen)
				return
			}
			if keyLength > maxCtxKeyLen {
				err = fmt.Errorf("%w: %w: key %q: length of %d, cannot be more than %d byte(s)",
					ErrInvalidPayload, ErrInvalidContextField, string(field.Key), keyLength, minCtxKeyLen)
				return
			}
			if !isPrintableASCII(field.Key) {
				err = fmt.Errorf("%w: %w: key %q: non-ASCII key text is not supported",
					ErrInvalidPayload, ErrInvalidContextField, field.Key)
				return
			}

			// Value
			valueLength := len(field.Value)
			if valueLength < minCtxValLen || valueLength > MaxCtxValLen {
				err = fmt.Errorf("%w: %w: key %q: value length %d is invalid, must be between %d and %d",
					ErrInvalidPayload, ErrInvalidContextField, field.Key, valueLength, minCtxValLen, MaxCtxValLen)
				return
			}
			// Only strings enforce utf8
			if field.valType == ContextString {
				if !utf8.Valid(field.Value) {
					err = fmt.Errorf("%w: %w: key %q: non-UTF8 values are unsupported",
						ErrInvalidPayload, ErrInvalidContextField, string(field.Key))
					return
				}
			}

			var value any
			value, err = deserializeAnyValue(field.valType, field.Value)
			if err != nil {
				err = fmt.Errorf("%w: key %q: %w",
					ErrSerialization, string(field.Key), err)
				return
			}

			validated.CustomFields[string(field.Key)] = value
		}
	}

	// Validate data length
	if len(proto.Data) == 0 {
		err = fmt.Errorf("%w: empty data text", ErrInvalidPayload)
		return
	}
	validated.Data = proto.Data

	// Validate PaddingLen
	if proto.PaddingLen < minPaddingLen || proto.PaddingLen > maxPaddingLen {
		err = fmt.Errorf("%w: invalid PaddingLen %d: must be between %d and %d",
			ErrInvalidPayload, proto.PaddingLen, minPaddingLen, maxPaddingLen)
		return
	}
	validated.PaddingLen = proto.PaddingLen

	return
}

// Creates a concatenated byte slice of protocol fields that are signed/verified
func (proto innerWireFormat) SerializeForSignature() (signable []byte) {
	// Total length = ctx + timestamp(64b) + hostid(32b) + hostname
	signable = make([]byte, len(IdentitySignatureContext)+8+4+len(proto.Hostname))

	copy(signable, IdentitySignatureContext)

	tsStartIndex := len(IdentitySignatureContext)
	tsEndIndex := tsStartIndex + lenTimestamp
	binary.BigEndian.PutUint64(signable[tsStartIndex:tsEndIndex], proto.Timestamp)

	idEndIndex := tsEndIndex + lenHostID
	binary.BigEndian.PutUint32(signable[tsEndIndex:idEndIndex], proto.HostID)

	copy(signable[idEndIndex:], proto.Hostname)
	return
}

// Calculates the total byte size of the protocol, both inner and outer
func CalculateProtocolOverhead(suiteID uint8, primaryPayload Payload) (fixedOverhead int, err error) {
	cryptoInfo, validSuiteID := registry.GetSuiteInfo(suiteID)
	if !validSuiteID {
		err = fmt.Errorf("%w: ID %d", ErrUnknownSuite, suiteID)
		return
	}

	// Outer length is fixed based on chosen crypto suite
	outerTotal := registry.SuiteIDLen +
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
				err = fmt.Errorf("%w: failed to get length for field %q: %w",
					ErrSerialization, key, err)
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
