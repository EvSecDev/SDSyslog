package fipr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sdsyslog/internal/tests/utils"
	"strings"
	"sync"
	"testing"
)

func TestReadFrame(t *testing.T) {
	tests := []struct {
		name        string
		frameChunks [][][]byte // Frames|Chunks|Bytes - input and expected output
		expectedErr error
	}{
		{
			name: "single full frame",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte("hello"), 0, 0)
				frames = append(frames, f1)
				return
			}(),
		},
		{
			name: "minimum frame length",
			frameChunks: func() (frames [][][]byte) {
				payload := []byte{} // empty payload
				f := buildTestFrame(opAck, 1, payload, 0, 0)
				frames = append(frames, f)
				return
			}(),
		},
		{
			name: "invalid frame length field",
			frameChunks: func() (frames [][][]byte) {
				buf := make([]byte, lenFieldFrameLen)
				binary.BigEndian.PutUint32(buf, uint32(minFrameLen-1))
				frames = append(frames, [][]byte{buf})
				return
			}(),
			expectedErr: ErrInvalidFrameLen,
		},
		{
			name: "multiple frames in one buffer",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte(strings.Repeat("a", 50)), 2, 15)
				frames = append(frames, f1)
				f2 := buildTestFrame(opRejected, 1, []byte(strings.Repeat("b", 65)), 2, 13)
				frames = append(frames, f2)
				return
			}(),
		},
		{
			name: "frame split into chunks",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte(strings.Repeat("a", 26)), 2, 8)
				frames = append(frames, f1)
				return
			}(),
		},
		{
			name: "large multiple frames in one buffer",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte(strings.Repeat("a", 532)), 10, 58)
				frames = append(frames, f1)
				f2 := buildTestFrame(opRejected, 1, []byte(strings.Repeat("b", 65)), 3, 13)
				frames = append(frames, f2)
				f3 := buildTestFrame(opRejected, 1, []byte(strings.Repeat("b", 1243)), 5, 1)
				frames = append(frames, f3)
				return
			}(),
		},
		{
			name: "frame boundary: only length field first",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte("boundary field length"), 2, 4)
				frames = append(frames, f1)
				return
			}(),
		},
		{
			name: "frame boundary: less than length field first",
			frameChunks: func() (frames [][][]byte) {
				f1 := buildTestFrame(opAck, 0, []byte("boundary field length 2"), 2, 2)
				frames = append(frames, f1)
				return
			}(),
		},
		{
			name: "zero byte read before frame arrives",
			frameChunks: func() (frames [][][]byte) {
				f := buildTestFrame(opAck, 0, []byte("hello"), 1, 0)
				frames = append(frames, [][]byte{
					{}, // zero-byte read
					f[0],
				})
				return
			}(),
		},
		{
			name: "frame larger than internal read buffer",
			frameChunks: func() (frames [][][]byte) {
				payload := bytes.Repeat([]byte("x"), 10*4096)
				f := buildTestFrame(opAck, 0, payload, 1, 0)
				frames = append(frames, f)
				return
			}(),
		},
		{
			name: "transport layer read timeout",
			frameChunks: func() (frames [][][]byte) {
				frames = append(frames, [][]byte{})
				return
			}(),
			expectedErr: os.ErrDeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			session := &Session{
				conn: client,
				seq:  0,
			}

			var wg sync.WaitGroup
			wg.Add(1)

			errChan := make(chan error, 1)

			expectedWholeFrames := make([][]byte, len(tt.frameChunks)) // source of truth for test output
			go func() {
				defer wg.Done()
				// Inject chunk by chunk into mocked connection buffer
				for i, frame := range tt.frameChunks {
					for _, chunk := range frame {
						_, err := server.Write(chunk)
						if err != nil {
							errChan <- fmt.Errorf("unexpected error writing frame chunk: %w", err)
							return
						}
						expectedWholeFrames[i] = append(expectedWholeFrames[i], chunk...)
					}
				}
			}()

			// Simulate transport layer closing
			if tt.expectedErr == ErrTransportFailure {
				err := client.Close()
				if err != nil {
					t.Fatalf("unexpected error closing client connection: %v", err)
				}
			}

			// Test - get each output frame
			var gotFrames [][]byte
			for i := 0; i < len(expectedWholeFrames); i++ {
				frame, err := session.readFrame()
				gotExpected, err := utils.MatchWrappedError(err, tt.expectedErr)
				if err != nil {
					t.Fatalf("readFrame: %v", err)
				} else if gotExpected {
					return
				}

				gotFrames = append(gotFrames, frame)
			}
			wg.Wait()

			if len(errChan) > 0 {
				mockWriterError := <-errChan
				if mockWriterError != nil {
					t.Fatalf("%s", mockWriterError.Error())
				}
			}

			// Ensure ordering and contents stayed the same
			for i, want := range expectedWholeFrames {
				if !bytes.Equal(gotFrames[i], want) {
					t.Errorf("frame %d mismatch\nexpected: %v\nactual:   %v", i, want, gotFrames[i])
				}
			}

			if len(session.transportBuffer) != 0 {
				t.Errorf("buffer not empty after reading all frames, len=%d", len(session.transportBuffer))
			}
		})
	}
}

func buildTestFrame(op opCode, seq uint16, payload []byte, chunks int, firstChunkSize int) (frameChunks [][]byte) {
	frameLen := uint32(lenFieldSequence + lenFieldOpCode + len(payload) + HMACSize)
	frame := make([]byte, lenFieldFrameLen+int(frameLen))

	// Length
	binary.BigEndian.PutUint32(frame[:lenFieldFrameLen], frameLen)
	// Sequence
	binary.BigEndian.PutUint16(frame[lenFieldFrameLen:lenFieldFrameLen+lenFieldSequence], seq)
	// Opcode
	frame[lenFieldFrameLen+lenFieldSequence] = byte(op)
	// Payload
	copy(frame[lenFieldFrameLen+lenFieldSequence+lenFieldOpCode:], payload)
	// Fake HMAC (zeros)
	for i := lenFieldFrameLen + lenFieldSequence + lenFieldOpCode + len(payload); i < len(frame); i++ {
		frame[i] = 0
	}

	if chunks <= 1 {
		frameChunks = [][]byte{frame}
		return
	}

	// First chunk fixed size
	if firstChunkSize >= len(frame) {
		firstChunkSize = len(frame) / 2
	}
	frameChunks = [][]byte{frame[:firstChunkSize]}

	// Remaining bytes
	remaining := frame[firstChunkSize:]
	remChunkSize := (len(remaining) + chunks - 2) / (chunks - 1)

	for i := 0; i < len(remaining); i += remChunkSize {
		end := i + remChunkSize
		if end > len(remaining) {
			end = len(remaining)
		}
		frameChunks = append(frameChunks, remaining[i:end])
	}

	return
}
