// Handles all serialization/decryption for the protocol
package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
)

// Main Entry Point: Takes in a new log message to be sent and creates packets (transport layer payload)
func Create(newMsg Message, hostID int, maxPayloadSize int) (packets [][]byte, err error) {
	newLogID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate random log identifier: %v", err)
		return
	}

	// Create internal log object
	newLog := Payload{
		HostID:          hostID,
		LogID:           newLogID,
		Facility:        newMsg.Facility,
		Severity:        newMsg.Severity,
		Timestamp:       newMsg.Timestamp,
		ProcessID:       newMsg.ProcessID,
		Hostname:        newMsg.Hostname,
		ApplicationName: newMsg.ApplicationName,
		LogText:         []byte(newMsg.LogText),
	}

	// Default to only supported
	cryptoSuiteInUse := uint8(1)

	protocolOverhead, err := CalculateProtocolOverhead(cryptoSuiteInUse, newLog)
	if err != nil {
		err = fmt.Errorf("failed to calculate protocol overhead: %v", err)
		return
	}

	fragments, err := Fragment(newLog, maxPayloadSize, protocolOverhead)
	if err != nil {
		err = fmt.Errorf("failed to fragment log message: %v", err)
		return
	}

	for _, fragment := range fragments {
		var payload InnerWireFormat
		payload, err = ValidatePayload(fragment)
		if err != nil {
			err = fmt.Errorf("invalid payload: %v", err)
			return
		}

		var innerPayload []byte
		innerPayload, err = ConstructInnerPayload(payload)
		if err != nil {
			err = fmt.Errorf("failed to serialize inner payload: %v", err)
			return
		}

		var outterPayload []byte
		outterPayload, err = ContructOuterPayload(innerPayload, cryptoSuiteInUse)
		if err != nil {
			err = fmt.Errorf("failed to serialize outer payload: %v", err)
			return
		}

		packets = append(packets, outterPayload)
	}
	return
}
