package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto"
	"time"
	"unicode"
)

// Creates individual packet inner payload
func ValidatePayload(request Payload) (proto InnerWireFormat, err error) {
	// MessageHostID
	proto.HostID = uint32(request.HostID)

	// MessageID
	proto.LogID = uint32(request.LogID)

	// Sequence ID and Sequence Max
	if request.MessageSeq > request.MessageSeqMax {
		err = fmt.Errorf("message sequence cannot be larger than maximum sequence")
		return
	}
	proto.MessageSeq = uint16(request.MessageSeq)
	proto.MessageSeqMax = uint16(request.MessageSeqMax)

	// Facility: Convert to numeric code
	proto.Facility, err = FacilityToCode(request.Facility)
	if err != nil {
		err = fmt.Errorf("invalid facility: %v", err)
		return
	}

	// Severity: Convert to numeric code
	proto.Severity, err = SeverityToCode(request.Severity)
	if err != nil {
		err = fmt.Errorf("invalid severity: %v", err)
		return
	}

	// Timestamp: Convert time.Time to epoch milliseconds
	proto.Timestamp = uint64(request.Timestamp.UnixMilli())

	// Process ID
	proto.ProcessID = uint32(request.ProcessID)

	// Hostname: Clean and validate
	proto.Hostname = cleanStringToBytes(request.Hostname, maxHostnameLen)

	// ApplicationName: Clean and validate
	proto.ApplicationName = cleanStringToBytes(request.ApplicationName, maxAppNameLen)

	// LogText: Clean and validate
	proto.LogText = cleanLogBytes(request.LogText)

	// Padding Length (random padding is generated as part of the trailer)
	if request.PaddingLen < minPaddingLen || request.PaddingLen > maxPaddingLen {
		err = fmt.Errorf("invalid padding length %d: must be between %d and %d", request.PaddingLen, minPaddingLen, maxPaddingLen)
		return
	}
	proto.PaddingLen = request.PaddingLen

	return
}

// Converts a string to bytes, truncates to maxLength and removes non-ASCII characters
func cleanStringToBytes(input string, maxLength int) (cleanBytes []byte) {
	if input == "" {
		input = emptyFieldChar // mandator empty placeholder
	}

	// Remove non-ASCII characters
	var cleanStr string
	for _, char := range input {
		if char <= unicode.MaxASCII {
			cleanStr += string(char)
		}
	}

	// Truncate to maxLength
	if len(cleanStr) > maxLength {
		cleanStr = cleanStr[:maxLength]
	}

	// Convert to byte slice
	cleanBytes = []byte(cleanStr)
	return
}

// Removes null-byte sequences from LogText and ensures UTF-8 encoding
func cleanLogBytes(log []byte) (cleanBytes []byte) {
	for _, b := range log {
		if b != 0 {
			cleanBytes = append(cleanBytes, b)
		}
	}
	return
}

// Validates and extracts packet inner payload
func ParsePayload(proto InnerWireFormat) (validated Payload, err error) {
	// Validate HostID
	if proto.HostID == 0 {
		err = fmt.Errorf("empty host ID")
		return
	}
	validated.HostID = int(proto.HostID)

	// Validate LogID
	if proto.LogID == 0 {
		err = fmt.Errorf("empty log ID")
		return
	}
	validated.LogID = int(proto.LogID)

	// Validate Sequence ID and Sequence Max
	if proto.MessageSeq > proto.MessageSeqMax {
		err = fmt.Errorf("message sequence greater than maximum: %d > %d", proto.MessageSeq, proto.MessageSeqMax)
		return
	}
	validated.MessageSeq = int(proto.MessageSeq)
	validated.MessageSeqMax = int(proto.MessageSeqMax)

	// Validate Facility: Convert numeric code back to string
	validated.Facility, err = CodeToFacility(proto.Facility)
	if err != nil {
		err = fmt.Errorf("invalid facility: %v", err)
		return
	}

	// Validate Severity: Convert numeric code back to string
	validated.Severity, err = CodeToSeverity(proto.Severity)
	if err != nil {
		err = fmt.Errorf("invalid severity: %v", err)
		return
	}

	// Validate Timestamp: Convert from milliseconds back to time.Time
	validated.Timestamp = time.UnixMilli(int64(proto.Timestamp))

	// Validate ProcessID
	if proto.ProcessID == 0 {
		err = fmt.Errorf("empty process ID")
		return
	}
	validated.ProcessID = int(proto.ProcessID)

	// Validate Hostname length and convert back to string
	if len(proto.Hostname) == 0 {
		err = fmt.Errorf("empty hostname")
		return
	}
	if len(proto.Hostname) > maxHostnameLen {
		err = fmt.Errorf("exceeded maximum hostname length %d", maxHostnameLen)
		return
	}
	validated.Hostname = string(proto.Hostname)

	// Validate ApplicationName length and convert back to string
	if len(proto.ApplicationName) == 0 {
		err = fmt.Errorf("empty application name")
		return
	}
	if len(proto.ApplicationName) > maxAppNameLen {
		err = fmt.Errorf("exceeded maximum application name length %d", maxAppNameLen)
		return
	}
	validated.ApplicationName = string(proto.ApplicationName)

	// Validate LogText length and convert back to string
	if len(proto.LogText) == 0 {
		err = fmt.Errorf("empty log text")
		return
	}
	validated.LogText = proto.LogText

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
	innerVariableLength :=
		len(primaryPayload.Hostname) +
			len(primaryPayload.ApplicationName)

		// Return sum of overheads
	fixedOverhead = outerTotal + minInnerPayloadLenFixedOnly + innerVariableLength
	return
}
