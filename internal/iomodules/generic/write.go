package generic

import (
	"context"
	"sdsyslog/pkg/protocol"
)

func (mod *OutModule) Write(ctx context.Context, msg *protocol.Payload) (entriesWritten int, err error) {
	if mod == nil {
		return
	}

	data := msg.Data

	// Ensure exactly one trailing newline
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	_, err = mod.sink.Write(data)
	if err != nil {
		return
	}

	entriesWritten = 1
	return
}

// No-op - satisfies common type
func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	return
}
