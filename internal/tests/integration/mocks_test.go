package integration

import (
	"fmt"
	"io"
	"os"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/pkg/protocol"
	"strings"
	"sync"
	"time"
)

// PipeBuffer is a thread-safe, non-deadlocking in-memory pipe implementing io.ReadCloser and io.WriteCloser
type PipeBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buffer []byte
	closed bool
	maxCap int
}

// NewPipeBuffer creates a PipeBuffer with optional max capacity (0 = unlimited)
func NewPipeBuffer(maxCap int) (new *PipeBuffer) {
	new = &PipeBuffer{
		buffer: make([]byte, 0),
		maxCap: maxCap,
	}
	new.cond = sync.NewCond(&new.mu)
	return
}

// Write appends bytes to the buffer, signals readers.
// Returns error if buffer is closed or would exceed maxCap
func (pipe *PipeBuffer) Write(data []byte) (bytesWritten int, err error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if pipe.closed {
		err = os.ErrClosed
		return
	}

	if pipe.maxCap > 0 && len(pipe.buffer)+len(data) > pipe.maxCap {
		err = fmt.Errorf("buffer full")
		return
	}

	pipe.buffer = append(pipe.buffer, data...)
	pipe.cond.Broadcast() // wake up all waiting readers
	bytesWritten = len(data)
	return
}

// Read reads bytes from the buffer into pBytes. Blocks if empty.
// Returns io.EOF if closed and no more data is available.
func (pipe *PipeBuffer) Read(pBytes []byte) (bytesRead int, err error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	for len(pipe.buffer) == 0 && !pipe.closed {
		pipe.cond.Wait() // safely wait. Wakeup on signal or broadcast
	}

	if len(pipe.buffer) == 0 && pipe.closed {
		err = io.EOF
		return
	}

	bytesRead = copy(pBytes, pipe.buffer)
	pipe.buffer = pipe.buffer[bytesRead:]
	return
}

// Close marks the buffer as closed and wakes up all readers
func (pipe *PipeBuffer) Close() (err error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if pipe.closed {
		err = os.ErrClosed
		return
	}

	pipe.closed = true
	pipe.cond.Broadcast() // unblock any waiting readers
	return
}

// Truncate shortens the buffer to numBytes bytes (if n < len(buffer))
func (pipe *PipeBuffer) Truncate(numBytes int) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if numBytes < len(pipe.buffer) {
		pipe.buffer = pipe.buffer[:numBytes]
	}
}

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

	newMsg := &protocol.Message{
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
