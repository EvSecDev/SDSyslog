package protocol

import (
	"errors"
	"sdsyslog/pkg/crypto/registry"
)

const (
	EmptyFieldChar             string = "-"
	MissingFragmentPlaceholder string = "[missing fragment]"
	HostPrefixUnkSig           string = "[UNKNOWN]"
	HostPrefixUnverified       string = "[UNVERIFIED]"
	IdentitySignatureContext   string = "D3P-ID-SIG"

	terminatorByte          byte   = 0x00
	customFieldsEmptyMarker uint16 = 0x0001

	ContextInt8       uint8 = 0x01
	ContextInt16      uint8 = 0x02
	ContextInt32      uint8 = 0x03
	ContextInt64      uint8 = 0x04
	ContextFloat32    uint8 = 0x05
	ContextFloat64    uint8 = 0x06
	ContextBool       uint8 = 0x07
	ContextString     uint8 = 0x08
	ContextSliceBytes uint8 = 0x09

	// Protocol wire field lengths (variable)
	minHostnameLen   int = 1
	maxHostnameLen   int = 255
	minSignatureLen  int = 0
	maxSignatureLen  int = 255
	maxCtxSectionLen int = (1 << (8 * lenContextSectionNxtLen)) - 1
	minCtxKeyLen     int = 1
	maxCtxKeyLen     int = 32
	minCtxValLen     int = 1
	maxCtxValLen     int = 255
	minDataLen       int = 1
	maxDataLen       int = (1 << (8 * lenDataNxtLen)) - 1
	minPaddingLen    int = 10
	maxPaddingLen    int = 60
	// Protocol wire field lengths (fixed fields)
	lenHostID    int = 4
	lenMsgID     int = 4
	lenMsgSeq    int = 2
	lenSeqMax    int = 2
	lenTimestamp int = 8
	// Len and terminator field lengths
	lenHostnameTerminator       int = 1
	lenContextSectionTerminator int = 1
	lenCtxKeyTerminator         int = 1
	lenCtxValTerminator         int = 1
	lenHostnameNxtLen           int = 1
	lenSigIDLen                 int = 1
	lenSigNxtLen                int = 1
	lenContextSectionNxtLen     int = 2
	lenCtxKeyNxtLen             int = 1
	lenCtxTypeVal               int = 1
	lenCtxValNxtLen             int = 1
	lenDataNxtLen               int = 2

	// Calculated
	minInnerPayloadLenFixedOnly int = lenHostID +
		lenMsgID +
		lenMsgSeq +
		lenSeqMax +
		lenTimestamp +
		lenContextSectionNxtLen +
		lenContextSectionTerminator +
		lenHostnameNxtLen +
		lenHostnameTerminator +
		lenSigIDLen +
		lenSigNxtLen +
		lenDataNxtLen
	minInnerPayloadLen int = lenHostID +
		lenMsgID +
		lenMsgSeq +
		lenSeqMax +
		lenTimestamp +
		lenContextSectionNxtLen +
		lenContextSectionTerminator +
		lenHostnameNxtLen +
		minHostnameLen +
		lenHostnameTerminator +
		lenSigIDLen +
		lenSigNxtLen +
		lenDataNxtLen +
		minDataLen +
		minPaddingLen
	MinOuterPayloadLen int = registry.SuiteIDLen + minInnerPayloadLen
	ctxFieldOverhead   int = lenCtxKeyNxtLen +
		lenCtxKeyTerminator +
		lenCtxTypeVal +
		lenCtxValNxtLen +
		lenCtxValTerminator
	minCtxSecLenWithData int = lenContextSectionNxtLen +
		lenCtxKeyNxtLen +
		minCtxKeyLen +
		lenCtxKeyTerminator +
		lenCtxTypeVal +
		lenCtxValNxtLen +
		minCtxValLen +
		lenCtxValTerminator +
		lenContextSectionTerminator
)

var (
	ErrInvalidPayload        = errors.New("invalid payload")
	ErrProtocolViolation     = errors.New("protocol violation")
	ErrSerialization         = errors.New("serialization failure")
	ErrFragmentation         = errors.New("fragmentation failure")
	ErrUnknownSuite          = errors.New("unknown crypto suite")
	ErrUnknownSignatureSuite = errors.New("unknown signature suite")
	ErrInvalidContextField   = errors.New("invalid context field")
	ErrCryptoFailure         = errors.New("cryptographic failure")
)
