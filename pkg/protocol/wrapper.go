// Handles all serialization/decryption for the protocol
package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
)

// Main Entry Point: Takes in a new message to be sent and creates packets (transport layer payload)
func Create(sendMsg Message, hostID int, maxPayloadSize int) (packets [][]byte, err error) {
	newMessageID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate random message identifier: %w", err)
		return
	}

	// Create internal payload object
	newMsg := Payload{
		HostID:       hostID,
		MsgID:        newMessageID,
		Timestamp:    sendMsg.Timestamp,
		Hostname:     sendMsg.Hostname,
		CustomFields: sendMsg.Fields,
		Data:         []byte(sendMsg.Data),
	}

	// Default to only supported
	cryptoSuiteInUse := uint8(1)

	protocolOverhead, err := CalculateProtocolOverhead(cryptoSuiteInUse, newMsg)
	if err != nil {
		err = fmt.Errorf("failed to calculate protocol overhead: %w", err)
		return
	}

	fragments, err := Fragment(newMsg, maxPayloadSize, protocolOverhead)
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

// Main Entry Point: Takes in multiple raw packets (transport layer payload) and returns the message
func Extract(packets [][]byte) (recvMsg Message, hostID int, err error) {
	if len(packets) == 0 {
		return
	}

	var fragments []Payload
	for index, packet := range packets {
		var innerPayload []byte
		innerPayload, err = DeconstructOuterPayload(packet)
		if err != nil {
			err = fmt.Errorf("failed to deserialize outer payload for packet %d: %w", index, err)
			return
		}

		var payload innerWireFormat
		payload, err = DeconstructInnerPayload(innerPayload)
		if err != nil {
			err = fmt.Errorf("failed to deserialize inner payload for packet %d: %w", index, err)
			return
		}

		var messageFragment Payload
		messageFragment, err = ParsePayload(payload)
		if err != nil {
			err = fmt.Errorf("invalid payload for packet %d: %w", index, err)
			return
		}

		fragments = append(fragments, messageFragment)
	}

	primaryPayload, err := Defragment(fragments)
	if err != nil {
		err = fmt.Errorf("failed to defragment message: %w", err)
		return
	}

	recvMsg = Message{
		Timestamp: primaryPayload.Timestamp,
		Hostname:  primaryPayload.Hostname,
		Fields:    primaryPayload.CustomFields,
		Data:      string(primaryPayload.Data),
	}
	hostID = primaryPayload.HostID
	return
}
