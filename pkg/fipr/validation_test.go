package fipr

import (
	"sdsyslog/internal/tests/utils"
	"testing"
)

func TestValidateFrame(t *testing.T) {
	session := &Session{
		state:      stateInit,
		seq:        0,
		sentFrames: make(map[uint16]framebody),
	}

	tests := []struct {
		name         string
		op           opCode
		payload      []byte
		wantErr      error
		sessionState sessionState
		sessionSeq   uint16
		frameSeq     uint16
	}{
		{"Unknown OpCode", 0x94, []byte{}, ErrInvalidOpcode, stateInit, 1, 1},
		{"ValidStart", opStart, []byte("id"), nil, stateInit, 0, 0},
		{"LateStart", opStart, []byte("id"), ErrLateStart, stateStarted, 0, 0},
		{"OutOfOrderStart", opStart, []byte("id"), ErrOutOfOrderStart, stateInit, 0, 1},
		{"NilStart", opStart, nil, ErrFrameHasNoPayload, stateInit, 0, 0},
		{"EmptyStart", opStart, []byte{}, ErrFrameHasNoPayload, stateInit, 0, 0},
		{"OBOEmpty", opOBO, nil, ErrFrameHasNoPayload, stateStarted, 2, 2},
		{"ShardCheckWithPayload", opShardCheck, []byte{1}, ErrFrameHasPayload, stateStarted, 3, 3},
		{"ShardStatusWrongLen", opShardStatus, []byte{1, 2}, ErrPayloadWrongLength, stateStarted, 4, 4},
		{"MessageStatusEmpty", opMsgStatus, []byte{}, ErrFrameHasNoPayload, stateStarted, 4, 4},
		{"AckCorrectLen", opAck, []byte{0, 1}, nil, stateStarted, 5, 5},
		{"AckWrongLen", opAck, []byte{0}, ErrPayloadWrongLength, stateStarted, 6, 6},
		{"MsgCheckShutdown", opMsgCheck, []byte{0}, ErrSessionClosed, stateClosed, 7, 7},
		{"FragmentEmpty", opFrgRoute, []byte{}, ErrFrameHasNoPayload, stateStarted, 4, 4},
		{"Accept", opAccepted, []byte{}, nil, stateStarted, 4, 4},
		{"Rejected", opRejected, []byte{}, ErrRemoteRejected, stateStarted, 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session.state = tt.sessionState
			session.seq = tt.sessionSeq
			frame := &framebody{op: tt.op, payload: tt.payload, sequence: tt.frameSeq}
			err := session.validateFrame(frame)
			_, err = utils.MatchWrappedError(err, tt.wantErr)
			if err != nil {
				t.Fatalf("server: %v", err)
			}
		})
	}
}
