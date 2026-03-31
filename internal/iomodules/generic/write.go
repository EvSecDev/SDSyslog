package generic

import (
	"context"
	"sdsyslog/pkg/protocol"
)

func (mod *OutModule) Write(ctx context.Context, msg protocol.Payload) (entriesWritten int, err error) {
	if mod == nil {
		return
	}

	mod.buffer = append(mod.buffer, msg)

	if len(mod.buffer) >= mod.batchSize {
		entriesWritten, err = mod.FlushBuffer()
		return
	}

	entriesWritten = 1
	return
}

func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	if mod == nil {
		return
	}

	for _, msg := range mod.buffer {
		data := msg.Data

		// Ensure exactly one trailing newline
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}

		_, err = mod.sink.Write(data)
		if err != nil {
			return
		}
	}

	flushedCnt = len(mod.buffer)
	mod.buffer = mod.buffer[:0]
	return
}
