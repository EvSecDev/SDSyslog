// Handles all serialization/decryption for the protocol
package protocol

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
)

// Main Entry Point: Takes in a new message to be sent and creates packets (transport layer payload)
func Create(sendMsg *Message, hostID int, maxPayloadSize int, cryptoSuite, signatureSuite uint8) (packets [][]byte, err error) {
	newMessageID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate random message identifier: %w", err)
		return
	}

	// Create internal payload object
	newMsg := &Payload{
		HostID:       hostID,
		MsgID:        newMessageID,
		Timestamp:    sendMsg.Timestamp,
		Hostname:     sendMsg.Hostname,
		CustomFields: sendMsg.Fields,
		Data:         sendMsg.Data,
	}

	// Create signature pre-fragmentation (signature validation is done in payload construction)
	if signatureSuite > 0 {
		bitTime := uint64(newMsg.Timestamp.UnixMilli())
		bytesToSign := SerializeSignature([]byte(newMsg.Hostname), uint32(newMsg.HostID), bitTime)
		newMsg.Signature, err = wrappers.CreateSignature(bytesToSign, signatureSuite)
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrCryptoFailure, err)
			return
		}
	}
	newMsg.SignatureID = signatureSuite

	protocolOverhead, err := CalculateProtocolOverhead(cryptoSuite, newMsg)
	if err != nil {
		err = fmt.Errorf("failed to calculate protocol overhead: %w", err)
		return
	}

	fragments, err := Fragment(newMsg, maxPayloadSize, protocolOverhead)
	if err != nil {
		return
	}

	for index, fragment := range fragments {
		var payload *innerWireFormat
		payload, err = ConstructPayload(fragment, signatureSuite)
		if err != nil {
			err = fmt.Errorf("fragment %d: %w", index, err)
			return
		}

		var innerPayload []byte
		innerPayload, err = ConstructInnerPayload(payload)
		if err != nil {
			err = fmt.Errorf("failed to serialize inner fragment %d: %w", index, err)
			return
		}

		var outterPayload []byte
		outterPayload, err = ConstructOuterPayload(innerPayload, cryptoSuite)
		if err != nil {
			err = fmt.Errorf("failed to serialize outer fragment %d: %w", index, err)
			return
		}

		packets = append(packets, outterPayload)
	}
	return
}

// Main Entry Point: Takes in multiple raw packets (transport layer payload) and returns the message
func Extract(packets [][]byte) (recvMsg *Message, hostID int, err error) {
	if len(packets) == 0 {
		return
	}

	var fragments []*Payload
	for index, packet := range packets {
		var innerPayload []byte
		innerPayload, err = DeconstructOuterPayload(packet)
		if err != nil {
			err = fmt.Errorf("failed to deserialize outer payload for fragment %d: %w", index, err)
			return
		}

		var payload *innerWireFormat
		payload, err = DeconstructInnerPayload(innerPayload)
		if err != nil {
			err = fmt.Errorf("failed to deserialize inner payload for fragment %d: %w", index, err)
			return
		}

		var messageFragment *Payload
		messageFragment, err = DeconstructPayload(payload)
		if err != nil {
			err = fmt.Errorf("fragment %d: %w", index, err)
			return
		}

		fragments = append(fragments, messageFragment)
	}

	primaryPayload, err := Defragment(fragments)
	if err != nil {
		return
	}

	recvMsg = &Message{
		Timestamp: primaryPayload.Timestamp,
		Hostname:  primaryPayload.Hostname,
		Fields:    primaryPayload.CustomFields,
		Data:      primaryPayload.Data,
	}
	hostID = primaryPayload.HostID
	return
}
