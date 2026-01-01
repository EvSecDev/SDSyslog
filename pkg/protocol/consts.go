package protocol

import "sdsyslog/internal/crypto"

const (
	terminatorByte        byte   = 0x00
	emptyFieldChar        string = "-"
	missingLogPlaceholder string = "[missing fragment]"

	// Protocol wire field lengths (variable)
	minHostnameLen int = 1
	minAppNameLen  int = 1
	maxHostnameLen int = 255
	maxAppNameLen  int = 48
	minLogTextLen  int = 1
	maxLogTextLen  int = (1 << (8 * lenLogTextNxtLen)) - 1
	minPaddingLen  int = 10
	maxPaddingLen  int = 60
	// Protocol wire field lengths (fixed fields)
	lenHostID    int = 4
	lenLogID     int = 4
	lenMsgSeq    int = 2
	lenSeqMax    int = 2
	lenFacility  int = 2
	lenSeverity  int = 2
	lenTimestamp int = 8
	lenProcID    int = 4
	// Len and terminator field lengths
	lenHostnameTerminator int = 1
	lenAppNameTerminator  int = 1
	lenLogTextTerminator  int = 1
	lenHostnameNxtLen     int = 1
	lenAppNameNxtLen      int = 1
	lenLogTextNxtLen      int = 2

	// Calculated
	minInnerPayloadLenFixedOnly int = lenHostID +
		lenLogID +
		lenMsgSeq +
		lenSeqMax +
		lenFacility +
		lenSeverity +
		lenTimestamp +
		lenProcID +
		lenAppNameNxtLen +
		lenAppNameTerminator +
		lenHostnameNxtLen +
		lenHostnameTerminator +
		lenLogTextNxtLen +
		lenLogTextTerminator
	minInnerPayloadLen int = lenHostID +
		lenLogID +
		lenMsgSeq +
		lenSeqMax +
		lenFacility +
		lenSeverity +
		lenTimestamp +
		lenProcID +
		lenAppNameNxtLen +
		minAppNameLen +
		lenAppNameTerminator +
		lenHostnameNxtLen +
		minHostnameLen +
		lenHostnameTerminator +
		lenLogTextNxtLen +
		minLogTextLen +
		lenLogTextTerminator +
		minPaddingLen
	MinOuterPayloadLen int = crypto.SuiteIDLen + minInnerPayloadLen
)
