// Handles all serialization/decryption for the protocol
package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
)

// Main Entry Point: Takes in a new log message to be sent and creates packets (transport layer payload)
func Create(newMsg Message, hostID int, maxPayloadSize int) (packets [][]byte, err error) {
	newMessageID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate random message identifier: %w", err)
		return
	}

	// Create internal log object
	newLog := Payload{
		HostID:       hostID,
		MsgID:        newMessageID,
		Timestamp:    newMsg.Timestamp,
		Hostname:     newMsg.Hostname,
		CustomFields: newMsg.Fields,
		Data:         []byte(newMsg.Data),
	}

	// Default to only supported
	cryptoSuiteInUse := uint8(1)

	protocolOverhead, err := CalculateProtocolOverhead(cryptoSuiteInUse, newLog)
	if err != nil {
		err = fmt.Errorf("failed to calculate protocol overhead: %w", err)
		return
	}

	fragments, err := Fragment(newLog, maxPayloadSize, protocolOverhead)
	if err != nil {
		err = fmt.Errorf("failed to fragment message: %w", err)
		return
	}

	for _, fragment := range fragments {
		var payload innerWireFormat
		payload, err = ValidatePayload(fragment)
		if err != nil {
			err = fmt.Errorf("invalid payload: %w", err)
			return
		}

		var innerPayload []byte
		innerPayload, err = ConstructInnerPayload(payload)
		if err != nil {
			err = fmt.Errorf("failed to serialize inner payload: %w", err)
			return
		}

		var outterPayload []byte
		outterPayload, err = ConstructOuterPayload(innerPayload, cryptoSuiteInUse)
		if err != nil {
			err = fmt.Errorf("failed to serialize outer payload: %w", err)
			return
		}

		packets = append(packets, outterPayload)
	}
	return
}
