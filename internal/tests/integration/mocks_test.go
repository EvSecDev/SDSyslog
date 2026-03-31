package integration

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/iomodules/syslog"
	"sdsyslog/pkg/protocol"
	"strings"
	"time"
)

// In/Out Module for tests


// Creates a repeated string targeting desired length
func mockMessage(seedText string, targetPktSizeBytes int) (messageText string, err error) {
	mockLen := len(seedText)
	if mockLen > targetPktSizeBytes {
		err = fmt.Errorf("cannot create mock packets with individual sizes of %d bytes if the mock content is only %d bytes", targetPktSizeBytes, mockLen)
		return
	}

	// Repeat target message to approach targeted size
	msgRepetition := targetPktSizeBytes / mockLen
	messageText = strings.Repeat(seedText, msgRepetition)
	return
}

// Creates set number of packets with desired content (attempts to hit target size, but not exact)
func mockPackets(numMessages int, rawMessage []byte, maxPayloadSize int, publicKey []byte) (packets [][]byte, err error) {
	if numMessages == 0 {
		err = fmt.Errorf("cannot create mock packets if requested number of packets is 0")
		return
	}

	// Pre-startup
	syslog.InitBidiMaps()
	err = wrappers.SetupEncryptInnerPayload(publicKey)
	if err != nil {
		err = fmt.Errorf("failed setting up encryption function: %w", err)
		return
	}

	mainHostID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate new unique host identifier: %w", err)
		return
	}

	fields := map[string]any{
		"Facility":        22,
		"Severity":        5,
		"ProcessID":       3483,
		"ApplicationName": "test-app",
	}

	newMsg := protocol.Message{
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Fields:    fields,
		Data:      rawMessage,
	}

	for range numMessages {
		var fragments [][]byte
		fragments, err = protocol.Create(newMsg, mainHostID, maxPayloadSize, 1, 0)
		if err != nil {
			err = fmt.Errorf("failed serialize test data for mock packets: %w", err)
			return
		}
		packets = append(packets, fragments...)
	}

	return
}
