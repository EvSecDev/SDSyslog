package protocol

import (
	"bytes"
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sort"
)

// Creates payload objects based on main payload and the transports maximum payload size
// Sets random padding length per fragment
func Fragment(primaryPayload Payload, maxPayloadSize int, fixedProtocolSize int) (payloads []Payload, err error) {
	if maxPayloadSize <= 0 {
		err = fmt.Errorf("maxPayloadSize must be greater than 0")
		return
	}
	if fixedProtocolSize <= 0 {
		err = fmt.Errorf("fixedProtocolSize must be greater than 0")
		return
	}

	remaining := []byte(primaryPayload.Data)
	seq := 0

	// Step through the payload to dynamically create fragment sizes
	for len(remaining) > 0 {
		payloadFragment := primaryPayload
		payloadFragment.MessageSeq = seq

		// Get a random padding length for this section of the data
		payloadFragment.PaddingLen, err = random.NumberInRange(minPaddingLen, maxPaddingLen)
		if err != nil {
			err = fmt.Errorf("failed to generate padding length: %w", err)
			return
		}

		// Compute the expected max fragment size for this fragment only
		maxMessageSize := maxPayloadSize - fixedProtocolSize - payloadFragment.PaddingLen
		if maxMessageSize <= 0 {
			err = fmt.Errorf("max_payload_size=%d bytes < protocol_overhead=%d bytes", maxPayloadSize, fixedProtocolSize+payloadFragment.PaddingLen)
			err = fmt.Errorf("no room left for message in packet: %w", err)
			err = fmt.Errorf("protocol overhead (including custom fields) exceeded max payload size: %w", err)
			return
		}

		// Slice message
		if len(remaining) > maxMessageSize {
			payloadFragment.Data = remaining[:maxMessageSize]
			remaining = remaining[maxMessageSize:]
		} else {
			payloadFragment.Data = remaining
			remaining = nil
		}

		payloads = append(payloads, payloadFragment)
		seq++
	}

	// Set max seq on all fragments
	for i := range payloads {
		payloads[i].MessageSeqMax = seq - 1
	}

	return
}

// Recombines payload objects into singular object
// Expects validated (individual) payloads - only run post payload parsing
func Defragment(payloads []Payload) (primaryPayload Payload, err error) {
	if len(payloads) == 0 {
		err = fmt.Errorf("received no payloads to defrag")
		return
	}

	// Inconsistent shared fields are not corruption (would have failed decryption)
	if !allFieldsEqual(payloads) {
		err = fmt.Errorf("some received payloads have shared fields that are not identical - could indicate client misbehavior")
		return
	}

	// Sort incoming based on sequence
	sort.Slice(payloads, func(a, b int) bool {
		return payloads[a].MessageSeq < payloads[b].MessageSeq
	})

	var reassemblyBuffer bytes.Buffer

	// Detect missing start (missing seq 0)
	if len(payloads) > 0 && payloads[0].MessageSeq > 0 {
		for missing := 0; missing < payloads[0].MessageSeq; missing++ {
			reassemblyBuffer.WriteString(missingLogPlaceholder)
		}
	}

	prev := -1 // so that seq 0 is expected first

	// Loop payloads for reassembly + middle gap detection
	for _, payload := range payloads {
		if prev != -1 {
			expected := prev + 1

			// Detect mid gaps
			if payload.MessageSeq > expected {
				for missing := expected; missing < payload.MessageSeq; missing++ {
					reassemblyBuffer.WriteString(missingLogPlaceholder)
				}
			}
		}

		reassemblyBuffer.Write(payload.Data)
		prev = payload.MessageSeq
	}

	// Detect missing end
	expectedFinal := payloads[0].MessageSeqMax // Pull from any payload, already asserted as all equal
	if prev < expectedFinal {
		for missing := prev + 1; missing <= expectedFinal; missing++ {
			reassemblyBuffer.WriteString(missingLogPlaceholder)
		}
	}

	// Create singular payload - Leave unused fields as default (0)
	// We can use one of the payloads as a template
	primaryPayload.RemoteIP = payloads[0].RemoteIP
	primaryPayload.HostID = payloads[0].HostID
	primaryPayload.MsgID = payloads[0].MsgID
	primaryPayload.Timestamp = payloads[0].Timestamp
	primaryPayload.CustomFields = payloads[0].CustomFields
	primaryPayload.Hostname = payloads[0].Hostname

	// Include the, now whole, log message
	primaryPayload.Data = reassemblyBuffer.Bytes()
	return
}

// Validates whether all shared fields in payload are equal across all payloads
func allFieldsEqual(payloads []Payload) (valid bool) {
	ref := payloads[0]
	for _, payload := range payloads[1:] {
		// Check custom fields equality
		if len(ref.CustomFields) != len(payload.CustomFields) {
			valid = false
			return
		}
		for refIndex, customField := range ref.CustomFields {
			if payload.CustomFields[refIndex] != customField {
				valid = false
				return
			}
		}

		if payload.RemoteIP != ref.RemoteIP || payload.HostID != ref.HostID ||
			payload.MsgID != ref.MsgID || !payload.Timestamp.Equal(ref.Timestamp) ||
			payload.Hostname != ref.Hostname {

			valid = false
			return
		}
	}
	valid = true
	return
}
