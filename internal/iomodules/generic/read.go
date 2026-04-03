package generic

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"time"
)

func (mod *InModule) read() {
	defer mod.wg.Done()

	pid := os.Getpid()

	reader := bufio.NewReaderSize(mod.sink, 64*1024)

	for {
		select {
		case <-mod.ctx.Done():
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			} else {
				logctx.LogStdErr(mod.ctx, "failed to read from raw sink: %w\n", err)
				continue
			}
		}

		// Do not allow newlines in data
		line = bytes.ReplaceAll(line, []byte{'\n'}, []byte{})

		msg := &protocol.Message{
			Hostname: mod.localHostname,
			Fields: map[string]any{
				iomodules.CtxKey:      logctx.NSoRaw,
				iomodules.CFappname:   global.ProgBaseName,
				iomodules.CFprocessid: pid,
				iomodules.CFfacility:  iomodules.DefaultFacility,
				iomodules.CFseverity:  iomodules.DefaultSeverity,
			},
			Data:      line,
			Timestamp: time.Now(),
		}

		mod.metrics.CompleteReads.Add(1)

		totalSize := msg.Size()
		err = mod.outbox.PushWithRetry(msg, uint64(totalSize), 4)
		if err != nil {
			logctx.LogStdErr(mod.ctx, "failed to push raw sink data to queue: %w\n", err)
			continue
		}

		mod.metrics.Success.Add(1)
	}
}
